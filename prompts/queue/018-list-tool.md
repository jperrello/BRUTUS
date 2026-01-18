# Subtask 018: List Files Tool

## Goal
Implement the list_files tool for directory listing.

## Create File
`internal/tools/list.go`

## Code to Write

```go
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"brutus/internal/tool"
	"brutus/internal/truncate"
)

type ListInput struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive,omitempty"`
}

type ListTool struct {
	workDir string
}

func NewListTool(workDir string) *ListTool {
	return &ListTool{workDir: workDir}
}

func (t *ListTool) Name() string { return "list_files" }

func (t *ListTool) Description() string {
	return `List files and directories at the given path.
Use recursive=true to list all files recursively.`
}

func (t *ListTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Directory path to list"},
			"recursive": {"type": "boolean", "description": "List recursively"}
		},
		"required": ["path"]
	}`)
}

func (t *ListTool) Execute(ctx context.Context, input json.RawMessage) (*tool.Result, error) {
	var args ListInput
	if err := json.Unmarshal(input, &args); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	path := args.Path
	if path == "" {
		path = "."
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(t.workDir, path)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("path not found: %s", args.Path)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", args.Path)
	}

	var files []string

	if args.Recursive {
		filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(path, p)
			if rel == "." {
				return nil
			}
			if info.IsDir() {
				files = append(files, rel+"/")
			} else {
				files = append(files, rel)
			}
			return nil
		})
	} else {
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read directory: %w", err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() {
				name += "/"
			}
			files = append(files, name)
		}
	}

	sort.Strings(files)
	output := strings.Join(files, "\n")

	truncResult := truncate.Output(output)

	return &tool.Result{
		Output:     truncResult.Content,
		Title:      args.Path,
		Truncated:  truncResult.Truncated,
		OutputPath: truncResult.OutputPath,
	}, nil
}
```

## Create Test
Add to `internal/tools/list_test.go`:

```go
package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListTool(t *testing.T) {
	// Create temp directory with files
	dir, _ := os.MkdirTemp("", "brutus-test-*")
	defer os.RemoveAll(dir)

	os.WriteFile(filepath.Join(dir, "file1.txt"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "file2.txt"), []byte(""), 0644)
	os.Mkdir(filepath.Join(dir, "subdir"), 0755)

	tool := NewListTool("")
	input, _ := json.Marshal(ListInput{Path: dir})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result.Output, "file1.txt") {
		t.Error("Should list file1.txt")
	}
	if !strings.Contains(result.Output, "subdir/") {
		t.Error("Should list subdir/")
	}
}
```

## Verification
```bash
go test ./internal/tools/...
```

## Done When
- [ ] `internal/tools/list.go` exists
- [ ] Test passes

## Then
Delete this file and exit.
