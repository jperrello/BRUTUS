// Stage 1: Simple Chatbot
//
// This is the foundation - a basic chatbot with no tools.
// Run: go run examples/01-chat/main.go
//
// Key concepts:
// - Creating an Anthropic client
// - The conversation loop
// - Sending messages and receiving responses
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
)

func main() {
	// Create the Anthropic client (uses ANTHROPIC_API_KEY env var)
	client := anthropic.NewClient()

	// Conversation history - this is how the model maintains context
	var conversation []anthropic.MessageParam

	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("Simple Chat (Ctrl+C to quit)")
	fmt.Println("----------------------------")

	// THE LOOP - this is the heart of any chat application
	for {
		fmt.Print("You: ")
		if !scanner.Scan() {
			break
		}
		userInput := scanner.Text()
		if userInput == "" {
			continue
		}

		// Add user message to conversation
		conversation = append(conversation,
			anthropic.NewUserMessage(anthropic.NewTextBlock(userInput)))

		// Send to Claude
		response, err := client.Messages.New(context.Background(), anthropic.MessageNewParams{
			Model:     anthropic.ModelClaude3_7SonnetLatest,
			MaxTokens: 1024,
			Messages:  conversation,
		})
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		// Add assistant response to conversation (for context in next turn)
		conversation = append(conversation, response.ToParam())

		// Print the response
		for _, block := range response.Content {
			if block.Type == "text" {
				fmt.Printf("Claude: %s\n", block.Text)
			}
		}
		fmt.Println()
	}
}
