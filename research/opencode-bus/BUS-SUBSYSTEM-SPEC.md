# OpenCode Event Bus Subsystem Specification

## Overview

The Bus is OpenCode's internal publish/subscribe event system that enables decoupled communication between subsystems. It provides typed events with Zod schema validation, instance-scoped state management, and cross-instance event propagation.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        OpenCode Process                         │
│                                                                 │
│  ┌──────────────────┐     ┌──────────────────┐                 │
│  │   Instance A     │     │   Instance B     │                 │
│  │  (directory: /a) │     │  (directory: /b) │                 │
│  │                  │     │                  │                 │
│  │  ┌────────────┐  │     │  ┌────────────┐  │                 │
│  │  │ Bus State  │  │     │  │ Bus State  │  │                 │
│  │  │ subscriptions│ │     │ subscriptions│  │                 │
│  │  └────────────┘  │     │  └────────────┘  │                 │
│  │       │          │     │       │          │                 │
│  │       ▼          │     │       ▼          │                 │
│  │  Bus.publish()   │     │  Bus.publish()   │                 │
│  │       │          │     │       │          │                 │
│  └───────┼──────────┘     └───────┼──────────┘                 │
│          │                        │                             │
│          └────────┬───────────────┘                             │
│                   ▼                                             │
│           ┌──────────────┐                                      │
│           │  GlobalBus   │ ◄── EventEmitter                     │
│           │  (cross-inst)│                                      │
│           └──────┬───────┘                                      │
│                  │                                              │
│                  ▼                                              │
│           ┌──────────────┐                                      │
│           │   Server     │                                      │
│           │  SSE Stream  │ ─────► External Clients (UI/CLI)     │
│           └──────────────┘                                      │
└─────────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. BusEvent (bus-event.ts)

Defines the event type system using Zod schemas.

```typescript
export namespace BusEvent {
  export type Definition = ReturnType<typeof define>

  const registry = new Map<string, Definition>()

  export function define<Type extends string, Properties extends ZodType>(
    type: Type,
    properties: Properties
  ) {
    const result = { type, properties }
    registry.set(type, result)
    return result
  }

  export function payloads() {
    // Returns a Zod discriminated union of all registered events
    // Enables runtime validation of any event payload
  }
}
```

**Key Behaviors:**
- Events are registered globally in a Map by type string
- Each event definition pairs a type literal with a Zod schema
- `payloads()` generates a discriminated union for runtime validation

### 2. GlobalBus (global.ts)

A Node.js EventEmitter for cross-instance event propagation.

```typescript
import { EventEmitter } from "events"

export const GlobalBus = new EventEmitter<{
  event: [{
    directory?: string
    payload: any
  }]
}>()
```

**Purpose:** Allows events from one instance to be observed by processes that need cross-instance visibility (e.g., the server SSE endpoint).

### 3. Bus (index.ts)

The main bus namespace providing publish/subscribe operations.

```typescript
export namespace Bus {
  // Instance disposal event (built-in)
  export const InstanceDisposed = BusEvent.define(
    "server.instance.disposed",
    z.object({ directory: z.string() })
  )

  // State is scoped per Instance.directory
  const state = Instance.state(() => ({
    subscriptions: new Map<any, Subscription[]>()
  }), async (entry) => {
    // On disposal: notify wildcard subscribers
    const wildcard = entry.subscriptions.get("*")
    if (wildcard) {
      const event = {
        type: InstanceDisposed.type,
        properties: { directory: Instance.directory }
      }
      for (const sub of wildcard) sub(event)
    }
  })

  export async function publish<D extends BusEvent.Definition>(
    def: D,
    properties: z.output<D["properties"]>
  ) {
    const payload = { type: def.type, properties }

    // Notify instance-local subscribers
    for (const key of [def.type, "*"]) {
      const match = state().subscriptions.get(key)
      for (const sub of match ?? []) {
        pending.push(sub(payload))
      }
    }

    // Propagate to GlobalBus for cross-instance observers
    GlobalBus.emit("event", {
      directory: Instance.directory,
      payload
    })

    return Promise.all(pending)
  }

  export function subscribe<D extends BusEvent.Definition>(
    def: D,
    callback: (event: {...}) => void
  ) { ... }

  export function once<D extends BusEvent.Definition>(
    def: D,
    callback: (event: {...}) => "done" | undefined
  ) { ... }

  export function subscribeAll(callback: (event: any) => void) { ... }
}
```

