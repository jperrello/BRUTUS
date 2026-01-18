# OpenCode Share Subsystem Specification

## 1. Purpose

Enable sharing of coding sessions via URL. Viewers see conversations update in real-time without authentication.

## 2. Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  OpenCode CLI   │    │ Cloudflare API  │    │   Web Viewer    │
│  (share-next)   │───▶│  (DurableObj)   │◀───│  (WebSocket)    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                     │                      │
         │ POST /share         │                      │
         │ POST /share/sync    │                      │
         │                     ├──────────────────────┤
         │                     │  WebSocket Messages  │
         └─────────────────────┴──────────────────────┘
```

## 3. Data Model

### 3.1 ShareInfo (stored locally)
```typescript
interface ShareInfo {
  id: string      // Share ID (also used in URL)
  secret: string  // Authentication token
  url: string     // Full shareable URL
}
```

### 3.2 Syncable Data Types
```typescript
type SyncData =
  | { type: "session"; data: Session }
  | { type: "message"; data: Message }
  | { type: "part"; data: Part }
  | { type: "session_diff"; data: FileDiff[] }
  | { type: "model"; data: Model[] }
```

### 3.3 Session Structure
```typescript
interface Session {
  id: string
  slug: string
  projectID: string
  directory: string
  parentID?: string
  summary?: { diffs: FileDiff[]; stats: ChangeStats }
  share?: { url: string }
  title: string
  version: number
  time: { created: number; updated: number }
  permission?: PermissionRuleset
}
```

## 4. API Endpoints

### 4.1 Create Share
```
POST https://opncd.ai/api/share
Content-Type: application/json

Request:
{ "sessionID": "session_01JXXX..." }

Response:
{
  "id": "abc123",
  "url": "https://opncd.ai/s/abc123",
  "secret": "550e8400-e29b-41d4-a716-446655440000"
}
```

### 4.2 Sync Data
```
POST https://opncd.ai/api/share/{id}/sync
Content-Type: application/json

Request:
{
  "secret": "550e8400-e29b-41d4-a716-446655440000",
  "data": [
    { "type": "message", "data": { ... } },
    { "type": "part", "data": { ... } }
  ]
}

Response: {} (empty on success)
```

### 4.3 Delete Share
```
DELETE https://opncd.ai/api/share/{id}
Content-Type: application/json

Request:
{ "secret": "550e8400-e29b-41d4-a716-446655440000" }

Response: {} (empty on success)
```

## 5. Client-Side Logic (share-next.ts)

### 5.1 Initialization
```typescript
function init() {
  Bus.subscribe(Session.Event.Updated, sync session)
  Bus.subscribe(MessageV2.Event.Updated, sync message + model)
  Bus.subscribe(MessageV2.Event.PartUpdated, sync part)
  Bus.subscribe(Session.Event.Diff, sync file diffs)
}
```

### 5.2 Sync Queue Mechanism
```
┌─────────────────────────────────────────────────────┐
│ queue: Map<sessionID, {timeout, data: Map<id,Data>}>│
└─────────────────────────────────────────────────────┘
       │
       ▼
   sync(sessionID, data[])
       │
       ├─── Queue exists? Merge data into existing map
       │
       └─── No queue? Create with 1000ms timeout
                          │
                          ▼ (after 1000ms)
                     POST /share/{id}/sync
```

Key behaviors:
- **Debouncing**: 1000ms delay before flush
- **Deduplication**: Same ID overwrites previous version
- **Batching**: Multiple items sent in single request

### 5.3 Full Sync (on share create)
```typescript
async function fullSync(sessionID: string) {
  const session = await Session.get(sessionID)
  const diffs = await Session.diff(sessionID)
  const messages = await Array.fromAsync(MessageV2.stream(sessionID))
  const models = /* extract unique models from user messages */

  await sync(sessionID, [
    { type: "session", data: session },
    ...messages.map(m => ({ type: "message", data: m.info })),
    ...messages.flatMap(m => m.parts.map(p => ({ type: "part", data: p }))),
    { type: "session_diff", data: diffs },
    { type: "model", data: models }
  ])
}
```

## 6. Server-Side Logic (Cloudflare Durable Object)

### 6.1 SyncServer Class
```
┌────────────────────────────────────────┐
│          SyncServer (DurableObject)    │
├────────────────────────────────────────┤
│ state.storage  ─▶ session metadata     │
│ bucket (R2)    ─▶ large content        │
│ websockets[]   ─▶ connected viewers    │
├────────────────────────────────────────┤
│ share()        ─▶ create secret        │
│ publish()      ─▶ store + broadcast    │
│ getData()      ─▶ return all data      │
│ assertSecret() ─▶ validate auth        │
│ clear()        ─▶ delete all           │
└────────────────────────────────────────┘
```

### 6.2 Durable Object ID Derivation
```typescript
const shortName = sessionID.slice(-8)  // Last 8 chars
const id = env.SYNC.idFromName(shortName)
const stub = env.SYNC.get(id)
```

### 6.3 Storage Strategy
- **Durable Storage**: Small metadata, secrets
- **R2 Bucket**: Message content, parts (larger data)
- **Key Pattern**: `session/{type}/{sessionID}/{id}`

### 6.4 WebSocket Protocol
```
Client connects:
  1. Upgrade to WebSocket
  2. Server sends all stored data
  3. Server keeps connection open

On new data:
  1. API receives POST /share/{id}/sync
  2. SyncServer.publish() stores data
  3. SyncServer broadcasts to all websockets
```

## 7. Session Integration

### 7.1 Share Creation Flow
```
User: /share
    │
    ▼
Session.share(id)
    │
    ├─▶ Check config.share !== "disabled"
    │
    ├─▶ ShareNext.create(id)
    │       └─▶ POST /api/share
    │       └─▶ Storage.write(["session_share", id], result)
    │       └─▶ fullSync(id)
    │
    └─▶ Session.update(id, { share: { url } })
```

### 7.2 Unshare Flow
```
User: /unshare
    │
    ▼
Session.unshare(id)
    │
    ├─▶ ShareNext.remove(id)
    │       └─▶ Storage.read(["session_share", id])
    │       └─▶ DELETE /api/share/{id}
    │       └─▶ Storage.remove(["session_share", id])
    │
    └─▶ Session.update(id, { share: undefined })
```

## 8. Configuration

```typescript
// Enterprise can override share URL
const url = config.enterprise?.url ?? "https://opncd.ai"

// Sharing can be disabled
if (config.share === "disabled") {
  throw new Error("Sharing is disabled in configuration")
}
```

## 9. Security Model

| Actor | Capability |
|-------|------------|
| Session Owner | Has secret, can sync/delete |
| Viewer | WebSocket connect, read-only |
| API Admin | Can delete via admin secret |

The secret is:
- Generated server-side (UUID v4)
- Stored locally in `["session_share", sessionID]`
- Required for all write operations
- Never exposed to viewers

## 10. Legacy vs Next

Two implementations exist:

| Aspect | share.ts (legacy) | share-next.ts (current) |
|--------|-------------------|-------------------------|
| API URL | OPENCODE_API env or Installation.isPreview() | Config.enterprise?.url or opncd.ai |
| Endpoints | /share_create, /share_sync, /share_delete | /api/share, /api/share/{id}/sync |
| Storage | ["share", sessionID] | ["session_share", sessionID] |
| Full sync | None | fullSync() on create |
| Model tracking | No | Yes (for user messages) |

The legacy module is likely deprecated but still present for backwards compatibility.
