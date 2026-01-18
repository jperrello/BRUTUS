# Subtask 020: Search Tool

## Goal
Implement code search tool.

## Create File
`internal/tools/search.go`

## Code to Write

```go
package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"brutus/internal/tool"
	"brutus/internal/truncate"
)

type SearchInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
	Glob    string `json:"glob,omitempty"`
}

type SearchTool struct {
	workDir string
}

func NewSearchTool(workDir string) *SearchTool {
	return &SearchTool{workDir: workDir}
}

func (t *SearchTool) Name() string { return "search" }

func (t *SearchTool) Description() string {
	return `Search for a pattern in files. Returns matching lines with file and line number.
Use glob to filter files (e.g., "*.go" or "**/*.ts").`
}

func (t *SearchTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {"type": "string", "description": "Regex pattern to search for"},
			"path": {"type": "string", "description": "Directory to search in (default: current)"},
			"glob": {"type": "string", "description": "File glob pattern (e.g., *.go)"}
		},
		"required": ["pattern"]
	}`)
}

func (t *SearchTool) Execute(ctx context.Context, input json.RawMessage) (*tool.Result, error) {
	var args SearchInput
	if err := json.Unmarshal(input, &args); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	if args.Pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}

	re, err := regexp.Compile(args.Pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex: %w", err)
	}

	searchPath := args.Path
	if searchPath == "" {
		searchPath = "."
	}
	if !filepath.IsAbs(searchPath) {
		searchPath = filepath.Join(t.workDir, searchPath)
	}

	var results []string

	filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Skip binary files, hidden files, common ignores
		name := info.Name()
		if strings.HasPrefix(name, ".") {
			return nil
		}
		ext := filepath.Ext(name)
		if ext == ".exe" || ext == ".bin" || ext == ".dll" || ext == ".so" {
			return nil
		}

		// Apply glob filter if specified
		if args.Glob != "" {
			matched, _ := filepath.Match(args.Glob, name)
			if !matched {
				return nil
			}
		}

		// Search file
		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		relPath, _ := filepath.Rel(t.workDir, path)
		scanner := bufio.NewScanner(file)
		lineNum := 0

		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if re.MatchString(line) {
				results = append(results, fmt.Sprintf("%s:%d: %s", relPath, lineNum, line))
			}
		}

		return nil
	})

	output := strings.Join(results, "\n")
	if output == "" {
		output = "No matches found"
	}

	truncResult := truncate.Output(output)

	return &tool.Result{
		Output:     truncResult.Content,
		Title:      args.Pattern,
		Truncated:  truncResult.Truncated,
		OutputPath: truncResult.OutputPath,
	}, nil
}
```

## Create Test
Add to `internal/tools/search_test.go`:

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

func TestSearchTool(t *testing.T) {
	// Create temp directory with a file
	dir, _ := os.MkdirTemp("", "brutus-test-*")
	defer os.RemoveAll(dir)

	os.WriteFile(filepath.Join(dir, "test.go"), []byte("func hello() {\n  return\n}"), 0644)

	tool := NewSearchTool(dir)
	input, _ := json.Marshal(SearchInput{
		Pattern: "func.*hello",
		Path:    ".",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result.Output, "func hello") {
		t.Errorf("Should find 'func hello' pattern")
	}
}
```

## Verification
```bash
go test ./internal/tools/...
```

## Done When
- [ ] `internal/tools/search.go` exists
- [ ] Test passes

## Then
Delete this file and exit.
