# Subtask 003: Output Truncation Utility

## Goal
Create output truncation to prevent context explosion from large tool outputs.

## Research Reference
`research/opencode-tool/TRUNCATION-ALGORITHM.md`

## Create File
`internal/truncate/truncate.go`

## Code to Write

```go
package truncate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	MaxLines = 2000
	MaxBytes = 50 * 1024 // 50KB
)

type Result struct {
	Content    string
	Truncated  bool
	OutputPath string // Path to full output if truncated
}

func Output(content string) Result {
	lines := strings.Split(content, "\n")
	bytes := len(content)

	// Check if truncation needed
	if len(lines) <= MaxLines && bytes <= MaxBytes {
		return Result{Content: content, Truncated: false}
	}

	// Truncate
	truncated := content
	if len(lines) > MaxLines {
		truncated = strings.Join(lines[:MaxLines], "\n")
	}
	if len(truncated) > MaxBytes {
		truncated = truncated[:MaxBytes]
	}

	// Write full output to temp file
	outputPath := writeFullOutput(content)

	// Add truncation notice
	notice := fmt.Sprintf("\n\n[OUTPUT TRUNCATED - %d lines, %d bytes. Full output: %s]",
		len(lines), bytes, outputPath)
	truncated += notice

	return Result{
		Content:    truncated,
		Truncated:  true,
		OutputPath: outputPath,
	}
}

func writeFullOutput(content string) string {
	dir := filepath.Join(os.TempDir(), "brutus-output")
	os.MkdirAll(dir, 0755)

	f, err := os.CreateTemp(dir, "output-*.txt")
	if err != nil {
		return "(failed to write full output)"
	}
	defer f.Close()

	f.WriteString(content)
	return f.Name()
}
```

## Create Test File
`internal/truncate/truncate_test.go`

```go
package truncate

import (
	"strings"
	"testing"
)

func TestOutput_NoTruncation(t *testing.T) {
	content := "short content"
	result := Output(content)
	if result.Truncated {
		t.Error("Should not truncate short content")
	}
	if result.Content != content {
		t.Error("Content should be unchanged")
	}
}

func TestOutput_TruncateLines(t *testing.T) {
	lines := make([]string, MaxLines+100)
	for i := range lines {
		lines[i] = "line"
	}
	content := strings.Join(lines, "\n")

	result := Output(content)
	if !result.Truncated {
		t.Error("Should truncate content over MaxLines")
	}
	if result.OutputPath == "" {
		t.Error("Should have output path when truncated")
	}
}

func TestOutput_TruncateBytes(t *testing.T) {
	content := strings.Repeat("x", MaxBytes+1000)

	result := Output(content)
	if !result.Truncated {
		t.Error("Should truncate content over MaxBytes")
	}
}
```

## Verification
```bash
mkdir -p internal/truncate
# Write the files
go test ./internal/truncate/...
```

## Done When
- [ ] `internal/truncate/truncate.go` exists
- [ ] `internal/truncate/truncate_test.go` exists
- [ ] `go test ./internal/truncate/...` passes

## Then
Delete this file and exit.
