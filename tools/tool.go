package tools

import (
	"encoding/json"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/invopop/jsonschema"
)

// Tool represents a capability the agent can use.
// This is the core abstraction - everything the agent can DO is a Tool.
//
// To add a new tool:
// 1. Create a new file (e.g., mytool.go)
// 2. Define an input struct with json tags
// 3. Create a function matching ToolFunc signature
// 4. Create a Definition variable using NewTool()
// 5. Register it in the agent's tool list
type Tool struct {
	Name        string
	Description string
	InputSchema anthropic.ToolInputSchemaParam
	Function    ToolFunc
}

// ToolFunc is the signature for tool execution.
// It receives JSON input and returns a string result or error.
type ToolFunc func(input json.RawMessage) (string, error)

// NewTool creates a Tool definition with auto-generated JSON schema.
// The generic type T should be your input struct.
func NewTool[T any](name, description string, fn ToolFunc) Tool {
	return Tool{
		Name:        name,
		Description: description,
		InputSchema: generateSchema[T](),
		Function:    fn,
	}
}

// generateSchema uses reflection to create a JSON schema from a struct.
// This is how the LLM knows what parameters your tool accepts.
func generateSchema[T any]() anthropic.ToolInputSchemaParam {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T
	schema := reflector.Reflect(v)

	return anthropic.ToolInputSchemaParam{
		Properties: schema.Properties,
	}
}

// ToAnthropic converts a Tool to the Anthropic SDK format.
func (t Tool) ToAnthropic() anthropic.ToolUnionParam {
	return anthropic.ToolUnionParam{
		OfTool: &anthropic.ToolParam{
			Name:        t.Name,
			Description: anthropic.String(t.Description),
			InputSchema: t.InputSchema,
		},
	}
}

// Registry holds all available tools.
// Use this to organize tools and make them discoverable.
type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(t Tool) {
	r.tools[t.Name] = t
}

func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) All() []Tool {
	result := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}
