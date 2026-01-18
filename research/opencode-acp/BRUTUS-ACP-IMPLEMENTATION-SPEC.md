# BRUTUS ACP Implementation Specification

Specification for implementing Agent Client Protocol (ACP) support in BRUTUS.

## Goals

1. Enable BRUTUS to integrate with Zed, Neovim, JetBrains, and other ACP clients
2. Provide real-time file access including unsaved buffer state
3. Stream tool execution progress to editor UI
4. Support permission workflows

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                         BRUTUS                               │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────────┐     ┌──────────────────────────────┐  │
│  │   ACP Server     │────►│       Agent Loop             │  │
│  │  (JSON-RPC)      │     │  (agent/agent.go)            │  │
│  └────────┬─────────┘     └──────────────────────────────┘  │
│           │                            │                     │
│           │  ┌─────────────────────────┼──────────────┐     │
│           │  │                         ▼              │     │
│           │  │  ┌──────────────────────────────────┐ │     │
│           │  │  │          Tool Registry           │ │     │
│           ▼  │  └──────────────────────────────────┘ │     │
│  ┌──────────────────┐                                │     │
│  │  Session Manager │                                │     │
│  │  (state cache)   │                                │     │
│  └──────────────────┘                                │     │
│                                                      │     │
└──────────────────────────────────────────────────────┼─────┘
                                                       │
                              ┌────────────────────────┘
                              ▼
                    ┌──────────────────┐
                    │   ACP Client     │
                    │  (Zed, Neovim)   │
                    └──────────────────┘
```

## New Packages

### `acp/` - Protocol Implementation

```
acp/
├── server.go       # JSON-RPC server (stdio + websocket)
├── protocol.go     # Message types matching ACP schema
├── session.go      # Session state management
├── handler.go      # Method handlers
└── bridge.go       # Agent loop integration
```

## Type Definitions

### protocol.go

```go
package acp

// Protocol version
const ProtocolVersion = 1

// JSON-RPC base
type Request struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      interface{}     `json:"id,omitempty"`
    Method  string          `json:"method"`
    Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      interface{}     `json:"id,omitempty"`
    Result  interface{}     `json:"result,omitempty"`
    Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}

// Initialize
type InitializeParams struct {
    ProtocolVersion    int                `json:"protocolVersion"`
    ClientInfo         ClientInfo         `json:"clientInfo"`
    ClientCapabilities ClientCapabilities `json:"clientCapabilities"`
}

type ClientInfo struct {
    Name    string `json:"name"`
    Title   string `json:"title,omitempty"`
    Version string `json:"version,omitempty"`
}

type ClientCapabilities struct {
    FS       *FSCapabilities       `json:"fs,omitempty"`
    Terminal bool                  `json:"terminal,omitempty"`
    Prompt   *PromptCapabilities   `json:"prompt,omitempty"`
    MCP      *MCPCapabilities      `json:"mcp,omitempty"`
}

type FSCapabilities struct {
    ReadTextFile  bool `json:"readTextFile,omitempty"`
    WriteTextFile bool `json:"writeTextFile,omitempty"`
}

type PromptCapabilities struct {
    Image           bool `json:"image,omitempty"`
    Audio           bool `json:"audio,omitempty"`
    EmbeddedContext bool `json:"embeddedContext,omitempty"`
}

type MCPCapabilities struct {
    Transport *MCPTransport `json:"transport,omitempty"`
}

type MCPTransport struct {
    HTTP bool `json:"http,omitempty"`
    SSE  bool `json:"sse,omitempty"`
}

type InitializeResult struct {
    ProtocolVersion   int               `json:"protocolVersion"`
    AgentInfo         AgentInfo         `json:"agentInfo"`
    AgentCapabilities AgentCapabilities `json:"agentCapabilities"`
    AuthMethods       []string          `json:"authMethods,omitempty"`
}

type AgentInfo struct {
    Name    string `json:"name"`
    Title   string `json:"title,omitempty"`
    Version string `json:"version,omitempty"`
}

type AgentCapabilities struct {
    Modes bool `json:"modes,omitempty"`
    Plan  bool `json:"plan,omitempty"`
}

// Session
type NewSessionParams struct {
    Cwd        string      `json:"cwd"`
    Model      *ModelRef   `json:"model,omitempty"`
    ModeID     string      `json:"modeId,omitempty"`
    MCPServers []MCPServer `json:"mcpServers,omitempty"`
}

type ModelRef struct {
    ProviderID string `json:"providerID"`
    ModelID    string `json:"modelID"`
}

