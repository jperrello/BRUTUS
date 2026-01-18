# BRUTUS Session Implementation Specification

## Overview

This document provides implementation guidance for adding session management to BRUTUS based on reverse engineering of OpenCode's session subsystem.

## Current BRUTUS State

BRUTUS currently has:
- `agent/agent.go` - The core inference loop
- In-memory message handling
- No persistence between sessions
- No session forking/branching

## Proposed Architecture

### 1. Session Package Structure

```
session/
├── session.go      // Session entity and operations
├── message.go      // Message and part types
├── storage.go      // File-based persistence
├── id.go           // Monotonic ID generation
└── processor.go    // Stream processing (integrate with agent loop)
```

### 2. ID Generation (id.go)

```go
package session

import (
    "crypto/rand"
    "encoding/hex"
    "sync"
    "time"
)

const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

type IDPrefix string

const (
    PrefixSession IDPrefix = "ses"
    PrefixMessage IDPrefix = "msg"
    PrefixPart    IDPrefix = "prt"
)

var (
    idMu          sync.Mutex
    lastTimestamp int64
    counter       int64
)

func NewID(prefix IDPrefix, descending bool) string {
    idMu.Lock()
    defer idMu.Unlock()

    now := time.Now().UnixMilli()
    if now != lastTimestamp {
        lastTimestamp = now
        counter = 0
    }
    counter++

    // Pack timestamp (36 bits) + counter (12 bits) = 48 bits
    packed := uint64(now)*0x1000 + uint64(counter)

    if descending {
        packed = ^packed // Bitwise NOT for reverse order
    }

    // Encode to 6 bytes
    timeBytes := make([]byte, 6)
    for i := 0; i < 6; i++ {
        timeBytes[i] = byte((packed >> (40 - 8*i)) & 0xFF)
    }

    return string(prefix) + "_" + hex.EncodeToString(timeBytes) + randomBase62(14)
}

func randomBase62(length int) string {
    b := make([]byte, length)
    rand.Read(b)
    result := make([]byte, length)
    for i := 0; i < length; i++ {
        result[i] = base62Chars[b[i]%62]
    }
    return string(result)
}

// NewSessionID creates descending ID (newest first)
func NewSessionID() string {
    return NewID(PrefixSession, true)
}

// NewMessageID creates ascending ID (chronological)
func NewMessageID() string {
    return NewID(PrefixMessage, false)
}

// NewPartID creates ascending ID
func NewPartID() string {
    return NewID(PrefixPart, false)
}
```

### 3. Message Types (message.go)

```go
package session

import "time"

type Role string

const (
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
)

type Message struct {
    ID        string    `json:"id"`
    SessionID string    `json:"sessionID"`
    Role      Role      `json:"role"`
    Time      TimeInfo  `json:"time"`
    Agent     string    `json:"agent"`
    Model     ModelRef  `json:"model"`
    ParentID  string    `json:"parentID,omitempty"` // For assistant messages
    Tokens    TokenInfo `json:"tokens,omitempty"`
    Cost      float64   `json:"cost,omitempty"`
    Finish    string    `json:"finish,omitempty"`
    Error     *Error    `json:"error,omitempty"`
}

type ModelRef struct {
    ProviderID string `json:"providerID"`
    ModelID    string `json:"modelID"`
}

type TimeInfo struct {
    Created   int64 `json:"created"`
    Completed int64 `json:"completed,omitempty"`
}

type TokenInfo struct {
    Input     int `json:"input"`
    Output    int `json:"output"`
    Reasoning int `json:"reasoning"`
    Cache     struct {
        Read  int `json:"read"`
        Write int `json:"write"`
    } `json:"cache"`
}

// Part types
type PartType string

const (
    PartTypeText      PartType = "text"
    PartTypeTool      PartType = "tool"
    PartTypeFile      PartType = "file"
    PartTypeReasoning PartType = "reasoning"
    PartTypeSnapshot  PartType = "snapshot"
    PartTypePatch     PartType = "patch"
    PartTypeStepStart PartType = "step-start"
    PartTypeStepEnd   PartType = "step-finish"
)

type Part struct {
    ID        string   `json:"id"`
    SessionID string   `json:"sessionID"`
    MessageID string   `json:"messageID"`
    Type      PartType `json:"type"`

    // Type-specific fields (use interface{} or separate types)
    Text      string                 `json:"text,omitempty"`
    Tool      string                 `json:"tool,omitempty"`
    CallID    string                 `json:"callID,omitempty"`
    State     *ToolState             `json:"state,omitempty"`
    Metadata  map[string]interface{} `json:"metadata,omitempty"`
    Synthetic bool                   `json:"synthetic,omitempty"`
}

type ToolStatus string

const (
    ToolStatusPending   ToolStatus = "pending"
    ToolStatusRunning   ToolStatus = "running"
    ToolStatusCompleted ToolStatus = "completed"
    ToolStatusError     ToolStatus = "error"
)

type ToolState struct {
    Status   ToolStatus             `json:"status"`
    Input    map[string]interface{} `json:"input"`
    Output   string                 `json:"output,omitempty"`
    Error    string                 `json:"error,omitempty"`
    Title    string                 `json:"title,omitempty"`
    Metadata map[string]interface{} `json:"metadata,omitempty"`
    Time     struct {
        Start int64 `json:"start"`
        End   int64 `json:"end,omitempty"`
    } `json:"time"`
}
```

