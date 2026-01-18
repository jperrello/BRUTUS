# BRUTUS Tool Implementation Specification

This document provides implementation guidance for building the Tool subsystem in BRUTUS based on reverse-engineering OpenCode.

## Architecture Mapping

### OpenCode → BRUTUS

| OpenCode | BRUTUS Equivalent |
|----------|-------------------|
| `Tool.Info` | `tools.Tool` interface |
| `Tool.Context` | `tools.Context` struct |
| `ToolRegistry` | `tools.Registry` |
| `Truncate` namespace | `tools/truncate` package |

## Core Types (Go)

### Tool Interface

```go
package tools

import "context"

type Tool interface {
    ID() string
    Description() string
    Schema() any  // JSON Schema for parameters
    Execute(ctx Context, params map[string]any) (*Result, error)
}

type Context struct {
    Ctx        context.Context
    SessionID  string
    MessageID  string
    Agent      string
    Extra      map[string]any

    // Callbacks
    OnMetadata func(title string, metadata map[string]any)
    OnAsk      func(req PermissionRequest) error
}

type Result struct {
    Title       string
    Output      string
    Metadata    map[string]any
    Attachments []Attachment
}

type Attachment struct {
    ID        string
    Type      string  // "file"
    Mime      string
    URL       string  // data: URL or file path
}
```

### Tool Factory Pattern

Match OpenCode's lazy init pattern:

```go
type ToolFactory func(initCtx *InitContext) (Tool, error)

type InitContext struct {
    Agent *AgentInfo
}

// Registration
func RegisterFactory(id string, factory ToolFactory)

// Resolution
func GetTools(providerID string, agent *AgentInfo) ([]Tool, error)
```

## Registry Implementation

```go
package tools

type Registry struct {
    mu       sync.RWMutex
    builtins []ToolFactory
    custom   []ToolFactory
}

func NewRegistry() *Registry {
    r := &Registry{}
    r.registerBuiltins()
    return r
}

func (r *Registry) registerBuiltins() {
    r.builtins = []ToolFactory{
        NewInvalidTool,
        NewBashTool,
        NewReadTool,
        NewGlobTool,
        NewGrepTool,
        NewEditTool,
        NewWriteTool,
        NewTaskTool,
        // ...
    }
}

func (r *Registry) Register(factory ToolFactory) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.custom = append(r.custom, factory)
}

func (r *Registry) Tools(ctx *InitContext) ([]Tool, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    var tools []Tool
    for _, factory := range append(r.builtins, r.custom...) {
        tool, err := factory(ctx)
        if err != nil {
            continue  // Skip failed init
        }
        tools = append(tools, tool)
    }
    return tools, nil
}
```

## Validation Pattern

Use JSON Schema validation similar to OpenCode's Zod:

```go
import "github.com/santhosh-tekuri/jsonschema/v5"

type ToolBase struct {
    id          string
    description string
    schema      *jsonschema.Schema
    execute     func(Context, map[string]any) (*Result, error)
}

func (t *ToolBase) Execute(ctx Context, params map[string]any) (*Result, error) {
    // Validate
    if err := t.schema.Validate(params); err != nil {
        return nil, fmt.Errorf("invalid arguments for %s: %w", t.id, err)
    }
    return t.execute(ctx, params)
}
```

## Truncation Module

```go
package truncate

const (
    MaxLines = 2000
    MaxBytes = 50 * 1024
    RetentionDays = 7
)

type Result struct {
    Content    string
    Truncated  bool
    OutputPath string  // Only set if Truncated
}

type Options struct {
    MaxLines  int
    MaxBytes  int
    Direction string  // "head" or "tail"
}

func Output(text string, opts Options, hasTaskTool bool) (*Result, error) {
    lines := strings.Split(text, "\n")
    totalBytes := len(text)

    maxLines := opts.MaxLines
    if maxLines == 0 {
        maxLines = MaxLines
    }
    maxBytes := opts.MaxBytes
    if maxBytes == 0 {
        maxBytes = MaxBytes
    }

    if len(lines) <= maxLines && totalBytes <= maxBytes {
        return &Result{Content: text, Truncated: false}, nil
    }

    // Collect lines within limits
    var out []string
    bytes := 0
    hitBytes := false
    // ... (implement head/tail logic)

    // Save full output
    path := saveOutput(text)

    // Build hint
    hint := buildHint(path, hasTaskTool)

    // Format message
    preview := strings.Join(out, "\n")
    removed := calculateRemoved(lines, out, hitBytes, totalBytes, bytes)
    unit := "lines"
    if hitBytes {
        unit = "bytes"
    }

    var msg string
    if opts.Direction == "tail" {
        msg = fmt.Sprintf("...%d %s truncated...\n\n%s\n\n%s", removed, unit, hint, preview)
    } else {
        msg = fmt.Sprintf("%s\n\n...%d %s truncated...\n\n%s", preview, removed, unit, hint)
    }

    return &Result{Content: msg, Truncated: true, OutputPath: path}, nil
}
```

