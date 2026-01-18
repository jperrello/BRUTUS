# MCP Wire Protocol Analysis

**Research Date**: 2026-01-17
**Derived From**: OpenCode implementation analysis

---

## Protocol Overview

MCP (Model Context Protocol) uses JSON-RPC 2.0 over various transports:

1. **stdio** - For local subprocess servers
2. **StreamableHTTP** - For remote servers (preferred)
3. **SSE** - Server-Sent Events fallback for remote

---

## JSON-RPC 2.0 Message Format

### Request

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/list",
  "params": {}
}
```

### Response

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "tools": [...]
  }
}
```

### Notification (no id, no response expected)

```json
{
  "jsonrpc": "2.0",
  "method": "notifications/tools/list_changed",
  "params": {}
}
```

---

## Core Methods

### Initialization

```
Method: initialize
Request:
{
  "protocolVersion": "2024-11-05",
  "capabilities": {
    "roots": { "listChanged": true },
    "sampling": {}
  },
  "clientInfo": {
    "name": "opencode",
    "version": "0.x.x"
  }
}

Response:
{
  "protocolVersion": "2024-11-05",
  "capabilities": {
    "tools": { "listChanged": true },
    "resources": { "subscribe": true },
    "prompts": { "listChanged": true }
  },
  "serverInfo": {
    "name": "example-server",
    "version": "1.0.0"
  }
}
```

### Tool Discovery

```
Method: tools/list
Request: {}

Response:
{
  "tools": [
    {
      "name": "read_file",
      "description": "Read a file from the filesystem",
      "inputSchema": {
        "type": "object",
        "properties": {
          "path": {
            "type": "string",
            "description": "Path to the file"
          }
        },
        "required": ["path"]
      }
    }
  ]
}
```

### Tool Invocation

```
Method: tools/call
Request:
{
  "name": "read_file",
  "arguments": {
    "path": "/etc/hosts"
  }
}

Response:
{
  "content": [
    {
      "type": "text",
      "text": "127.0.0.1 localhost\n..."
    }
  ],
  "isError": false
}
```

### Prompt Discovery

```
Method: prompts/list
Request: {}

Response:
{
  "prompts": [
    {
      "name": "code_review",
      "description": "Review code for issues",
      "arguments": [
        {
          "name": "code",
          "description": "The code to review",
          "required": true
        }
      ]
    }
  ]
}
```

### Prompt Retrieval

```
Method: prompts/get
Request:
{
  "name": "code_review",
  "arguments": {
    "code": "function foo() { ... }"
  }
}

Response:
{
  "description": "Code review prompt",
  "messages": [
    {
      "role": "user",
      "content": {
        "type": "text",
        "text": "Review this code: ..."
      }
    }
  ]
}
```

### Resource Discovery

```
Method: resources/list
Request: {}

Response:
{
  "resources": [
    {
      "uri": "file:///project/README.md",
      "name": "README",
      "description": "Project documentation",
      "mimeType": "text/markdown"
    }
  ]
}
```

### Resource Reading

```
Method: resources/read
Request:
{
  "uri": "file:///project/README.md"
}

Response:
{
  "contents": [
    {
      "uri": "file:///project/README.md",
      "mimeType": "text/markdown",
      "text": "# Project\n..."
    }
  ]
}
```

---

## Notifications

Servers can send notifications to indicate state changes:

```
notifications/tools/list_changed
notifications/prompts/list_changed
notifications/resources/list_changed
notifications/resources/updated { uri: string }
```

OpenCode registers a handler for `ToolListChangedNotificationSchema`:

```typescript
client.setNotificationHandler(ToolListChangedNotificationSchema, async () => {
  Bus.publish(ToolsChanged, { server: serverName })
})
```

---

## Transport: stdio

### Connection

1. Spawn subprocess with command and args
2. Write JSON-RPC messages to stdin (newline-delimited)
3. Read JSON-RPC responses from stdout
4. stderr is typically ignored or logged

### OpenCode Implementation

```typescript
const transport = new StdioClientTransport({
  stderr: "ignore",
  command: cmd,
  args,
  cwd,
  env: {
    ...process.env,
    ...mcp.environment
  }
})
```

### Message Framing

