package sdk

import (
	"context"
	"fmt"
	"sync"
	"time"

	"brutus/provider"
	"brutus/tools"
)

type LiveAgentConfig struct {
	ID           string
	SystemPrompt string
	InitialTask  string
	WorkingDir   string
}

type LiveAgentResult struct {
	AgentID      string
	Success      bool
	FinalMessage string
	ToolCalls    []provider.ToolCall
	Conversation []provider.Message
	Error        error
	Duration     time.Duration
}

type LiveMultiAgentHarness struct {
	providerConfig provider.SaturnConfig
	registry       *tools.Registry
	verbose        bool
	maxTurns       int
}

func NewLiveMultiAgentHarness(cfg provider.SaturnConfig) *LiveMultiAgentHarness {
	return &LiveMultiAgentHarness{
		providerConfig: cfg,
		registry:       tools.NewRegistry(),
		maxTurns:       10,
	}
}

func (h *LiveMultiAgentHarness) WithVerbose(v bool) *LiveMultiAgentHarness {
	h.verbose = v
	return h
}

func (h *LiveMultiAgentHarness) WithMaxTurns(n int) *LiveMultiAgentHarness {
	h.maxTurns = n
	return h
}

func (h *LiveMultiAgentHarness) WithDefaultTools() *LiveMultiAgentHarness {
	h.registry.Register(tools.ReadFileTool)
	h.registry.Register(tools.ListFilesTool)
	h.registry.Register(tools.EditFileTool)
	h.registry.Register(tools.BashTool)
	h.registry.Register(tools.CodeSearchTool)
	h.registry.Register(tools.BroadcastTool)
	h.registry.Register(tools.ObserveAgentsTool)
	return h
}

func (h *LiveMultiAgentHarness) WithTool(t tools.Tool) *LiveMultiAgentHarness {
	h.registry.Register(t)
	return h
}

func (h *LiveMultiAgentHarness) RunConcurrent(ctx context.Context, agents []LiveAgentConfig) ([]LiveAgentResult, error) {
	var wg sync.WaitGroup
	resultsCh := make(chan LiveAgentResult, len(agents))

	for _, cfg := range agents {
		wg.Add(1)
		go func(agentCfg LiveAgentConfig) {
			defer wg.Done()
			result := h.runSingleAgent(ctx, agentCfg)
			resultsCh <- result
		}(cfg)
	}

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	var results []LiveAgentResult
	for result := range resultsCh {
		results = append(results, result)
	}

	return results, nil
}

func (h *LiveMultiAgentHarness) RunSequential(ctx context.Context, agents []LiveAgentConfig) ([]LiveAgentResult, error) {
	var results []LiveAgentResult
	for _, cfg := range agents {
		result := h.runSingleAgent(ctx, cfg)
		results = append(results, result)
	}
	return results, nil
}

func (h *LiveMultiAgentHarness) runSingleAgent(ctx context.Context, cfg LiveAgentConfig) LiveAgentResult {
	start := time.Now()

	result := LiveAgentResult{
		AgentID: cfg.ID,
	}

	p, err := provider.NewSaturn(ctx, h.providerConfig)
	if err != nil {
		result.Error = fmt.Errorf("failed to create Saturn provider: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	var conversation []provider.Message
	conversation = append(conversation, provider.Message{
		Role:    "user",
		Content: cfg.InitialTask,
	})

	turn := 0
	for turn < h.maxTurns {
		turn++

		if h.verbose {
			fmt.Printf("[%s] Turn %d: sending to LLM\n", cfg.ID, turn)
		}

		response, err := p.Chat(ctx, cfg.SystemPrompt, conversation, h.registry.All())
		if err != nil {
			result.Error = fmt.Errorf("chat failed on turn %d: %w", turn, err)
			result.Duration = time.Since(start)
			result.Conversation = conversation
			return result
		}

		conversation = append(conversation, response)

		if len(response.ToolCalls) == 0 {
			result.FinalMessage = response.Content
			break
		}

		var toolResults []provider.ToolResult
		for _, tc := range response.ToolCalls {
			result.ToolCalls = append(result.ToolCalls, tc)

			if h.verbose {
				fmt.Printf("[%s] Executing tool: %s\n", cfg.ID, tc.Name)
			}

			tool, ok := h.registry.Get(tc.Name)
			if !ok {
				toolResults = append(toolResults, provider.ToolResult{
					ID:      tc.ID,
					Content: fmt.Sprintf("tool '%s' not found", tc.Name),
					IsError: true,
				})
				continue
			}

			output, toolErr := tool.Function(tc.Input)
			tr := provider.ToolResult{
				ID:      tc.ID,
				Content: output,
				IsError: toolErr != nil,
			}
			if toolErr != nil {
				tr.Content = toolErr.Error()
			}
			toolResults = append(toolResults, tr)
		}

		conversation = append(conversation, provider.Message{
			Role:        "user",
			ToolResults: toolResults,
		})
	}

	result.Success = result.Error == nil
	result.Duration = time.Since(start)
	result.Conversation = conversation

	return result
}
