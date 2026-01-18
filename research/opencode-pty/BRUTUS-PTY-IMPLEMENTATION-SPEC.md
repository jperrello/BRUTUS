# BRUTUS PTY Implementation Specification

## Overview

This document specifies how to implement OpenCode's PTY subsystem in Go for BRUTUS. The PTY subsystem provides interactive terminal sessions separate from the non-interactive bash tool.

## Dependencies

### Required Packages

```go
// Cross-platform PTY
"github.com/creack/pty"                 // Unix (Linux, macOS)
"github.com/ActiveState/termtest/conpty" // Windows (optional)

// WebSocket
"github.com/gorilla/websocket"          // Or stdlib golang.org/x/net/websocket

// Utilities
"github.com/oklog/ulid/v2"              // For monotonic IDs (or custom impl)
```

### Alternative: Use Bun's Native PTY (via FFI)

If BRUTUS runs embedded in a Bun process, consider using `bun-pty` via FFI rather than reimplementing.

## Data Structures

### pty/types.go

```go
package pty

import (
    "sync"
    "time"

    "github.com/gorilla/websocket"
)

type Status string

const (
    StatusRunning Status = "running"
    StatusExited  Status = "exited"
)

type Info struct {
    ID      string   `json:"id"`
    Title   string   `json:"title"`
    Command string   `json:"command"`
    Args    []string `json:"args"`
    Cwd     string   `json:"cwd"`
    Status  Status   `json:"status"`
    PID     int      `json:"pid"`
}

type CreateInput struct {
    Command string            `json:"command,omitempty"`
    Args    []string          `json:"args,omitempty"`
    Cwd     string            `json:"cwd,omitempty"`
    Title   string            `json:"title,omitempty"`
    Env     map[string]string `json:"env,omitempty"`
}

type UpdateInput struct {
    Title *string `json:"title,omitempty"`
    Size  *Size   `json:"size,omitempty"`
}

type Size struct {
    Rows int `json:"rows"`
    Cols int `json:"cols"`
}

type activeSession struct {
    info        Info
    pty         *os.File              // PTY file descriptor (Unix)
    cmd         *exec.Cmd             // Process handle
    buffer      *ringBuffer           // Output buffer
    subscribers map[*websocket.Conn]struct{}
    mu          sync.RWMutex
    done        chan struct{}
}
```

### pty/buffer.go

```go
package pty

const (
    BufferLimit = 2 * 1024 * 1024  // 2MB
    BufferChunk = 64 * 1024        // 64KB
)

type ringBuffer struct {
    data []byte
    mu   sync.Mutex
}

func newRingBuffer() *ringBuffer {
    return &ringBuffer{
        data: make([]byte, 0, BufferLimit),
    }
}

func (b *ringBuffer) Write(p []byte) {
    b.mu.Lock()
    defer b.mu.Unlock()

    b.data = append(b.data, p...)
    if len(b.data) > BufferLimit {
        // Truncate from front (keep recent)
        b.data = b.data[len(b.data)-BufferLimit:]
    }
}

func (b *ringBuffer) Flush() []byte {
    b.mu.Lock()
    defer b.mu.Unlock()

    data := b.data
    b.data = make([]byte, 0, BufferLimit)
    return data
}

func (b *ringBuffer) Len() int {
    b.mu.Lock()
    defer b.mu.Unlock()
    return len(b.data)
}
```

## Core Implementation

### pty/manager.go

