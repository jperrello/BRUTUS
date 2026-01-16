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

	// Name returns the provider name for logging.
	Name() string
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