## Fuzzy Replacement

Port the replacer cascade:

```go
package edit

type Replacer func(content, find string) []string

var replacers = []Replacer{
    simpleReplacer,
    lineTrimmedReplacer,
    blockAnchorReplacer,
    whitespaceNormalizedReplacer,
    indentationFlexibleReplacer,
    escapeNormalizedReplacer,
    trimmedBoundaryReplacer,
    contextAwareReplacer,
    multiOccurrenceReplacer,
}

func Replace(content, oldStr, newStr string, replaceAll bool) (string, error) {
    if oldStr == newStr {
        return "", errors.New("oldString and newString must be different")
    }

    notFound := true

    for _, replacer := range replacers {
        for _, search := range replacer(content, oldStr) {
            idx := strings.Index(content, search)
            if idx == -1 {
                continue
            }
            notFound = false

            if replaceAll {
                return strings.ReplaceAll(content, search, newStr), nil
            }

            lastIdx := strings.LastIndex(content, search)
            if idx != lastIdx {
                continue  // Multiple matches, try next replacer
            }

            return content[:idx] + newStr + content[idx+len(search):], nil
        }
    }

    if notFound {
        return "", errors.New("oldString not found in content")
    }
    return "", errors.New("multiple matches found - provide more context")
}
```

## Permission Integration

```go
type PermissionRequest struct {
    Permission string            // "read", "edit", "bash", etc.
    Patterns   []string          // Specific patterns for this call
    Always     []string          // Patterns for "always allow"
    Metadata   map[string]any    // Context for user
}

// In tool execution
func (t *ReadTool) Execute(ctx Context, params map[string]any) (*Result, error) {
    path := params["filePath"].(string)

    // Request permission
    if err := ctx.OnAsk(PermissionRequest{
        Permission: "read",
        Patterns:   []string{path},
        Always:     []string{"*"},
        Metadata:   map[string]any{},
    }); err != nil {
        return nil, err
    }

    // Proceed with read...
}
```

## Metadata Streaming

```go
// In bash tool
func (t *BashTool) Execute(ctx Context, params map[string]any) (*Result, error) {
    cmd := exec.CommandContext(ctx.Ctx, "bash", "-c", params["command"].(string))

    var output strings.Builder

    stdout, _ := cmd.StdoutPipe()
    stderr, _ := cmd.StderrPipe()

    cmd.Start()

    // Stream output
    go func() {
        buf := make([]byte, 1024)
        for {
            n, err := stdout.Read(buf)
            if n > 0 {
                output.Write(buf[:n])
                ctx.OnMetadata("", map[string]any{
                    "output":      output.String()[:min(len(output.String()), 30000)],
                    "description": params["description"],
                })
            }
            if err != nil {
                break
            }
        }
    }()
    // Similar for stderr...

    cmd.Wait()

    return &Result{
        Title:  params["description"].(string),
        Output: output.String(),
        Metadata: map[string]any{
            "output":      output.String(),
            "exit":        cmd.ProcessState.ExitCode(),
            "description": params["description"],
        },
    }, nil
}
```

## External Directory Check

```go
package tools

func AssertExternalDirectory(ctx Context, target string, bypass bool) error {
    if target == "" || bypass {
        return nil
    }

    if containsPath(projectRoot, target) {
        return nil
    }

    parentDir := filepath.Dir(target)
    glob := filepath.Join(parentDir, "*")

    return ctx.OnAsk(PermissionRequest{
        Permission: "external_directory",
        Patterns:   []string{glob},
        Always:     []string{glob},
        Metadata: map[string]any{
            "filepath":  target,
            "parentDir": parentDir,
        },
    })
}
```

## Priority Implementation Order

1. **Core Tool Interface** - Base types and registry
2. **Read Tool** - Simple, foundational
3. **Bash Tool** - Complex permissions, streaming
4. **Edit Tool** - Fuzzy matching critical for usability
5. **Truncation** - Needed for all tools
6. **Task Tool** - Subagent support

## Testing Strategy

1. **Unit tests** for each replacer strategy
2. **Integration tests** for permission flow
3. **Fuzzing** the edit tool with malformed inputs
4. **Benchmark** truncation performance on large outputs

## Key Differences from BRUTUS Current State

Based on BRUTUS codebase:
- BRUTUS uses `tools.NewTool[T]` generic factory
- OpenCode uses `Tool.define()` with Zod schemas
- BRUTUS should add:
  - Automatic truncation wrapper
  - Fuzzy edit matching (currently uses exact match only)
  - Permission ask callbacks
  - Metadata streaming
