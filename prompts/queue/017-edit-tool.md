# Subtask 017: Edit Tool with Fuzzy Matching

## Goal
Implement the edit_file tool using fuzzy replacement.

## Create File
`internal/tools/edit.go`

## Code to Write

```go
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"brutus/internal/fuzzy"
	"brutus/internal/tool"
	"brutus/internal/tool/readtrack"
)

type EditInput struct {
	Path       string `json:"path"`
	OldStr     string `json:"old_str"`
	NewStr     string `json:"new_str"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

type EditTool struct {
	workDir string
}

func NewEditTool(workDir string) *EditTool {
	return &EditTool{workDir: workDir}
}

func (t *EditTool) Name() string { return "edit_file" }

func (t *EditTool) Description() string {
	return `Edit a file by replacing text. Uses fuzzy matching to handle indentation differences.
If old_str is empty and file doesn't exist, creates the file.
If old_str is empty and file exists, appends new_str.`
}

func (t *EditTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Path to the file to edit"},
			"old_str": {"type": "string", "description": "Text to find and replace (empty to create/append)"},
			"new_str": {"type": "string", "description": "Replacement text"},
			"replace_all": {"type": "boolean", "description": "Replace all occurrences"}
		},
		"required": ["path", "old_str", "new_str"]
	}`)
}

func (t *EditTool) Execute(ctx context.Context, input json.RawMessage) (*tool.Result, error) {
	var args EditInput
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

	// Handle file creation (empty old_str, file doesn't exist)
	if args.OldStr == "" {
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			// Create directory if needed
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create directory: %w", err)
			}
			if err := os.WriteFile(path, []byte(args.NewStr), 0644); err != nil {
				return nil, fmt.Errorf("failed to create file: %w", err)
			}
			readtrack.RecordRead(path)
			return &tool.Result{
				Output: fmt.Sprintf("Created file: %s", args.Path),
				Title:  args.Path,
			}, nil
		}
	}

	// File must exist for edits
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("file not found: %s", args.Path)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory: %s", args.Path)
	}

	// Must have read the file first
	if err := readtrack.AssertReadable(path); err != nil {
		return nil, err
	}

	// Read current content
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var newContent string

	if args.OldStr == "" {
		// Append mode
		newContent = string(content) + args.NewStr
	} else {
		// Replace mode with fuzzy matching
		newContent, err = fuzzy.Replace(string(content), args.OldStr, args.NewStr, args.ReplaceAll)
		if err != nil {
			return nil, err
		}
	}

	// Write new content
	if err := os.WriteFile(path, []byte(newContent), info.Mode()); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	// Update read tracking
	readtrack.RecordRead(path)

	return &tool.Result{
		Output: "Edit applied successfully",
		Title:  args.Path,
	}, nil
}
```

## Create Test File
`internal/tools/edit_test.go`

```go
package tools

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"brutus/internal/tool/readtrack"
)

func TestEditTool_Create(t *testing.T) {
	path := filepath.Join(os.TempDir(), "brutus-test-create.txt")
	defer os.Remove(path)

	tool := NewEditTool("")
	input, _ := json.Marshal(EditInput{
		Path:   path,
		OldStr: "",
		NewStr: "new file content",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	if result.Output == "" {
		t.Error("Expected success message")
	}

	content, _ := os.ReadFile(path)
	if string(content) != "new file content" {
		t.Errorf("File content mismatch")
	}
}

func TestEditTool_Replace(t *testing.T) {
	// Create temp file
	f, _ := os.CreateTemp("", "test-*.txt")
	f.WriteString("hello world")
	f.Close()
	defer os.Remove(f.Name())

	readtrack.Clear()
	readtrack.RecordRead(f.Name())

	tool := NewEditTool("")
	input, _ := json.Marshal(EditInput{
		Path:   f.Name(),
		OldStr: "world",
		NewStr: "universe",
	})

	_, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(f.Name())
	if string(content) != "hello universe" {
		t.Errorf("Expected 'hello universe', got %q", string(content))
	}
}

func TestEditTool_MustReadFirst(t *testing.T) {
	f, _ := os.CreateTemp("", "test-*.txt")
	f.WriteString("content")
	f.Close()
	defer os.Remove(f.Name())

	readtrack.Clear() // Don't mark as read

	tool := NewEditTool("")
	input, _ := json.Marshal(EditInput{
		Path:   f.Name(),
		OldStr: "content",
		NewStr: "new",
	})

	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("Should fail when file not read first")
	}
}
```

## Verification
```bash
go test ./internal/tools/...
```

## Done When
- [ ] `internal/tools/edit.go` exists
- [ ] `internal/tools/edit_test.go` exists
- [ ] Fuzzy matching works
- [ ] Read-before-edit enforced

## Then
Delete this file and exit.