```go
package pty

import (
    "context"
    "fmt"
    "io"
    "os"
    "os/exec"
    "sync"

    "github.com/creack/pty"
    "github.com/gorilla/websocket"
)

type Manager struct {
    sessions map[string]*activeSession
    mu       sync.RWMutex
    eventBus EventPublisher
    idGen    IDGenerator
}

type EventPublisher interface {
    Publish(eventType string, payload any)
}

type IDGenerator interface {
    Create(prefix string) string
}

func NewManager(eventBus EventPublisher, idGen IDGenerator) *Manager {
    return &Manager{
        sessions: make(map[string]*activeSession),
        eventBus: eventBus,
        idGen:    idGen,
    }
}

func (m *Manager) List() []Info {
    m.mu.RLock()
    defer m.mu.RUnlock()

    result := make([]Info, 0, len(m.sessions))
    for _, s := range m.sessions {
        result = append(result, s.info)
    }
    return result
}

func (m *Manager) Get(id string) (*Info, bool) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    s, ok := m.sessions[id]
    if !ok {
        return nil, false
    }
    return &s.info, true
}

func (m *Manager) Create(ctx context.Context, input CreateInput, instanceDir string) (*Info, error) {
    id := m.idGen.Create("pty")

    // Resolve shell
    command := input.Command
    if command == "" {
        command = preferredShell()
    }

    args := input.Args
    if isShellCommand(command) && len(args) == 0 {
        args = []string{"-l"} // Login shell
    }

    cwd := input.Cwd
    if cwd == "" {
        cwd = instanceDir
    }

    // Build environment
    env := os.Environ()
    for k, v := range input.Env {
        env = append(env, fmt.Sprintf("%s=%s", k, v))
    }
    env = append(env, "TERM=xterm-256color")

    // Create command
    cmd := exec.CommandContext(ctx, command, args...)
    cmd.Dir = cwd
    cmd.Env = env

    // Start PTY
    ptmx, err := pty.Start(cmd)
    if err != nil {
        return nil, fmt.Errorf("failed to start pty: %w", err)
    }

    title := input.Title
    if title == "" {
        title = fmt.Sprintf("Terminal %s", id[len(id)-4:])
    }

    info := Info{
        ID:      id,
        Title:   title,
        Command: command,
        Args:    args,
        Cwd:     cwd,
        Status:  StatusRunning,
        PID:     cmd.Process.Pid,
    }

    session := &activeSession{
        info:        info,
        pty:         ptmx,
        cmd:         cmd,
        buffer:      newRingBuffer(),
        subscribers: make(map[*websocket.Conn]struct{}),
        done:        make(chan struct{}),
    }

    m.mu.Lock()
    m.sessions[id] = session
    m.mu.Unlock()

    // Start output reader
    go m.readOutput(session)

    // Start process waiter
    go m.waitProcess(session)

    m.eventBus.Publish("pty.created", map[string]any{"info": info})

    return &info, nil
}

func (m *Manager) readOutput(s *activeSession) {
    buf := make([]byte, 4096)
    for {
        n, err := s.pty.Read(buf)
        if err != nil {
            if err != io.EOF {
                // Log error
            }
            return
        }

        data := buf[:n]

        s.mu.RLock()
        hasSubscribers := len(s.subscribers) > 0
        s.mu.RUnlock()

        if hasSubscribers {
            m.broadcast(s, data)
        } else {
            s.buffer.Write(data)
        }
    }
}

func (m *Manager) broadcast(s *activeSession, data []byte) {
    s.mu.Lock()
    defer s.mu.Unlock()

    for conn := range s.subscribers {
        err := conn.WriteMessage(websocket.TextMessage, data)
        if err != nil {
            conn.Close()
            delete(s.subscribers, conn)
        }
    }
}

func (m *Manager) waitProcess(s *activeSession) {
    err := s.cmd.Wait()

    exitCode := 0
    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            exitCode = exitErr.ExitCode()
        }
    }

    s.mu.Lock()
    s.info.Status = StatusExited
    s.mu.Unlock()

    m.eventBus.Publish("pty.exited", map[string]any{
        "id":       s.info.ID,
        "exitCode": exitCode,
    })

    m.mu.Lock()
    delete(m.sessions, s.info.ID)
    m.mu.Unlock()

    close(s.done)
}

func (m *Manager) Update(id string, input UpdateInput) (*Info, error) {
    m.mu.RLock()
    s, ok := m.sessions[id]
    m.mu.RUnlock()

    if !ok {
        return nil, fmt.Errorf("session not found")
    }

    s.mu.Lock()
    defer s.mu.Unlock()

    if input.Title != nil {
        s.info.Title = *input.Title
    }

    if input.Size != nil {
        pty.Setsize(s.pty, &pty.Winsize{
            Rows: uint16(input.Size.Rows),
            Cols: uint16(input.Size.Cols),
        })
    }

    m.eventBus.Publish("pty.updated", map[string]any{"info": s.info})

    return &s.info, nil
}

func (m *Manager) Remove(id string) error {
    m.mu.Lock()
    s, ok := m.sessions[id]
    if !ok {
        m.mu.Unlock()
        return fmt.Errorf("session not found")
    }
    delete(m.sessions, id)
    m.mu.Unlock()

    // Kill process
    if s.cmd.Process != nil {
        s.cmd.Process.Kill()
    }
    s.pty.Close()

    // Close all subscribers
    s.mu.Lock()
    for conn := range s.subscribers {
        conn.Close()
    }
    s.mu.Unlock()

    m.eventBus.Publish("pty.deleted", map[string]any{"id": id})

    return nil
}

func (m *Manager) Write(id string, data []byte) error {
    m.mu.RLock()
    s, ok := m.sessions[id]
    m.mu.RUnlock()

    if !ok {
        return fmt.Errorf("session not found")
    }

    s.mu.RLock()
    if s.info.Status != StatusRunning {
        s.mu.RUnlock()
        return fmt.Errorf("session not running")
    }
    s.mu.RUnlock()

    _, err := s.pty.Write(data)
    return err
}

func (m *Manager) Resize(id string, cols, rows int) error {
    m.mu.RLock()
    s, ok := m.sessions[id]
    m.mu.RUnlock()

    if !ok {
        return fmt.Errorf("session not found")
    }

    s.mu.RLock()
    if s.info.Status != StatusRunning {
        s.mu.RUnlock()
        return fmt.Errorf("session not running")
    }
    s.mu.RUnlock()

    return pty.Setsize(s.pty, &pty.Winsize{
        Rows: uint16(rows),
        Cols: uint16(cols),
    })
}

func (m *Manager) Connect(id string, conn *websocket.Conn) error {
    m.mu.RLock()
    s, ok := m.sessions[id]
    m.mu.RUnlock()

    if !ok {
        conn.Close()
        return fmt.Errorf("session not found")
    }

    // Flush buffer
    buffered := s.buffer.Flush()
    if len(buffered) > 0 {
        // Send in chunks
        for i := 0; i < len(buffered); i += BufferChunk {
            end := i + BufferChunk
            if end > len(buffered) {
                end = len(buffered)
            }
            if err := conn.WriteMessage(websocket.TextMessage, buffered[i:end]); err != nil {
                // Restore buffer and close
                s.buffer.Write(buffered)
                conn.Close()
                return err
            }
        }
    }

    // Add subscriber
    s.mu.Lock()
    s.subscribers[conn] = struct{}{}
    s.mu.Unlock()

    return nil
}

func (m *Manager) Disconnect(id string, conn *websocket.Conn) {
    m.mu.RLock()
    s, ok := m.sessions[id]
    m.mu.RUnlock()

    if !ok {
        return
    }

    s.mu.Lock()
    delete(s.subscribers, conn)
    s.mu.Unlock()
}

func (m *Manager) Dispose() {
    m.mu.Lock()
    defer m.mu.Unlock()

    for id, s := range m.sessions {
        if s.cmd.Process != nil {
            s.cmd.Process.Kill()
        }
        s.pty.Close()

        s.mu.Lock()
        for conn := range s.subscribers {
            conn.Close()
        }
        s.mu.Unlock()

        delete(m.sessions, id)
    }
}
```

