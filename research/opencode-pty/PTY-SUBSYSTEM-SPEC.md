# OpenCode PTY Subsystem Specification

## Overview

The PTY subsystem manages interactive pseudo-terminal sessions. Unlike the bash tool (which executes non-interactive commands), PTY sessions provide full terminal emulation for interactive programs (vim, htop, shells, etc.).

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                          HTTP Server (Hono)                          │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │                     PtyRoutes                                 │   │
│  │  GET /pty              → list()                               │   │
│  │  POST /pty             → create()                             │   │
│  │  GET /pty/:id          → get()                                │   │
│  │  PUT /pty/:id          → update()                             │   │
│  │  DELETE /pty/:id       → remove()                             │   │
│  │  WS /pty/:id/connect   → connect()                            │   │
│  └──────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         Pty Namespace                                │
│                                                                      │
│  ┌───────────────┐    ┌───────────────┐    ┌───────────────┐        │
│  │  ActiveSession │    │  ActiveSession │    │  ActiveSession │       │
│  │  ┌──────────┐ │    │  ┌──────────┐ │    │  ┌──────────┐ │        │
│  │  │  info    │ │    │  │  info    │ │    │  │  info    │ │        │
│  │  │  process │ │    │  │  process │ │    │  │  process │ │        │
│  │  │  buffer  │ │    │  │  buffer  │ │    │  │  buffer  │ │        │
│  │  │  subs    │ │    │  │  subs    │ │    │  │  subs    │ │        │
│  │  └──────────┘ │    │  └──────────┘ │    │  └──────────┘ │        │
│  └───────────────┘    └───────────────┘    └───────────────┘        │
│                                                                      │
│  state: Map<string, ActiveSession>                                   │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         bun-pty (Native FFI)                         │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │                    IPty Interface                            │    │
│  │                                                              │    │
│  │  Properties:                                                 │    │
│  │    pid: number      (process ID)                             │    │
│  │    cols: number     (terminal columns)                       │    │
│  │    rows: number     (terminal rows)                          │    │
│  │    process: string  (process name)                           │    │
│  │                                                              │    │
│  │  Methods:                                                    │    │
│  │    write(data: string): void                                 │    │
│  │    resize(cols: number, rows: number): void                  │    │
│  │    kill(signal?: string): void                               │    │
│  │                                                              │    │
│  │  Events:                                                     │    │
│  │    onData: (listener: (data: string) => void) => IDisposable │    │
│  │    onExit: (listener: (event: IExitEvent) => void)           │    │
│  └─────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────┘
```

## Data Structures

### Pty.Info (Session Metadata)

```typescript
const Info = z.object({
  id: Identifier.schema("pty"),   // "pty_" prefixed monotonic ID
  title: z.string(),               // Display name
  command: z.string(),             // Shell/command path
  args: z.array(z.string()),       // Command arguments
  cwd: z.string(),                 // Working directory
  status: z.enum(["running", "exited"]),
  pid: z.number(),                 // Process ID
})
```

### Pty.CreateInput

```typescript
const CreateInput = z.object({
  command: z.string().optional(),  // Defaults to preferred shell
  args: z.array(z.string()).optional(),
  cwd: z.string().optional(),      // Defaults to Instance.directory
  title: z.string().optional(),    // Defaults to "Terminal {id-suffix}"
  env: z.record(z.string(), z.string()).optional(),
})
```

### Pty.UpdateInput

```typescript
const UpdateInput = z.object({
  title: z.string().optional(),
  size: z.object({
    rows: z.number(),
    cols: z.number(),
  }).optional(),
})
```

### ActiveSession (Internal State)

```typescript
interface ActiveSession {
  info: Info                    // Session metadata
  process: IPty                 // Native PTY handle
  buffer: string                // Output buffer (when no clients)
  subscribers: Set<WSContext>   // Connected WebSocket clients
}
```

## Constants

```typescript
const BUFFER_LIMIT = 1024 * 1024 * 2   // 2MB maximum buffer size
const BUFFER_CHUNK = 64 * 1024         // 64KB chunks for WS transmission
```

## State Management

### Instance-Scoped State

PTY state is scoped to the current project instance via `Instance.state()`:

```typescript
const state = Instance.state(
  () => new Map<string, ActiveSession>(),  // Initializer
  async (sessions) => {                     // Cleanup on dispose
    for (const session of sessions.values()) {
      try { session.process.kill() } catch {}
      for (const ws of session.subscribers) {
        ws.close()
      }
    }
    sessions.clear()
  },
)
```

This ensures:
1. Each project instance has isolated PTY sessions
2. Sessions are cleaned up when the instance is disposed
3. WebSocket connections are closed on cleanup

## Session Lifecycle

### create()

```
Input → Resolve Shell → Spawn PTY → Create Info → Setup Handlers → Publish Event
```

1. Generate monotonic ID with `pty_` prefix
2. Resolve command (default: `Shell.preferred()`)
3. Add `-l` (login shell) for sh-based shells
4. Set environment: `{ ...process.env, ...input.env, TERM: "xterm-256color" }`
5. Spawn PTY via `bun-pty`
6. Create `ActiveSession` with empty buffer and subscriber set
7. Setup `onData` handler (broadcast or buffer)
8. Setup `onExit` handler (update status, publish event, cleanup)
9. Publish `pty.created` event
10. Return `Info`

### onData Handler

```typescript
ptyProcess.onData((data) => {
  let open = false
  for (const ws of session.subscribers) {
    if (ws.readyState !== 1) {
      session.subscribers.delete(ws)
      continue
    }
    open = true
    ws.send(data)
  }
  if (open) return  // Data sent to clients, no buffering

  // No open clients - buffer the output
  session.buffer += data
  if (session.buffer.length <= BUFFER_LIMIT) return
  session.buffer = session.buffer.slice(-BUFFER_LIMIT)  // Truncate from front
})
```

Key behaviors:
- Broadcasts to all connected WebSocket clients
- Removes closed connections automatically
- Only buffers when NO clients are connected
- Truncates buffer from the front (keeps recent output)

### onExit Handler

```typescript
ptyProcess.onExit(({ exitCode }) => {
  session.info.status = "exited"
  Bus.publish(Event.Exited, { id, exitCode })
  state().delete(id)  // Remove from state map
})
```

### connect()

WebSocket connection flow:

```typescript
function connect(id: string, ws: WSContext) {
  const session = state().get(id)
  if (!session) { ws.close(); return }

  session.subscribers.add(ws)

  // Flush buffered output
  if (session.buffer) {
    const buffer = session.buffer.length <= BUFFER_LIMIT
      ? session.buffer
      : session.buffer.slice(-BUFFER_LIMIT)
    session.buffer = ""  // Clear buffer

    // Send in chunks
    for (let i = 0; i < buffer.length; i += BUFFER_CHUNK) {
      ws.send(buffer.slice(i, i + BUFFER_CHUNK))
    }
  }

  return {
    onMessage: (message) => session.process.write(String(message)),
    onClose: () => session.subscribers.delete(ws),
  }
}
```

### remove()

```typescript
async function remove(id: string) {
  const session = state().get(id)
  if (!session) return

  session.process.kill()
  for (const ws of session.subscribers) { ws.close() }
  state().delete(id)
  Bus.publish(Event.Deleted, { id })
}
```

## Shell Detection

### Shell.preferred()

Used for PTY sessions - respects user's shell preference:

```typescript
function preferred() {
  const s = process.env.SHELL
  if (s) return s
  return fallback()
}
```

### Shell.acceptable()

Used for bash tool - blacklists problematic shells:

```typescript
const BLACKLIST = new Set(["fish", "nu"])

