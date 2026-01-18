# ACP Protocol Specification

Reverse-engineered from agentclientprotocol/agent-client-protocol and OpenCode implementation.

## Transport Layer

ACP uses **JSON-RPC 2.0** over stdio or WebSocket. Messages are newline-delimited JSON.

```
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{...}}
{"jsonrpc":"2.0","id":1,"result":{...}}
{"jsonrpc":"2.0","method":"session/update","params":{...}}  // notification (no id)
```

## Protocol Lifecycle

```
┌─────────────────────────────────────────────────────────────────┐
│                        INITIALIZATION                           │
├─────────────────────────────────────────────────────────────────┤
│  Client ──────── initialize ────────► Agent                     │
│  Client ◄─────── InitializeResponse ── Agent                    │
│  Client ──────── authenticate ───────► Agent (optional)         │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      SESSION MANAGEMENT                          │
├─────────────────────────────────────────────────────────────────┤
│  Client ──────── session/new ────────► Agent                    │
│  Client ◄─────── NewSessionResponse ── Agent                    │
│       OR                                                        │
│  Client ──────── session/load ───────► Agent                    │
│  Client ◄─────── LoadSessionResponse ─ Agent                    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                        PROMPT TURN                               │
├─────────────────────────────────────────────────────────────────┤
│  Client ──────── session/prompt ─────► Agent                    │
│  Client ◄─────── session/update ────── Agent (streaming)        │
│  Client ◄─────── session/update ────── Agent (streaming)        │
│  ...                                                            │
│  Client ◄─────── PromptResponse ────── Agent (final)            │
└─────────────────────────────────────────────────────────────────┘
```

## JSON-RPC Methods

### initialize

**Direction**: Client → Agent
**Purpose**: Negotiate protocol version and capabilities

**Request**:
```typescript
{
  protocolVersion: number,          // Single integer (e.g., 1)
  clientInfo: {
    name: string,                   // "zed", "neovim", etc.
    title?: string,                 // Human-readable name
    version?: string
  },
  clientCapabilities: {
    fs?: {
      readTextFile?: boolean,
      writeTextFile?: boolean
    },
    terminal?: boolean,
    prompt?: {
      image?: boolean,
      audio?: boolean,
      embeddedContext?: boolean
    },
    mcp?: {
      transport?: {
        http?: boolean,
        sse?: boolean
      }
    }
  }
}
```

**Response**:
```typescript
{
  protocolVersion: number,
  agentInfo: {
    name: string,
    title?: string,
    version?: string
  },
  agentCapabilities: {
    modes?: boolean,
    plan?: boolean
  },
  authMethods?: Array<"none" | "password" | "oauth">
}
```

### authenticate

**Direction**: Client → Agent
**Purpose**: Authenticate using negotiated method

**Request**:
```typescript
{
  method: "password",
  password: string
}
// OR
{
  method: "oauth",
  token: string
}
```

### session/new

**Direction**: Client → Agent
**Purpose**: Create a new conversation session

**Request**:
```typescript
{
  cwd: string,                      // Working directory (absolute path)
  model?: {
    providerID: string,
    modelID: string
  },
  modeId?: string,                  // Starting mode (e.g., "code", "plan")
  mcpServers?: McpServer[]          // Additional MCP servers to connect
}
```

**Response**:
```typescript
{
  sessionId: string,
  modes?: Array<{
    id: string,
    name: string,
    description?: string
  }>,
  currentModeId?: string
}
```

### session/load

**Direction**: Client → Agent
**Purpose**: Resume existing session

**Request**:
```typescript
{
  sessionId: string,
  cwd?: string,                     // Override working directory
  mcpServers?: McpServer[]          // Override MCP servers
}
```

**Response**: Same as session/new, plus session history replay via `session/update` notifications

### session/prompt

**Direction**: Client → Agent
**Purpose**: Send user message and trigger agent response

**Request**:
```typescript
{
  sessionId: string,
  content: ContentBlock[]
}
```

**Response** (after streaming completes):
```typescript
{
  stopReason: "end_turn" | "max_tokens" | "cancelled" | "tool_error"
}
```

### session/update (notification)

**Direction**: Agent → Client
**Purpose**: Stream agent progress in real-time

