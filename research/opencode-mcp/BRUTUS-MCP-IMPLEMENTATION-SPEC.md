# BRUTUS MCP Implementation Specification

**Research Date**: 2026-01-17
**Status**: Specification Only (No Implementation)
**Based On**: OpenCode MCP subsystem reverse engineering

---

## Purpose

This document specifies how BRUTUS should implement MCP (Model Context Protocol) support, translated from OpenCode's TypeScript implementation to Go idioms.

---

## Proposed Package Structure

```
brutus/
├── mcp/
│   ├── mcp.go           # Core MCP client manager
│   ├── transport.go     # Transport abstraction
│   ├── stdio.go         # Stdio transport implementation
│   ├── http.go          # HTTP transport (future)
│   ├── protocol.go      # JSON-RPC message types
│   ├── tool.go          # Tool type definitions
│   └── config.go        # MCP configuration types
```

---

## Core Types

### Configuration

```go
// MCPConfig represents a configured MCP server
type MCPConfig struct {
    Type        string            `json:"type"`        // "local" or "remote"
    Command     []string          `json:"command"`     // For local: ["cmd", "arg1", "arg2"]
    URL         string            `json:"url"`         // For remote
    Environment map[string]string `json:"environment"` // Extra env vars
    Headers     map[string]string `json:"headers"`     // For remote HTTP
    Enabled     *bool             `json:"enabled"`     // nil = enabled
    Timeout     time.Duration     `json:"timeout"`     // Connection timeout
}

func (c *MCPConfig) IsEnabled() bool {
    return c.Enabled == nil || *c.Enabled
}
```

### Connection Status

```go
type Status int

const (
    StatusConnected Status = iota
    StatusDisabled
    StatusFailed
    StatusNeedsAuth
)

type ConnectionStatus struct {
    Status Status
    Error  string // Non-empty if Status == StatusFailed
}
```

### JSON-RPC Messages

```go
type Request struct {
    JSONRPC string      `json:"jsonrpc"` // Always "2.0"
    ID      int64       `json:"id"`
    Method  string      `json:"method"`
    Params  interface{} `json:"params,omitempty"`
}

type Response struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      int64           `json:"id"`
    Result  json.RawMessage `json:"result,omitempty"`
    Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}

type Notification struct {
    JSONRPC string      `json:"jsonrpc"`
    Method  string      `json:"method"`
    Params  interface{} `json:"params,omitempty"`
}
```

### MCP Tool Definition

```go
type MCPTool struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    InputSchema json.RawMessage `json:"inputSchema"`
}

type ToolCallResult struct {
    Content []ContentBlock `json:"content"`
    IsError bool           `json:"isError"`
}

type ContentBlock struct {
    Type     string `json:"type"` // "text", "image", "resource"
    Text     string `json:"text,omitempty"`
    Data     string `json:"data,omitempty"`     // base64 for images
    MimeType string `json:"mimeType,omitempty"`
}
```

---

## Transport Interface

```go
type Transport interface {
    // Connect establishes the connection
    Connect(ctx context.Context) error

    // Send sends a request and waits for response
    Send(ctx context.Context, req *Request) (*Response, error)

    // OnNotification registers a callback for server notifications
    OnNotification(handler func(Notification))

    // Close terminates the connection
    Close() error
}
```

---

## Stdio Transport Implementation

