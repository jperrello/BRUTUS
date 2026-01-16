package agent

import (
	"context"
	"fmt"
	"log"
	"strings"

	"brutus/provider"
	"brutus/tools"
)

// Agent is the core of BRUTUS - it runs THE LOOP.
//
// The agent loop is the heart of any coding agent. It's surprisingly simple:
//
//	1. Get user input
//	2. Send to LLM for inference
//	3. Check if LLM wants to use a tool
//	4. If yes: execute tool, send result back to LLM, goto 3
//	5. If no: show response to user, goto 1
//
// That's it. 300 lines of code running in a loop with LLM tokens.
// Everything else is just tools (what the agent CAN do) and prompts (HOW it behaves).
type Agent struct {
	provider     provider.Provider
	getUserInput func() (string, bool)
	tools        *tools.Registry
	systemPrompt string
	verbose      bool
	workingDir   string
}

// Config holds agent configuration.
type Config struct {
	Provider     provider.Provider
	GetUserInput func() (string, bool)
	Tools        *tools.Registry
	SystemPrompt string
	Verbose      bool
	WorkingDir   string
}

// New creates a new Agent with the given configuration.
func New(cfg Config) *Agent {
	return &Agent{
		provider:     cfg.Provider,
		getUserInput: cfg.GetUserInput,
		tools:        cfg.Tools,
		systemPrompt: cfg.SystemPrompt,
		verbose:      cfg.Verbose,
		workingDir:   cfg.WorkingDir,
	}
}

// Run starts the agent loop.
// This is THE function to understand. Everything else supports this loop.
func (a *Agent) Run(ctx context.Context) error {
	var conversation []provider.Message

	a.printBanner()

	// THE LOOP - this runs until the user exits
	for {
		// Step 1: Get user input
		fmt.Print("\033[94mYou\033[0m: ")
		userInput, ok := a.getUserInput()
		if !ok {
			a.log("User input stream ended")
			break
		}

		userInput = strings.TrimSpace(userInput)
		if userInput == "" {
			continue
		}
		if userInput == "quit" || userInput == "exit" {
			fmt.Println("\033[90mGoodbye!\033[0m")
			break
		}

		a.log("User: %q", userInput)

		// Add user message to conversation
		conversation = append(conversation, provider.Message{
			Role:    "user",
			Content: userInput,
		})

		// Step 2: Send to LLM for inference
		response, err := a.provider.Chat(ctx, a.systemPrompt, conversation, a.tools.All())
		if err != nil {
			return fmt.Errorf("inference failed: %w", err)
		}

		// Add assistant response to conversation
		conversation = append(conversation, response)

		// Step 3-4: Tool loop - keep going while LLM wants to use tools
		for len(response.ToolCalls) > 0 {
			a.log("Processing %d tool calls", len(response.ToolCalls))

			var toolResults []provider.ToolResult

			// Execute each tool the LLM requested
			for _, tc := range response.ToolCalls {
				fmt.Printf("\033[96m[tool]\033[0m %s\n", tc.Name)

				result, toolErr := a.executeTool(tc)

				// Show truncated result to user
				displayResult := result
				if len(displayResult) > 500 {
					displayResult = displayResult[:500] + "..."
				}
				fmt.Printf("\033[92m[result]\033[0m %s\n", displayResult)

				if toolErr != nil {
					fmt.Printf("\033[91m[error]\033[0m %s\n", toolErr.Error())
					result = toolErr.Error()
				}

				toolResults = append(toolResults, provider.ToolResult{
					ID:      tc.ID,
					Content: result,
					IsError: toolErr != nil,
				})
			}

			// Send tool results back to LLM
			conversation = append(conversation, provider.Message{
				Role:        "user",
				ToolResults: toolResults,
			})

			// Get next response (might request more tools)
			response, err = a.provider.Chat(ctx, a.systemPrompt, conversation, a.tools.All())
			if err != nil {
				return fmt.Errorf("inference failed: %w", err)
			}
			conversation = append(conversation, response)
		}

		// Step 5: Show text response to user
		if response.Content != "" {
			fmt.Printf("\033[93mBRUTUS\033[0m: %s\n", response.Content)
		}
		fmt.Println()
	}

	return nil
}

// executeTool runs a tool and returns its result.
func (a *Agent) executeTool(tc provider.ToolCall) (string, error) {
	tool, ok := a.tools.Get(tc.Name)
	if !ok {
		return "", fmt.Errorf("tool '%s' not found", tc.Name)
	}

	a.log("Executing tool: %s", tc.Name)
	result, err := tool.Function(tc.Input)
	if err != nil {
		a.log("Tool error: %v", err)
	} else {
		a.log("Tool success, result length: %d", len(result))
	}

	return result, err
}

func (a *Agent) log(format string, args ...interface{}) {
	if a.verbose {
		log.Printf(format, args...)
	}
}

func (a *Agent) printBanner() {
	fmt.Println("\033[1;35m" + `
 ____  ____  _     _____  _     ____
/  _ \/  __\/ \ /\/__ __\/ \ /\/ ___\
| | //|  \/|| | ||  / \  | | |||    \
| |_\\|    /| \_/|  | |  | \_/|\___ |
\____/\_/\_\\____/  \_/  \____/\____/
` + "\033[0m")
	fmt.Println("\033[1;33mCoding Agent\033[0m")
	if a.workingDir != "" {
		fmt.Printf("\033[90mWorking in: %s\033[0m\n", a.workingDir)
	}
	fmt.Println("\033[90mType 'quit' or 'exit' to end session\033[0m")
	fmt.Println()
}
