# OpenCode Server Subsystem Specification

## Purpose

The server subsystem provides an HTTP API layer that decouples the OpenCode agent engine from its user interfaces. This enables multiple frontends (TUI, Desktop app, Web interface, IDE extensions) to interact with the same running agent through a standardized REST + SSE protocol.

## Technology Stack

| Component | Technology |
|-----------|------------|
| HTTP Framework | Hono (lightweight, edge-ready) |
| Runtime | Bun |
| Validation | Zod schemas |
| API Documentation | OpenAPI 3.1.1 via hono-openapi |
| Service Discovery | Bonjour/mDNS |
| Realtime Events | Server-Sent Events (SSE) |
| Terminal Streaming | WebSocket |

## Server Namespace

```typescript
namespace Server {
  function url(): URL              // Returns server URL (default: http://localhost:4096)
  function App(): Hono             // Lazy-loaded Hono application instance
  function openapi(): OpenAPISpec  // Generates OpenAPI 3.1.1 specification
  function listen(opts: ListenOptions): BunServer
}

interface ListenOptions {
  port: number
  hostname: string
  mdns?: boolean     // Enable mDNS service publishing
  cors?: string[]    // Additional CORS origins
}
```

## Middleware Stack

The server applies middleware in this exact order:

### 1. Error Handler
```
Catches: NamedError, HTTPException, generic Error
Returns: { data: null, errors: [...], success: false }
Logs: Error details to console
```

### 2. Basic Authentication (Optional)
```
Enabled: Via environment flag
Credentials: From environment variables
Skip: When flag is disabled
```

### 3. Request Logger
```
Logs: Method, path, timing for all requests
Skip: /log endpoint (to prevent infinite loops)
```

### 4. CORS Configuration
```
Allowed Origins:
- http://localhost:*
- http://127.0.0.1:*
- tauri://localhost (Desktop app)
- https://*.opencode.ai
- User-provided whitelist
```

### 5. Instance Directory Provider
```
Source: ?directory= query param OR x-directory header
Purpose: Routes requests to correct project instance
Fallback: Default directory from config
```

### 6. Query Validation
```
Validates: All query parameters against Zod schemas
Returns: 400 error on validation failure
```

## Port Selection

```
Default Port: 4096
Environment Override: OPENCODE_PORT
URL Pattern: http://localhost:{port}
```

## mDNS Service Publishing

When `mdns: true` is passed to listen():

```
Service Name: opencode-{port}
Service Type: _http._tcp
Hostname: opencode.local
Path: /
```

This enables discovery by:
- Other devices on local network
- IDE extensions scanning for running agents
- Mobile companion apps

### mDNS Lifecycle

```typescript
// On server start
mdns.publish(port)  // Registers service

// On server stop
mdns.unpublish()    // Removes registration, destroys instance
```

## Route Registration

Routes are organized into logical groups:

```typescript
const app = new Hono()

// Core routes (defined inline in server.ts)
app.get('/doc', ...)        // OpenAPI documentation
app.get('/path', ...)       // Directory paths
app.get('/vcs', ...)        // Version control info
app.get('/command', ...)    // Available commands
app.get('/agent', ...)      // Available agents
app.get('/skill', ...)      // Available skills
app.get('/lsp', ...)        // LSP status
app.get('/formatter', ...)  // Formatter status
app.post('/log', ...)       // Client logging
app.put('/auth/:providerID', ...)  // Auth credentials
app.get('/event', ...)      // SSE stream
app.post('/instance/dispose', ...)  // Cleanup

// Nested route modules
app.route('/global', GlobalRoutes())
app.route('/project', ProjectRoutes())
app.route('/pty', PtyRoutes())
app.route('/config', ConfigRoutes())
app.route('/experimental', ExperimentalRoutes())
app.route('/session', SessionRoutes())
app.route('/permission', PermissionRoutes())
app.route('/question', QuestionRoutes())
app.route('/provider', ProviderRoutes())
app.route('/mcp', McpRoutes())
app.route('/tui', TuiRoutes())
app.route('/', FileRoutes())

// Fallback: Proxy to web app
app.all('/*', proxy('https://app.opencode.ai'))
```

## Error Response Schema

```typescript
interface ErrorResponse {
  data: any                    // null for errors
  errors: Array<{
    [key: string]: any         // Error details
  }>
  success: false               // Literal false
}

// HTTP Status Codes Used
400: Bad Request (validation failure)
404: Not Found (resource missing)
500: Internal Server Error (unhandled exception)
```

## Event Types

The server defines these bus events:

```typescript
// Client connected to SSE stream
BusEvent.define("server.connected", z.object({}))

// 30-second heartbeat for connection keep-alive
BusEvent.define("server.heartbeat", z.object({}))

// All instances disposed
BusEvent.define("global.disposed", z.object({}))
```

## Instance Management

Each request is routed to a specific project instance:

```
1. Extract directory from request (query or header)
2. Look up or create Instance for that directory
3. Pass Instance context to all route handlers
4. Handlers operate on that instance's data
```

This enables:
- Multiple projects open simultaneously
- Each with its own sessions, config, state
- Isolated from other projects

## Security Considerations

1. **CORS Restriction**: Only allows localhost and opencode.ai domains
2. **Basic Auth Option**: Can require credentials for all requests
3. **No External Network by Default**: Binds to localhost only
4. **mDNS is Opt-in**: Must explicitly enable network discovery

## OpenAPI Generation

```typescript
Server.openapi() → {
  openapi: "3.1.1",
  info: {
    title: "OpenCode API",
    version: "1.0.0"
  },
  servers: [{ url: Server.url() }],
  paths: { ... },
  components: { schemas: { ... } }
}
```

Accessible at: `GET /doc`

## Bun Server Configuration

```typescript
Bun.serve({
  port: opts.port,
  hostname: opts.hostname,
  fetch: app.fetch,
  websocket: {
    // WebSocket handlers for PTY connections
    open(ws) { ... },
    message(ws, data) { ... },
    close(ws) { ... }
  }
})
```
