# BRUTUS Server Implementation Specification

## Overview

This document specifies how to implement an HTTP API server for BRUTUS based on the OpenCode server architecture. The server enables separation between the agent engine and user interfaces.

## Go Implementation Approach

### Technology Selection

| OpenCode | BRUTUS Equivalent |
|----------|-------------------|
| Hono (TypeScript) | Chi or Echo (Go) |
| Zod schemas | Go structs + validation |
| hono-openapi | go-swagger or oapi-codegen |
| Bun SSE | Go SSE library or manual |
| Bonjour mDNS | hashicorp/mdns or grandcat/zeroconf |

### Recommended: Chi Router

Chi is lightweight and idiomatic Go:

```go
package server

import (
    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "github.com/go-chi/cors"
)

type Server struct {
    router   *chi.Mux
    port     int
    hostname string
    mdns     *MDNSPublisher
}

func New(port int, hostname string) *Server {
    r := chi.NewRouter()

    // Middleware stack (matches OpenCode order)
    r.Use(ErrorHandler)
    r.Use(RequestLogger)
    r.Use(CORSConfig)
    r.Use(InstanceProvider)

    s := &Server{
        router:   r,
        port:     port,
        hostname: hostname,
    }

    s.registerRoutes()
    return s
}
```

## Route Structure

### Core Routes

```go
func (s *Server) registerRoutes() {
    r := s.router

    // Core endpoints
    r.Get("/doc", s.handleOpenAPI)
    r.Get("/path", s.handlePath)
    r.Get("/vcs", s.handleVCS)
    r.Get("/command", s.handleCommands)
    r.Get("/agent", s.handleAgents)
    r.Get("/skill", s.handleSkills)
    r.Get("/lsp", s.handleLSPStatus)
    r.Get("/formatter", s.handleFormatterStatus)
    r.Post("/log", s.handleLog)
    r.Put("/auth/{providerID}", s.handleAuth)
    r.Get("/event", s.handleSSE)
    r.Post("/instance/dispose", s.handleDispose)

    // Route groups
    r.Route("/global", s.globalRoutes)
    r.Route("/project", s.projectRoutes)
    r.Route("/session", s.sessionRoutes)
    r.Route("/pty", s.ptyRoutes)
    r.Route("/config", s.configRoutes)
    r.Route("/provider", s.providerRoutes)
    r.Route("/permission", s.permissionRoutes)
    r.Route("/file", s.fileRoutes)
}
```

### Session Routes (Most Complex)

```go
func (s *Server) sessionRoutes(r chi.Router) {
    r.Get("/", s.listSessions)
    r.Get("/status", s.sessionStatus)
    r.Post("/", s.createSession)

    r.Route("/{sessionID}", func(r chi.Router) {
        r.Get("/", s.getSession)
        r.Delete("/", s.deleteSession)
        r.Patch("/", s.updateSession)
        r.Post("/init", s.initSession)
        r.Post("/fork", s.forkSession)
        r.Post("/abort", s.abortSession)
        r.Post("/share", s.shareSession)
        r.Delete("/share", s.unshareSession)
        r.Get("/diff", s.sessionDiff)
        r.Post("/summarize", s.summarizeSession)
        r.Get("/children", s.sessionChildren)
        r.Get("/todo", s.sessionTodo)

        r.Route("/message", func(r chi.Router) {
            r.Get("/", s.listMessages)
            r.Post("/", s.sendPrompt)
            r.Get("/{messageID}", s.getMessage)

            r.Route("/{messageID}/part/{partID}", func(r chi.Router) {
                r.Delete("/", s.deletePart)
                r.Patch("/", s.updatePart)
            })
        })

        r.Post("/command", s.executeCommand)
        r.Post("/shell", s.executeShell)
        r.Post("/revert", s.revert)
        r.Post("/unrevert", s.unrevert)
        r.Post("/permissions/{permissionID}", s.respondPermission)
    })
}
```

## SSE Implementation