```go
type StdioTransport struct {
    cmd     *exec.Cmd
    stdin   io.WriteCloser
    stdout  *bufio.Reader

    mu      sync.Mutex
    nextID  int64
    pending map[int64]chan *Response

    notifyHandler func(Notification)
    done          chan struct{}
}

func NewStdioTransport(command []string, env map[string]string, cwd string) *StdioTransport

func (t *StdioTransport) Connect(ctx context.Context) error {
    // 1. Start subprocess
    t.cmd = exec.CommandContext(ctx, t.command[0], t.command[1:]...)
    t.cmd.Env = append(os.Environ(), formatEnv(t.env)...)
    t.cmd.Dir = t.cwd

    // 2. Get stdin/stdout pipes
    t.stdin, _ = t.cmd.StdinPipe()
    stdout, _ := t.cmd.StdoutPipe()
    t.stdout = bufio.NewReader(stdout)

    // 3. Start process
    t.cmd.Start()

    // 4. Start reader goroutine
    go t.readLoop()

    return nil
}

func (t *StdioTransport) readLoop() {
    scanner := bufio.NewScanner(t.stdout)
    for scanner.Scan() {
        line := scanner.Bytes()

        // Try to parse as response (has "id" field)
        var resp Response
        if json.Unmarshal(line, &resp); resp.ID != 0 {
            t.mu.Lock()
            if ch, ok := t.pending[resp.ID]; ok {
                ch <- &resp
                delete(t.pending, resp.ID)
            }
            t.mu.Unlock()
            continue
        }

        // Parse as notification
        var notif Notification
        if json.Unmarshal(line, &notif); notif.Method != "" {
            if t.notifyHandler != nil {
                t.notifyHandler(notif)
            }
        }
    }
}

func (t *StdioTransport) Send(ctx context.Context, req *Request) (*Response, error) {
    t.mu.Lock()
    req.ID = t.nextID
    t.nextID++

    ch := make(chan *Response, 1)
    t.pending[req.ID] = ch
    t.mu.Unlock()

    // Encode and send
    data, _ := json.Marshal(req)
    t.stdin.Write(data)
    t.stdin.Write([]byte("\n"))

    // Wait for response with timeout
    select {
    case resp := <-ch:
        return resp, nil
    case <-ctx.Done():
        t.mu.Lock()
        delete(t.pending, req.ID)
        t.mu.Unlock()
        return nil, ctx.Err()
    }
}
```

---

## MCP Client

```go
type Client struct {
    name      string
    transport Transport
    status    ConnectionStatus
    tools     []MCPTool

    mu        sync.RWMutex
}

func NewClient(name string, transport Transport) *Client

func (c *Client) Connect(ctx context.Context) error {
    if err := c.transport.Connect(ctx); err != nil {
        c.status = ConnectionStatus{Status: StatusFailed, Error: err.Error()}
        return err
    }

    // Initialize connection
    resp, err := c.call(ctx, "initialize", map[string]interface{}{
        "protocolVersion": "2024-11-05",
        "capabilities": map[string]interface{}{
            "roots": map[string]interface{}{"listChanged": true},
        },
        "clientInfo": map[string]interface{}{
            "name":    "brutus",
            "version": "0.1.0",
        },
    })
    if err != nil {
        c.status = ConnectionStatus{Status: StatusFailed, Error: err.Error()}
        return err
    }

    // Discover tools
    if err := c.refreshTools(ctx); err != nil {
        c.status = ConnectionStatus{Status: StatusFailed, Error: err.Error()}
        return err
    }

    c.status = ConnectionStatus{Status: StatusConnected}
    return nil
}

func (c *Client) refreshTools(ctx context.Context) error {
    resp, err := c.call(ctx, "tools/list", nil)
    if err != nil {
        return err
    }

    var result struct {
        Tools []MCPTool `json:"tools"`
    }
    json.Unmarshal(resp.Result, &result)

    c.mu.Lock()
    c.tools = result.Tools
    c.mu.Unlock()

    return nil
}

func (c *Client) Tools() []MCPTool {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.tools
}

func (c *Client) CallTool(ctx context.Context, name string, args map[string]interface{}) (*ToolCallResult, error) {
    resp, err := c.call(ctx, "tools/call", map[string]interface{}{
        "name":      name,
        "arguments": args,
    })
    if err != nil {
        return nil, err
    }

    var result ToolCallResult
    json.Unmarshal(resp.Result, &result)
    return &result, nil
}

func (c *Client) call(ctx context.Context, method string, params interface{}) (*Response, error) {
    req := &Request{
        JSONRPC: "2.0",
        Method:  method,
        Params:  params,
    }
    return c.transport.Send(ctx, req)
}
```

---

## MCP Manager