type MCPServer struct {
    Type        string            `json:"type"` // "local" or "remote"
    Command     string            `json:"command,omitempty"`
    Args        []string          `json:"args,omitempty"`
    Environment map[string]string `json:"environment,omitempty"`
    URL         string            `json:"url,omitempty"`
    Headers     map[string]string `json:"headers,omitempty"`
}

type NewSessionResult struct {
    SessionID     string `json:"sessionId"`
    Modes         []Mode `json:"modes,omitempty"`
    CurrentModeID string `json:"currentModeId,omitempty"`
}

type Mode struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
}

// Prompt
type PromptParams struct {
    SessionID string         `json:"sessionId"`
    Content   []ContentBlock `json:"content"`
}

type ContentBlock struct {
    Type     string `json:"type"` // text, image, resource_link, etc.
    Text     string `json:"text,omitempty"`
    Data     string `json:"data,omitempty"`
    MimeType string `json:"mimeType,omitempty"`
    URI      string `json:"uri,omitempty"`
    Name     string `json:"name,omitempty"`
}

type PromptResult struct {
    StopReason string `json:"stopReason"`
}

// Session Update (notification)
type SessionUpdate struct {
    SessionID    string        `json:"sessionId"`
    ContentChunk *ContentChunk `json:"contentChunk,omitempty"`
    ToolCall     *ToolCallInfo `json:"toolCall,omitempty"`
    Plan         *Plan         `json:"plan,omitempty"`
}

type ContentChunk struct {
    ID    string `json:"id"`
    Index int    `json:"index"`
    Delta string `json:"delta"`
}

type ToolCallInfo struct {
    ToolCallID string         `json:"toolCallId"`
    Title      string         `json:"title,omitempty"`
    Kind       string         `json:"kind,omitempty"`
    Status     string         `json:"status,omitempty"`
    Content    []ContentBlock `json:"content,omitempty"`
    Locations  []FileLocation `json:"locations,omitempty"`
    Diff       *Diff          `json:"diff,omitempty"`
}

type FileLocation struct {
    Path string `json:"path"`
    Line int    `json:"line,omitempty"`
}

type Diff struct {
    Path       string `json:"path"`
    OldContent string `json:"oldContent,omitempty"`
    NewContent string `json:"newContent,omitempty"`
}

type Plan struct {
    Steps []PlanStep `json:"steps"`
}

type PlanStep struct {
    Title  string `json:"title"`
    Status string `json:"status"` // pending, in_progress, completed
}

// Permission
type RequestPermissionParams struct {
    SessionID    string         `json:"sessionId"`
    PermissionID string         `json:"permissionId"`
    Title        string         `json:"title"`
    Kind         string         `json:"kind"`
    ToolCallID   string         `json:"toolCallId,omitempty"`
    Content      []ContentBlock `json:"content,omitempty"`
}

type PermissionResult struct {
    Decision  string `json:"decision"` // once, always, reject
    Cancelled bool   `json:"cancelled,omitempty"`
}

// File System
type ReadTextFileParams struct {
    SessionID string `json:"sessionId"`
    Path      string `json:"path"`
    Line      int    `json:"line,omitempty"`
    Limit     int    `json:"limit,omitempty"`
}

type ReadTextFileResult struct {
    Content string `json:"content"`
}

type WriteTextFileParams struct {
    SessionID string `json:"sessionId"`
    Path      string `json:"path"`
    Content   string `json:"content"`
}

// Terminal
type TerminalCreateParams struct {
    SessionID        string            `json:"sessionId"`
    Command          string            `json:"command"`
    Args             []string          `json:"args,omitempty"`
    Env              map[string]string `json:"env,omitempty"`
    Cwd              string            `json:"cwd,omitempty"`
    OutputBytesLimit int               `json:"outputBytesLimit,omitempty"`
}

type TerminalCreateResult struct {
    TerminalID string `json:"terminalId"`
}

type TerminalOutputParams struct {
    SessionID  string `json:"sessionId"`
    TerminalID string `json:"terminalId"`
}

type TerminalOutputResult struct {
    Output   string `json:"output"`
    ExitCode *int   `json:"exitCode,omitempty"`
    Signal   string `json:"signal,omitempty"`
}
```

### session.go

```go
package acp

import (
    "sync"
    "time"
)

type SessionState struct {
    ID         string
    Cwd        string
    MCPServers []MCPServer
    Model      *ModelRef
    ModeID     string
    CreatedAt  time.Time
}

type SessionManager struct {
    mu       sync.RWMutex
    sessions map[string]*SessionState
}

func NewSessionManager() *SessionManager {
    return &SessionManager{
        sessions: make(map[string]*SessionState),
    }
}