```go
type SSEHandler struct {
    bus *bus.Bus
}

func (h *SSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "SSE not supported", http.StatusInternalServerError)
        return
    }

    // Get instance from context
    instance := r.Context().Value(InstanceKey).(*Instance)

    // Subscribe to bus
    sub := h.bus.Subscribe(instance.Directory)
    defer sub.Close()

    // Send connected event
    h.sendEvent(w, flusher, "server.connected", "{}")

    // Heartbeat ticker
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-r.Context().Done():
            return
        case <-ticker.C:
            h.sendEvent(w, flusher, "server.heartbeat", "{}")
        case event := <-sub.Events():
            payload, _ := json.Marshal(event.Payload)
            h.sendEvent(w, flusher, event.Type, string(payload))
        }
    }
}

func (h *SSEHandler) sendEvent(w http.ResponseWriter, f http.Flusher, eventType, data string) {
    fmt.Fprintf(w, "event: %s\n", eventType)
    fmt.Fprintf(w, "data: %s\n\n", data)
    f.Flush()
}
```

## mDNS Service Discovery

```go
package server

import (
    "github.com/hashicorp/mdns"
)

type MDNSPublisher struct {
    server *mdns.Server
    port   int
}

func NewMDNSPublisher() *MDNSPublisher {
    return &MDNSPublisher{}
}

func (p *MDNSPublisher) Publish(port int) error {
    if p.server != nil && p.port == port {
        return nil // Already published
    }

    p.Unpublish()

    service, err := mdns.NewMDNSService(
        fmt.Sprintf("brutus-%d", port),  // Instance name
        "_http._tcp",                     // Service type
        "",                               // Domain (default)
        "brutus.local.",                  // Host
        port,                             // Port
        nil,                              // IPs (auto-detect)
        []string{"path=/"},               // TXT records
    )
    if err != nil {
        return err
    }

    p.server, err = mdns.NewServer(&mdns.Config{Zone: service})
    if err != nil {
        return err
    }

    p.port = port
    return nil
}

func (p *MDNSPublisher) Unpublish() {
    if p.server != nil {
        p.server.Shutdown()
        p.server = nil
        p.port = 0
    }
}
```

## Instance Management

```go
type Instance struct {
    Directory string
    Sessions  *SessionManager
    Config    *Config
    Bus       *bus.Bus
}

type InstanceManager struct {
    instances map[string]*Instance
    mu        sync.RWMutex
}

func (m *InstanceManager) Get(directory string) (*Instance, error) {
    m.mu.RLock()
    if inst, ok := m.instances[directory]; ok {
        m.mu.RUnlock()
        return inst, nil
    }
    m.mu.RUnlock()

    // Create new instance
    m.mu.Lock()
    defer m.mu.Unlock()

    // Double-check after acquiring write lock
    if inst, ok := m.instances[directory]; ok {
        return inst, nil
    }

    inst := &Instance{
        Directory: directory,
        Sessions:  NewSessionManager(directory),
        Config:    LoadConfig(directory),
        Bus:       bus.New(),
    }

    m.instances[directory] = inst
    return inst, nil
}

func (m *InstanceManager) Dispose(directory string) {
    m.mu.Lock()
    defer m.mu.Unlock()

    if inst, ok := m.instances[directory]; ok {
        inst.Sessions.Close()
        inst.Bus.Close()
        delete(m.instances, directory)
    }
}
```

## Middleware Implementation

### Instance Provider

```go
func InstanceProvider(manager *InstanceManager) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Get directory from query or header
            directory := r.URL.Query().Get("directory")
            if directory == "" {
                directory = r.Header.Get("X-Directory")
            }
            if directory == "" {
                directory = DefaultDirectory()
            }

            instance, err := manager.Get(directory)
            if err != nil {
                http.Error(w, "Failed to get instance", http.StatusInternalServerError)
                return
            }

            ctx := context.WithValue(r.Context(), InstanceKey, instance)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

### CORS Configuration

```go
func CORSConfig() func(http.Handler) http.Handler {
    return cors.Handler(cors.Options{
        AllowedOrigins: []string{
            "http://localhost:*",
            "http://127.0.0.1:*",
            "tauri://localhost",
        },
        AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
        AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Directory"},
        AllowCredentials: true,
        MaxAge:           300,
    })
}
```

### Error Handler

```go
type ErrorResponse struct {
    Data    interface{}            `json:"data"`
    Errors  []map[string]interface{} `json:"errors"`
    Success bool                   `json:"success"`
}

