package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"brutus/provider"
	"brutus/tools"
)

type MockProvider struct {
	mu            sync.Mutex
	responses     []provider.Message
	responseIndex int
	model         string
	models        []provider.ModelInfo
	calls         []MockCall
}

type MockCall struct {
	SystemPrompt string
	Messages     []provider.Message
	ToolNames    []string
}

func NewMockProvider() *MockProvider {
	return &MockProvider{
		model: "mock-model",
		models: []provider.ModelInfo{
			{ID: "mock-model", Name: "Mock Model"},
		},
	}
}

func (m *MockProvider) QueueResponse(msg provider.Message) *MockProvider {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = append(m.responses, msg)
	return m
}

func (m *MockProvider) QueueTextResponse(content string) *MockProvider {
	return m.QueueResponse(provider.Message{
		Role:    "assistant",
		Content: content,
	})
}

func (m *MockProvider) QueueToolCall(toolName string, input map[string]interface{}) *MockProvider {
	inputJSON, _ := json.Marshal(input)
	return m.QueueResponse(provider.Message{
		Role: "assistant",
		ToolCalls: []provider.ToolCall{
			{
				ID:    fmt.Sprintf("call_%d", len(m.responses)),
				Name:  toolName,
				Input: inputJSON,
			},
		},
	})
}

func (m *MockProvider) QueueToolCallWithFollowup(toolName string, input map[string]interface{}, followup string) *MockProvider {
	m.QueueToolCall(toolName, input)
	return m.QueueTextResponse(followup)
}

func (m *MockProvider) Chat(ctx context.Context, systemPrompt string, messages []provider.Message, availableTools []tools.Tool) (provider.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var toolNames []string
	for _, t := range availableTools {
		toolNames = append(toolNames, t.Name)
	}
	m.calls = append(m.calls, MockCall{
		SystemPrompt: systemPrompt,
		Messages:     messages,
		ToolNames:    toolNames,
	})

	if m.responseIndex >= len(m.responses) {
		return provider.Message{
			Role:    "assistant",
			Content: "[MockProvider: no more queued responses]",
		}, nil
	}

	response := m.responses[m.responseIndex]
	m.responseIndex++
	return response, nil
}

func (m *MockProvider) ChatStream(ctx context.Context, systemPrompt string, messages []provider.Message, availableTools []tools.Tool) (<-chan provider.StreamDelta, error) {
	ch := make(chan provider.StreamDelta, 1)
	go func() {
		defer close(ch)
		msg, err := m.Chat(ctx, systemPrompt, messages, availableTools)
		if err != nil {
			ch <- provider.StreamDelta{Error: err, Done: true}
			return
		}
		ch <- provider.StreamDelta{Content: msg.Content, Done: true}
	}()
	return ch, nil
}

func (m *MockProvider) Name() string {
	return "mock"
}

func (m *MockProvider) ListModels(ctx context.Context) ([]provider.ModelInfo, error) {
	return m.models, nil
}

func (m *MockProvider) SetModel(model string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.model = model
}

func (m *MockProvider) GetModel() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.model
}

func (m *MockProvider) GetCalls() []MockCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

func (m *MockProvider) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = nil
	m.responseIndex = 0
	m.calls = nil
}