### 4. Session Entity (session.go)

```go
package session

import (
    "path/filepath"
    "time"
)

type Session struct {
    ID        string    `json:"id"`
    Slug      string    `json:"slug"`
    ProjectID string    `json:"projectID"`
    Directory string    `json:"directory"`
    Title     string    `json:"title"`
    Version   string    `json:"version"`
    ParentID  string    `json:"parentID,omitempty"`
    Time      SessionTime `json:"time"`
    Summary   *SessionSummary `json:"summary,omitempty"`
}

type SessionTime struct {
    Created  int64 `json:"created"`
    Updated  int64 `json:"updated"`
    Archived int64 `json:"archived,omitempty"`
}

type SessionSummary struct {
    Additions int `json:"additions"`
    Deletions int `json:"deletions"`
    Files     int `json:"files"`
}

type SessionManager struct {
    storage  *Storage
    projectID string
}

func NewSessionManager(dataDir, projectID string) *SessionManager {
    return &SessionManager{
        storage:   NewStorage(dataDir),
        projectID: projectID,
    }
}

func (m *SessionManager) Create(title string) (*Session, error) {
    now := time.Now().UnixMilli()
    sess := &Session{
        ID:        NewSessionID(),
        Slug:      generateSlug(),
        ProjectID: m.projectID,
        Title:     title,
        Version:   "0.1.0", // BRUTUS version
        Time: SessionTime{
            Created: now,
            Updated: now,
        },
    }

    if err := m.storage.WriteSession(sess); err != nil {
        return nil, err
    }
    return sess, nil
}

func (m *SessionManager) Get(id string) (*Session, error) {
    return m.storage.ReadSession(m.projectID, id)
}

func (m *SessionManager) List() ([]*Session, error) {
    return m.storage.ListSessions(m.projectID)
}

func (m *SessionManager) Fork(sourceID string, upToMessageID string) (*Session, error) {
    // 1. Create new session
    forked, err := m.Create("Fork of " + sourceID)
    if err != nil {
        return nil, err
    }
    forked.ParentID = sourceID

    // 2. Copy messages
    messages, err := m.storage.ListMessages(sourceID)
    if err != nil {
        return nil, err
    }

    idMap := make(map[string]string)
    for _, msg := range messages {
        if upToMessageID != "" && msg.ID >= upToMessageID {
            break
        }

        newID := NewMessageID()
        idMap[msg.ID] = newID

        newMsg := *msg
        newMsg.ID = newID
        newMsg.SessionID = forked.ID
        if newMsg.ParentID != "" {
            if mapped, ok := idMap[newMsg.ParentID]; ok {
                newMsg.ParentID = mapped
            }
        }

        if err := m.storage.WriteMessage(&newMsg); err != nil {
            return nil, err
        }

        // Copy parts
        parts, err := m.storage.ListParts(msg.ID)
        if err != nil {
            return nil, err
        }
        for _, part := range parts {
            newPart := *part
            newPart.ID = NewPartID()
            newPart.SessionID = forked.ID
            newPart.MessageID = newID
            if err := m.storage.WritePart(&newPart); err != nil {
                return nil, err
            }
        }
    }

    return forked, nil
}
```

