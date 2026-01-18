# OpenCode ACP Implementation Analysis

Analysis of `packages/opencode/src/acp/` in the OpenCode repository.

## File Structure

```
packages/opencode/src/acp/
├── README.md           # Implementation notes
├── agent.ts            # Core ACP agent (37KB) - main logic
├── session.ts          # Session state management
└── types.ts            # TypeScript interfaces
```

## Core Architecture

### Agent Class

```typescript
export class Agent implements ACPAgent {
  private connection: AgentSideConnection   // JSON-RPC transport
  private config: ACPConfig                  // SDK + default model
  private sdk: OpencodeClient               // OpenCode SDK instance
  private sessionManager: ACPSessionManager  // Session state cache
}
```

The Agent implements the `ACPAgent` interface from `@agentclientprotocol/sdk`.

### Session Manager

```typescript
class ACPSessionManager {
  private sessions: Map<string, ACPSessionState>
  private sdk: OpencodeClient

  create(cwd, mcpServers, model, modeId): Promise<ACPSessionState>
  load(sessionId): Promise<ACPSessionState>
  get(sessionId): ACPSessionState
  setModel(sessionId, model): void
  setMode(sessionId, modeId): void
}
```

## Method Implementations

### initialize()

Returns:
- Protocol version: 1
- Agent capabilities (modes, planning support)
- Authentication methods (password if configured)

### newSession()

1. Calls `sessionManager.create()` with cwd, MCP servers
2. Resolves default model if not specified
3. Returns session ID and available modes

### loadSession()

1. Loads session state from SDK
2. Registers MCP servers (local or remote)
3. Replays message history via `session/update` notifications
4. Uses `processMessage()` to convert stored messages to ACP format

### prompt()

Core prompt handling:

```typescript
async prompt(params: PromptRequest): Promise<PromptResponse> {
  const session = this.sessionManager.get(params.sessionId)

  // Parse content blocks - handles text, images, resource links
  const content = params.content.map(block => parseUri(block))

  // Check for command prefix
  const text = extractText(content)
  if (text?.startsWith("/")) {
    // Route to command handler (e.g., /compact)
    return this.handleCommand(session, text)
  }

  // Normal prompt flow
  return this.sessionPrompt(session, content)
}
```

### setupEventSubscriptions()

Two critical event handlers:

**1. Permission Events**
```typescript
bus.subscribe("permission", async (event) => {
  // Map OpenCode permission to ACP permission request
  const response = await connection.sendRequest("session/request_permission", {
    sessionId: event.sessionId,
    permissionId: event.id,
    title: event.title,
    kind: toToolKind(event.toolName)
  })

  // Handle decision: once, always, reject
  if (response.decision === "always") {
    // Grant permanent permission
  }

  // For edit permissions, apply the diff
  if (event.toolName === "edit" && response.decision !== "reject") {
    await applyPatch(event.path, event.diff)
  }
})
```

**2. Message Update Events**
```typescript
bus.subscribe("message.update", async (event) => {
  for (const part of event.parts) {
    if (part.type === "tool_call") {
      // Stream tool call status to client
      connection.sendNotification("session/update", {
        sessionId: event.sessionId,
        toolCall: {
          toolCallId: part.id,
          title: part.name,
          kind: toToolKind(part.name),
          status: mapStatus(part.state),
          content: formatToolResult(part),
          locations: extractLocations(part),
          diff: extractDiff(part)
        }
      })
    }

    if (part.type === "text") {
      // Stream text chunks
      connection.sendNotification("session/update", {
        sessionId: event.sessionId,
        contentChunk: {
          id: part.id,
          index: part.index,
          delta: part.delta
        }
      })
    }
  }
})
```

## Tool Kind Mapping

OpenCode maps internal tool names to ACP tool kinds:

```typescript
function toToolKind(toolName: string): ToolKind {
  switch (toolName.toLowerCase()) {
    case "bash":
    case "execute":
      return "execute"

    case "webfetch":
      return "fetch"

    case "edit":
    case "patch":
    case "write":
      return "edit"

    case "grep":
    case "glob":
      return "search"

    case "read":
    case "list":
      return "read"

    default:
      return "other"
  }
}
```

## Model Resolution

Priority for selecting default model:

1. User-specified model in session config
2. User's global configuration
3. "big-pickle" model from opencode provider (internal)
4. First available model from any provider

## Resource URI Parsing

```typescript
function parseUri(block: ContentBlock): ParsedContent {
  if (block.type !== "resource_link") return block

  const uri = block.uri
  if (uri.startsWith("file://")) {
    // Local file reference
    return { type: "file", path: uri.slice(7) }
  }

  if (uri.startsWith("zed://")) {
    // Zed-specific file link (includes buffer state)
    return { type: "zed_file", uri }
  }

  // Treat as text
  return { type: "text", text: uri }
}
```

## Diff Application

When permission is granted for edit operations:

```typescript
async function applyPatch(path: string, diff: string): Promise<void> {
  try {
    const result = await unifiedDiff.apply(path, diff)
    if (!result.success) {
      log.error("Diff application failed", { path, error: result.error })
    }
  } catch (err) {
    log.error("Patch error", { path, err })
  }
}
```

## Session State

```typescript
interface ACPSessionState {
  id: string                        // Session identifier
  cwd: string                       // Working directory
  mcpServers: McpServer[]           // Connected MCP servers
  createdAt: number                 // Timestamp
  model?: {
    providerID: string
    modelID: string
  }
  modeId?: string                   // Current operating mode
}
```

## Server Integration

The ACP agent runs alongside OpenCode's HTTP server. From `server.ts`:

```typescript
// SSE endpoint for real-time events
app.get("/event", async (c) => {
  return streamSSE(c, async (stream) => {
    // Heartbeat every 30s (WKWebView timeout prevention)
    const heartbeat = setInterval(() => stream.write("ping"), 30000)

    // Subscribe to bus events
    bus.subscribe("*", (event) => {
      stream.write(JSON.stringify(event))
    })
  })
})
```

## Connection Types

ACP supports two connection modes:

1. **Stdio**: For CLI-based clients
2. **WebSocket/HTTP**: For GUI clients (Zed, desktop apps)

OpenCode uses the `AgentSideConnection` from `@agentclientprotocol/sdk` which abstracts the transport.

## Authentication

```typescript
async authenticate(params: AuthenticateRequest): Promise<void> {
  // Currently not implemented - throws error
  throw new Error("Authentication not implemented")
}
```

Password authentication is configured via `Flag.OPENCODE_SERVER_PASSWORD` but the ACP authenticate method is stubbed.

## Key Dependencies

```typescript
import { AgentSideConnection, ACPAgent } from "@agentclientprotocol/sdk"
import { OpencodeClient } from "@opencode-ai/sdk/v2"
```

The implementation bridges between:
- `@agentclientprotocol/sdk` - ACP protocol implementation
- `@opencode-ai/sdk/v2` - OpenCode's internal SDK for sessions, providers, tools
