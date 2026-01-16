package main

import (
	"bufio"
	"context"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"brutus/agent"
	"brutus/provider"
	"brutus/tools"
)

//go:embed BRUTUS.md
var embeddedPrompt string

const Version = "2.0.0"

func main() {
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	version := flag.Bool("version", false, "Print version and exit")
	model := flag.String("model", "", "Model to request from Saturn server")
	maxTokens := flag.Int("max-tokens", 8192, "Maximum tokens for responses")
	timeout := flag.Duration("timeout", 5*time.Second, "Saturn discovery timeout")
	cwd := flag.String("cwd", "", "Working directory (defaults to current directory)")
	flag.Parse()

	if *version {
		fmt.Printf("BRUTUS v%s\n", Version)
		os.Exit(0)
	}

	setupLogging(*verbose)

	workDir := getWorkingDir(*cwd)
	if workDir != "." {
		if err := os.Chdir(workDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot change to directory %s: %v\n", workDir, err)
			os.Exit(1)
		}
	}

	// Initialize tools
	registry := tools.NewRegistry()
	registry.Register(tools.ReadFileTool)
	registry.Register(tools.ListFilesTool)
	registry.Register(tools.BashTool)
	registry.Register(tools.EditFileTool)
	registry.Register(tools.CodeSearchTool)

	if *verbose {
		log.Printf("Registered %d tools: %v", len(registry.All()), registry.Names())
	}

	// Discover Saturn services - this is the ONLY way to get AI
	log.Println("Discovering Saturn services on network...")

	prov, err := provider.NewSaturn(context.Background(), provider.SaturnConfig{
		DiscoveryTimeout: *timeout,
		Model:            *model,
		MaxTokens:        *maxTokens,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "BRUTUS requires a Saturn server on your network.")
		fmt.Fprintln(os.Stderr, "Start a Saturn beacon or server, then try again.")
		fmt.Fprintln(os.Stderr, "See: https://github.com/jperrello/Saturn")
		os.Exit(1)
	}

	log.Printf("Connected to: %s", prov.Name())

	// Load system prompt
	systemPrompt := loadSystemPrompt()

	// Create input reader
	scanner := bufio.NewScanner(os.Stdin)
	getUserInput := func() (string, bool) {
		if !scanner.Scan() {
			return "", false
		}
		return scanner.Text(), true
	}

	// Get absolute path of working directory for display
	absWorkDir, _ := os.Getwd()

	// Create and run agent
	a := agent.New(agent.Config{
		Provider:     prov,
		GetUserInput: getUserInput,
		Tools:        registry,
		SystemPrompt: systemPrompt,
		Verbose:      *verbose,
		WorkingDir:   absWorkDir,
	})

	if err := a.Run(context.Background()); err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		os.Exit(1)
	}
}

func setupLogging(verbose bool) {
	if verbose {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Println("BRUTUS starting with verbose logging")
	} else {
		log.SetOutput(os.Stdout)
		log.SetFlags(0)
		log.SetPrefix("")
	}
}

func getWorkingDir(cwd string) string {
	if cwd != "" {
		absPath, err := filepath.Abs(cwd)
		if err != nil {
			return cwd
		}
		return absPath
	}
	return "."
}

func loadSystemPrompt() string {
	promptFiles := []string{"BRUTUS.md", "CLAUDE.md", "AGENTS.md"}
	for _, filename := range promptFiles {
		if content, err := os.ReadFile(filename); err == nil {
			return string(content)
		}
	}
	return embeddedPrompt
}