Each message is a complete JSON object followed by newline:

```
{"jsonrpc":"2.0","id":1,"method":"initialize",...}\n
{"jsonrpc":"2.0","id":1,"result":{...}}\n
```

---

## Transport: StreamableHTTP

### Endpoint

Single HTTP endpoint handling bidirectional communication.

### Request Flow

1. Client sends POST with JSON-RPC request
2. Server responds with JSON-RPC response
3. Connection can be kept alive for multiple requests

### Headers

```http
POST /mcp HTTP/1.1
Content-Type: application/json
Authorization: Bearer <token>  // If authenticated
```

---

## Transport: SSE (Server-Sent Events)

### Connection

1. Client opens GET request to SSE endpoint
2. Server sends events as they occur
3. Client sends requests via POST to separate endpoint

### Event Format

```
event: message
data: {"jsonrpc":"2.0","id":1,"result":{...}}

event: message
data: {"jsonrpc":"2.0","method":"notifications/tools/list_changed"}
```

---

## OAuth 2.0 Integration

### Discovery

Remote servers may require OAuth. The MCP SDK detects this via:

1. Server returns 401 Unauthorized
2. SDK throws `UnauthorizedError`
3. Client initiates OAuth flow

### Flow

```
1. Client → Server: Connect attempt
2. Server → Client: 401 + WWW-Authenticate (optional discovery info)
3. Client: Create OAuth provider
4. Client → Auth Server: Authorization request with PKCE
5. User: Authorizes in browser
6. Auth Server → Callback: code + state
7. Client: Exchange code for tokens
8. Client → Server: Retry with Bearer token
```

### PKCE (RFC 7636)

```
code_verifier: Random 43-128 char string
code_challenge: BASE64URL(SHA256(code_verifier))
code_challenge_method: S256
```

### State Parameter (CSRF Protection)

OpenCode generates cryptographic state:

```typescript
const oauthState = Array.from(crypto.getRandomValues(new Uint8Array(32)))
  .map((b) => b.toString(16).padStart(2, "0"))
  .join("")
```

---

## Content Types

Tool results and prompt messages use content blocks:

### Text Content

```json
{
  "type": "text",
  "text": "Hello, world!"
}
```

### Image Content

```json
{
  "type": "image",
  "data": "<base64>",
  "mimeType": "image/png"
}
```

### Resource Content

```json
{
  "type": "resource",
  "resource": {
    "uri": "file:///path",
    "text": "...",
    "mimeType": "text/plain"
  }
}
```

---

## Error Handling

### JSON-RPC Errors

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32600,
    "message": "Invalid Request",
    "data": { ... }
  }
}
```

### Standard Error Codes

| Code | Meaning |
|------|---------|
| -32700 | Parse error |
| -32600 | Invalid Request |
| -32601 | Method not found |
| -32602 | Invalid params |
| -32603 | Internal error |

### Tool Execution Errors

```json
{
  "content": [
    {
      "type": "text",
      "text": "Error: File not found"
    }
  ],
  "isError": true
}
```

---

## Timeouts

OpenCode uses progressive timeouts:

| Phase | Timeout |
|-------|---------|
| Connection | 30s (configurable) |
| Tool call | 30s (configurable) |
| Reset on progress | Yes |

```typescript
client.callTool({ name, arguments }, CallToolResultSchema, {
  resetTimeoutOnProgress: true,
  timeout
})
```

---

## Implementation Checklist for BRUTUS

### Minimum (Local stdio)

- [ ] Spawn subprocess with command/args
- [ ] JSON-RPC message encoding/decoding
- [ ] Newline framing
- [ ] `initialize` handshake
- [ ] `tools/list` discovery
- [ ] `tools/call` invocation
- [ ] Timeout handling

### Extended (Resources/Prompts)

- [ ] `resources/list` and `resources/read`
- [ ] `prompts/list` and `prompts/get`
- [ ] Notification handling

### Remote (HTTP)

- [ ] StreamableHTTP transport
- [ ] SSE transport fallback
- [ ] Bearer token authentication

### Full (OAuth)

- [ ] OAuth provider interface
- [ ] PKCE flow
- [ ] Local callback server
- [ ] Token persistence
- [ ] Token refresh
