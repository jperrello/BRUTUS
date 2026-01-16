package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
)

// EditFileInput defines parameters for the edit_file tool.
type EditFileInput struct {
	Path   string `json:"path" jsonschema_description:"The path to the file to edit or create."`
	OldStr string `json:"old_str" jsonschema_description:"The exact text to find and replace. Must be unique in the file. Use empty string to create new file or append."`
	NewStr string `json:"new_str" jsonschema_description:"The replacement text."`
}

// EditFile performs precise text replacement in files.
// This is how the agent modifies code - by finding exact text and replacing it.
//
// Key behaviors:
// - If file doesn't exist and old_str is empty, creates new file with new_str
// - If old_str is empty on existing file, appends new_str
// - old_str must match exactly ONE location (prevents ambiguous edits)
func EditFile(input json.RawMessage) (string, error) {
	var args EditFileInput
	if err := json.Unmarshal(input, &args); err != nil {
		return "", err
	}

	if args.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	if args.OldStr == args.NewStr {
		return "", fmt.Errorf("old_str and new_str must be different")
	}

	content, err := os.ReadFile(args.Path)
	if err != nil {
		if os.IsNotExist(err) && args.OldStr == "" {
			// Create new file
			dir := path.Dir(args.Path)
			if dir != "." {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return "", fmt.Errorf("failed to create directory: %w", err)
				}
			}
			if err := os.WriteFile(args.Path, []byte(args.NewStr), 0644); err != nil {
				return "", fmt.Errorf("failed to create file: %w", err)
			}
			return fmt.Sprintf("Created file %s", args.Path), nil
		}
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	oldContent := string(content)
	var newContent string

	if args.OldStr == "" {
		// Append mode
		newContent = oldContent + args.NewStr
	} else {
		// Replace mode - must be unique
		count := strings.Count(oldContent, args.OldStr)
		if count == 0 {
			return "", fmt.Errorf("old_str not found in file")
		}
		if count > 1 {
			return "", fmt.Errorf("old_str found %d times, must be unique", count)
		}
		newContent = strings.Replace(oldContent, args.OldStr, args.NewStr, 1)
	}

	if err := os.WriteFile(args.Path, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return "OK", nil
}

// EditFileTool is the tool definition for file editing.
var EditFileTool = NewTool[EditFileInput](
	"edit_file",
	`Edit a file by replacing text. Provide the file path, the exact text to find (old_str), and the replacement text (new_str).
If the file doesn't exist and old_str is empty, a new file will be created with new_str as content.
The old_str must match exactly one location in the file.`,
	EditFile,
)