func ErrorHandler(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                w.Header().Set("Content-Type", "application/json")
                w.WriteHeader(http.StatusInternalServerError)
                json.NewEncoder(w).Encode(ErrorResponse{
                    Data:    nil,
                    Errors:  []map[string]interface{}{{"message": fmt.Sprint(err)}},
                    Success: false,
                })
            }
        }()
        next.ServeHTTP(w, r)
    })
}
```

## PTY WebSocket Handler

```go
func (s *Server) handlePTYConnect(w http.ResponseWriter, r *http.Request) {
    ptyID := chi.URLParam(r, "ptyID")
    instance := r.Context().Value(InstanceKey).(*Instance)

    ptySession, err := instance.PTY.Get(ptyID)
    if err != nil {
        http.Error(w, "PTY not found", http.StatusNotFound)
        return
    }

    upgrader := websocket.Upgrader{
        CheckOrigin: func(r *http.Request) bool {
            return true // Already handled by CORS middleware
        },
    }

    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }
    defer conn.Close()

    // Bidirectional streaming
    done := make(chan struct{})

    // PTY -> WebSocket
    go func() {
        buf := make([]byte, 4096)
        for {
            n, err := ptySession.Read(buf)
            if err != nil {
                close(done)
                return
            }
            conn.WriteMessage(websocket.BinaryMessage, buf[:n])
        }
    }()

    // WebSocket -> PTY
    for {
        select {
        case <-done:
            return
        default:
            _, msg, err := conn.ReadMessage()
            if err != nil {
                return
            }
            ptySession.Write(msg)
        }
    }
}
```

## Server Startup

```go
func (s *Server) Listen(opts ListenOptions) error {
    addr := fmt.Sprintf("%s:%d", opts.Hostname, opts.Port)

    log.Printf("Starting BRUTUS server on %s", addr)

    if opts.MDNS {
        if err := s.mdns.Publish(opts.Port); err != nil {
            log.Printf("Failed to publish mDNS: %v", err)
        } else {
            log.Printf("Published mDNS service: brutus-%d", opts.Port)
        }
    }

    srv := &http.Server{
        Addr:    addr,
        Handler: s.router,
    }

    return srv.ListenAndServe()
}

type ListenOptions struct {
    Port     int
    Hostname string
    MDNS     bool
    CORS     []string // Additional allowed origins
}
```

## Phased Implementation

### Phase 1: Core Server
1. Basic Chi router setup
2. CORS middleware
3. Instance provider middleware
4. Error handling
5. Health endpoint
6. Path endpoint

### Phase 2: Session API
1. Session CRUD endpoints
2. Message endpoints
3. Prompt submission
4. Command execution

### Phase 3: SSE Events
1. Event stream endpoint
2. Bus integration
3. Heartbeat mechanism
4. Reconnection handling

### Phase 4: PTY Support
1. PTY CRUD endpoints
2. WebSocket upgrade
3. Bidirectional streaming

### Phase 5: Discovery
1. mDNS publishing
2. Service unpublishing on shutdown

### Phase 6: Full API
1. Provider routes
2. Config routes
3. File routes
4. Permission routes
5. OpenAPI generation

## File Structure

```
brutus/
├── server/
│   ├── server.go          # Main server struct and Listen()
│   ├── middleware.go      # All middleware implementations
│   ├── routes.go          # Route registration
│   ├── sse.go             # SSE handler
│   ├── mdns.go            # mDNS publisher
│   ├── instance.go        # Instance management
│   ├── routes/
│   │   ├── session.go     # Session handlers
│   │   ├── pty.go         # PTY handlers
│   │   ├── file.go        # File handlers
│   │   ├── config.go      # Config handlers
│   │   ├── provider.go    # Provider handlers
│   │   ├── permission.go  # Permission handlers
│   │   └── global.go      # Global handlers
│   └── types/
│       ├── request.go     # Request DTOs
│       └── response.go    # Response DTOs
```
