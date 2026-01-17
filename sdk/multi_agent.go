package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"brutus/provider"
	"brutus/tools"
)

type AgentConfig struct {
	ID           string
	SystemPrompt string
	WorkingDir   string
}

type AgentResult struct {
	AgentID          string
	Success          bool
	FinalMessage     string
	ToolCalls        []provider.ToolCall
	Error            error
	Duration         time.Duration
}

type MultiAgentHarness struct {
	agents   map[string]*TestHarness
	configs  map[string]AgentConfig
	registry *tools.Registry
	mu       sync.Mutex
	verbose  bool
}

func NewMultiAgentHarness() *MultiAgentHarness {
	return &MultiAgentHarness{
		agents:   make(map[string]*TestHarness),
		configs:  make(map[string]AgentConfig),
		registry: tools.NewRegistry(),
	}
}

func (m *MultiAgentHarness) WithVerbose(v bool) *MultiAgentHarness {
	m.verbose = v
	return m
}

func (m *MultiAgentHarness) AddAgent(cfg AgentConfig) *MultiAgentHarness {
	m.mu.Lock()
	defer m.mu.Unlock()

	harness := NewHarness().
		WithDefaultTools().
		WithSystemPrompt(cfg.SystemPrompt).
		WithVerbose(m.verbose)

	if cfg.WorkingDir != "" {
		harness.WithWorkingDir(cfg.WorkingDir)
	}

	m.agents[cfg.ID] = harness
	m.configs[cfg.ID] = cfg

	return m
}

func (m *MultiAgentHarness) GetAgent(id string) *TestHarness {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.agents[id]
}

func (m *MultiAgentHarness) QueueResponseForAgent(agentID string, responses ...MockResponse) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	harness, ok := m.agents[agentID]
	if !ok {
		return fmt.Errorf("agent %s not found", agentID)
	}

	for _, resp := range responses {
		if resp.Content != "" {
			harness.QueueTextResponse(resp.Content)
		} else if resp.ToolCall != "" {
			harness.QueueToolCall(resp.ToolCall, resp.Input)
		}
	}

	return nil
}

func (m *MultiAgentHarness) RunSequential(ctx context.Context, messages map[string][]string) ([]AgentResult, error) {
	var results []AgentResult

	for agentID, msgs := range messages {
		m.mu.Lock()
		harness, ok := m.agents[agentID]
		m.mu.Unlock()

		if !ok {
			results = append(results, AgentResult{
				AgentID: agentID,
				Success: false,
				Error:   fmt.Errorf("agent not found"),
			})
			continue
		}

		start := time.Now()
		var lastErr error

		for _, msg := range msgs {
			harness.SendUserMessage(msg)
			if err := harness.Run(ctx); err != nil {
				lastErr = err
				break
			}
		}

		results = append(results, AgentResult{
			AgentID:      agentID,
			Success:      lastErr == nil,
			FinalMessage: harness.LastAssistantMessage(),
			ToolCalls:    harness.GetToolCalls(),
			Error:        lastErr,
			Duration:     time.Since(start),
		})
	}

	return results, nil
}

func (m *MultiAgentHarness) RunConcurrent(ctx context.Context, messages map[string][]string) ([]AgentResult, error) {
	var wg sync.WaitGroup
	resultsCh := make(chan AgentResult, len(messages))

	for agentID, msgs := range messages {
		wg.Add(1)

		go func(id string, agentMsgs []string) {
			defer wg.Done()

			m.mu.Lock()
			harness, ok := m.agents[id]
			m.mu.Unlock()

			if !ok {
				resultsCh <- AgentResult{
					AgentID: id,
					Success: false,
					Error:   fmt.Errorf("agent not found"),
				}
				return
			}

			start := time.Now()
			var lastErr error

			for _, msg := range agentMsgs {
				harness.SendUserMessage(msg)
				if err := harness.Run(ctx); err != nil {
					lastErr = err
					break
				}
			}

			resultsCh <- AgentResult{
				AgentID:      id,
				Success:      lastErr == nil,
				FinalMessage: harness.LastAssistantMessage(),
				ToolCalls:    harness.GetToolCalls(),
				Error:        lastErr,
				Duration:     time.Since(start),
			}
		}(agentID, msgs)
	}

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	var results []AgentResult
	for result := range resultsCh {
		results = append(results, result)
	}

	return results, nil
}

