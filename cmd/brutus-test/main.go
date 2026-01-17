package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"brutus/provider"
	"brutus/sdk"
	"brutus/tools"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "tools":
		listTools()
	case "tool":
		runTool(args)
	case "scenario":
		runScenario(args)
	case "multi-agent":
		runMultiAgent(args)
	case "live-multi-agent":
		runLiveMultiAgent(args)
	case "harness":
		runHarness(args)
	case "help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`brutus-test - Testing SDK for BRUTUS

Usage:
  brutus-test <command> [arguments]

Commands:
  tools                    List all available tools
  tool <name> <json>       Execute a tool with JSON input
  scenario <file>          Run a test scenario from JSON file
  multi-agent <file>       Run a multi-agent scenario from JSON file (mocked LLM)
  live-multi-agent <file>  Run a multi-agent scenario with real Saturn LLM
  harness                  Run interactive harness mode
  help                     Show this help

Examples:
  brutus-test tools
  brutus-test tool read_file '{"path": "main.go"}'
  brutus-test tool list_files '{"path": ".", "recursive": false}'
  brutus-test tool code_search '{"pattern": "func main", "path": "."}'
  brutus-test scenario testdata/read-scenario.json
  brutus-test multi-agent testdata/multi-agent/multi-scenario.json
  brutus-test live-multi-agent -v testdata/multi-agent/live-scenario.json

