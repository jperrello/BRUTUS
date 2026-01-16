package tools

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// CodeSearchInput defines parameters for the code_search tool.
type CodeSearchInput struct {
	Pattern       string `json:"pattern" jsonschema_description:"The search pattern (regex supported with ripgrep)."`
	Path          string `json:"path,omitempty" jsonschema_description:"Directory or file to search in. Defaults to current directory."`
	FileType      string `json:"file_type,omitempty" jsonschema_description:"File extension to filter by (e.g., 'go', 'js', 'py')."`
	CaseSensitive bool   `json:"case_sensitive,omitempty" jsonschema_description:"Whether the search is case sensitive. Default: false."`
}

// CodeSearch finds patterns in code using ripgrep (or fallback).
// This is what ghuntley calls "the most sophisticated" tool - but it's just ripgrep.
// The power comes from using existing tools, not building proprietary indexing.
func CodeSearch(input json.RawMessage) (string, error) {
	var args CodeSearchInput
	if err := json.Unmarshal(input, &args); err != nil {
		return "", err
	}

	if args.Pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}

	searchPath := "."
	if args.Path != "" {
		searchPath = args.Path
	}

	// Try ripgrep first (best option)
	_, err := exec.LookPath("rg")
	if err != nil {
		return fallbackSearch(args.Pattern, searchPath, args.CaseSensitive)
	}

	cmdArgs := []string{"--line-number", "--with-filename", "--color=never"}

	if !args.CaseSensitive {
		cmdArgs = append(cmdArgs, "--ignore-case")
	}

	if args.FileType != "" {
		cmdArgs = append(cmdArgs, "--type", args.FileType)
	}

	cmdArgs = append(cmdArgs, args.Pattern, searchPath)

	cmd := exec.Command("rg", cmdArgs...)
	output, err := cmd.Output()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			return "No matches found", nil
		}
		return "", fmt.Errorf("search failed: %w", err)
	}

	return limitResults(string(output), 50), nil
}

// fallbackSearch uses platform-native tools when ripgrep isn't available.
func fallbackSearch(pattern, searchPath string, caseSensitive bool) (string, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		args := []string{"/S", "/N"}
		if !caseSensitive {
			args = append(args, "/I")
		}
		args = append(args, "/C:"+pattern, searchPath+"\\*")
		cmd = exec.Command("findstr", args...)
	} else {
		args := []string{"-r", "-n"}
		if !caseSensitive {
			args = append(args, "-i")
		}
		args = append(args, pattern, searchPath)
		cmd = exec.Command("grep", args...)
	}

	output, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			return "No matches found", nil
		}
		return "", fmt.Errorf("search failed: %w", err)
	}

	return limitResults(string(output), 50), nil
}

// limitResults truncates output to a reasonable size.
func limitResults(output string, maxLines int) string {
	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")

	if len(lines) > maxLines {
		result = strings.Join(lines[:maxLines], "\n") +
			fmt.Sprintf("\n... (showing %d of %d matches)", maxLines, len(lines))
	}

	return result
}

// CodeSearchTool is the tool definition for code searching.
var CodeSearchTool = NewTool[CodeSearchInput](
	"code_search",
	`Search for patterns in code using ripgrep. Use this to find function definitions, variable usage, imports, or any text pattern across the codebase.
Falls back to findstr on Windows if ripgrep is not available.`,
	CodeSearch,
)
