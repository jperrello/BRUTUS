// Stage 3: Multiple Tools
//
// Adding more tools (read, list, bash) to see how they work together.
// Run: go run examples/03-multi-tool/main.go
//
// Key concepts:
// - Tool registry pattern
// - Multiple tools in one response
// - Tool chaining (one tool informs another)
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/invopop/jsonschema"
)

// Tool definition structure
type Tool struct {
	Name        string
	Description string
	Schema      anthropic.ToolInputSchemaParam
	Execute     func(json.RawMessage) (string, error)
}

// Tool inputs
type ReadFileInput struct {
	Path string `json:"path" jsonschema_description:"Path to the file to read"`
}

type ListFilesInput struct {
	Path string `json:"path,omitempty" jsonschema_description:"Directory to list (default: current)"`
}

type BashInput struct {
	Command string `json:"command" jsonschema_description:"Shell command to execute"`
}

// Tool implementations
func readFile(input json.RawMessage) (string, error) {
	var args ReadFileInput
	json.Unmarshal(input, &args)
	content, err := os.ReadFile(args.Path)
	return string(content), err
}

func listFiles(input json.RawMessage) (string, error) {
	var args ListFilesInput
	json.Unmarshal(input, &args)

	dir := "."
	if args.Path != "" {
		dir = args.Path
	}

	var files []string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || strings.HasPrefix(path, ".git") {
			if info != nil && info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		if rel != "." {
			files = append(files, rel)
		}
		return nil
	})

	result, _ := json.Marshal(files)
	return string(result), nil
}

func bash(input json.RawMessage) (string, error) {
	var args BashInput
	json.Unmarshal(input, &args)

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", args.Command)
	} else {
		cmd = exec.Command("bash", "-c", args.Command)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("Error: %v\nOutput: %s", err, output), nil
	}
	return strings.TrimSpace(string(output)), nil
}

func generateSchema[T any]() anthropic.ToolInputSchemaParam {
	reflector := jsonschema.Reflector{AllowAdditionalProperties: false, DoNotReference: true}
	var v T
	schema := reflector.Reflect(v)
	return anthropic.ToolInputSchemaParam{Properties: schema.Properties}
}

func main() {
	client := anthropic.NewClient()
	var conversation []anthropic.MessageParam

	// Register tools
	tools := []Tool{
		{"read_file", "Read file contents", generateSchema[ReadFileInput](), readFile},
		{"list_files", "List directory contents", generateSchema[ListFilesInput](), listFiles},
		{"bash", "Execute shell command", generateSchema[BashInput](), bash},
	}

	// Convert to Anthropic format
	anthropicTools := make([]anthropic.ToolUnionParam, len(tools))
	for i, t := range tools {
		anthropicTools[i] = anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name: t.Name, Description: anthropic.String(t.Description), InputSchema: t.Schema,
			},
		}
	}

	// Create tool lookup map
	toolMap := make(map[string]func(json.RawMessage) (string, error))
	for _, t := range tools {
		toolMap[t.Name] = t.Execute
	}

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Multi-Tool Agent (Ctrl+C to quit)")
	fmt.Println("Try: 'List files and show me the first Go file'")
	fmt.Println("----------------------------------")

	for {
		fmt.Print("You: ")
		if !scanner.Scan() {
			break
		}
		userInput := scanner.Text()
		if userInput == "" {
			continue
		}

		conversation = append(conversation,
			anthropic.NewUserMessage(anthropic.NewTextBlock(userInput)))

		response, _ := client.Messages.New(context.Background(), anthropic.MessageNewParams{
			Model: anthropic.ModelClaude3_7SonnetLatest, MaxTokens: 2048,
			Messages: conversation, Tools: anthropicTools,
		})
		conversation = append(conversation, response.ToParam())

		// Tool loop
		for {
			var hasToolUse bool
			var toolResults []anthropic.ContentBlockParamUnion

			for _, block := range response.Content {
				switch block.Type {
				case "text":
					fmt.Printf("Claude: %s\n", block.Text)
				case "tool_use":
					hasToolUse = true
					tu := block.AsToolUse()
					fmt.Printf("[%s]\n", tu.Name)

					if fn, ok := toolMap[tu.Name]; ok {
						result, err := fn(tu.Input)
						if err != nil {
							toolResults = append(toolResults,
								anthropic.NewToolResultBlock(tu.ID, err.Error(), true))
						} else {
							display := result
							if len(display) > 300 {
								display = display[:300] + "..."
							}
							fmt.Printf("  → %s\n", display)
							toolResults = append(toolResults,
								anthropic.NewToolResultBlock(tu.ID, result, false))
						}
					}
				}
			}

			if !hasToolUse {
				break
			}

			conversation = append(conversation, anthropic.NewUserMessage(toolResults...))
			response, _ = client.Messages.New(context.Background(), anthropic.MessageNewParams{
				Model: anthropic.ModelClaude3_7SonnetLatest, MaxTokens: 2048,
				Messages: conversation, Tools: anthropicTools,
			})
			conversation = append(conversation, response.ToParam())
		}
		fmt.Println()
	}
}
