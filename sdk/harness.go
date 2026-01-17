package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"brutus/provider"
	"brutus/tools"
)

type TestHarness struct {
	provider     *MockProvider
	registry     *tools.Registry
	systemPrompt string
	workingDir   string
	verbose      bool

	mu           sync.Mutex
	conversation []provider.Message
	toolCalls    []provider.ToolCall
	toolResults  []provider.ToolResult
	errors       []error
}

func NewHarness() *TestHarness {
	return &TestHarness{
		provider:   NewMockProvider(),
		registry:   tools.NewRegistry(),
		workingDir: ".",
	}
}

func (h *TestHarness) WithSystemPrompt(prompt string) *TestHarness {
	h.systemPrompt = prompt
	return h
}

func (h *TestHarness) WithWorkingDir(dir string) *TestHarness {
	h.workingDir = dir
	return h
}

func (h *TestHarness) WithVerbose(v bool) *TestHarness {
	h.verbose = v
	return h
}

func (h *TestHarness) WithTool(t tools.Tool) *TestHarness {
	h.registry.Register(t)
	return h
}

func (h *TestHarness) WithDefaultTools() *TestHarness {
	h.registry.Register(tools.ReadFileTool)
	h.registry.Register(tools.ListFilesTool)
	h.registry.Register(tools.EditFileTool)
	h.registry.Register(tools.BashTool)
	h.registry.Register(tools.CodeSearchTool)
	return h
}

func (h *TestHarness) QueueTextResponse(content string) *TestHarness {
	h.provider.QueueTextResponse(content)
	return h
}

func (h *TestHarness) QueueToolCall(toolName string, input map[string]interface{}) *TestHarness {
	h.provider.QueueToolCall(toolName, input)
	return h
}

func (h *TestHarness) QueueToolCallWithFollowup(toolName string, input map[string]interface{}, followup string) *TestHarness {
	h.provider.QueueToolCallWithFollowup(toolName, input, followup)
	return h
}

func (h *TestHarness) SendUserMessage(message string) *TestHarness {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conversation = append(h.conversation, provider.Message{
		Role:    "user",
		Content: message,
	})
	return h
}

func (h *TestHarness) Run(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.conversation) == 0 {
		return fmt.Errorf("no user messages to process")
	}

	response, err := h.provider.Chat(ctx, h.systemPrompt, h.conversation, h.registry.All())
	if err != nil {
		h.errors = append(h.errors, err)
		return err
	}
	h.conversation = append(h.conversation, response)

	for len(response.ToolCalls) > 0 {
		var toolResults []provider.ToolResult

		for _, tc := range response.ToolCalls {
			h.toolCalls = append(h.toolCalls, tc)

			if h.verbose {
				fmt.Printf("[harness] executing tool: %s\n", tc.Name)
			}

			tool, ok := h.registry.Get(tc.Name)
			if !ok {
				result := provider.ToolResult{
					ID:      tc.ID,
					Content: fmt.Sprintf("tool '%s' not found", tc.Name),
					IsError: true,
				}
				toolResults = append(toolResults, result)
				h.toolResults = append(h.toolResults, result)
				continue
			}

			output, toolErr := tool.Function(tc.Input)
			result := provider.ToolResult{
				ID:      tc.ID,
				Content: output,
				IsError: toolErr != nil,
			}
			if toolErr != nil {
				result.Content = toolErr.Error()
			}
			toolResults = append(toolResults, result)
			h.toolResults = append(h.toolResults, result)

			if h.verbose {
				if len(output) > 200 {
					fmt.Printf("[harness] result: %s...\n", output[:200])
				} else {
					fmt.Printf("[harness] result: %s\n", output)
				}
			}
		}

		h.conversation = append(h.conversation, provider.Message{
			Role:        "user",
			ToolResults: toolResults,
		})

		response, err = h.provider.Chat(ctx, h.systemPrompt, h.conversation, h.registry.All())
		if err != nil {
			h.errors = append(h.errors, err)
			return err
		}
		h.conversation = append(h.conversation, response)
	}

	return nil
}

func (h *TestHarness) RunMultiple(ctx context.Context, messages []string) error {
	for _, msg := range messages {
		h.SendUserMessage(msg)
		if err := h.Run(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (h *TestHarness) GetConversation() []provider.Message {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.conversation
}

func (h *TestHarness) GetToolCalls() []provider.ToolCall {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.toolCalls
}

func (h *TestHarness) GetToolResults() []provider.ToolResult {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.toolResults
}

func (h *TestHarness) GetErrors() []error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.errors
}

func (h *TestHarness) GetProvider() *MockProvider {
	return h.provider
}

func (h *TestHarness) GetRegistry() *tools.Registry {
	return h.registry
}

func (h *TestHarness) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.provider.Reset()
	h.conversation = nil
	h.toolCalls = nil
	h.toolResults = nil
	h.errors = nil
}

func (h *TestHarness) ToolWasCalled(name string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, tc := range h.toolCalls {
		if tc.Name == name {
			return true
		}
	}
	return false
}

func (h *TestHarness) ToolCallCount(name string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	count := 0
	for _, tc := range h.toolCalls {
		if tc.Name == name {
			count++
		}
	}
	return count
}

func (h *TestHarness) LastAssistantMessage() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := len(h.conversation) - 1; i >= 0; i-- {
		if h.conversation[i].Role == "assistant" && h.conversation[i].Content != "" {
			return h.conversation[i].Content
		}
	}
	return ""
}

func (h *TestHarness) AllAssistantMessages() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var messages []string
	for _, msg := range h.conversation {
		if msg.Role == "assistant" && msg.Content != "" {
			messages = append(messages, msg.Content)
		}
	}
	return messages
}

func (h *TestHarness) GetToolCallInput(name string) (json.RawMessage, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, tc := range h.toolCalls {
		if tc.Name == name {
			return tc.Input, true
		}
	}
	return nil, false
}

func (h *TestHarness) GetToolResult(name string) (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, tc := range h.toolCalls {
		if tc.Name == name && i < len(h.toolResults) {
			return h.toolResults[i].Content, true
		}
	}
	return "", false
}

func (h *TestHarness) AssertConversationContains(substring string) error {
	for _, msg := range h.conversation {
		if strings.Contains(msg.Content, substring) {
			return nil
		}
	}
	return fmt.Errorf("conversation does not contain '%s'", substring)
}

func (h *TestHarness) Summary() string {
	h.mu.Lock()
	defer h.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("=== Test Harness Summary ===\n")
	sb.WriteString(fmt.Sprintf("Conversation length: %d messages\n", len(h.conversation)))
	sb.WriteString(fmt.Sprintf("Tool calls: %d\n", len(h.toolCalls)))
	sb.WriteString(fmt.Sprintf("Errors: %d\n", len(h.errors)))

	if len(h.toolCalls) > 0 {
		sb.WriteString("\nTool calls made:\n")
		for i, tc := range h.toolCalls {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, tc.Name))
		}
	}

	if len(h.errors) > 0 {
		sb.WriteString("\nErrors:\n")
		for i, err := range h.errors {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, err.Error()))
		}
	}

	return sb.String()
}