```go
type Manager struct {
    clients map[string]*Client
    status  map[string]ConnectionStatus

    mu      sync.RWMutex
}

func NewManager() *Manager

func (m *Manager) Add(ctx context.Context, name string, cfg MCPConfig) error {
    if !cfg.IsEnabled() {
        m.mu.Lock()
        m.status[name] = ConnectionStatus{Status: StatusDisabled}
        m.mu.Unlock()
        return nil
    }

    var transport Transport
    switch cfg.Type {
    case "local":
        transport = NewStdioTransport(cfg.Command, cfg.Environment, ".")
    case "remote":
        return fmt.Errorf("remote MCP not yet implemented")
    default:
        return fmt.Errorf("unknown MCP type: %s", cfg.Type)
    }

    client := NewClient(name, transport)
    if err := client.Connect(ctx); err != nil {
        m.mu.Lock()
        m.status[name] = ConnectionStatus{Status: StatusFailed, Error: err.Error()}
        m.mu.Unlock()
        return err
    }

    m.mu.Lock()
    m.clients[name] = client
    m.status[name] = client.status
    m.mu.Unlock()

    return nil
}

func (m *Manager) AllTools() map[string]MCPTool {
    m.mu.RLock()
    defer m.mu.RUnlock()

    result := make(map[string]MCPTool)
    for clientName, client := range m.clients {
        if m.status[clientName].Status != StatusConnected {
            continue
        }
        for _, tool := range client.Tools() {
            // Key format: clientname_toolname
            key := sanitizeName(clientName) + "_" + sanitizeName(tool.Name)
            result[key] = tool
        }
    }
    return result
}

func sanitizeName(s string) string {
    return strings.Map(func(r rune) rune {
        if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
           (r >= '0' && r <= '9') || r == '_' || r == '-' {
            return r
        }
        return '_'
    }, s)
}
```

---

## Integration with BRUTUS Tool Registry

```go
// In tools/mcp_wrapper.go

func WrapMCPTool(manager *mcp.Manager, clientName, toolName string) tools.Tool {
    key := sanitizeName(clientName) + "_" + sanitizeName(toolName)
    mcpTool := manager.AllTools()[key]

    return tools.Tool{
        Name:        key,
        Description: mcpTool.Description,
        InputSchema: mcpTool.InputSchema,
        Execute: func(ctx context.Context, input json.RawMessage) (string, error) {
            var args map[string]interface{}
            json.Unmarshal(input, &args)

            result, err := manager.clients[clientName].CallTool(ctx, toolName, args)
            if err != nil {
                return "", err
            }

            // Convert content blocks to string
            var output strings.Builder
            for _, block := range result.Content {
                if block.Type == "text" {
                    output.WriteString(block.Text)
                }
            }

            if result.IsError {
                return "", fmt.Errorf("tool error: %s", output.String())
            }

            return output.String(), nil
        },
    }
}
```

---

## Configuration Loading

```go
// In config/mcp.go

type Config struct {
    MCP map[string]MCPConfig `json:"mcp"`
}

func LoadMCPConfig(path string) (map[string]MCPConfig, error) {
    // Load from .brutus/config.json or similar
    // Return map of server name -> config
}
```

---

## Initialization Sequence

```go
// In main.go or agent initialization

func initMCP(cfg *config.Config) (*mcp.Manager, error) {
    manager := mcp.NewManager()

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    for name, mcpCfg := range cfg.MCP {
        if err := manager.Add(ctx, name, mcpCfg); err != nil {
            log.Printf("Failed to connect MCP %s: %v", name, err)
            // Continue - don't fail entire startup
        }
    }

    return manager, nil
}
```

---

## Test Plan

### Unit Tests

1. **Protocol encoding**: Test JSON-RPC message serialization
2. **Stdio transport**: Mock subprocess communication
3. **Client initialization**: Test handshake flow
4. **Tool discovery**: Test parsing tool definitions
5. **Tool invocation**: Test argument passing and result parsing

### Integration Tests

1. **Real MCP server**: Start a simple MCP server, connect, call tools
2. **Timeout handling**: Verify timeout works correctly
3. **Error handling**: Test various error conditions

### Suggested Test MCP Server

Use `@modelcontextprotocol/server-everything` or write a simple Go MCP server for testing.

---

## Future Work (Not for Initial Implementation)

1. **Remote HTTP transport**: StreamableHTTP and SSE
2. **OAuth support**: For authenticated remote servers
3. **Resource support**: `resources/list` and `resources/read`
4. **Prompt support**: `prompts/list` and `prompts/get`
5. **Notification handling**: Tool list change events
6. **Dynamic server addition**: Add/remove servers at runtime

---

## References

- [MCP Specification](https://modelcontextprotocol.io/specification)
- [JSON-RPC 2.0 Specification](https://www.jsonrpc.org/specification)
- OpenCode implementation: `packages/opencode/src/mcp/`
