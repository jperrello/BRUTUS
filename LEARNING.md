# Learning Coding Agents with BRUTUS

This document explains how BRUTUS works and how to extend it.

## The Big Picture

A coding agent is **300 lines of code running in a loop with LLM tokens**.

That's it. There's no magic. The entire architecture is:

```
┌─────────────────────────────────────────────────────────────┐
│                        THE LOOP                              │
│                                                              │
│   ┌──────────┐    ┌──────────┐    ┌──────────┐              │
│   │  User    │───▶│   LLM    │───▶│  Tool?   │              │
│   │  Input   │    │ Inference│    │          │              │
│   └──────────┘    └──────────┘    └────┬─────┘              │
│                                        │                     │
│                         ┌──────────────┼──────────────┐      │
│                         │ Yes          │ No           │      │
│                         ▼              ▼              │      │
│                   ┌──────────┐   ┌──────────┐         │      │
│                   │ Execute  │   │  Show    │         │      │
│                   │  Tool    │   │ Response │         │      │
│                   └────┬─────┘   └──────────┘         │      │
│                        │                              │      │
│                        └──────────────────────────────┘      │
│                        (loop back to LLM with result)        │
└─────────────────────────────────────────────────────────────┘
```

## The Core Files

### `agent/agent.go` - THE LOOP

This is what you should read first. The `Run()` function implements the loop:

```go
for {
    // 1. Get user input
    userInput := getUserInput()

    // 2. Send to LLM
    response := provider.Chat(conversation, tools)

    // 3. Tool loop - keep going while LLM wants tools
    for len(response.ToolCalls) > 0 {
        // Execute each tool
        for _, tc := range response.ToolCalls {
            result := executeTool(tc)
            // Send result back to LLM
        }
        response = provider.Chat(conversation, tools)
    }

    // 4. Show response to user
    print(response.Content)
}
```

### `tools/` - What the Agent Can DO

Each file defines one capability:

| File | Tool | Purpose |
|------|------|---------|
| `read.go` | read_file | Look at files |
| `list.go` | list_files | Explore directories |
| `bash.go` | bash | Run commands |
| `edit.go` | edit_file | Modify code |
| `search.go` | code_search | Find patterns |

### `provider/` - Where the LLM Comes From

BRUTUS uses Saturn for AI access:

| File | Purpose |
|------|---------|
| `provider.go` | Provider interface |
| `discovery.go` | mDNS service discovery |
| `saturn.go` | OpenAI-compatible client |

## Learning Path

### Stage 1: Read `examples/01-chat/main.go`

A simple chatbot with no tools. Understand:
- How the Anthropic client works
- How conversations are structured
- The basic loop

### Stage 2: Read `examples/02-read-tool/main.go`

Adds the first tool. Understand:
- How tools are defined (JSON schema)
- How the LLM requests tool use
- The tool execution loop

### Stage 3: Read `examples/03-multi-tool/main.go`

Multiple tools working together. Understand:
- Tool registry pattern
- How tools chain together
- Error handling

### Stage 4: Read the Full BRUTUS Implementation

Now look at the production code:
1. `tools/tool.go` - The tool abstraction
2. `agent/agent.go` - The loop (with polish)
3. `provider/provider.go` - Provider interface
4. `main.go` - How it all connects

## How to Add a New Tool

1. **Create the input struct** in `tools/`:

```go
// tools/mytool.go
package tools

import "encoding/json"

type MyToolInput struct {
    Param1 string `json:"param1" jsonschema_description:"What this param does"`
    Param2 int    `json:"param2,omitempty" jsonschema_description:"Optional param"`
}
```

2. **Implement the function**:

```go
func MyTool(input json.RawMessage) (string, error) {
    var args MyToolInput
    if err := json.Unmarshal(input, &args); err != nil {
        return "", err
    }

    // Do the thing
    result := doSomething(args.Param1, args.Param2)

    return result, nil
}
```

3. **Create the definition**:

```go
var MyToolDef = NewTool[MyToolInput](
    "my_tool",                    // Name (what LLM calls)
    "Description for the LLM",    // Help LLM know when to use it
    MyTool,                       // Function to execute
)
```

4. **Register in main.go**:

```go
registry.Register(tools.MyToolDef)
```

That's it. The LLM will now be able to use your tool.

## Key Concepts

### Context Window

The LLM sees the entire conversation history. This includes:
- System prompt (from BRUTUS.md)
- All user messages
- All assistant responses
- All tool calls and results

**Less is more.** Too much context degrades performance.

### Tool Schemas

Tools are defined by JSON schemas. The LLM uses these to:
1. Know what tools exist
2. Know what parameters each tool needs
3. Generate valid tool calls

The schema is auto-generated from your Go structs using `jsonschema` tags.

### The Provider Abstraction

The provider interface separates "what the agent does" from "where the LLM comes from".
BRUTUS uses Saturn exclusively, but the abstraction means you could add other providers
if needed (e.g., direct API access for testing).

## Saturn Integration

BRUTUS uses Saturn exclusively for AI access:

```bash
# Just run it - Saturn discovery happens automatically
brutus

# With custom timeout
brutus -timeout=10s
```

When BRUTUS starts, it:
1. Searches for `_saturn._tcp.local.` services via mDNS
2. Gets service metadata (priority, credentials)
3. Connects to the best available service
4. Errors if no Saturn services found

This means: **network presence = AI access**. No API keys needed.

## Debugging

Run with `-verbose` to see:
- Tool calls being made
- API requests
- Response processing

```bash
brutus -verbose
```

## Philosophy

From Geoffrey Huntley's original article:

> "Understanding how to build agents is 2025's equivalent of understanding what a primary key does."

The point isn't to use BRUTUS as-is. It's to understand how it works so you can:
1. Build your own agents
2. Customize existing agents
3. Know what's really happening when you use Claude Code, Cursor, etc.

**Go forward and build.**