func (m *MultiAgentHarness) Summary() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("=== Multi-Agent Harness Summary ===\n")
	sb.WriteString(fmt.Sprintf("Total agents: %d\n\n", len(m.agents)))

	for id, harness := range m.agents {
		sb.WriteString(fmt.Sprintf("--- Agent: %s ---\n", id))
		sb.WriteString(fmt.Sprintf("  Tool calls: %d\n", len(harness.GetToolCalls())))
		sb.WriteString(fmt.Sprintf("  Errors: %d\n", len(harness.GetErrors())))
		if msg := harness.LastAssistantMessage(); msg != "" {
			if len(msg) > 100 {
				msg = msg[:100] + "..."
			}
			sb.WriteString(fmt.Sprintf("  Last message: %s\n", msg))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (m *MultiAgentHarness) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, harness := range m.agents {
		harness.Reset()
	}
}

type MockResponse struct {
	Content  string                 `json:"content,omitempty"`
	ToolCall string                 `json:"tool_call,omitempty"`
	Input    map[string]interface{} `json:"input,omitempty"`
}

type MultiAgentScenario struct {
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	Agents      []MultiAgentScenarioAgent    `json:"agents"`
	Assertions  []MultiAgentAssertion        `json:"assertions"`
}

type MultiAgentScenarioAgent struct {
	ID            string         `json:"id"`
	SystemPrompt  string         `json:"system_prompt"`
	UserMessages  []string       `json:"user_messages"`
	MockResponses []MockResponse `json:"mock_responses"`
}

type MultiAgentAssertion struct {
	AgentID string `json:"agent_id"`
	Type    string `json:"type"`
	Value   string `json:"value"`
}

func LoadMultiAgentScenario(filename string) (*MultiAgentScenario, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read scenario file: %w", err)
	}

	var scenario MultiAgentScenario
	if err := json.Unmarshal(data, &scenario); err != nil {
		return nil, fmt.Errorf("failed to parse scenario: %w", err)
	}

	return &scenario, nil
}

func (m *MultiAgentHarness) RunScenario(ctx context.Context, scenario *MultiAgentScenario, concurrent bool) ([]AgentResult, error) {
	for _, agentCfg := range scenario.Agents {
		m.AddAgent(AgentConfig{
			ID:           agentCfg.ID,
			SystemPrompt: agentCfg.SystemPrompt,
		})

		for _, resp := range agentCfg.MockResponses {
			m.QueueResponseForAgent(agentCfg.ID, resp)
		}
	}

	messages := make(map[string][]string)
	for _, agentCfg := range scenario.Agents {
		messages[agentCfg.ID] = agentCfg.UserMessages
	}

	if concurrent {
		return m.RunConcurrent(ctx, messages)
	}
	return m.RunSequential(ctx, messages)
}

func (m *MultiAgentHarness) ValidateAssertions(results []AgentResult, assertions []MultiAgentAssertion) []error {
	var errors []error

	resultMap := make(map[string]AgentResult)
	for _, r := range results {
		resultMap[r.AgentID] = r
	}

	for _, assertion := range assertions {
		result, ok := resultMap[assertion.AgentID]
		if !ok {
			errors = append(errors, fmt.Errorf("agent %s not found in results", assertion.AgentID))
			continue
		}

		harness := m.GetAgent(assertion.AgentID)
		if harness == nil {
			errors = append(errors, fmt.Errorf("agent %s harness not found", assertion.AgentID))
			continue
		}

		switch assertion.Type {
		case "tool_called":
			if !harness.ToolWasCalled(assertion.Value) {
				errors = append(errors, fmt.Errorf("agent %s: expected tool '%s' to be called", 
					assertion.AgentID, assertion.Value))
			}
		case "contains":
			if !strings.Contains(result.FinalMessage, assertion.Value) {
				errors = append(errors, fmt.Errorf("agent %s: expected message to contain '%s'", 
					assertion.AgentID, assertion.Value))
			}
		case "success":
			if !result.Success {
				errors = append(errors, fmt.Errorf("agent %s: expected success but got error: %v", 
					assertion.AgentID, result.Error))
			}
		}
	}

	return errors
}
