package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"brutus/agent"
	"brutus/provider"
	"brutus/tools"
)

func main() {
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	workDir := flag.String("dir", ".", "Working directory")
	model := flag.String("model", "", "Model to use (optional)")
	flag.Parse()

	ctx := context.Background()

	systemPrompt, err := os.ReadFile("BRUTUS.md")
	if err != nil {
		systemPrompt = []byte("You are BRUTUS, a coding agent.")
	}

	fmt.Println("\033[90mDiscovering Saturn services...\033[0m")

	prov, err := provider.NewSaturn(ctx, provider.SaturnConfig{
		Model:     *model,
		MaxTokens: 4096,
	})
	if err != nil {
		log.Fatalf("Failed to connect to Saturn: %v", err)
	}

	fmt.Printf("\033[92mConnected to %s\033[0m\n", prov.Name())

	registry := tools.NewRegistry()
	registry.Register(tools.ReadFileTool)
	registry.Register(tools.ListFilesTool)
	registry.Register(tools.EditFileTool)
	registry.Register(tools.BashTool)
	registry.Register(tools.CodeSearchTool)

	ag := agent.New(agent.Config{
		Provider:     prov,
		Tools:        registry,
		SystemPrompt: string(systemPrompt),
		Verbose:      *verbose,
		WorkingDir:   *workDir,
	})

	if err := ag.Run(ctx); err != nil {
		log.Fatalf("Agent error: %v", err)
	}
}
