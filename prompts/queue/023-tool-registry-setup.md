# Subtask 023: Tool Registry Setup

## Goal
Create a function to set up all tools in the registry.

## Create File
`internal/tools/registry.go`

## Code to Write

```go
package tools

import (
	"brutus/internal/tool"
)

func NewDefaultRegistry(workDir string) *tool.Registry {
	registry := tool.NewRegistry()

	registry.Register(NewReadTool(workDir))
	registry.Register(NewEditTool(workDir))
	registry.Register(NewListTool(workDir))
	registry.Register(NewBashTool(workDir))
	registry.Register(NewSearchTool(workDir))

	return registry
}
```

## Verification
```bash
go build ./internal/tools/...
```

## Done When
- [ ] `internal/tools/registry.go` exists
- [ ] All tools registered
- [ ] Compiles

## Then
Delete this file and exit.
