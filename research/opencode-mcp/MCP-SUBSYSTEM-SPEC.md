# OpenCode MCP Subsystem Specification

**Research Date**: 2026-01-17
**Source**: https://github.com/anomalyco/opencode
**Focus Area**: Model Context Protocol (MCP) Client Implementation
**Files Analyzed**: `packages/opencode/src/mcp/`

---

## Executive Summary

OpenCode implements a full MCP client that enables AI agents to communicate with external tool servers. The implementation supports both **local** (stdio-based) and **remote** (HTTP/SSE-based) MCP servers, with a complete OAuth 2.0 authentication flow for remote servers.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                          MCP Namespace                          │
├─────────────────────────────────────────────────────────────────┤
│  State Management (Instance.state)                              │
│  ├── clients: Record<string, MCPClient>                         │
│  └── status: Record<string, Status>                             │
├─────────────────────────────────────────────────────────────────┤
│  Transport Layer                                                │
│  ├── StdioClientTransport (local servers)                       │
│  ├── StreamableHTTPClientTransport (remote - preferred)         │
│  └── SSEClientTransport (remote - fallback)                     │
├─────────────────────────────────────────────────────────────────┤
│  OAuth Layer                                                    │
│  ├── McpOAuthProvider (implements OAuthClientProvider)          │
│  ├── McpOAuthCallback (local HTTP server for OAuth redirect)    │
│  └── McpAuth (credential persistence)                           │
└─────────────────────────────────────────────────────────────────┘
```

---

## Module Breakdown

### 1. `index.ts` - Core MCP Client Manager

**Purpose**: Central orchestrator for all MCP server connections, tool discovery, and resource management.

#### Key Data Structures

```typescript
// Connection status discriminated union
Status =
  | { status: "connected" }
  | { status: "disabled" }
  | { status: "failed", error: string }
  | { status: "needs_auth" }
  | { status: "needs_client_registration", error: string }

// Resource type from MCP servers
Resource = {
  name: string
  uri: string
  description?: string
  mimeType?: string
  client: string  // Which MCP server provides this
}
```

#### State Management Pattern

Uses `Instance.state()` - a project-scoped singleton pattern:

```typescript
const state = Instance.state(
  async () => {
    // Initialization: connect to all configured MCP servers
    // Returns { status, clients }
  },
  async (state) => {
    // Cleanup: close all clients, clear pending OAuth
  }
)
```

**Key Insight**: State is lazy-initialized on first access and cleaned up when the project instance is destroyed.

#### Connection Flow

1. Read MCP configuration from `Config.get().mcp`
2. For each configured server:
   - If `enabled === false` → status = "disabled"
   - If `type === "local"` → use `StdioClientTransport`
   - If `type === "remote"` → try transports in order:
     1. `StreamableHTTPClientTransport` (primary)
     2. `SSEClientTransport` (fallback)
3. Register notification handlers for tool list changes
4. Verify connection by calling `listTools()`

#### Transport Selection Logic (Remote)

```typescript
const transports = [
  { name: "StreamableHTTP", transport: new StreamableHTTPClientTransport(...) },
  { name: "SSE", transport: new SSEClientTransport(...) }
]