## State Management Integration

The Bus relies on `Instance.state()` for per-project-directory state isolation.

### Instance.state Mechanics

```typescript
// From project/instance.ts
export function state<S>(
  init: () => S,
  dispose?: (state: Awaited<S>) => Promise<void>
): () => S {
  return State.create(() => Instance.directory, init, dispose)
}
```

### State.create Mechanics

```typescript
// From project/state.ts
const entries = new Map<string, Map<any, Entry>>()

function create<S>(
  root: () => string,
  init: () => S,
  dispose?: (s: Awaited<S>) => Promise<void>
): () => S {
  return function() {
    const key = root()
    let byInit = entries.get(key)
    if (!byInit) {
      byInit = new Map()
      entries.set(key, byInit)
    }

    let entry = byInit.get(init)
    if (!entry) {
      entry = { state: init(), dispose }
      byInit.set(init, entry)
    }

    return entry.state
  }
}

async function dispose(key: string) {
  const byInit = entries.get(key)
  if (!byInit) return

  for (const entry of byInit.values()) {
    const resolved = await entry.state
    if (entry.dispose) {
      await entry.dispose(resolved)
    }
  }

  entries.delete(key)
}
```

**Key Insight:** State is keyed by `(directory, init_function)` tuple. This means:
- Different instances (directories) have isolated state
- Different `init` functions within the same instance get separate state slots
- Disposal is coordinated per-directory, cleaning up all state for that instance

## Server-Side Event (SSE) Integration

The server exposes bus events to external clients via SSE streaming.

```typescript
// From server/server.ts
app.get("/events", (c) => {
  return streamSSE(c, async (stream) => {
    // Subscribe to ALL events for this instance
    const unsub = Bus.subscribeAll(async (event) => {
      await stream.writeSSE({
        data: JSON.stringify(event)
      })

      // Close stream on instance disposal
      if (event.type === Bus.InstanceDisposed.type) {
        stream.close()
      }
    })

    // Heartbeat to prevent WKWebView 60s timeout
    const heartbeat = setInterval(() => {
      stream.writeSSE({
        data: JSON.stringify({
          type: "server.heartbeat",
          properties: {}
        })
      })
    }, 30000)

    stream.onAbort(() => {
      unsub()
      clearInterval(heartbeat)
    })

    // Block until client disconnects
    await new Promise(() => {})
  })
})
```

## Event Payload Structure

All events follow this structure:

```typescript
interface EventPayload<T extends string, P> {
  type: T           // String literal event type
  properties: P     // Zod-validated properties object
}
```

Example:
```json
{
  "type": "session.created",
  "properties": {
    "id": "abc123",
    "title": "New Session"
  }
}
```

## API Summary

| Function | Description |
|----------|-------------|
| `BusEvent.define(type, schema)` | Register a new event type with Zod schema |
| `Bus.publish(def, properties)` | Publish an event to all subscribers |
| `Bus.subscribe(def, callback)` | Subscribe to a specific event type |
| `Bus.subscribeAll(callback)` | Subscribe to all events (wildcard) |
| `Bus.once(def, callback)` | Subscribe until callback returns "done" |

## Design Principles

1. **Type Safety**: All events are Zod-validated at definition and runtime
2. **Instance Isolation**: State is scoped per project directory
3. **Global Propagation**: GlobalBus enables cross-instance observation
4. **Lifecycle Aware**: Automatic cleanup on instance disposal
5. **Stream-Friendly**: SSE integration for external clients
6. **Async-First**: `publish` returns Promise.all of subscriber handlers

## Error Handling

- Subscriber exceptions are not caught by the bus
- Disposal has a 10-second timeout warning (logged, not enforced)
- SSE streams close gracefully on instance disposal