```typescript
{
  sessionId: string,

  // Content chunks (text streaming)
  contentChunk?: {
    id: string,                     // Part ID
    index: number,                  // Position in message
    delta: string                   // New text to append
  },

  // Tool call updates
  toolCall?: {
    toolCallId: string,
    title?: string,
    kind?: ToolKind,
    status?: "pending" | "in_progress" | "completed" | "failed",
    content?: ContentBlock[],
    locations?: FileLocation[],
    diff?: {
      path: string,
      oldContent?: string,
      newContent?: string
    }
  },

  // Plan updates (if agent supports planning)
  plan?: {
    steps: Array<{
      title: string,
      status: "pending" | "in_progress" | "completed"
    }>
  },

  // Mode changes
  currentModeId?: string
}
```

### session/request_permission

**Direction**: Agent → Client
**Purpose**: Request user approval for sensitive operations

**Request**:
```typescript
{
  sessionId: string,
  permissionId: string,
  title: string,
  kind: "edit" | "execute" | "delete" | "other",
  toolCallId?: string,
  content?: ContentBlock[]
}
```

**Response** (after user interaction):
```typescript
{
  decision: "once" | "always" | "reject",
  cancelled?: boolean
}
```

### session/set_mode

**Direction**: Client → Agent
**Purpose**: Change agent operating mode

**Request**:
```typescript
{
  sessionId: string,
  modeId: string
}
```

### session/cancel

**Direction**: Client → Agent
**Purpose**: Abort ongoing prompt turn

```typescript
{
  sessionId: string
}
```

## File System Methods

### fs/read_text_file

**Direction**: Agent → Client
**Requires**: `clientCapabilities.fs.readTextFile`

**Request**:
```typescript
{
  sessionId: string,
  path: string,                     // Absolute path
  line?: number,                    // 1-based start line
  limit?: number                    // Max lines to read
}
```

**Response**:
```typescript
{
  content: string
}
```

### fs/write_text_file

**Direction**: Agent → Client
**Requires**: `clientCapabilities.fs.writeTextFile`

**Request**:
```typescript
{
  sessionId: string,
  path: string,
  content: string
}
```

**Response**: `null`

## Terminal Methods

All require `clientCapabilities.terminal`

### terminal/create

**Request**:
```typescript
{
  sessionId: string,
  command: string,
  args?: string[],
  env?: Record<string, string>,
  cwd?: string,
  outputBytesLimit?: number
}
```

**Response**:
```typescript
{
  terminalId: string
}
```

### terminal/output

**Request**:
```typescript
{
  sessionId: string,
  terminalId: string
}
```

**Response**:
```typescript
{
  output: string,
  exitCode?: number,
  signal?: string
}
```

### terminal/wait_for_exit

**Request**:
```typescript
{
  sessionId: string,
  terminalId: string
}
```

**Response**:
```typescript
{
  exitCode?: number,
  signal?: string
}
```

### terminal/kill

**Request**:
```typescript
{
  sessionId: string,
  terminalId: string
}
```

### terminal/release

**Request**:
```typescript
{
  sessionId: string,
  terminalId: string
}
```

## Content Blocks

```typescript
type ContentBlock =
  | { type: "text", text: string, annotations?: Annotation[] }
  | { type: "image", data: string, mimeType: string, uri?: string }
  | { type: "audio", data: string, mimeType: string }
  | { type: "resource_link", uri: string, name: string, mimeType?: string }
  | { type: "embedded_resource", uri: string, text?: string, blob?: string }
```

## Tool Kinds

```typescript
type ToolKind =
  | "read"      // File reading
  | "edit"      // File modification
  | "delete"    // File deletion
  | "move"      // File relocation
  | "search"    // Code/file search
  | "execute"   // Command execution
  | "think"     // Agent reasoning
  | "fetch"     // Web/API requests
  | "other"     // Catch-all
```

## MCP Server Configuration

```typescript
type McpServer =
  | {
      type: "local",
      command: string,
      args?: string[],
      environment?: Record<string, string>
    }
  | {
      type: "remote",
      url: string,
      headers?: Record<string, string>
    }
```

## Version Negotiation

Protocol uses single-integer versioning. Current version: **1**

If client requests version N and agent only supports M:
- Agent responds with version M
- Client must decide to continue or disconnect
- No backwards compatibility layer

## Error Handling

JSON-RPC errors with standard codes:
- `-32600`: Invalid request
- `-32601`: Method not found
- `-32602`: Invalid params
- `-32603`: Internal error
- `-32000 to -32099`: Server-defined errors

Custom error for auth: `RequestError.authRequired()`
