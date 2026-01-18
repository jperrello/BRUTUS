# OpenCode Share Sync Protocol Specification

## 1. Overview

Real-time synchronization of coding sessions via WebSocket, backed by Cloudflare Durable Objects.

## 2. Wire Protocol

### 2.1 Connection Establishment
```
GET wss://api.opencode.ai/share_poll?sessionID={short_id}
Upgrade: websocket
Connection: Upgrade
```

The `short_id` is the last 8 characters of the session ID.

### 2.2 Initial Data Load
Upon connection, server sends all stored data:
```json
{
  "session/info/{sessionID}": { /* Session object */ },
  "session/message/{sessionID}/{msgID}": { /* Message object */ },
  "session/part/{sessionID}/{msgID}/{partID}": { /* Part object */ }
}
```

### 2.3 Live Updates
As new data arrives via POST /share/{id}/sync:
```json
{
  "key": "session/message/{sessionID}/{msgID}",
  "content": { /* Message object */ }
}
```

## 3. Key Schema

```
session/
├── info/{sessionID}                    → Session metadata
├── message/{sessionID}/{messageID}     → Message envelope
├── part/{sessionID}/{messageID}/{partID} → Message content parts
└── share/{sessionID}                   → (excluded from sync)
```

## 4. Publish Flow (Server)

```typescript
async publish(key: string, content: any) {
  // Validate key format
  const [root, ...rest] = key.split("/")
  if (root !== "session") return

  // Store in R2 bucket for persistence
  await bucket.put(key, JSON.stringify(content))

  // Store in durable storage for fast retrieval
  await state.storage.put(key, content)

  // Broadcast to all connected WebSocket clients
  for (const ws of this.ctx.getWebSockets()) {
    ws.send(JSON.stringify({ key, content }))
  }
}
```

## 5. Sync Queue (Client)

### 5.1 Data Structure
```typescript
const queue = new Map<string, {
  timeout: NodeJS.Timeout
  data: Map<string, Data>  // Keyed by item ID for deduplication
}>()
```

### 5.2 Queue Algorithm
```
sync(sessionID, dataItems[])
    │
    ├─ if queue.has(sessionID):
    │     for item in dataItems:
    │         queue.get(sessionID).data.set(item.id, item)
    │     return  // Timer already running
    │
    └─ else:
          dataMap = new Map()
          for item in dataItems:
              dataMap.set(item.id ?? ulid(), item)

          timer = setTimeout(1000ms, () => {
              items = queue.get(sessionID).data.values()
              queue.delete(sessionID)
              POST /api/share/{shareId}/sync with items
          })

          queue.set(sessionID, {timeout: timer, data: dataMap})
```

### 5.3 Timing Guarantees
- **Debounce**: 1000ms minimum between flushes per session
- **Batching**: All updates within window combined
- **Ordering**: Within batch, arbitrary order (clients must handle)
- **Idempotency**: Same ID overwrites previous; safe to retry

## 6. Data Types

### 6.1 Session Sync
```typescript
{
  type: "session"
  data: {
    id: string
    slug: string
    projectID: string
    directory: string
    title: string
    version: number
    time: { created: number, updated: number }
    share?: { url: string }
  }
}
```

### 6.2 Message Sync
```typescript
{
  type: "message"
  data: {
    id: string
    sessionID: string
    role: "user" | "assistant"
    model: { providerID: string, modelID: string }
    time: number
    // ... additional fields
  }
}
```

### 6.3 Part Sync
```typescript
{
  type: "part"
  data: {
    id: string
    sessionID: string
    messageID: string
    type: "text" | "tool-invocation" | "tool-result" | ...
    content: any  // Type-specific content
  }
}
```

### 6.4 File Diff Sync
```typescript
{
  type: "session_diff"
  data: Array<{
    path: string
    additions: number
    deletions: number
    patch?: string
  }>
}
```

### 6.5 Model Info Sync
```typescript
{
  type: "model"
  data: Array<{
    id: string
    providerID: string
    name: string
    // ... model metadata
  }>
}
```

## 7. Error Handling

### 7.1 Client-Side
```typescript
// Secret validation failure
POST /api/share/{id}/sync
  → 401 Unauthorized
  → Client should: clear local share, notify user

// Share not found
POST /api/share/{id}/sync
  → 404 Not Found
  → Client should: clear local share, stop syncing

// Network failure
  → Retry with exponential backoff
  → Data remains in queue until successful
```

### 7.2 Server-Side
```typescript
// Invalid key format
publish("invalid-key", data)
  → Silently ignored (no storage, no broadcast)

// Storage failure
  → HTTP 500 returned
  → Client should retry
```

## 8. WebSocket Lifecycle

```
┌─────────────────────────────────────────────────────┐
│                  VIEWER CLIENT                       │
├─────────────────────────────────────────────────────┤
│ 1. Connect to /share_poll?sessionID=XXXX            │
│ 2. Receive initial data dump                        │
│ 3. Listen for incremental updates                   │
│ 4. Reconnect on disconnect (with fresh data load)   │
└─────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│                  DURABLE OBJECT                      │
├─────────────────────────────────────────────────────┤
│ • Maintains set of active WebSocket connections     │
│ • Hibernates when no connections (cost saving)      │
│ • Wakes on new connection or HTTP request           │
│ • Broadcasts to all on publish()                    │
└─────────────────────────────────────────────────────┘
```

## 9. Consistency Model

| Property | Guarantee |
|----------|-----------|
| Order | Updates may arrive out-of-order; use version/timestamp |
| Durability | Data persisted before broadcast |
| Exactly-once delivery | Not guaranteed; clients must be idempotent |
| Catch-up | New connections receive full state |

## 10. Bandwidth Optimization

The queue mechanism provides:
1. **Debouncing**: Rapid edits combined (typing, streaming)
2. **Deduplication**: Same message part updated once per window
3. **Batching**: Single HTTP request per flush

Example savings:
```
Without queue:
  100 part updates → 100 HTTP requests

With queue (1s window):
  100 part updates → 1 HTTP request with final states
```