### pty/shell.go

```go
package pty

import (
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
    "strings"
)

var blacklistedShells = map[string]bool{
    "fish": true,
    "nu":   true,
}

func preferredShell() string {
    if shell := os.Getenv("SHELL"); shell != "" {
        return shell
    }
    return fallbackShell()
}

func acceptableShell() string {
    shell := os.Getenv("SHELL")
    if shell != "" {
        name := filepath.Base(shell)
        if runtime.GOOS == "windows" {
            name = strings.TrimSuffix(name, ".exe")
        }
        if !blacklistedShells[name] {
            return shell
        }
    }
    return fallbackShell()
}

func fallbackShell() string {
    switch runtime.GOOS {
    case "windows":
        // Try Git Bash
        if gitBash := os.Getenv("OPENCODE_GIT_BASH_PATH"); gitBash != "" {
            return gitBash
        }
        if git, err := exec.LookPath("git"); err == nil {
            bash := filepath.Join(filepath.Dir(filepath.Dir(git)), "bin", "bash.exe")
            if _, err := os.Stat(bash); err == nil {
                return bash
            }
        }
        if comspec := os.Getenv("COMSPEC"); comspec != "" {
            return comspec
        }
        return "cmd.exe"

    case "darwin":
        return "/bin/zsh"

    default: // Linux
        if bash, err := exec.LookPath("bash"); err == nil {
            return bash
        }
        return "/bin/sh"
    }
}

func isShellCommand(cmd string) bool {
    base := filepath.Base(cmd)
    if runtime.GOOS == "windows" {
        base = strings.TrimSuffix(base, ".exe")
    }
    return strings.HasSuffix(base, "sh")
}
```

