# Subtask 024: Saturn Provider Adapter

## Goal
Create an adapter that wraps existing Saturn provider to match the new agent.Provider interface.

## Create File
`internal/saturn/adapter.go`

## Code to Write

```go
package saturn

import (
	"context"
	"encoding/json"

	"brutus/internal/agent"
	"brutus/internal/tool"
	"brutus/provider"
)

type Adapter struct {
	saturn provider.Provider
}

func NewAdapter(saturn provider.Provider) *Adapter {
	return &Adapter{saturn: saturn}
}

func (a *Adapter) Chat(ctx context.Context, systemPrompt string, messages []agent.Message, tools []tool.Tool) (agent.Message, error) {
	// Convert messages to provider format
	providerMsgs := make([]provider.Message, 0, len(messages))
	for _, m := range messages {
		pm := provider.Message{
			Role:    m.Role,
			Content: m.Content,
		}

		// Handle tool calls
		for _, tc := range m.ToolCalls {
			pm.ToolCalls = append(pm.ToolCalls, provider.ToolCall{
				ID:    tc.ID,
				Name:  tc.Name,
				Input: tc.Input,
			})
		}

		// Handle tool results
		if m.ToolResult != nil {
			pm.ToolResults = []provider.ToolResult{{
				ID:      m.ToolResult.ID,
				Content: m.ToolResult.Content,
				IsError: m.ToolResult.IsError,
			}}
		}

		providerMsgs = append(providerMsgs, pm)
	}

	// Convert tools to provider format
	providerTools := make([]provider.Tool, 0, len(tools))
	for _, t := range tools {
		providerTools = append(providerTools, provider.Tool{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		})
	}

	// Call Saturn
	response, err := a.saturn.Chat(ctx, systemPrompt, providerMsgs, providerTools)
	if err != nil {
		return agent.Message{}, err
	}

	// Convert response back
	result := agent.Message{
		Role:    response.Role,
		Content: response.Content,
	}

	for _, tc := range response.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, agent.ToolCall{
			ID:    tc.ID,
			Name:  tc.Name,
			Input: tc.Input,
		})
	}

	return result, nil
}
```

## Note
This adapter bridges the existing Saturn provider with the new agent loop interface.
If the existing provider.Provider interface doesn't match exactly, adjust as needed.

## Verification
```bash
mkdir -p internal/saturn
go build ./internal/saturn/...
```

## Done When
- [ ] `internal/saturn/adapter.go` exists
- [ ] Compiles (may need adjustments based on actual provider interface)

## Then
Delete this file and exit.
