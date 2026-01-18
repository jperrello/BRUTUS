# OpenCode LSP Subsystem Specification

## Overview

OpenCode's LSP subsystem manages Language Server Protocol clients and servers, providing code intelligence features (go-to-definition, find-references, hover, diagnostics) to the AI agent.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        LSP Namespace                            │
│  (packages/opencode/src/lsp/index.ts)                          │
├─────────────────────────────────────────────────────────────────┤
│  State:                                                         │
│  - clients: LSPClient[]      // Active connections              │
│  - servers: LSPServer.Info[] // Available server configs        │
│  - spawning: Map<string, Promise>  // In-flight initializations │
│  - broken: Set<string>       // Failed servers (skip retries)   │
├─────────────────────────────────────────────────────────────────┤
│  Query Functions:                                               │
│  - hover(position) → HoverInfo                                  │
│  - definition(position) → Location[]                            │
│  - references(position) → Location[]                            │
│  - implementation(position) → Location[]                        │
│  - documentSymbol(uri) → DocumentSymbol[]                       │
│  - workspaceSymbol(query) → Symbol[]                            │
│  - prepareCallHierarchy(position) → CallHierarchyItem[]         │
│  - incomingCalls(position) → IncomingCall[]                     │
│  - outgoingCalls(position) → OutgoingCall[]                     │
│  - diagnostics() → Map<string, Diagnostic[]>                    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ spawns
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                        LSPServer                                │
│  (packages/opencode/src/lsp/server.ts)                         │
├─────────────────────────────────────────────────────────────────┤
│  Interface Info:                                                │
│  - id: string                // "typescript", "rust-analyzer"   │
│  - extensions: string[]      // [".ts", ".tsx"]                 │
│  - root(file): string|null   // Project root finder             │
│  - spawn(): Handle|undefined // Start server process            │
│                                                                 │
│  Handle:                                                        │
│  - process: ChildProcessWithoutNullStreams                      │
│  - initialization?: object   // Custom init options             │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ manages
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                        LSPClient                                │
│  (packages/opencode/src/lsp/client.ts)                         │
├─────────────────────────────────────────────────────────────────┤
│  - connection: MessageConnection  // JSON-RPC over stdio        │
│  - diagnostics: Map<string, Diagnostic[]>                       │
│                                                                 │
│  Functions:                                                     │
│  - create(handle, root) → LSPClient                            │
│  - notify.open(file) → void                                    │
│  - waitForDiagnostics(file, timeout) → Diagnostic[]            │
│  - shutdown() → void                                           │
└─────────────────────────────────────────────────────────────────┘
```

## Initialization Flow

```
init()
  │
  ├─ Load all server configs from LSPServer.servers[]
  │
  ├─ Filter experimental servers based on feature flags
  │    - If "experimental-ty" enabled: use "ty" instead of "pyright"
  │
  ├─ Remove disabled servers from config
  │
  └─ Ready to lazily spawn on first file access
```

## File Access Flow (touchFile)

```
touchFile(file, waitForDiagnostics?)
  │
  ├─ Find applicable servers by file extension
  │    └─ LSPServer.servers.filter(s => s.extensions.includes(ext))
  │
  ├─ For each applicable server:
  │    │
  │    ├─ Check if already broken → skip
  │    │
  │    ├─ Check if already spawning → await existing promise
  │    │
  │    ├─ Check if already connected → reuse client
  │    │
  │    └─ Spawn new server:
  │         │
  │         ├─ server.spawn() → Handle
  │         │
  │         ├─ LSPClient.create(handle, root)
  │         │    │
  │         │    ├─ createMessageConnection(stdin, stdout)
  │         │    │
  │         │    ├─ connection.sendRequest("initialize", {
  │         │    │    processId,
  │         │    │    rootUri,
  │         │    │    capabilities,
  │         │    │    workspaceFolders
  │         │    │  })
  │         │    │
  │         │    └─ connection.sendNotification("initialized")
  │         │
  │         └─ Add to clients[]
  │
  └─ client.notify.open(file)
       │
       └─ If waitForDiagnostics:
            waitForDiagnostics(file, 3000ms)
```

## Query Execution Pattern

All query functions follow this pattern:

```typescript
async function definition(position: Position): Promise<Location[]> {
  const results: Location[] = []

  // Query all applicable clients in parallel
  await Promise.all(
    clients
      .filter(c => c.appliesTo(position.file))
      .map(async (client) => {
        try {
          const locations = await client.connection.sendRequest(
            "textDocument/definition",
            {
              textDocument: { uri: pathToFileURL(position.file).href },
              position: {
                line: position.line,
                character: position.character
              }
            }
          )
          results.push(...normalize(locations))
        } catch {
          // Silently ignore per-client failures
        }
      })
  )

  return dedupe(results)
}
```

## Diagnostic Aggregation

```
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│  TypeScript  │    │    ESLint    │    │    Biome     │
│    Client    │    │    Client    │    │    Client    │
└──────┬───────┘    └──────┬───────┘    └──────┬───────┘
       │                   │                   │
       │ publishDiagnostics│                   │
       │   (debounced)     │                   │
       ▼                   ▼                   ▼
┌─────────────────────────────────────────────────────┐
│              Bus.publish("lsp.diagnostics")         │
│  Event contains: { serverId, file, diagnostics[] } │
└─────────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────┐
│           LSP.diagnostics() aggregator             │
│  Returns: Map<file, Diagnostic[]> from all clients │
└─────────────────────────────────────────────────────┘
```

## Key Data Structures

### Position
```typescript
interface Position {
  file: string      // Absolute file path
  line: number      // 0-based line index
  character: number // 0-based character offset
}
```

### Range (Zod Schema)
```typescript
const Range = z.object({
  start: z.object({
    line: z.number(),
    character: z.number()
  }),
  end: z.object({
    line: z.number(),
    character: z.number()
  })
})
```

### Symbol
```typescript
const Symbol = z.object({
  name: z.string(),
  kind: z.number(),      // LSP SymbolKind enum
  location: z.object({
    uri: z.string(),
    range: Range
  })
})
```

### Diagnostic
```typescript
interface Diagnostic {
  range: Range
  message: string
  severity: 1 | 2 | 3 | 4  // Error, Warning, Information, Hint
  source?: string          // "typescript", "eslint"
  code?: string | number
}
```

## Configuration Integration

Servers can be disabled via OpenCode config:

```typescript
// Config path: ~/.opencode/config.json
{
  "lsp": {
    "disabled": ["eslint", "biome"]  // Servers to skip
  }
}
```

Environment variable `OPENCODE_DISABLE_LSP_DOWNLOAD` prevents automatic server downloads.

## Error Handling

1. **Broken servers**: Marked in `broken` Set, never retried in session
2. **Spawn failures**: Logged, server added to broken set
3. **Query failures**: Silently caught per-client, other clients still queried
4. **Initialization timeout**: 45 seconds, then InitializeError thrown

## Thread Safety

- `spawning` Map prevents duplicate server spawns
- Promise-based initialization allows concurrent requests to wait
- Client operations are async but not concurrent within a client