## HTTP Routes

### pty/routes.go

```go
package pty

import (
    "encoding/json"
    "net/http"

    "github.com/gorilla/mux"
    "github.com/gorilla/websocket"
)

type Routes struct {
    manager  *Manager
    upgrader websocket.Upgrader
}

func NewRoutes(manager *Manager) *Routes {
    return &Routes{
        manager: manager,
        upgrader: websocket.Upgrader{
            CheckOrigin: func(r *http.Request) bool { return true },
        },
    }
}

func (r *Routes) Register(router *mux.Router) {
    router.HandleFunc("/pty", r.list).Methods("GET")
    router.HandleFunc("/pty", r.create).Methods("POST")
    router.HandleFunc("/pty/{id}", r.get).Methods("GET")
    router.HandleFunc("/pty/{id}", r.update).Methods("PUT")
    router.HandleFunc("/pty/{id}", r.remove).Methods("DELETE")
    router.HandleFunc("/pty/{id}/connect", r.connect).Methods("GET")
}

func (r *Routes) list(w http.ResponseWriter, req *http.Request) {
    sessions := r.manager.List()
    json.NewEncoder(w).Encode(sessions)
}

func (r *Routes) create(w http.ResponseWriter, req *http.Request) {
    var input CreateInput
    if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    info, err := r.manager.Create(req.Context(), input, "/") // TODO: get instance dir
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(info)
}

func (r *Routes) get(w http.ResponseWriter, req *http.Request) {
    id := mux.Vars(req)["id"]
    info, ok := r.manager.Get(id)
    if !ok {
        http.Error(w, "session not found", http.StatusNotFound)
        return
    }
    json.NewEncoder(w).Encode(info)
}

func (r *Routes) update(w http.ResponseWriter, req *http.Request) {
    id := mux.Vars(req)["id"]

    var input UpdateInput
    if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    info, err := r.manager.Update(id, input)
    if err != nil {
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }

    json.NewEncoder(w).Encode(info)
}

func (r *Routes) remove(w http.ResponseWriter, req *http.Request) {
    id := mux.Vars(req)["id"]
    if err := r.manager.Remove(id); err != nil {
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }
    json.NewEncoder(w).Encode(true)
}

func (r *Routes) connect(w http.ResponseWriter, req *http.Request) {
    id := mux.Vars(req)["id"]

    conn, err := r.upgrader.Upgrade(w, req, nil)
    if err != nil {
        return
    }
    defer conn.Close()

    if err := r.manager.Connect(id, conn); err != nil {
        return
    }
    defer r.manager.Disconnect(id, conn)

    // Read loop - forward input to PTY
    for {
        _, message, err := conn.ReadMessage()
        if err != nil {
            break
        }
        if err := r.manager.Write(id, message); err != nil {
            break
        }
    }
}
```

## ID Generation

### id/id.go

