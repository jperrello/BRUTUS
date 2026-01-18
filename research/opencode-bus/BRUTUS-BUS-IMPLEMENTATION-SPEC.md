# BRUTUS Event Bus Implementation Specification

## Overview

This document specifies how to implement an event bus system in BRUTUS based on the OpenCode architecture. The implementation should be in Go and adapted to BRUTUS's existing patterns.

## Goals

1. Enable decoupled communication between BRUTUS subsystems
2. Support typed, validated events
3. Allow external observers (GUI/CLI) to receive events via SSE
4. Integrate with existing BRUTUS agent loop

## Proposed Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      BRUTUS Process                             │
│                                                                 │
│  ┌────────────────┐    ┌────────────────┐                      │
│  │  Agent Loop    │    │   Tools        │                      │
│  │  agent/agent.go│    │  tools/*.go    │                      │
│  └───────┬────────┘    └───────┬────────┘                      │
│          │ publish              │ publish                       │
│          └──────────┬──────────┘                               │
│                     ▼                                           │
│             ┌──────────────┐                                    │
│             │     Bus      │                                    │
│             │  bus/bus.go  │                                    │
│             └──────┬───────┘                                    │
│                    │                                            │
│          ┌─────────┼─────────┐                                  │
│          ▼         ▼         ▼                                  │
│    subscribers  GlobalBus  SSE Server                           │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Go Implementation

### Event Definition (bus/event.go)

```go
package bus

import "encoding/json"

type EventType string

type Event struct {
    Type       EventType       `json:"type"`
    Properties json.RawMessage `json:"properties"`
}

type EventDef[P any] struct {
    Type EventType
}

func Define[P any](eventType EventType) EventDef[P] {
    return EventDef[P]{Type: eventType}
}

func (d EventDef[P]) New(props P) Event {
    data, _ := json.Marshal(props)
    return Event{
        Type:       d.Type,
        Properties: data,
    }
}

func (d EventDef[P]) Parse(e Event) (P, error) {
    var props P
    err := json.Unmarshal(e.Properties, &props)
    return props, err
}
```

### Bus Core (bus/bus.go)

```go
package bus

import (
    "sync"
)

type Subscription func(Event)

type Bus struct {
    mu            sync.RWMutex
    subscriptions map[EventType][]Subscription
    wildcards     []Subscription
    global        chan Event
}

func New() *Bus {
    b := &Bus{
        subscriptions: make(map[EventType][]Subscription),
        global:        make(chan Event, 100),
    }
    return b
}

func (b *Bus) Publish(event Event) {
    b.mu.RLock()
    defer b.mu.RUnlock()

    // Notify type-specific subscribers
    for _, sub := range b.subscriptions[event.Type] {
        sub(event)
    }

    // Notify wildcard subscribers
    for _, sub := range b.wildcards {
        sub(event)
    }

    // Send to global channel (non-blocking)
    select {
    case b.global <- event:
    default:
        // Channel full, event dropped
    }
}

func (b *Bus) Subscribe(eventType EventType, callback Subscription) func() {
    b.mu.Lock()
    defer b.mu.Unlock()

    b.subscriptions[eventType] = append(b.subscriptions[eventType], callback)

    return func() {
        b.mu.Lock()
        defer b.mu.Unlock()
        subs := b.subscriptions[eventType]
        for i, sub := range subs {
            if &sub == &callback {
                b.subscriptions[eventType] = append(subs[:i], subs[i+1:]...)
                break
            }
        }
    }
}

func (b *Bus) SubscribeAll(callback Subscription) func() {
    b.mu.Lock()
    defer b.mu.Unlock()

    b.wildcards = append(b.wildcards, callback)

    return func() {
        b.mu.Lock()
        defer b.mu.Unlock()
        for i, sub := range b.wildcards {
            if &sub == &callback {
                b.wildcards = append(b.wildcards[:i], b.wildcards[i+1:]...)
                break
            }
        }
    }
}

func (b *Bus) GlobalChannel() <-chan Event {
    return b.global
}
```

### Predefined Events (bus/events.go)

```go
package bus

// Core events
var InstanceDisposed = Define[InstanceDisposedProps]("instance.disposed")

type InstanceDisposedProps struct {
    Directory string `json:"directory"`
}

// Agent events
var AgentStarted = Define[AgentStartedProps]("agent.started")
var AgentStopped = Define[AgentStoppedProps]("agent.stopped")
var AgentThinking = Define[AgentThinkingProps]("agent.thinking")

type AgentStartedProps struct {
    SessionID string `json:"session_id"`
}

type AgentStoppedProps struct {
    SessionID string `json:"session_id"`
    Reason    string `json:"reason"`
}

type AgentThinkingProps struct {
    SessionID string `json:"session_id"`
}

// Tool events
var ToolExecuting = Define[ToolExecutingProps]("tool.executing")
var ToolCompleted = Define[ToolCompletedProps]("tool.completed")

type ToolExecutingProps struct {
    SessionID string `json:"session_id"`
    ToolName  string `json:"tool_name"`
    Input     string `json:"input"`
}

type ToolCompletedProps struct {
    SessionID string `json:"session_id"`
    ToolName  string `json:"tool_name"`
    Success   bool   `json:"success"`
}

// Message events
var MessageCreated = Define[MessageCreatedProps]("message.created")

type MessageCreatedProps struct {
    SessionID string `json:"session_id"`
    Role      string `json:"role"`
    Content   string `json:"content"`
}
```

### SSE Server Integration

```go
package server

import (
    "encoding/json"
    "net/http"
    "time"

    "brutus/bus"
)

func (s *Server) HandleEvents(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "SSE not supported", http.StatusInternalServerError)
        return
    }

    // Subscribe to all events
    events := make(chan bus.Event, 10)
    unsub := s.bus.SubscribeAll(func(e bus.Event) {
        select {
        case events <- e:
        default:
        }
    })
    defer unsub()

    // Heartbeat ticker
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case event := <-events:
            data, _ := json.Marshal(event)
            fmt.Fprintf(w, "data: %s\n\n", data)
            flusher.Flush()

        case <-ticker.C:
            fmt.Fprintf(w, "data: {\"type\":\"heartbeat\",\"properties\":{}}\n\n")
            flusher.Flush()

        case <-r.Context().Done():
            return
        }
    }
}
```

## Integration Points

### Agent Loop Integration

Modify `agent/agent.go` to publish events:

```go
func (a *Agent) Run(ctx context.Context, input string) error {
    a.bus.Publish(bus.AgentStarted.New(bus.AgentStartedProps{
        SessionID: a.sessionID,
    }))
    defer func() {
        a.bus.Publish(bus.AgentStopped.New(bus.AgentStoppedProps{
            SessionID: a.sessionID,
            Reason:    "completed",
        }))
    }()

    // ... existing loop logic ...
}
```

### Tool Execution Integration

Modify tool execution to publish events:

```go
func (r *Registry) Execute(ctx context.Context, name string, input json.RawMessage) (string, error) {
    r.bus.Publish(bus.ToolExecuting.New(bus.ToolExecutingProps{
        ToolName: name,
        Input:    string(input),
    }))

    result, err := tool.Execute(ctx, input)

    r.bus.Publish(bus.ToolCompleted.New(bus.ToolCompletedProps{
        ToolName: name,
        Success:  err == nil,
    }))

    return result, err
}
```

## Key Differences from OpenCode

| Aspect | OpenCode | BRUTUS |
|--------|----------|--------|
| Language | TypeScript | Go |
| Validation | Zod schemas | Go generics + JSON |
| State scope | Per-directory Instance | Single process (simpler) |
| Global channel | EventEmitter | Go channel |
| Async | Promise-based | Goroutine-safe |

## Implementation Order

1. `bus/event.go` - Event type system
2. `bus/bus.go` - Core pub/sub
3. `bus/events.go` - Predefined events
4. Integrate into agent loop
5. Add SSE endpoint
6. Wire up tool events

## Testing Strategy

```go
func TestBusPublishSubscribe(t *testing.T) {
    b := bus.New()

    received := make(chan bus.Event, 1)
    unsub := b.Subscribe(bus.AgentStarted.Type, func(e bus.Event) {
        received <- e
    })
    defer unsub()

    b.Publish(bus.AgentStarted.New(bus.AgentStartedProps{
        SessionID: "test-123",
    }))

    select {
    case e := <-received:
        props, _ := bus.AgentStarted.Parse(e)
        assert.Equal(t, "test-123", props.SessionID)
    case <-time.After(time.Second):
        t.Fatal("event not received")
    }
}
```

## Future Considerations

1. **Event persistence**: Store events for replay/debugging
2. **Event filtering**: Allow SSE clients to filter by event type
3. **Backpressure**: Handle slow consumers gracefully
4. **Metrics**: Track event counts and latencies