### 5. Storage Layer (storage.go)

```go
package session

import (
    "encoding/json"
    "os"
    "path/filepath"
    "sync"
)

type Storage struct {
    baseDir string
    mu      sync.RWMutex
}

func NewStorage(baseDir string) *Storage {
    return &Storage{baseDir: baseDir}
}

func (s *Storage) path(parts ...string) string {
    return filepath.Join(append([]string{s.baseDir, "storage"}, parts...)...)
}

func (s *Storage) WriteSession(sess *Session) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    path := s.path("session", sess.ProjectID, sess.ID+".json")
    return s.writeJSON(path, sess)
}

func (s *Storage) ReadSession(projectID, sessionID string) (*Session, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()

    path := s.path("session", projectID, sessionID+".json")
    var sess Session
    if err := s.readJSON(path, &sess); err != nil {
        return nil, err
    }
    return &sess, nil
}

func (s *Storage) WriteMessage(msg *Message) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    path := s.path("message", msg.SessionID, msg.ID+".json")
    return s.writeJSON(path, msg)
}

func (s *Storage) WritePart(part *Part) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    path := s.path("part", part.MessageID, part.ID+".json")
    return s.writeJSON(path, part)
}

func (s *Storage) ListSessions(projectID string) ([]*Session, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()

    dir := s.path("session", projectID)
    entries, err := os.ReadDir(dir)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, nil
        }
        return nil, err
    }

    var sessions []*Session
    for _, entry := range entries {
        if entry.IsDir() {
            continue
        }
        var sess Session
        if err := s.readJSON(filepath.Join(dir, entry.Name()), &sess); err != nil {
            continue
        }
        sessions = append(sessions, &sess)
    }
    return sessions, nil
}

func (s *Storage) ListMessages(sessionID string) ([]*Message, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()

    dir := s.path("message", sessionID)
    entries, err := os.ReadDir(dir)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, nil
        }
        return nil, err
    }

    var messages []*Message
    for _, entry := range entries {
        if entry.IsDir() {
            continue
        }
        var msg Message
        if err := s.readJSON(filepath.Join(dir, entry.Name()), &msg); err != nil {
            continue
        }
        messages = append(messages, &msg)
    }
    return messages, nil
}

func (s *Storage) ListParts(messageID string) ([]*Part, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()

    dir := s.path("part", messageID)
    entries, err := os.ReadDir(dir)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, nil
        }
        return nil, err
    }

    var parts []*Part
    for _, entry := range entries {
        if entry.IsDir() {
            continue
        }
        var part Part
        if err := s.readJSON(filepath.Join(dir, entry.Name()), &part); err != nil {
            continue
        }
        parts = append(parts, &part)
    }
    return parts, nil
}

func (s *Storage) writeJSON(path string, v interface{}) error {
    if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
        return err
    }
    data, err := json.MarshalIndent(v, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(path, data, 0644)
}

func (s *Storage) readJSON(path string, v interface{}) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return err
    }
    return json.Unmarshal(data, v)
}
```

## 6. Integration with Agent Loop

Modify `agent/agent.go` to use session management:

```go
// In agent.go

type Agent struct {
    // ... existing fields
    session     *session.SessionManager
    currentSession *session.Session
    currentMessage *session.Message
}

func (a *Agent) StartSession(title string) error {
    sess, err := a.session.Create(title)
    if err != nil {
        return err
    }
    a.currentSession = sess
    return nil
}

func (a *Agent) ResumeSession(sessionID string) error {
    sess, err := a.session.Get(sessionID)
    if err != nil {
        return err
    }
    a.currentSession = sess

    // Load message history
    messages, err := a.session.storage.ListMessages(sessionID)
    if err != nil {
        return err
    }

    // Reconstruct conversation state
    for _, msg := range messages {
        parts, _ := a.session.storage.ListParts(msg.ID)
        // Add to agent's message history...
    }

    return nil
}

// Modify the inference loop to persist messages and parts
func (a *Agent) processUserInput(input string) {
    // Create user message
    userMsg := &session.Message{
        ID:        session.NewMessageID(),
        SessionID: a.currentSession.ID,
        Role:      session.RoleUser,
        Time:      session.TimeInfo{Created: time.Now().UnixMilli()},
        Agent:     a.config.Agent,
        Model:     session.ModelRef{ProviderID: a.config.Provider, ModelID: a.config.Model},
    }
    a.session.storage.WriteMessage(userMsg)

    // Create text part
    textPart := &session.Part{
        ID:        session.NewPartID(),
        SessionID: a.currentSession.ID,
        MessageID: userMsg.ID,
        Type:      session.PartTypeText,
        Text:      input,
    }
    a.session.storage.WritePart(textPart)

    a.currentMessage = userMsg
}
```