```go
package id

import (
    "crypto/rand"
    "encoding/binary"
    "sync"
    "time"
)

const (
    base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
    idLength    = 26
)

var prefixes = map[string]string{
    "session":    "ses",
    "message":    "msg",
    "permission": "per",
    "question":   "que",
    "user":       "usr",
    "part":       "prt",
    "pty":        "pty",
    "tool":       "tool",
}

type Generator struct {
    mu        sync.Mutex
    lastTS    int64
    counter   int64
}

func NewGenerator() *Generator {
    return &Generator{}
}

func (g *Generator) Create(prefix string) string {
    p, ok := prefixes[prefix]
    if !ok {
        p = prefix
    }

    g.mu.Lock()
    defer g.mu.Unlock()

    now := time.Now().UnixMilli()
    if now != g.lastTS {
        g.lastTS = now
        g.counter = 0
    }
    g.counter++

    encoded := (now << 12) | (g.counter & 0xFFF)

    // Encode timestamp bytes
    timeBytes := make([]byte, 6)
    for i := 0; i < 6; i++ {
        timeBytes[i] = byte((encoded >> (40 - 8*i)) & 0xFF)
    }

    // Random suffix
    randomBytes := make([]byte, idLength-12)
    rand.Read(randomBytes)

    result := p + "_"
    for _, b := range timeBytes {
        result += fmt.Sprintf("%02x", b)
    }
    for _, b := range randomBytes {
        result += string(base62Chars[int(b)%62])
    }

    return result
}
```

## Integration Points

### With Event Bus

```go
type eventBus struct {
    subscribers map[string][]func(any)
    mu          sync.RWMutex
}

func (b *eventBus) Publish(eventType string, payload any) {
    b.mu.RLock()
    defer b.mu.RUnlock()

    for _, sub := range b.subscribers[eventType] {
        go sub(payload)
    }
    for _, sub := range b.subscribers["*"] {
        go sub(map[string]any{"type": eventType, "properties": payload})
    }
}
```

### With Instance Manager

```go
type instanceManager struct {
    instances map[string]*instance
    mu        sync.RWMutex
}

type instance struct {
    directory string
    ptyMgr    *pty.Manager
    // ... other subsystems
}

func (m *instanceManager) Dispose(directory string) {
    m.mu.Lock()
    inst, ok := m.instances[directory]
    if ok {
        delete(m.instances, directory)
    }
    m.mu.Unlock()

    if ok {
        inst.ptyMgr.Dispose()
    }
}
```

## Windows Support

For Windows, use `conpty` instead of `creack/pty`:

```go
//go:build windows

package pty

import (
    "github.com/ActiveState/termtest/conpty"
)

func startPTY(cmd *exec.Cmd) (*os.File, error) {
    cpty, err := conpty.New(80, 24)
    if err != nil {
        return nil, err
    }

    if err := cpty.Start(cmd); err != nil {
        return nil, err
    }

    return cpty.InPipe(), nil
}
```

## Testing

### Unit Tests

```go
func TestManager_Create(t *testing.T) {
    bus := &mockEventBus{}
    idGen := &mockIDGenerator{next: "pty_test123"}
    mgr := NewManager(bus, idGen)

    info, err := mgr.Create(context.Background(), CreateInput{
        Command: "echo",
        Args:    []string{"hello"},
    }, "/tmp")

    require.NoError(t, err)
    assert.Equal(t, "pty_test123", info.ID)
    assert.Equal(t, StatusRunning, info.Status)

    // Wait for process to exit
    time.Sleep(100 * time.Millisecond)

    _, ok := mgr.Get(info.ID)
    assert.False(t, ok) // Should be removed after exit
}
```

## Implementation Priority

1. **Phase 1: Basic PTY** - Create, list, get, remove (no WebSocket)
2. **Phase 2: WebSocket** - Connect, read/write streaming
3. **Phase 3: Buffer** - Output buffering when no clients
4. **Phase 4: Events** - Integration with event bus
5. **Phase 5: Shell Detection** - Cross-platform shell resolution
6. **Phase 6: Windows** - ConPTY support