func (m *SessionManager) Create(cwd string, model *ModelRef, modeID string) *SessionState {
    m.mu.Lock()
    defer m.mu.Unlock()

    id := generateSessionID() // Use existing ID generation
    state := &SessionState{
        ID:        id,
        Cwd:       cwd,
        Model:     model,
        ModeID:    modeID,
        CreatedAt: time.Now(),
    }
    m.sessions[id] = state
    return state
}

func (m *SessionManager) Get(id string) (*SessionState, bool) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    s, ok := m.sessions[id]
    return s, ok
}

func (m *SessionManager) SetModel(id string, model *ModelRef) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    s, ok := m.sessions[id]
    if !ok {
        return ErrSessionNotFound
    }
    s.Model = model
    return nil
}

func (m *SessionManager) SetMode(id string, modeID string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    s, ok := m.sessions[id]
    if !ok {
        return ErrSessionNotFound
    }
    s.ModeID = modeID
    return nil
}
```

### handler.go

```go
package acp

type Handler struct {
    sessions *SessionManager
    agent    *agent.Agent  // From agent/agent.go
    conn     *Connection   // JSON-RPC connection
    caps     *ClientCapabilities
}

func (h *Handler) Initialize(params *InitializeParams) (*InitializeResult, error) {
    h.caps = &params.ClientCapabilities

    return &InitializeResult{
        ProtocolVersion: ProtocolVersion,
        AgentInfo: AgentInfo{
            Name:    "brutus",
            Title:   "BRUTUS",
            Version: version.Version,
        },
        AgentCapabilities: AgentCapabilities{
            Modes: true,
            Plan:  true,
        },
        AuthMethods: []string{"none"},
    }, nil
}

func (h *Handler) NewSession(params *NewSessionParams) (*NewSessionResult, error) {
    state := h.sessions.Create(params.Cwd, params.Model, params.ModeID)

    return &NewSessionResult{
        SessionID: state.ID,
        Modes: []Mode{
            {ID: "code", Name: "Code", Description: "Write and edit code"},
            {ID: "plan", Name: "Plan", Description: "Plan before implementing"},
        },
        CurrentModeID: state.ModeID,
    }, nil
}

func (h *Handler) Prompt(params *PromptParams) (*PromptResult, error) {
    session, ok := h.sessions.Get(params.SessionID)
    if !ok {
        return nil, ErrSessionNotFound
    }

    // Extract text content
    var text string
    for _, block := range params.Content {
        if block.Type == "text" {
            text = block.Text
            break
        }
    }

    // Create event handler for streaming updates
    updates := make(chan ToolUpdate)
    go h.streamUpdates(params.SessionID, updates)

    // Run agent loop
    result, err := h.agent.Run(text, session.Cwd, updates)
    if err != nil {
        return &PromptResult{StopReason: "tool_error"}, nil
    }

    return &PromptResult{StopReason: "end_turn"}, nil
}

func (h *Handler) streamUpdates(sessionID string, updates <-chan ToolUpdate) {
    for update := range updates {
        h.conn.Notify("session/update", &SessionUpdate{
            SessionID: sessionID,
            ToolCall: &ToolCallInfo{
                ToolCallID: update.ID,
                Title:      update.Name,
                Kind:       mapToolKind(update.Name),
                Status:     update.Status,
                Content:    update.Content,
            },
        })
    }
}

func mapToolKind(toolName string) string {
    switch toolName {
    case "bash":
        return "execute"
    case "read_file":
        return "read"
    case "edit_file", "patch_file":
        return "edit"
    case "list_files", "code_search":
        return "search"
    default:
        return "other"
    }
}
```

### server.go

```go
package acp

import (
    "bufio"
    "encoding/json"
    "io"
    "os"
)

type Server struct {
    handler *Handler
    input   io.Reader
    output  io.Writer
}

func NewStdioServer(agent *agent.Agent) *Server {
    return &Server{
        handler: &Handler{
            sessions: NewSessionManager(),
            agent:    agent,
        },
        input:  os.Stdin,
        output: os.Stdout,
    }
}

func (s *Server) Run() error {
    scanner := bufio.NewScanner(s.input)

    for scanner.Scan() {
        line := scanner.Bytes()

        var req Request
        if err := json.Unmarshal(line, &req); err != nil {
            s.sendError(nil, -32700, "Parse error")
            continue
        }

        result, err := s.dispatch(&req)
        if err != nil {
            s.sendError(req.ID, -32603, err.Error())
            continue
        }

        s.sendResult(req.ID, result)
    }

    return scanner.Err()
}