## 7. CLI Integration

Add session commands to BRUTUS CLI:

```go
// cmd/session.go

var sessionCmd = &cobra.Command{
    Use:   "session",
    Short: "Manage sessions",
}

var sessionListCmd = &cobra.Command{
    Use:   "list",
    Short: "List all sessions",
    Run: func(cmd *cobra.Command, args []string) {
        mgr := session.NewSessionManager(dataDir, projectID)
        sessions, _ := mgr.List()
        for _, s := range sessions {
            fmt.Printf("%s  %s  %s\n", s.ID, s.Title, time.UnixMilli(s.Time.Created).Format(time.RFC3339))
        }
    },
}

var sessionResumeCmd = &cobra.Command{
    Use:   "resume <session-id>",
    Short: "Resume a session",
    Run: func(cmd *cobra.Command, args []string) {
        if len(args) < 1 {
            fmt.Println("Session ID required")
            return
        }
        agent.ResumeSession(args[0])
        agent.Run()
    },
}

var sessionForkCmd = &cobra.Command{
    Use:   "fork <session-id>",
    Short: "Fork a session",
    Run: func(cmd *cobra.Command, args []string) {
        mgr := session.NewSessionManager(dataDir, projectID)
        forked, _ := mgr.Fork(args[0], "")
        fmt.Printf("Created fork: %s\n", forked.ID)
    },
}
```

## 8. Migration Path

### Phase 1: Storage Foundation
1. Implement `id.go` with monotonic ID generation
2. Implement `storage.go` with JSON file persistence
3. Add basic session and message types

### Phase 2: Session Management
1. Implement `session.go` with CRUD operations
2. Add session forking
3. Integrate with agent loop for persistence

### Phase 3: Advanced Features
1. Part streaming (real-time persistence)
2. Tool state tracking
3. Cost/token accumulation
4. Compaction support

### Phase 4: CLI/GUI Integration
1. Session list/resume commands
2. Fork/branch support
3. Session sharing (optional)

## 9. Key Differences from OpenCode

| Aspect | OpenCode | BRUTUS Recommendation |
|--------|----------|----------------------|
| Language | TypeScript | Go |
| Runtime | Bun | Standard Go |
| Async | Promises/async-await | Goroutines/channels |
| Schema | Zod | Go structs with JSON tags |
| Events | BusEvent | Go channels or interface callbacks |
| Locking | Bun file locks | `sync.RWMutex` |

## 10. Testing Strategy

```go
func TestIDGeneration(t *testing.T) {
    // Test monotonic ordering
    ids := make([]string, 1000)
    for i := 0; i < 1000; i++ {
        ids[i] = NewMessageID()
    }
    for i := 1; i < len(ids); i++ {
        if ids[i] <= ids[i-1] {
            t.Errorf("Non-monotonic: %s <= %s", ids[i], ids[i-1])
        }
    }

    // Test descending for sessions
    sessIDs := make([]string, 100)
    for i := 0; i < 100; i++ {
        sessIDs[i] = NewSessionID()
        time.Sleep(time.Millisecond)
    }
    for i := 1; i < len(sessIDs); i++ {
        if sessIDs[i] >= sessIDs[i-1] {
            t.Errorf("Sessions should be descending: %s >= %s", sessIDs[i], sessIDs[i-1])
        }
    }
}

func TestSessionFork(t *testing.T) {
    mgr := NewSessionManager(t.TempDir(), "test-project")

    // Create and populate session
    sess, _ := mgr.Create("Original")
    // ... add messages

    // Fork
    forked, _ := mgr.Fork(sess.ID, "")

    // Verify fork
    if forked.ParentID != sess.ID {
        t.Errorf("Fork parent should be original")
    }
}
```
