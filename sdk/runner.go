package sdk

import (
	"encoding/json"
	"fmt"

	"brutus/tools"
)

type ToolRunner struct {
	registry *tools.Registry
	calls    []ToolExecution
}

type ToolExecution struct {
	ToolName string
	Input    json.RawMessage
	Result   string
	Error    error
}

func NewToolRunner() *ToolRunner {
	return &ToolRunner{
		registry: tools.NewRegistry(),
	}
}

func NewToolRunnerWithRegistry(registry *tools.Registry) *ToolRunner {
	return &ToolRunner{
		registry: registry,
	}
}

func (r *ToolRunner) Register(t tools.Tool) *ToolRunner {
	r.registry.Register(t)
	return r
}

func (r *ToolRunner) RegisterAll(toolList ...tools.Tool) *ToolRunner {
	for _, t := range toolList {
		r.registry.Register(t)
	}
	return r
}

func (r *ToolRunner) Execute(toolName string, inputJSON string) (string, error) {
	tool, ok := r.registry.Get(toolName)
	if !ok {
		return "", fmt.Errorf("tool '%s' not found in registry", toolName)
	}

	input := json.RawMessage(inputJSON)
	result, err := tool.Function(input)

	r.calls = append(r.calls, ToolExecution{
		ToolName: toolName,
		Input:    input,
		Result:   result,
		Error:    err,
	})

	return result, err
}

func (r *ToolRunner) ExecuteWithMap(toolName string, input map[string]interface{}) (string, error) {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("failed to marshal input: %w", err)
	}
	return r.Execute(toolName, string(inputJSON))
}

func (r *ToolRunner) GetCalls() []ToolExecution {
	return r.calls
}

func (r *ToolRunner) GetRegistry() *tools.Registry {
	return r.registry
}

func (r *ToolRunner) ListTools() []string {
	return r.registry.Names()
}

func (r *ToolRunner) Reset() {
	r.calls = nil
}

func DefaultToolRunner() *ToolRunner {
	runner := NewToolRunner()
	runner.Register(tools.ReadFileTool)
	runner.Register(tools.ListFilesTool)
	runner.Register(tools.EditFileTool)
	runner.Register(tools.BashTool)
	runner.Register(tools.CodeSearchTool)
	runner.Register(tools.BroadcastTool)
	runner.Register(tools.ObserveAgentsTool)
	return runner
}
