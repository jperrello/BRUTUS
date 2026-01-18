# OpenCode PTY Subsystem Research

**Researcher**: Claude Agent (Opus 4.5)
**Date**: 2026-01-17
**Subject**: Pseudo-terminal (PTY) management subsystem - interactive terminal sessions

---

## What Was Researched

Deep reverse engineering of OpenCode's PTY subsystem. This is the mechanism that enables interactive terminal sessions in the UI - separate from the non-interactive bash tool used by the agent.

## Files in This Directory

| File | Description |
|------|-------------|
| `PTY-SUBSYSTEM-SPEC.md` | Complete specification - architecture, data structures, state management, WebSocket protocol |
| `BRUTUS-PTY-IMPLEMENTATION-SPEC.md` | Go implementation specification for BRUTUS |

## Key Findings

### Architecture

The PTY subsystem provides:
1. **Process Spawning** - Creates pseudo-terminal sessions with configurable shell/command
2. **State Management** - Tracks active sessions with per-instance isolation
3. **Output Buffering** - Buffers output when no WebSocket clients connected (2MB limit)
4. **WebSocket Streaming** - Real-time bidirectional communication with UI
5. **Event System** - Broadcasts lifecycle events via the Bus

### Critical Constants

```typescript
BUFFER_LIMIT = 2 * 1024 * 1024    // 2MB output buffer per session
BUFFER_CHUNK = 64 * 1024          // 64KB chunks for WebSocket sending
```

### ID Format

PTY sessions use monotonic IDs with the `pty_` prefix:
```
pty_[12-char-hex-timestamp][14-char-random-base62]
```

Example: `pty_018d2a3b4c5d6e7f8g9h0i1j2k`

### Session States

```typescript
status: "running" | "exited"
```

### Events

| Event | Payload |
|-------|---------|
| `pty.created` | `{ info: Pty.Info }` |
| `pty.updated` | `{ info: Pty.Info }` |
| `pty.exited` | `{ id: string, exitCode: number }` |
| `pty.deleted` | `{ id: string }` |

### HTTP/WebSocket API

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/pty` | List all sessions |
| POST | `/pty` | Create new session |
| GET | `/pty/:id` | Get session info |
| PUT | `/pty/:id` | Update session (title, resize) |
| DELETE | `/pty/:id` | Remove session |
| GET (WS) | `/pty/:id/connect` | WebSocket stream |

### WebSocket Protocol

- **Server → Client**: Raw terminal output (UTF-8 strings)
- **Client → Server**: Keyboard input (UTF-8 strings)
- **Buffering**: When no clients connected, output buffered up to 2MB
- **Chunking**: Large buffers sent in 64KB chunks on reconnect

### Shell Detection

```typescript
// Priority:
1. $SHELL environment variable
2. Platform-specific fallback:
   - Windows: Git Bash → cmd.exe
   - macOS: /bin/zsh
   - Linux: bash → /bin/sh
```

### Blacklisted Shells

`fish` and `nu` are blacklisted for the `acceptable()` shell (used by bash tool), but allowed for `preferred()` shell (used by PTY).

## What Was NOT Researched

- TUI rendering of terminal sessions
- Terminal emulator frontend (xterm.js integration)
- Session persistence across server restarts
- Multi-instance coordination

## Next Steps for Future Agents

1. **Implement basic PTY** - Use Go's `github.com/creack/pty` or `github.com/ActiveState/termtest/conpty` (Windows)
2. **Add WebSocket endpoint** - Gorilla websocket or stdlib
3. **Add output buffering** - Ring buffer with 2MB limit
4. **Add session management** - In-memory map with cleanup
5. **Later**: Shell detection, environment configuration

## Source Files Analyzed

- `packages/opencode/src/pty/index.ts` (~185 lines)
- `packages/opencode/src/shell/shell.ts` (~65 lines)
- `packages/opencode/src/server/routes/pty.ts` (~130 lines)
- `packages/opencode/src/bus/index.ts` (~80 lines)
- `packages/opencode/src/bus/bus-event.ts` (~35 lines)
- `packages/opencode/src/id/id.ts` (~85 lines)
- `packages/opencode/src/project/instance.ts` (~100 lines)

## External Dependencies

- `bun-pty` - Native PTY implementation for Bun (Rust FFI)
- `hono` - Web framework for HTTP routes
- `hono/ws` - WebSocket upgrade support
