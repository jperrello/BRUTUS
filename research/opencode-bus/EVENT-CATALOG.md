# OpenCode Event Catalog

Complete catalog of known BusEvent definitions discovered through reverse engineering.

## Event Categories

### Core/Instance Events

| Event Type | Properties | Source |
|------------|------------|--------|
| `server.instance.disposed` | `{ directory: string }` | `bus/index.ts` |
| `server.heartbeat` | `{}` | `server/server.ts` (synthetic) |

### Session Events

| Event Type | Properties | Source |
|------------|------------|--------|
| `session.created` | `{ id, title, ... }` | `session/index.ts` |
| `session.updated` | `{ id, ... }` | `session/index.ts` |
| `session.deleted` | `{ id }` | `session/index.ts` |
| `session.diff` | `{ sessionID, diffs: FileDiff[] }` | `session/index.ts` |
| `session.error` | `{ sessionID, error }` | `session/index.ts` |

### Message Events

| Event Type | Properties | Source |
|------------|------------|--------|
| `message.updated` | `{ sessionID, message }` | `session/message-v2.ts` |
| `message.removed` | `{ sessionID, messageID }` | `session/message-v2.ts` |
| `message.part.updated` | `{ sessionID, messageID, part }` | `session/message-v2.ts` |
| `message.part.removed` | `{ sessionID, messageID, partID }` | `session/message-v2.ts` |

### Permission Events

| Event Type | Properties | Source |
|------------|------------|--------|
| `permission.asked` | `{ sessionID, requestID, ... }` | `permission/next.ts` |
| `permission.replied` | `{ sessionID, requestID, reply }` | `permission/next.ts` |

Reply types: `"once"`, `"always"`, `"reject"`

### Question Events

| Event Type | Properties | Source |
|------------|------------|--------|
| `question.asked` | `{ sessionID, requestID, questions }` | `question/index.ts` |
| `question.replied` | `{ sessionID, requestID, answers }` | `question/index.ts` |
| `question.rejected` | `{ sessionID, requestID }` | `question/index.ts` |

### MCP Events

| Event Type | Properties | Source |
|------------|------------|--------|
| `mcp.tools.changed` | `{ server: string }` | `mcp/index.ts` |
| `mcp.browser.open.failed` | `{ mcpName, url }` | `mcp/index.ts` |

### Server Events

| Event Type | Properties | Source |
|------------|------------|--------|
| `server.connected` | `{}` | `server/server.ts` |
| `server.disposed` | `{}` | `server/server.ts` |

## Event Flow Patterns

### Request-Reply Pattern

Used by Permission and Question systems:

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   System    │     │     Bus     │     │    Client   │
└──────┬──────┘     └──────┬──────┘     └──────┬──────┘
       │                   │                   │
       │ publish(Asked)    │                   │
       │──────────────────>│                   │
       │                   │ SSE: Asked        │
       │                   │──────────────────>│
       │                   │                   │
       │                   │                   │ User decides
       │                   │                   │
       │                   │ publish(Replied)  │
       │                   │<──────────────────│
       │ callback(Replied) │                   │
       │<──────────────────│                   │
       │                   │                   │
```

### State Update Pattern

Used by Session and Message systems:

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Session    │     │     Bus     │     │     UI      │
└──────┬──────┘     └──────┬──────┘     └──────┬──────┘
       │                   │                   │
       │ session.update()  │                   │
       │ (internal state)  │                   │
       │                   │                   │
       │ publish(Updated)  │                   │
       │──────────────────>│                   │
       │                   │ SSE: Updated      │
       │                   │──────────────────>│
       │                   │                   │
       │                   │                   │ Re-render
```

## Event Property Schemas

### Session Events Detail

```typescript
// session.created / session.updated
z.object({
  id: z.string(),
  title: z.string().optional(),
  model: z.string().optional(),
  // ... additional session metadata
})

// session.diff
z.object({
  sessionID: z.string(),
  diffs: z.array(z.object({
    path: z.string(),
    type: z.enum(["add", "modify", "delete"]),
    // ... diff content
  }))
})
```

### Permission Events Detail

```typescript
// permission.asked
z.object({
  sessionID: z.string(),
  requestID: z.string(),
  tool: z.string(),
  input: z.record(z.any()),
  metadata: z.object({
    title: z.string(),
    description: z.string().optional(),
    // ...
  })
})

// permission.replied
z.object({
  sessionID: z.string(),
  requestID: z.string(),
  reply: z.enum(["once", "always", "reject"])
})
```

### Question Events Detail

```typescript
// question.asked
z.object({
  sessionID: z.string(),
  requestID: z.string(),
  questions: z.array(z.object({
    id: z.string(),
    text: z.string(),
    type: z.enum(["text", "select", "multiselect"]),
    options: z.array(z.string()).optional()
  }))
})

// question.replied
z.object({
  sessionID: z.string(),
  requestID: z.string(),
  answers: z.array(z.object({
    questionID: z.string(),
    value: z.union([z.string(), z.array(z.string())])
  }))
})
```

## Discovery Notes

Events are discovered by searching for `BusEvent.define` calls across the codebase. The catalog above represents events found through:

1. Direct inspection of bus-related source files
2. Inspection of major subsystems (session, permission, question, mcp, server)
3. Analysis of SSE stream handlers

Additional events likely exist in subsystems not yet examined:
- `snapshot/` - File snapshot events
- `lsp/` - Language server events
- `skill/` - Skill system events
- `file/` - File watcher events