function acceptable() {
  const s = process.env.SHELL
  if (s && !BLACKLIST.has(basename(s))) return s
  return fallback()
}
```

### fallback()

Platform-specific defaults:

```typescript
function fallback() {
  if (process.platform === "win32") {
    // Try Git Bash first
    if (Flag.OPENCODE_GIT_BASH_PATH) return Flag.OPENCODE_GIT_BASH_PATH
    const git = Bun.which("git")
    if (git) {
      const bash = path.join(git, "..", "..", "bin", "bash.exe")
      if (Bun.file(bash).size) return bash
    }
    return process.env.COMSPEC || "cmd.exe"
  }
  if (process.platform === "darwin") return "/bin/zsh"
  const bash = Bun.which("bash")
  if (bash) return bash
  return "/bin/sh"
}
```

## Event System

### Event Definitions

```typescript
const Event = {
  Created: BusEvent.define("pty.created", z.object({ info: Info })),
  Updated: BusEvent.define("pty.updated", z.object({ info: Info })),
  Exited: BusEvent.define("pty.exited", z.object({
    id: Identifier.schema("pty"),
    exitCode: z.number()
  })),
  Deleted: BusEvent.define("pty.deleted", z.object({
    id: Identifier.schema("pty")
  })),
}
```

### Event Flow

```
Session Created → Bus.publish(Event.Created, { info })
                         │
                         ▼
              ┌─────────────────────┐
              │     Bus.publish()   │
              │                     │
              │  1. Log "publishing"│
              │  2. Find subscribers│
              │  3. Call callbacks  │
              │  4. GlobalBus.emit  │
              └─────────────────────┘
                         │
                         ▼
              ┌─────────────────────┐
              │     GlobalBus       │
              │  (cross-instance)   │
              │                     │
              │  { directory,       │
              │    payload }        │
              └─────────────────────┘
