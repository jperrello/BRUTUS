# Subtask 022: Agent Loop

## Goal
Implement the core agent loop that ties everything together.

## Research Reference
`research/opencode-agent-loop/BRUTUS-IMPLEMENTATION-SPEC.md`

## Create File
`internal/agent/loop.go`

## Code to Write

```go
package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"brutus/internal/tool"
)

type Provider interface {
	Chat(ctx context.Context, systemPrompt string, messages []Message, tools []tool.Tool) (Message, error)
}

type Message struct {
	Role       string       `json:"role"`
	Content    string       `json:"content,omitempty"`
	ToolCalls  []ToolCall   `json:"tool_calls,omitempty"`
	ToolResult *ToolResult  `json:"tool_result,omitempty"`
}

type ToolCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

type ToolResult struct {
	ID      string `json:"tool_call_id"`
	Content string `json:"content"`
	IsError bool   `json:"is_error,omitempty"`
}

type Loop struct {
	provider     Provider
	tools        *tool.Registry
	systemPrompt string
	doom         *DoomTracker
	onToolCall   func(name string, input json.RawMessage)
	onToolResult func(name string, result string, isError bool)
	onResponse   func(content string)
	onDoomLoop   func(name string) bool // Return true to continue anyway
}

type LoopConfig struct {
	Provider     Provider
	Tools        *tool.Registry
	SystemPrompt string
	OnToolCall   func(name string, input json.RawMessage)
	OnToolResult func(name string, result string, isError bool)
	OnResponse   func(content string)
	OnDoomLoop   func(name string) bool
}

func NewLoop(cfg LoopConfig) *Loop {
	return &Loop{
		provider:     cfg.Provider,
		tools:        cfg.Tools,
		systemPrompt: cfg.SystemPrompt,
		doom:         NewDoomTracker(),
		onToolCall:   cfg.OnToolCall,
		onToolResult: cfg.OnToolResult,
		onResponse:   cfg.OnResponse,
		onDoomLoop:   cfg.OnDoomLoop,
	}
}

func (l *Loop) Run(ctx context.Context, userMessage string) error {
	messages := []Message{
		{Role: "user", Content: userMessage},
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Get response from provider
		response, err := l.provider.Chat(ctx, l.systemPrompt, messages, l.tools.All())
		if err != nil {
			return fmt.Errorf("provider error: %w", err)
		}

		messages = append(messages, response)

		// No tool calls = done
		if len(response.ToolCalls) == 0 {
			if l.onResponse != nil && response.Content != "" {
				l.onResponse(response.Content)
			}
			return nil
		}

		// Execute tool calls
		for _, tc := range response.ToolCalls {
			// Check for doom loop
			if l.doom.IsDoomLoop(tc.Name, tc.Input) {
				if l.onDoomLoop != nil {
					if !l.onDoomLoop(tc.Name) {
						return fmt.Errorf("doom loop detected for tool %s", tc.Name)
					}
				}
			}

			if l.onToolCall != nil {
				l.onToolCall(tc.Name, tc.Input)
			}

			result, isError := l.executeTool(ctx, tc)

			if l.onToolResult != nil {
				l.onToolResult(tc.Name, result, isError)
			}

			l.doom.Track(tc.Name, tc.Input)

			messages = append(messages, Message{
				Role: "user",
				ToolResult: &ToolResult{
					ID:      tc.ID,
					Content: result,
					IsError: isError,
				},
			})
		}
	}
}

func (l *Loop) executeTool(ctx context.Context, tc ToolCall) (string, bool) {
	t, ok := l.tools.Get(tc.Name)
	if !ok {
		return fmt.Sprintf("tool not found: %s", tc.Name), true
	}

	result, err := t.Execute(ctx, tc.Input)
	if err != nil {
		return err.Error(), true
	}

	return result.Output, false
}
```

## Verification
```bash
go build ./internal/agent/...
```

## Done When
- [ ] `internal/agent/loop.go` exists
- [ ] Compiles without error
- [ ] Uses DoomTracker from subtask 021

## Then
Delete this file and exit.
