package provider

import (
	"context"
	"encoding/json"

	"brutus/tools"
)

// Provider is the interface for AI backends.
// This abstraction lets BRUTUS work with:
// - Anthropic API directly (needs ANTHROPIC_API_KEY)
// - Saturn-discovered services (auto-discovers on network)
// - Any OpenAI-compatible API
type Provider interface {
	// Chat sends a conversation to the LLM and gets a response.
	// The response may include tool calls that need to be executed.
	Chat(ctx context.Context, systemPrompt string, messages []Message, tools []tools.Tool) (Message, error)

	// ChatStream sends a conversation and returns a channel for streaming responses.
	ChatStream(ctx context.Context, systemPrompt string, messages []Message, tools []tools.Tool) (<-chan StreamDelta, error)

	// Name returns the provider name for logging.
	Name() string

	// ListModels returns available models from the provider.
	ListModels(ctx context.Context) ([]ModelInfo, error)

	// SetModel changes the active model.
	SetModel(model string)

	// GetModel returns the current model.
	GetModel() string
}

// ModelInfo describes an available model.
type ModelInfo struct {
	ID   string
	Name string
}

// Message represents a conversation message.
// This is a simplified, provider-agnostic format.
type Message struct {
	Role        string       // "user" or "assistant"
	Content     string       // Text content
	ToolCalls   []ToolCall   // Tools the assistant wants to use
	ToolResults []ToolResult // Results from tool execution
}

// ToolCall represents a request from the LLM to execute a tool.
type ToolCall struct {
	ID    string          // Unique identifier for this call
	Name  string          // Tool name
	Input json.RawMessage // Tool input as JSON
}

// ToolResult contains the output of a tool execution.
type ToolResult struct {
	ID      string // Matches ToolCall.ID
	Content string // Tool output
	IsError bool   // Whether the result is an error
}

// StreamDelta represents a chunk from streaming responses.
type StreamDelta struct {
	Content  string    // Text content chunk
	ToolCall *ToolCall // Partial tool call (accumulated)
	Error    error     // Error if streaming failed
	Done     bool      // True when stream is complete
}

// DiscoveryFilter specifies criteria for filtering discovered services.
type DiscoveryFilter struct {
	MinPriority   int      // Only services with priority <= this value
	RequiredAPI   string   // Required API type (e.g., "openai")
	RequiredGPU   bool     // Must have GPU
	MinVRAM       int      // Minimum VRAM in GB
	RequiredModel string   // Must support this model
	LocalOnly     bool     // Exclude remote APIs
}

// FilterServices applies a filter to a list of services.
func FilterServices(services []SaturnService, filter DiscoveryFilter) []SaturnService {
	var result []SaturnService
	for _, svc := range services {
		if filter.MinPriority > 0 && svc.Priority > filter.MinPriority {
			continue
		}
		if filter.RequiredAPI != "" && svc.APIType != filter.RequiredAPI {
			continue
		}
		if filter.RequiredGPU && svc.GPU == "" {
			continue
		}
		if filter.MinVRAM > 0 && svc.VRAMGb < filter.MinVRAM {
			continue
		}
		if filter.RequiredModel != "" {
			hasModel := false
			for _, m := range svc.Models {
				if m == filter.RequiredModel {
					hasModel = true
					break
				}
			}
			if !hasModel {
				continue
			}
		}
		if filter.LocalOnly && svc.APIBase != "" {
			continue
		}
		result = append(result, svc)
	}
	return result
}
