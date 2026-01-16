// Stage 2: Adding the Read File Tool
//
// This introduces the first tool - reading files.
// Run: go run examples/02-read-tool/main.go
//
// Key concepts:
// - Defining a tool with JSON schema
// - Tool use in the response
// - The tool execution loop
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/invopop/jsonschema"
)

// Tool input structure - the JSON schema is auto-generated from this
type ReadFileInput struct {
	Path string `json:"path" jsonschema_description:"Path to the file to read"`
}

// Tool execution function
func readFile(input json.RawMessage) (string, error) {
	var args ReadFileInput
	if err := json.Unmarshal(input, &args); err != nil {
		return "", err
	}
	content, err := os.ReadFile(args.Path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// Generate JSON schema from struct
func generateSchema[T any]() anthropic.ToolInputSchemaParam {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T
	schema := reflector.Reflect(v)
	return anthropic.ToolInputSchemaParam{
		Properties: schema.Properties,
	}
}

func main() {
	client := anthropic.NewClient()
	var conversation []anthropic.MessageParam

	// Define the read_file tool
	tools := []anthropic.ToolUnionParam{{
		OfTool: &anthropic.ToolParam{
			Name:        "read_file",
			Description: anthropic.String("Read the contents of a file"),
			InputSchema: generateSchema[ReadFileInput](),
		},
	}}

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Chat with Read Tool (Ctrl+C to quit)")
	fmt.Println("Try: 'What is in main.go?'")
	fmt.Println("-------------------------------------")

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

		// Send with tools
		response, err := client.Messages.New(context.Background(), anthropic.MessageNewParams{
			Model:     anthropic.ModelClaude3_7SonnetLatest,
			MaxTokens: 1024,
			Messages:  conversation,
			Tools:     tools,
		})
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		conversation = append(conversation, response.ToParam())

		// TOOL LOOP - keep going while model wants to use tools
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
					fmt.Printf("[Using tool: %s]\n", tu.Name)

					// Execute the tool
					result, err := readFile(tu.Input)
					if err != nil {
						toolResults = append(toolResults,
							anthropic.NewToolResultBlock(tu.ID, err.Error(), true))
					} else {
						// Show truncated result
						display := result
						if len(display) > 200 {
							display = display[:200] + "..."
						}
						fmt.Printf("[Result]: %s\n", display)
						toolResults = append(toolResults,
							anthropic.NewToolResultBlock(tu.ID, result, false))
					}
				}
			}

			if !hasToolUse {
				break
			}

			// Send tool results back
			conversation = append(conversation,
				anthropic.NewUserMessage(toolResults...))

			response, err = client.Messages.New(context.Background(), anthropic.MessageNewParams{
				Model:     anthropic.ModelClaude3_7SonnetLatest,
				MaxTokens: 1024,
				Messages:  conversation,
				Tools:     tools,
			})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				break
			}
			conversation = append(conversation, response.ToParam())
		}
		fmt.Println()
	}
}
