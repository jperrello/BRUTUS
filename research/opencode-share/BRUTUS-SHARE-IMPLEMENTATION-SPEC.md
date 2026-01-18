# BRUTUS Share Implementation Specification

## 1. Scope

Implement session sharing for BRUTUS that mirrors OpenCode's approach:
- Create shareable links via external service
- Real-time sync of conversation updates
- Secret-based authentication

## 2. Architecture Decision

### 2.1 Server Options

| Option | Pros | Cons |
|--------|------|------|
| Self-hosted | Full control, privacy | Ops burden, availability |
| Cloudflare Workers | Scales, cheap | Vendor lock-in |
| Saturn-based | Aligns with BRUTUS | Not designed for this |

**Recommendation**: Start with self-hosted Go server, deployable as single binary alongside BRUTUS. Can migrate to Cloudflare later if needed.

### 2.2 Storage Options

| Option | Pros | Cons |
|--------|------|------|
| SQLite | Simple, embedded | Single-node only |
| Turso | SQLite + replication | External dependency |
| Redis | Fast, pub/sub built-in | More ops |

**Recommendation**: SQLite for MVP, with abstraction layer for future migration.

## 3. Data Structures

### 3.1 Share Record
```go
type Share struct {
    ID        string    `json:"id"`
    SessionID string    `json:"session_id"`
    Secret    string    `json:"secret"`
    URL       string    `json:"url"`
    CreatedAt time.Time `json:"created_at"`
}
```

### 3.2 Sync Message
```go
type SyncData struct {
    Type string      `json:"type"`  // "session" | "message" | "part"
    Data interface{} `json:"data"`
}

type SyncPayload struct {
    Secret string     `json:"secret"`
    Data   []SyncData `json:"data"`
}
```

## 4. Client Implementation

### 4.1 File: `share/share.go`
```go
package share

type Manager struct {
    apiURL    string
    store     Storage      // Local storage for secrets
    bus       *bus.Bus     // Event subscriptions
    queue     *syncQueue   // Batching logic
}

func NewManager(apiURL string, store Storage, bus *bus.Bus) *Manager

func (m *Manager) Create(sessionID string) (*Share, error)
func (m *Manager) Remove(sessionID string) error
func (m *Manager) Init()  // Subscribe to bus events
```

### 4.2 Sync Queue
```go
type syncQueue struct {
    mu       sync.Mutex
    pending  map[string]*queueEntry  // sessionID -> entry
    interval time.Duration           // 1 second
}

type queueEntry struct {
    timer *time.Timer
    data  map[string]SyncData  // id -> data for dedup
}

func (q *syncQueue) Enqueue(sessionID string, items []SyncData)
func (q *syncQueue) flush(sessionID string)
```

### 4.3 Bus Integration
```go
func (m *Manager) Init() {
    m.bus.Subscribe("session.updated", func(e Event) {
        m.syncIfShared(e.SessionID, SyncData{Type: "session", Data: e.Session})
    })

    m.bus.Subscribe("message.updated", func(e Event) {
        m.syncIfShared(e.SessionID, SyncData{Type: "message", Data: e.Message})
    })

    m.bus.Subscribe("part.updated", func(e Event) {
        m.syncIfShared(e.SessionID, SyncData{Type: "part", Data: e.Part})
    })
}

func (m *Manager) syncIfShared(sessionID string, data SyncData) {
    share, err := m.store.GetShare(sessionID)
    if err != nil || share == nil {
        return  // Not shared, ignore
    }
    m.queue.Enqueue(sessionID, []SyncData{data})
}
```

## 5. Server Implementation

### 5.1 File: `cmd/brutus-share/main.go`
```go
package main

// Standalone share server, deployable separately

func main() {
    db := sqlite.Open("share.db")
    hub := websocket.NewHub()

    http.HandleFunc("POST /api/share", handleCreate)
    http.HandleFunc("POST /api/share/{id}/sync", handleSync)
    http.HandleFunc("DELETE /api/share/{id}", handleDelete)
    http.HandleFunc("GET /api/share/{id}/poll", hub.HandleWebSocket)

    http.ListenAndServe(":8090", nil)
}
```

