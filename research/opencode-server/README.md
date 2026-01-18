# OpenCode Server Subsystem Research

## Overview

This research documents OpenCode's HTTP API server architecture - the backbone that enables client-server separation between the TUI/Desktop/Web frontends and the core agent engine.

## Documents

| Document | Description |
|----------|-------------|
| [SERVER-SUBSYSTEM-SPEC.md](./SERVER-SUBSYSTEM-SPEC.md) | Core server architecture, middleware stack, initialization |
| [HTTP-API-SPEC.md](./HTTP-API-SPEC.md) | Complete HTTP API reference with all routes |
| [SSE-EVENT-PROTOCOL.md](./SSE-EVENT-PROTOCOL.md) | Server-Sent Events streaming protocol |
| [BRUTUS-SERVER-IMPLEMENTATION-SPEC.md](./BRUTUS-SERVER-IMPLEMENTATION-SPEC.md) | Implementation plan for BRUTUS |

## Key Insights

1. **Hono Framework**: OpenCode uses Hono for high-performance HTTP routing
2. **Client-Server Separation**: All UI interactions go through HTTP API
3. **mDNS Discovery**: Local network service discovery for multi-device access
4. **SSE for Realtime**: Server-Sent Events for push notifications (not WebSocket for most things)
5. **WebSocket for PTY**: Only terminal sessions use WebSocket for bidirectional streaming

## Architecture Significance

The server layer is what enables:
- Desktop apps (Tauri) to communicate with the agent
- Web interfaces to connect remotely
- Multiple frontends to share the same session
- IDE extensions to interact with running agents

## Source Files Analyzed

- `packages/opencode/src/server/server.ts` (18,689 bytes)
- `packages/opencode/src/server/routes/session.ts` (27,984 bytes)
- `packages/opencode/src/server/routes/tui.ts` (10,253 bytes)
- `packages/opencode/src/server/routes/file.ts` (5,130 bytes)
- `packages/opencode/src/server/routes/provider.ts` (5,159 bytes)
- `packages/opencode/src/server/routes/pty.ts` (4,835 bytes)
- `packages/opencode/src/server/routes/mcp.ts` (6,450 bytes)
- `packages/opencode/src/server/routes/experimental.ts` (4,603 bytes)
- `packages/opencode/src/server/routes/config.ts` (2,684 bytes)
- `packages/opencode/src/server/routes/global.ts` (3,887 bytes)
- `packages/opencode/src/server/routes/permission.ts` (1,962 bytes)
- `packages/opencode/src/server/routes/question.ts` (2,636 bytes)
- `packages/opencode/src/server/routes/project.ts` (2,396 bytes)
- `packages/opencode/src/server/mdns.ts` (1,346 bytes)
- `packages/opencode/src/server/error.ts` (839 bytes)
