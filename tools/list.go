package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ListFilesInput defines parameters for the list_files tool.
type ListFilesInput struct {
	Path string `json:"path,omitempty" jsonschema_description:"The directory path to list. Defaults to current directory if not provided."`
}

// ListFiles enumerates files and directories, skipping common non-code directories.
// This helps the agent understand project structure.
func ListFiles(input json.RawMessage) (string, error) {
	var args ListFilesInput
	if err := json.Unmarshal(input, &args); err != nil {
		return "", err
	}

	dir := "."
	if args.Path != "" {
		dir = args.Path
	}

	// Directories to skip (not useful for code exploration)
	skipDirs := map[string]bool{
		".git":         true,
		".devenv":      true,
		"node_modules": true,
		"vendor":       true,
		"__pycache__":  true,
		".venv":        true,
	}

	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		// Skip ignored directories
		if info.IsDir() {
			if skipDirs[relPath] || strings.HasPrefix(relPath, ".git/") {
				return filepath.SkipDir
			}
		}

		if relPath != "." {
			if info.IsDir() {
				files = append(files, relPath+"/")
			} else {
				files = append(files, relPath)
			}
		}
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to list files: %w", err)
	}

	result, err := json.Marshal(files)
	if err != nil {
		return "", err
	}

	return string(result), nil
}

// ListFilesTool is the tool definition for listing files.
var ListFilesTool = NewTool[ListFilesInput](
	"list_files",
	"List files and directories at a given path. Use this to explore project structure and find relevant files.",
	ListFiles,
)
