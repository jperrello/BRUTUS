# Subtask 004: Tool Interface Definition

## Goal
Define the core Tool interface that all tools will implement.

## Research Reference
`research/opencode-tool/TOOL-SUBSYSTEM-SPEC.md`

## Create File
`internal/tool/tool.go`

## Code to Write

```go
package tool

import (
	"context"
	"encoding/json"
)

type Tool interface {
	Name() string
	Description() string
	InputSchema() json.RawMessage
	Execute(ctx context.Context, input json.RawMessage) (*Result, error)
}

type Result struct {
	Output     string         `json:"output"`
	Title      string         `json:"title"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Truncated  bool           `json:"truncated,omitempty"`
	OutputPath string         `json:"outputPath,omitempty"`
}

type Context struct {
	SessionID string
	MessageID string
	WorkDir   string
	Abort     context.Context
}

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
```

## Create File
`internal/tool/registry.go`

```go
package tool

import "sync"

type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
}

func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) All() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}
```

## Verification
```bash
mkdir -p internal/tool
# Write the files
go build ./internal/tool/...
```

## Done When
- [ ] `internal/tool/tool.go` exists
- [ ] `internal/tool/registry.go` exists
- [ ] `go build ./internal/tool/...` succeeds

## Then
Delete this file and exit.