### 5.2 WebSocket Hub
```go
type Hub struct {
    mu       sync.RWMutex
    sessions map[string]map[*websocket.Conn]bool  // shareID -> connections
}

func (h *Hub) Broadcast(shareID string, data []byte) {
    h.mu.RLock()
    defer h.mu.RUnlock()

    for conn := range h.sessions[shareID] {
        conn.WriteMessage(websocket.TextMessage, data)
    }
}
```

### 5.3 Database Schema
```sql
CREATE TABLE shares (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    secret TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE share_data (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    share_id TEXT NOT NULL REFERENCES shares(id),
    key TEXT NOT NULL,
    content TEXT NOT NULL,  -- JSON
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(share_id, key)
);
```

## 6. API Endpoints

### 6.1 Create Share
```
POST /api/share
{ "sessionID": "..." }

→ 200 OK
{ "id": "abc123", "url": "https://share.brutus.ai/s/abc123", "secret": "uuid" }
```

### 6.2 Sync Data
```
POST /api/share/{id}/sync
{ "secret": "...", "data": [...] }

→ 200 OK {}
→ 401 Unauthorized (bad secret)
→ 404 Not Found (share deleted)
```

### 6.3 Delete Share
```
DELETE /api/share/{id}
{ "secret": "..." }

→ 200 OK {}
```

### 6.4 WebSocket Poll
```
GET /api/share/{id}/poll
Upgrade: websocket

→ Initial: all stored data
→ Live: new data as JSON messages
```

## 7. Integration Points

### 7.1 Agent Hook
```go
// In agent/agent.go, when creating share
func (a *Agent) ShareSession() (*share.Share, error) {
    return a.shareManager.Create(a.session.ID)
}
```

### 7.2 Tool Addition
```go
// tools/share.go
var ShareSessionTool = NewTool[ShareInput]("share_session",
    "Create a shareable link for this session",
    func(ctx context.Context, input ShareInput) (string, error) {
        share, err := ctx.Agent.ShareSession()
        if err != nil {
            return "", err
        }
        return fmt.Sprintf("Session shared: %s", share.URL), nil
    },
)
```

## 8. Configuration

```go
type ShareConfig struct {
    Enabled  bool   `json:"enabled"`
    APIURL   string `json:"api_url"`   // e.g., "https://share.brutus.ai"
    LocalDir string `json:"local_dir"` // Where to store share secrets
}
```

## 9. Security Considerations

1. **Secret Generation**: Use crypto/rand for UUID v4
2. **Secret Storage**: Local file, mode 0600
3. **HTTPS**: Mandatory for production
4. **Rate Limiting**: On share creation (prevent abuse)
5. **Expiration**: Consider auto-expire after N days

## 10. Viewer Frontend (Future)

Minimal static site:
```
share-viewer/
├── index.html
├── app.js       # WebSocket client, markdown rendering
└── style.css
```

Can be served from same binary via embedded FS.

## 11. Implementation Order

1. **Phase 1**: Local share manager with queue logic
2. **Phase 2**: Standalone share server with SQLite
3. **Phase 3**: WebSocket broadcast
4. **Phase 4**: Static viewer frontend
5. **Phase 5**: Tool integration

## 12. Testing Strategy

```go
// share/share_test.go
func TestQueueDebounce(t *testing.T) {
    // Verify multiple updates within window are batched
}

func TestQueueDeduplication(t *testing.T) {
    // Verify same ID updates overwrite
}

func TestSyncOnlyIfShared(t *testing.T) {
    // Verify unshared sessions don't trigger sync
}
```

## 13. Dependencies

| Package | Purpose |
|---------|---------|
| gorilla/websocket | WebSocket handling |
| mattn/go-sqlite3 | SQLite driver |
| google/uuid | Secret generation |

All can be vendored to keep BRUTUS self-contained.