for (const { name, transport } of transports) {
  try {
    await withTimeout(client.connect(transport), connectTimeout)
    // Success - use this transport
    break
  } catch (error) {
    if (error instanceof UnauthorizedError) {
      // Handle OAuth requirement
      break
    }
    // Try next transport
  }
}
```

#### Tool Conversion

Converts MCP tool definitions to AI SDK compatible tools:

```typescript
async function convertMcpTool(mcpTool: MCPToolDef, client: MCPClient, timeout?: number): Promise<Tool> {
  const schema: JSONSchema7 = {
    ...inputSchema,
    type: "object",  // Always force object type
    properties: inputSchema.properties ?? {},
    additionalProperties: false
  }

  return dynamicTool({
    description: mcpTool.description ?? "",
    inputSchema: jsonSchema(schema),
    execute: async (args) => client.callTool({ name, arguments: args }, CallToolResultSchema, {
      resetTimeoutOnProgress: true,
      timeout
    })
  })
}
```

**Tool Naming Convention**: `{sanitized_client_name}_{sanitized_tool_name}`
Sanitization: Replace non-alphanumeric (except `_-`) with `_`

#### Exported Functions

| Function | Purpose |
|----------|---------|
| `status()` | Get connection status for all configured servers |
| `clients()` | Get active client instances |
| `tools()` | Get all tools from connected servers (AI SDK format) |
| `prompts()` | Get all prompts from connected servers |
| `resources()` | Get all resources from connected servers |
| `connect(name)` | Connect to a specific server |
| `disconnect(name)` | Disconnect from a specific server |
| `add(name, config)` | Add and connect a new server |
| `startAuth(name)` | Begin OAuth flow, return auth URL |
| `authenticate(name)` | Full OAuth flow with browser |
| `finishAuth(name, code)` | Complete OAuth with authorization code |
| `removeAuth(name)` | Clear stored OAuth credentials |
| `supportsOAuth(name)` | Check if server supports OAuth |
| `hasStoredTokens(name)` | Check for stored tokens |
| `getAuthStatus(name)` | "authenticated" | "expired" | "not_authenticated" |

---

### 2. `auth.ts` - Credential Persistence

**Purpose**: Secure storage of OAuth tokens, client registrations, and PKCE flow state.

#### Storage Location
```typescript
const filepath = path.join(Global.Path.data, "mcp-auth.json")
```

#### Data Schema

```typescript
Entry = {
  tokens?: {
    accessToken: string
    refreshToken?: string
    expiresAt?: number      // Unix timestamp (seconds)
    scope?: string
  }
  clientInfo?: {
    clientId: string
    clientSecret?: string
    clientIdIssuedAt?: number
    clientSecretExpiresAt?: number
  }
  codeVerifier?: string     // PKCE code_verifier
  oauthState?: string       // CSRF protection state
  serverUrl?: string        // Validate credentials match server
}
```

#### Security Measures

1. **File permissions**: `chmod 0o600` after every write
2. **URL binding**: Credentials are bound to `serverUrl` - won't return credentials if URL changes
3. **Expiration tracking**: Both tokens and client secrets track expiration

#### Key Operations

```typescript
// URL-aware credential retrieval
async function getForUrl(mcpName: string, serverUrl: string): Promise<Entry | undefined>

// Atomic updates
async function updateTokens(mcpName: string, tokens: Tokens, serverUrl?: string)
async function updateClientInfo(mcpName: string, clientInfo: ClientInfo, serverUrl?: string)
async function updateCodeVerifier(mcpName: string, codeVerifier: string)
async function updateOAuthState(mcpName: string, oauthState: string)
```

---

### 3. `oauth-provider.ts` - OAuth Client Provider

**Purpose**: Implements the `OAuthClientProvider` interface from MCP SDK for OAuth 2.0 flows.

#### Constants

```typescript
OAUTH_CALLBACK_PORT = 19876
OAUTH_CALLBACK_PATH = "/mcp/oauth/callback"
// Redirect URL: http://127.0.0.1:19876/mcp/oauth/callback
```

#### Client Metadata (for Dynamic Registration)

```typescript
clientMetadata = {
  redirect_uris: [redirectUrl],
  client_name: "OpenCode",
  client_uri: "https://opencode.ai",
  grant_types: ["authorization_code", "refresh_token"],
  response_types: ["code"],
  token_endpoint_auth_method: hasClientSecret ? "client_secret_post" : "none"
}
```

#### Client Info Resolution Order

1. **Config-provided**: If `config.clientId` exists, use it (pre-registered client)
2. **Stored from dynamic registration**: Check `McpAuth.getForUrl()` for saved client info
3. **Trigger dynamic registration**: Return `undefined` to SDK

#### Token Handling

```typescript
// Convert internal format to SDK format
async tokens(): Promise<OAuthTokens | undefined> {
  const entry = await McpAuth.getForUrl(this.mcpName, this.serverUrl)
  return {
    access_token: entry.tokens.accessToken,
    token_type: "Bearer",
    refresh_token: entry.tokens.refreshToken,
    expires_in: calculateRemainingSeconds(entry.tokens.expiresAt),
    scope: entry.tokens.scope
  }
}
```

#### PKCE Support

Full PKCE (Proof Key for Code Exchange) implementation:
- `saveCodeVerifier(codeVerifier)` - Store before redirect
- `codeVerifier()` - Retrieve for token exchange

#### State Management (CSRF Protection)

- `saveState(state)` - Store before redirect
- `state()` - Retrieve for validation

---

### 4. `oauth-callback.ts` - Local OAuth Callback Server

**Purpose**: Local HTTP server to receive OAuth authorization codes.

#### Server Configuration

- **Port**: 19876 (fixed)
- **Path**: `/mcp/oauth/callback`
- **Timeout**: 5 minutes per auth attempt
- **Runtime**: Bun's built-in HTTP server

#### State Machine

```
┌──────────────┐    ensureRunning()    ┌─────────────┐
│   Stopped    │ ───────────────────▶  │   Running   │
└──────────────┘                       └──────┬──────┘
                                              │
                    waitForCallback(state)    │
                         ┌────────────────────┘
                         ▼
              ┌──────────────────────┐
              │  Pending Auth Map    │
              │  state → {resolve,   │
              │          reject,     │
              │          timeout}    │
              └──────────────────────┘