Tool Input Formats:
  read_file:    {"path": "file/path"}
  list_files:   {"path": "dir/path", "recursive": true}
  edit_file:    {"path": "file", "old_str": "old", "new_str": "new"}
  bash:         {"command": "echo hello"}
  code_search:  {"pattern": "regex", "path": ".", "file_type": "go"}`)
}

func listTools() {
	runner := sdk.DefaultToolRunner()
	fmt.Println("Available tools:")
	fmt.Println()
	for _, name := range runner.ListTools() {
		tool, _ := runner.GetRegistry().Get(name)
		fmt.Printf("  %-15s %s\n", name, tool.Description)
	}
}

func runTool(args []string) {
	fs := flag.NewFlagSet("tool", flag.ExitOnError)
	verbose := fs.Bool("v", false, "Verbose output")
	fs.Parse(args)

	remaining := fs.Args()
	if len(remaining) < 2 {
		fmt.Println("Usage: brutus-test tool <name> <json-input>")
		fmt.Println("Example: brutus-test tool read_file '{\"path\": \"main.go\"}'")
		os.Exit(1)
	}

	toolName := remaining[0]
	inputJSON := strings.Join(remaining[1:], " ")

	runner := sdk.DefaultToolRunner()

	if *verbose {
		fmt.Printf("Executing tool: %s\n", toolName)
		fmt.Printf("Input: %s\n", inputJSON)
		fmt.Println("---")
	}

	result, err := runner.Execute(toolName, inputJSON)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}

	fmt.Println(result)
}

func runScenario(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: brutus-test scenario <file>")
		os.Exit(1)
	}

	filename := args[0]
	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Error reading scenario file: %s\n", err)
		os.Exit(1)
	}

	var scenario Scenario
	if err := json.Unmarshal(data, &scenario); err != nil {
		fmt.Printf("Error parsing scenario file: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Running scenario: %s\n", scenario.Name)
	fmt.Printf("Description: %s\n", scenario.Description)
	fmt.Println("---")

	harness := sdk.NewHarness().WithDefaultTools().WithVerbose(true)

	for _, resp := range scenario.MockResponses {
		if resp.Content != "" {
			harness.QueueTextResponse(resp.Content)
		} else if resp.ToolCall != "" {
			harness.QueueToolCall(resp.ToolCall, resp.Input)
		}
	}

	ctx := context.Background()
	for i, msg := range scenario.UserMessages {
		fmt.Printf("\n[%d] User: %s\n", i+1, msg)
		harness.SendUserMessage(msg)
		if err := harness.Run(ctx); err != nil {
			fmt.Printf("Error: %s\n", err)
			os.Exit(1)
		}
		fmt.Printf("[%d] Assistant: %s\n", i+1, harness.LastAssistantMessage())
	}

	fmt.Println("\n" + harness.Summary())

	for _, assertion := range scenario.Assertions {
		switch assertion.Type {
		case "tool_called":
			if !harness.ToolWasCalled(assertion.Value) {
				fmt.Printf("FAIL: Expected tool '%s' to be called\n", assertion.Value)
				os.Exit(1)
			}
			fmt.Printf("PASS: Tool '%s' was called\n", assertion.Value)
		case "contains":
			if err := harness.AssertConversationContains(assertion.Value); err != nil {
				fmt.Printf("FAIL: %s\n", err)
				os.Exit(1)
			}
			fmt.Printf("PASS: Conversation contains '%s'\n", assertion.Value)
		}
	}

	fmt.Println("\nScenario completed successfully!")
}

type Scenario struct {
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	UserMessages  []string       `json:"user_messages"`
	MockResponses []MockResponse `json:"mock_responses"`
	Assertions    []Assertion    `json:"assertions"`
}

type MockResponse struct {
	Content  string                 `json:"content,omitempty"`
	ToolCall string                 `json:"tool_call,omitempty"`
	Input    map[string]interface{} `json:"input,omitempty"`
}

type Assertion struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

func runHarness(args []string) {
	fs := flag.NewFlagSet("harness", flag.ExitOnError)
	verbose := fs.Bool("v", false, "Verbose output")
	fs.Parse(args)

	harness := sdk.NewHarness().WithDefaultTools().WithVerbose(*verbose)

	fmt.Println("Interactive Harness Mode")
	fmt.Println("Commands:")
	fmt.Println("  queue <text>              Queue a text response")
	fmt.Println("  queue-tool <name> <json>  Queue a tool call response")
	fmt.Println("  send <message>            Send user message and run")
	fmt.Println("  summary                   Show harness summary")
	fmt.Println("  reset                     Reset harness state")
	fmt.Println("  tools                     List available tools")
	fmt.Println("  exit                      Exit harness mode")
	fmt.Println()

	var input string
	for {
		fmt.Print("harness> ")
		_, err := fmt.Scanln(&input)
		if err != nil {
			continue
		}

		parts := strings.SplitN(input, " ", 2)
		cmd := parts[0]
		arg := ""
		if len(parts) > 1 {
			arg = parts[1]
		}

		switch cmd {
		case "queue":
			harness.QueueTextResponse(arg)
			fmt.Println("Queued text response")
		case "queue-tool":
			toolParts := strings.SplitN(arg, " ", 2)
			if len(toolParts) < 2 {
				fmt.Println("Usage: queue-tool <name> <json>")
				continue
			}
			var input map[string]interface{}
			if err := json.Unmarshal([]byte(toolParts[1]), &input); err != nil {
				fmt.Printf("Invalid JSON: %s\n", err)
				continue
			}
			harness.QueueToolCall(toolParts[0], input)
			fmt.Println("Queued tool call")
		case "send":
			harness.SendUserMessage(arg)
			ctx := context.Background()
			if err := harness.Run(ctx); err != nil {
				fmt.Printf("Error: %s\n", err)
				continue
			}
			fmt.Printf("Response: %s\n", harness.LastAssistantMessage())
		case "summary":
			fmt.Println(harness.Summary())
		case "reset":
			harness.Reset()
			fmt.Println("Harness reset")
		case "tools":
			for _, name := range harness.GetRegistry().Names() {
				t, _ := harness.GetRegistry().Get(name)
				fmt.Printf("  %-15s %s\n", name, t.Description)
			}
		case "exit":
			fmt.Println("Goodbye!")
			return
		default:
			fmt.Printf("Unknown command: %s\n", cmd)
		}
	}
}

func registerDefaultTools(registry *tools.Registry) {
	registry.Register(tools.ReadFileTool)
	registry.Register(tools.ListFilesTool)
	registry.Register(tools.EditFileTool)
	registry.Register(tools.BashTool)
	registry.Register(tools.CodeSearchTool)
}

func runMultiAgent(args []string) {
	fs := flag.NewFlagSet("multi-agent", flag.ExitOnError)
	concurrent := fs.Bool("concurrent", true, "Run agents concurrently")
	verbose := fs.Bool("v", false, "Verbose output")
	fs.Parse(args)

	remaining := fs.Args()
	if len(remaining) < 1 {
		fmt.Println("Usage: brutus-test multi-agent [flags] <file>")
		fmt.Println("Flags:")
		fmt.Println("  -concurrent  Run agents concurrently (default: true)")
		fmt.Println("  -v           Verbose output")
		os.Exit(1)
	}

	filename := remaining[0]
	scenario, err := sdk.LoadMultiAgentScenario(filename)
	if err != nil {
		fmt.Printf("Error loading scenario: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Running multi-agent scenario: %s\n", scenario.Name)
	fmt.Printf("Description: %s\n", scenario.Description)
	fmt.Printf("Agents: %d\n", len(scenario.Agents))
	fmt.Printf("Concurrent: %v\n", *concurrent)
	fmt.Println("---")

	harness := sdk.NewMultiAgentHarness().WithVerbose(*verbose)

	ctx := context.Background()
	results, err := harness.RunScenario(ctx, scenario, *concurrent)
	if err != nil {
		fmt.Printf("Error running scenario: %s\n", err)
		os.Exit(1)
	}

	fmt.Println("\n=== Results ===")
	for _, result := range results {
		status := "SUCCESS"
		if !result.Success {
			status = "FAILED"
		}
		fmt.Printf("\nAgent %s: %s (duration: %v)\n", result.AgentID, status, result.Duration)
		if result.Error != nil {
			fmt.Printf("  Error: %s\n", result.Error)
		}
		fmt.Printf("  Tool calls: %d\n", len(result.ToolCalls))
		if result.FinalMessage != "" {
			msg := result.FinalMessage
			if len(msg) > 200 {
				msg = msg[:200] + "..."
			}
			fmt.Printf("  Final message: %s\n", msg)
		}
	}

	fmt.Println("\n" + harness.Summary())

	if len(scenario.Assertions) > 0 {
		fmt.Println("=== Assertions ===")
		errors := harness.ValidateAssertions(results, scenario.Assertions)
		if len(errors) > 0 {
			for _, err := range errors {
				fmt.Printf("FAIL: %s\n", err)
			}
			os.Exit(1)
		}
		fmt.Println("All assertions passed!")
	}

	fmt.Println("\nMulti-agent scenario completed successfully!")
}

type LiveScenario struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Agents      []LiveAgentConfig `json:"agents"`
}

type LiveAgentConfig struct {
	ID           string `json:"id"`
	SystemPrompt string `json:"system_prompt"`
	InitialTask  string `json:"initial_task"`
}

func runLiveMultiAgent(args []string) {
	fs := flag.NewFlagSet("live-multi-agent", flag.ExitOnError)
	concurrent := fs.Bool("concurrent", true, "Run agents concurrently")
	verbose := fs.Bool("v", false, "Verbose output")
	timeout := fs.Int("timeout", 5, "Saturn discovery timeout in seconds")
	maxTurns := fs.Int("max-turns", 10, "Maximum turns per agent")
	model := fs.String("model", "", "Model to use (optional)")
	fs.Parse(args)

	remaining := fs.Args()
	if len(remaining) < 1 {
		fmt.Println("Usage: brutus-test live-multi-agent [flags] <file>")
		fmt.Println("\nFlags:")
		fmt.Println("  -concurrent   Run agents concurrently (default: true)")
		fmt.Println("  -v            Verbose output")
		fmt.Println("  -timeout      Saturn discovery timeout in seconds (default: 5)")
		fmt.Println("  -max-turns    Maximum turns per agent (default: 10)")
		fmt.Println("  -model        Model to use (optional)")
		fmt.Println("\nNote: Requires a Saturn beacon on the network!")
		os.Exit(1)
	}

	filename := remaining[0]
	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Error reading scenario file: %s\n", err)
		os.Exit(1)
	}

	var scenario LiveScenario
	if err := json.Unmarshal(data, &scenario); err != nil {
		fmt.Printf("Error parsing scenario file: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Running LIVE multi-agent scenario: %s\n", scenario.Name)
	fmt.Printf("Description: %s\n", scenario.Description)
	fmt.Printf("Agents: %d\n", len(scenario.Agents))
	fmt.Printf("Concurrent: %v\n", *concurrent)
	fmt.Printf("Max turns: %d\n", *maxTurns)
	fmt.Println("---")

	fmt.Println("\n\033[93mDiscovering Saturn services...\033[0m")

	saturnCfg := provider.SaturnConfig{
		DiscoveryTimeout: time.Duration(*timeout) * time.Second,
		Model:            *model,
	}

	harness := sdk.NewLiveMultiAgentHarness(saturnCfg).
		WithDefaultTools().
		WithMaxTurns(*maxTurns).
		WithVerbose(*verbose)

	var agentConfigs []sdk.LiveAgentConfig
	for _, a := range scenario.Agents {
		agentConfigs = append(agentConfigs, sdk.LiveAgentConfig{
			ID:           a.ID,
			SystemPrompt: a.SystemPrompt,
			InitialTask:  a.InitialTask,
		})
	}

	ctx := context.Background()
	var results []sdk.LiveAgentResult
	if *concurrent {
		results, err = harness.RunConcurrent(ctx, agentConfigs)
	} else {
		results, err = harness.RunSequential(ctx, agentConfigs)
	}

	if err != nil {
		fmt.Printf("Error running scenario: %s\n", err)
		os.Exit(1)
	}

	fmt.Println("\n=== Results ===")
	allSuccess := true
	for _, result := range results {
		status := "\033[92mSUCCESS\033[0m"
		if !result.Success {
			status = "\033[91mFAILED\033[0m"
			allSuccess = false
		}
		fmt.Printf("\nAgent %s: %s (duration: %v)\n", result.AgentID, status, result.Duration)
		if result.Error != nil {
			fmt.Printf("  Error: %s\n", result.Error)
		}
		fmt.Printf("  Tool calls: %d\n", len(result.ToolCalls))
		if result.FinalMessage != "" {
			msg := result.FinalMessage
			if len(msg) > 300 {
				msg = msg[:300] + "..."
			}
			fmt.Printf("  Final message: %s\n", msg)
		}
	}

	if allSuccess {
		fmt.Println("\n\033[92mLive multi-agent scenario completed successfully!\033[0m")
	} else {
		fmt.Println("\n\033[91mSome agents failed.\033[0m")
		os.Exit(1)
	}
}