func (s *Server) dispatch(req *Request) (interface{}, error) {
    switch req.Method {
    case "initialize":
        var params InitializeParams
        json.Unmarshal(req.Params, &params)
        return s.handler.Initialize(&params)

    case "session/new":
        var params NewSessionParams
        json.Unmarshal(req.Params, &params)
        return s.handler.NewSession(&params)

    case "session/prompt":
        var params PromptParams
        json.Unmarshal(req.Params, &params)
        return s.handler.Prompt(&params)

    // ... other methods
    }
    return nil, fmt.Errorf("method not found: %s", req.Method)
}

func (s *Server) sendResult(id interface{}, result interface{}) {
    resp := Response{
        JSONRPC: "2.0",
        ID:      id,
        Result:  result,
    }
    data, _ := json.Marshal(resp)
    s.output.Write(data)
    s.output.Write([]byte("\n"))
}

func (s *Server) sendError(id interface{}, code int, msg string) {
    resp := Response{
        JSONRPC: "2.0",
        ID:      id,
        Error:   &RPCError{Code: code, Message: msg},
    }
    data, _ := json.Marshal(resp)
    s.output.Write(data)
    s.output.Write([]byte("\n"))
}

func (s *Server) Notify(method string, params interface{}) {
    req := Request{
        JSONRPC: "2.0",
        Method:  method,
    }
    if params != nil {
        data, _ := json.Marshal(params)
        req.Params = data
    }
    out, _ := json.Marshal(req)
    s.output.Write(out)
    s.output.Write([]byte("\n"))
}
```

## Integration Points

### Agent Loop Modification

The agent loop needs to emit events for ACP streaming:

```go
// In agent/agent.go

type ToolUpdate struct {
    ID      string
    Name    string
    Status  string  // pending, in_progress, completed, failed
    Content []acp.ContentBlock
}

func (a *Agent) Run(prompt string, cwd string, updates chan<- ToolUpdate) (*Result, error) {
    // ... existing loop

    for _, call := range toolCalls {
        // Emit pending
        updates <- ToolUpdate{
            ID:     call.ID,
            Name:   call.Name,
            Status: "pending",
        }

        // Emit in_progress
        updates <- ToolUpdate{
            ID:     call.ID,
            Name:   call.Name,
            Status: "in_progress",
        }

        result := executeTool(call)

        // Emit completed
        updates <- ToolUpdate{
            ID:      call.ID,
            Name:    call.Name,
            Status:  "completed",
            Content: resultToContent(result),
        }
    }
}
```

### Entry Point

Add ACP mode to main.go:

```go
func main() {
    if os.Getenv("BRUTUS_ACP") == "1" || hasArg("--acp") {
        // Run as ACP server (stdio mode)
        agent := agent.New(registry, provider)
        server := acp.NewStdioServer(agent)
        if err := server.Run(); err != nil {
            log.Fatal(err)
        }
        return
    }

    // Normal TUI mode
    // ...
}
```

## Client Configuration

### Zed

In Zed's settings.json:
```json
{
  "agents": {
    "brutus": {
      "command": "brutus",
      "args": ["--acp"]
    }
  }
}
```

### JetBrains

In AI Assistant settings, add custom agent:
- Command: `brutus --acp`
- Protocol: ACP

## Testing

### Manual Testing

```bash
# Start in ACP mode
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1,"clientInfo":{"name":"test"},"clientCapabilities":{}}}' | ./brutus --acp
```

### Automated Testing

```go
func TestACPInitialize(t *testing.T) {
    in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1,"clientInfo":{"name":"test"},"clientCapabilities":{}}}`)
    out := &bytes.Buffer{}

    server := &Server{input: in, output: out}
    server.Run()

    var resp Response
    json.Unmarshal(out.Bytes(), &resp)

    result := resp.Result.(*InitializeResult)
    assert.Equal(t, 1, result.ProtocolVersion)
    assert.Equal(t, "brutus", result.AgentInfo.Name)
}
```

## Implementation Phases

### Phase 1: Core Protocol
- [ ] JSON-RPC server (stdio)
- [ ] Initialize handshake
- [ ] Session management (new/load)
- [ ] Basic prompt handling

### Phase 2: Streaming
- [ ] session/update notifications
- [ ] Tool call status streaming
- [ ] Text content streaming

### Phase 3: Client Features
- [ ] fs/read_text_file (if client supports)
- [ ] fs/write_text_file (if client supports)
- [ ] Permission requests

### Phase 4: Terminal
- [ ] terminal/create
- [ ] terminal/output
- [ ] terminal/wait_for_exit
- [ ] terminal/kill/release

### Phase 5: Advanced
- [ ] WebSocket transport
- [ ] Mode switching
- [ ] Session persistence
- [ ] MCP server integration
