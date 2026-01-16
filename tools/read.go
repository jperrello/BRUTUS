package tools

import (
	"encoding/json"
	"fmt"
	"os"
)

// ReadFileInput defines the parameters for the read_file tool.
// The jsonschema_description tag becomes the parameter description in the schema.
type ReadFileInput struct {
	Path string `json:"path" jsonschema_description:"The relative or absolute path to the file to read."`
}

// ReadFile reads and returns the contents of a file.
// This is often the first tool an agent needs - you must understand code before modifying it.
func ReadFile(input json.RawMessage) (string, error) {
	var args ReadFileInput
	if err := json.Unmarshal(input, &args); err != nil {
		return "", err
	}

	content, err := os.ReadFile(args.Path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	return string(content), nil
}

// ReadFileTool is the tool definition for reading files.
var ReadFileTool = NewTool[ReadFileInput](
	"read_file",
	"Read the contents of a file at the given path. Use this to examine source code, configuration files, or any text file.",
	ReadFile,
)
