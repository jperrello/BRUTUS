# Subtask 019: Bash Tool (Basic)

## Goal
Implement basic bash tool for command execution.

## Create File
`internal/tools/bash.go`

## Code to Write

```go
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"brutus/internal/tool"
	"brutus/internal/truncate"
)

type BashInput struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"` // seconds
}

type BashTool struct {
	workDir string
}

func NewBashTool(workDir string) *BashTool {
	return &BashTool{workDir: workDir}
}

func (t *BashTool) Name() string { return "bash" }

func (t *BashTool) Description() string {
	return `Execute a bash command and return its output.
Use for running build commands, git operations, etc.`
}

func (t *BashTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "description": "Command to execute"},
			"timeout": {"type": "integer", "description": "Timeout in seconds (default 120)"}
		},
		"required": ["command"]
	}`)
}

func (t *BashTool) Execute(ctx context.Context, input json.RawMessage) (*tool.Result, error) {
	var args BashInput
	if err := json.Unmarshal(input, &args); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	if args.Command == "" {
		return nil, fmt.Errorf("command is required")
	}

	timeout := args.Timeout
	if timeout <= 0 {
		timeout = 120
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", args.Command)
	} else {
		cmd = exec.CommandContext(ctx, "bash", "-c", args.Command)
	}

	cmd.Dir = t.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\n[stderr]\n" + stderr.String()
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			output += "\n[timeout after " + fmt.Sprintf("%d", timeout) + " seconds]"
		} else {
			output += "\n[exit error: " + err.Error() + "]"
		}
	}

	truncResult := truncate.Output(output)

	return &tool.Result{
		Output:     truncResult.Content,
		Title:      args.Command,
		Truncated:  truncResult.Truncated,
		OutputPath: truncResult.OutputPath,
	}, nil
}
```

## Create Test
`internal/tools/bash_test.go`:

```go
package tools

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"
)

func TestBashTool(t *testing.T) {
	tool := NewBashTool("")

	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "echo hello"
	} else {
		cmd = "echo hello"
	}

	input, _ := json.Marshal(BashInput{Command: cmd})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result.Output, "hello") {
		t.Errorf("Expected output to contain 'hello', got %q", result.Output)
	}
}
```

## Verification
```bash
go test ./internal/tools/...
```

## Done When
- [ ] `internal/tools/bash.go` exists
- [ ] Test passes on current platform

## Then
Delete this file and exit.
