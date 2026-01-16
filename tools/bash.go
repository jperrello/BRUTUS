package tools

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// BashInput defines parameters for the bash tool.
type BashInput struct {
	Command string `json:"command" jsonschema_description:"The shell command to execute."`
}

// Bash executes a shell command and returns its output.
// This is powerful - it lets the agent run builds, tests, git commands, etc.
// Platform-aware: uses cmd.exe on Windows, bash elsewhere.
func Bash(input json.RawMessage) (string, error) {
	var args BashInput
	if err := json.Unmarshal(input, &args); err != nil {
		return "", err
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", args.Command)
	} else {
		cmd = exec.Command("bash", "-c", args.Command)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Return both the error and output - often useful for debugging
		return fmt.Sprintf("Command failed: %s\nOutput: %s", err.Error(), string(output)), nil
	}

	return strings.TrimSpace(string(output)), nil
}

// BashTool is the tool definition for shell execution.
var BashTool = NewTool[BashInput](
	"bash",
	"Execute a shell command and return its output. Use this for running builds, tests, git commands, or any other shell operations.",
	Bash,
)