```

#### Request Handling

```typescript
// Callback URL: ?code=xxx&state=yyy
if (!state) → 400 "Missing state parameter"
if (error) → Show error HTML, reject pending
if (!pendingAuths.has(state)) → 400 "Invalid state"
→ Resolve pending with code, show success HTML
```

#### HTML Responses

**Success Page**:
- Dark theme (#1a1a2e background)
- Green success message (#4ade80)
- Auto-closes after 2 seconds

**Error Page**:
- Dark theme
- Red error message (#f87171)
- Shows error description in monospace

#### Port Collision Handling

```typescript
async function isPortInUse(): Promise<boolean> {
  // Uses Bun.connect to check if something is already listening
}

async function ensureRunning() {
  if (server) return
  if (await isPortInUse()) {
    // Another OpenCode instance has the server - that's OK
    return
  }
  // Start server
}
```

---

## Event Bus Integration

### Published Events

```typescript
// Tool list changed notification from MCP server
MCP.ToolsChanged = BusEvent.define("mcp.tools.changed", z.object({
  server: z.string()
}))

// Browser failed to open for OAuth
MCP.BrowserOpenFailed = BusEvent.define("mcp.browser.open.failed", z.object({
  mcpName: z.string(),
  url: z.string()
}))
```

### Toast Notifications

OAuth status changes trigger TUI toasts:
- "MCP Authentication Required" (needs_auth)
- "Server requires pre-registered client ID" (needs_client_registration)

---

## Configuration Schema

```typescript
// From Config.Mcp
type McpConfig =
  | {
      type: "local"
      command: string[]          // e.g., ["npx", "mcp-server"]
      environment?: Record<string, string>
      enabled?: boolean
      timeout?: number
    }
  | {
      type: "remote"
      url: string
      headers?: Record<string, string>
      oauth?: false | {
        clientId?: string
        clientSecret?: string
        scope?: string
      }
      enabled?: boolean
      timeout?: number
    }
```

---

## Timeout Handling

| Operation | Default | Configurable |
|-----------|---------|--------------|
| Connection | 30,000ms | `mcp.timeout` |
| Tool call | 30,000ms | `experimental.mcp_timeout` |
| OAuth callback | 300,000ms (5min) | No |

All timeouts use `withTimeout()` utility for consistent handling.

---

## Error Handling

### Named Errors

```typescript
MCP.Failed = NamedError.create("MCPFailed", z.object({
  name: z.string()
}))
```

### Error Recovery

1. **Transport failure**: Try next transport, then mark as failed
2. **OAuth required**: Mark as `needs_auth`, store transport for later
3. **Client registration required**: Mark as `needs_client_registration`
4. **Tool listing failure**: Close client, mark as failed

---

## Dependencies

### External Packages

- `@modelcontextprotocol/sdk` - Official MCP SDK
- `ai` - Vercel AI SDK (for tool abstraction)
- `zod/v4` - Schema validation
- `open` - Cross-platform browser opening

### Internal Modules

- `Config` - Configuration management
- `Instance` - Project instance lifecycle
- `Installation` - Version info
- `Bus`, `BusEvent` - Event system
- `Log` - Structured logging
- `withTimeout` - Timeout utility
- `Global` - Global paths

---

## Implementation Notes for BRUTUS

### Key Patterns to Adopt

1. **Transport fallback chain**: Try StreamableHTTP first, fall back to SSE
2. **State-based OAuth**: Use cryptographic state parameter for CSRF protection
3. **URL-bound credentials**: Validate stored credentials match server URL
4. **Lazy state initialization**: `Instance.state()` pattern for project-scoped singletons

### Key Differences to Consider

1. **Runtime**: OpenCode uses Bun; BRUTUS uses Go
2. **Tool format**: OpenCode converts to Vercel AI SDK format
3. **Event system**: OpenCode has a custom Bus; BRUTUS needs equivalent

### Minimum Viable Implementation

For BRUTUS to support MCP:

1. **StdioClientTransport equivalent** - Spawn process, communicate via stdin/stdout
2. **Tool discovery** - Call `tools/list` method on connection
3. **Tool invocation** - Forward tool calls to MCP server
4. **Configuration** - Read MCP server configs from BRUTUS config

OAuth support can be added later for remote servers.

---

## File Checksums (for verification)

```
index.ts         ~29KB  (main orchestrator)
auth.ts          ~4KB   (credential storage)
oauth-provider.ts ~5KB  (OAuth client)
oauth-callback.ts ~6KB  (callback server)
```

---

## References

- [MCP Specification](https://modelcontextprotocol.io/specification)
- [MCP TypeScript SDK](https://github.com/modelcontextprotocol/typescript-sdk)
- [OAuth 2.0 RFC 6749](https://tools.ietf.org/html/rfc6749)
- [PKCE RFC 7636](https://tools.ietf.org/html/rfc7636)