```

## HTTP API

### GET /pty - List Sessions

```
Response: Pty.Info[]
```

### POST /pty - Create Session

```
Request Body: Pty.CreateInput
Response: Pty.Info
```

### GET /pty/:ptyID - Get Session

```
Response: Pty.Info
Error 404: Session not found
```

### PUT /pty/:ptyID - Update Session

```
Request Body: Pty.UpdateInput
Response: Pty.Info
```

### DELETE /pty/:ptyID - Remove Session

```
Response: true
```

### GET /pty/:ptyID/connect - WebSocket Connect

```
Upgrade: websocket
Protocol: Raw terminal I/O (UTF-8 strings both directions)
```

## WebSocket Protocol

### Connection Establishment

```
Client                                  Server
  │                                       │
  │──── GET /pty/:id/connect ────────────▶│
  │     Upgrade: websocket                │
  │                                       │
  │◀──── 101 Switching Protocols ─────────│
  │                                       │
  │◀──── [Buffered output in chunks] ─────│
  │                                       │
  │◀──── [Real-time output] ──────────────│
  │                                       │
  │──── [Keyboard input] ────────────────▶│
  │                                       │
```

### Message Format

- **Server → Client**: UTF-8 encoded terminal output (raw bytes as string)
- **Client → Server**: UTF-8 encoded keyboard input (raw bytes as string)

No JSON wrapping - pure terminal data for direct consumption by xterm.js.

### Buffer Replay

On connect, if there's buffered output:
1. Buffer is sliced to `BUFFER_LIMIT` (2MB)
2. Buffer is cleared
3. Buffer is sent in `BUFFER_CHUNK` (64KB) sized messages
4. If any send fails, buffer is restored and connection closed

## Process Tree Killing

For killing PTY processes (used elsewhere):

```typescript
async function killTree(proc: ChildProcess, opts?: { exited?: () => boolean }): Promise<void> {
  const pid = proc.pid
  if (!pid || opts?.exited?.()) return

  if (process.platform === "win32") {
    // Windows: Use taskkill with tree flag
    spawn("taskkill", ["/pid", String(pid), "/f", "/t"], { stdio: "ignore" })
    return
  }

  // Unix: Kill process group
  try {
    process.kill(-pid, "SIGTERM")        // Negative PID = process group
    await Bun.sleep(200)                  // 200ms grace period
    if (!opts?.exited?.()) {
      process.kill(-pid, "SIGKILL")       // Force kill
    }
  } catch {
    // Fallback: Kill individual process
    proc.kill("SIGTERM")
    await Bun.sleep(200)
    if (!opts?.exited?.()) {
      proc.kill("SIGKILL")
    }
  }
}
```

## Terminal Configuration

Default environment for spawned PTY:

```typescript
const env = {
  ...process.env,    // Inherit environment
  ...input.env,      // User overrides
  TERM: "xterm-256color"  // Force 256-color support
}
```

Spawn options for `bun-pty`:

```typescript
{
  name: "xterm-256color",  // Terminal type
  cwd: input.cwd || Instance.directory,
  env: env
}
```

## Thread Safety

The subsystem is designed for single-threaded execution (JavaScript event loop):
- State map accessed synchronously
- WebSocket callbacks queued via event loop
- Process events handled asynchronously

For Go implementation, explicit synchronization will be needed.
