# Adding New Tools to BRUTUS

This guide walks you through adding a new tool to BRUTUS.

## Quick Reference

```go
// 1. Define input struct with jsonschema tags
type MyInput struct {
    Required string `json:"required" jsonschema_description:"A required parameter"`
    Optional string `json:"optional,omitempty" jsonschema_description:"An optional parameter"`
}

// 2. Implement the function
func MyFunction(input json.RawMessage) (string, error) {
    var args MyInput
    if err := json.Unmarshal(input, &args); err != nil {
        return "", err
    }
    // Do work, return result
    return "result", nil
}

// 3. Create tool definition
var MyTool = NewTool[MyInput](
    "my_tool",
    "Description the LLM will see",
    MyFunction,
)

// 4. Register in main.go
registry.Register(tools.MyTool)
```

## Full Example: Weather Tool

Let's add a tool that gets weather information.

### Step 1: Create `tools/weather.go`

```go
package tools

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
)

type WeatherInput struct {
    City string `json:"city" jsonschema_description:"City name to get weather for"`
}

func Weather(input json.RawMessage) (string, error) {
    var args WeatherInput
    if err := json.Unmarshal(input, &args); err != nil {
        return "", err
    }

    // Using wttr.in for simplicity (no API key needed)
    url := fmt.Sprintf("https://wttr.in/%s?format=%%l:+%%c+%%t", args.City)
    resp, err := http.Get(url)
    if err != nil {
        return "", fmt.Errorf("failed to get weather: %w", err)
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    return string(body), nil
}

var WeatherTool = NewTool[WeatherInput](
    "weather",
    "Get current weather for a city",
    Weather,
)
```

### Step 2: Register in `main.go`

```go
// In main()
registry.Register(tools.WeatherTool)
```

### Step 3: Test it

```
You: What's the weather in Seattle?
[tool] weather
[result] Seattle: ⛅ +12°C
BRUTUS: The current weather in Seattle is partly cloudy with a temperature of 12°C.
```

## Tool Design Guidelines

### 1. Clear Descriptions

The description helps the LLM know when to use your tool:

```go
// Bad - too vague
"Do stuff with files"

// Good - specific about capability
"Read the contents of a file at the given path. Returns the full file content as text."
```

### 2. Meaningful Parameter Names

Use descriptive names and descriptions:

```go
// Bad
type Input struct {
    S string `json:"s"`
}

// Good
type Input struct {
    Pattern string `json:"pattern" jsonschema_description:"Regex pattern to search for"`
}
```

### 3. Handle Errors Gracefully

Return useful error messages:

```go
func MyTool(input json.RawMessage) (string, error) {
    // ...
    if err != nil {
        // Include context about what failed
        return "", fmt.Errorf("failed to connect to database: %w", err)
    }
}
```

### 4. Limit Output Size

Large outputs hurt performance:

```go
func MyTool(input json.RawMessage) (string, error) {
    result := getSomeLargeOutput()

    // Truncate if too large
    if len(result) > 10000 {
        result = result[:10000] + "\n... (truncated)"
    }

    return result, nil
}
```

### 5. Make Tools Composable

Design tools to work together:

- `list_files` finds files → `read_file` reads them
- `code_search` finds locations → `edit_file` modifies them
- `bash` can verify changes after `edit_file`

## Common Tool Patterns

### File Operations

```go
type FileInput struct {
    Path string `json:"path" jsonschema_description:"File path"`
}
```

### Command Execution

```go
type CmdInput struct {
    Command string `json:"command" jsonschema_description:"Command to run"`
    Args    []string `json:"args,omitempty" jsonschema_description:"Command arguments"`
}
```

### Search Operations

```go
type SearchInput struct {
    Query string `json:"query" jsonschema_description:"Search query"`
    Limit int    `json:"limit,omitempty" jsonschema_description:"Max results (default 10)"`
}
```

### API Calls

```go
type APIInput struct {
    Endpoint string            `json:"endpoint" jsonschema_description:"API endpoint"`
    Method   string            `json:"method,omitempty" jsonschema_description:"HTTP method (default GET)"`
    Body     map[string]string `json:"body,omitempty" jsonschema_description:"Request body"`
}
```

## Testing Your Tool

### Unit Test

```go
// tools/weather_test.go
func TestWeather(t *testing.T) {
    input := json.RawMessage(`{"city": "London"}`)
    result, err := Weather(input)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result == "" {
        t.Fatal("expected non-empty result")
    }
}
```

### Integration Test

Run BRUTUS and try using your tool:

```bash
go build && ./brutus
> What's the weather in Tokyo?
```

## The Five Essential Tools

Every coding agent needs these five capabilities (from ghuntley):

| Tool | Purpose | Why Essential |
|------|---------|---------------|
| read_file | See files | Can't modify what you can't see |
| list_files | Navigate | Need to find files first |
| bash | Execute | Run builds, tests, commands |
| edit_file | Modify | Actually make changes |
| code_search | Find | Locate code patterns efficiently |

BRUTUS includes all five. Add more to extend its capabilities.
