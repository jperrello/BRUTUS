# Subtask 025: Main CLI Entry Point

## Goal
Create a new main.go that wires everything together.

## Create File
`cmd/brutus/main.go`

## Code to Write

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"brutus/internal/agent"
	"brutus/internal/saturn"
	"brutus/internal/tools"
	"brutus/provider"
)

const systemPrompt = `You are BRUTUS, a coding assistant powered by Saturn.
You help users with programming tasks by reading files, making edits, and running commands.
Be concise and helpful. Use tools to accomplish tasks.`

func main() {
	// Get working directory
	workDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get working directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("🔱 BRUTUS - Saturn-Powered Coding Agent")
	fmt.Printf("Working in: %s\n\n", workDir)

	// Discover Saturn
	fmt.Println("Discovering Saturn...")
	saturnProvider, err := provider.NewSaturn()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to Saturn: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Connected to Saturn\n")

	// Create adapter
	adapter := saturn.NewAdapter(saturnProvider)

	// Create tool registry
	registry := tools.NewDefaultRegistry(workDir)
	fmt.Printf("Loaded %d tools: %v\n\n", len(registry.Names()), registry.Names())

	// Create agent loop
	loop := agent.NewLoop(agent.LoopConfig{
		Provider:     adapter,
		Tools:        registry,
		SystemPrompt: systemPrompt,
		OnToolCall: func(name string, input json.RawMessage) {
			fmt.Printf("\033[96m[tool]\033[0m %s\n", name)
		},
		OnToolResult: func(name string, result string, isError bool) {
			display := result
			if len(display) > 200 {
				display = display[:200] + "..."
			}
			if isError {
				fmt.Printf("\033[91m[error]\033[0m %s\n", display)
			} else {
				fmt.Printf("\033[92m[result]\033[0m %s\n", display)
			}
		},
		OnResponse: func(content string) {
			fmt.Printf("\033[93mBRUTUS\033[0m: %s\n", content)
		},
		OnDoomLoop: func(name string) bool {
			fmt.Printf("\033[91m[warning]\033[0m Repeated tool call detected: %s\n", name)
			fmt.Print("Continue anyway? [y/N] ")
			var response string
			fmt.Scanln(&response)
			return response == "y" || response == "Y"
		},
	})

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nInterrupted")
		cancel()
	}()

	// Simple REPL
	fmt.Println("Type your request (Ctrl+C to exit):")
	for {
		fmt.Print("\033[94mYou\033[0m: ")
		var input string
		_, err := fmt.Scanln(&input)
		if err != nil {
			break
		}

		if input == "" {
			continue
		}

		if err := loop.Run(ctx, input); err != nil {
			if ctx.Err() != nil {
				break
			}
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		fmt.Println()
	}

	fmt.Println("Goodbye!")
}
```

## Verification
```bash
mkdir -p cmd/brutus
go build ./cmd/brutus/...
```

## Done When
- [ ] `cmd/brutus/main.go` exists
- [ ] Compiles successfully
- [ ] Can be run (may fail at runtime if Saturn not available, that's OK)

## Then
Delete this file and exit.

## FINAL NOTE
This is the last subtask! After this:
1. Run `go build ./...` to verify full build
2. Run `go test ./...` to verify tests
3. The ralph loop should detect empty queue and exit
