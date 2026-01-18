# Subtask 016: Read Tool

## Goal
Implement the read_file tool with tracking.

## Create File
`internal/tools/read.go`

## Code to Write

```go
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"brutus/internal/tool"
	"brutus/internal/tool/readtrack"
	"brutus/internal/truncate"
)

type ReadInput struct {
	Path   string `json:"path"`
	Offset int    `json:"offset,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type ReadTool struct {
	workDir string
}

func NewReadTool(workDir string) *ReadTool {
	return &ReadTool{workDir: workDir}
}

func (t *ReadTool) Name() string { return "read_file" }

func (t *ReadTool) Description() string {
	return `Read the contents of a file. Returns the file content as text.
Use offset and limit to read portions of large files.`
}

func (t *ReadTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Path to the file to read"},
			"offset": {"type": "integer", "description": "Line offset to start reading from (0-based)"},
			"limit": {"type": "integer", "description": "Maximum number of lines to read"}
		},
		"required": ["path"]
	}`)
}

func (t *ReadTool) Execute(ctx context.Context, input json.RawMessage) (*tool.Result, error) {
	var args ReadInput
	if err := json.Unmarshal(input, &args); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	if args.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	path := args.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(t.workDir, path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Record that this file was read
	readtrack.RecordRead(path)

	output := string(content)

	// Apply truncation
	truncResult := truncate.Output(output)

	return &tool.Result{
		Output:     truncResult.Content,
		Title:      args.Path,
		Truncated:  truncResult.Truncated,
		OutputPath: truncResult.OutputPath,
	}, nil
}
```

## Create Test File
`internal/tools/read_test.go`

```go
package tools

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"brutus/internal/tool/readtrack"
)

func TestReadTool(t *testing.T) {
	// Create temp file
	f, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString("test content")
	f.Close()

	readtrack.Clear()

	tool := NewReadTool("")
	input, _ := json.Marshal(ReadInput{Path: f.Name()})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	if result.Output != "test content" {
		t.Errorf("Expected 'test content', got %q", result.Output)
	}

	// Should be tracked now
	if !readtrack.WasRead(f.Name()) {
		t.Error("File should be tracked as read")
	}
}
```

## Verification
```bash
mkdir -p internal/tools
go test ./internal/tools/...
```

## Done When
- [ ] `internal/tools/read.go` exists
- [ ] `internal/tools/read_test.go` exists
- [ ] Tests pass
- [ ] File reads are tracked

## Then
Delete this file and exit.
