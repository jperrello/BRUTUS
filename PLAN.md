# BRUTUS Build Fix Plan

## Context

- **BRUTUS**: Saturn-powered coding agent with Wails GUI
- **Saturn**: Zero-config AI service discovery - https://github.com/jperrello/Saturn
- **Current State**: Code does not compile due to incomplete refactoring

---

## Completion Criteria (Build Gates)

Before claiming ANY work is done, you MUST run:

```bash
go build ./...    # Must exit 0
go test ./...     # Must exit 0
go vet ./...      # Must exit 0
```

**If any fail, the work is NOT done.** Document the failure and continue working.

After build gates pass:
1. Update "Previous Agent Notes" section below
2. Run `bd sync` to commit beads changes


---

## Issues to Fix

### Phase 1: Make It Compile

| Priority | Issue | Description | Status |
|----------|-------|-------------|--------|
| P0 | Duplicate functions | `parseBrowseOutput`, `resolveInstance`, `parseResolveOutput` defined in both `discovery.go` AND `discovery_legacy.go` | DONE |
| P0 | Missing `SaturnService` fields | Need: `SaturnVersion`, `MaxConcurrent`, `CurrentLoad`, `Security`, `HealthEndpoint`, `Models`, `GPU`, `VRAMGb`, `HealthStatus` | DONE |
| P0 | Undefined `StreamDelta` | Type for streaming responses (Content, ToolCall, Error, Done) | DONE |
| P0 | Undefined `DiscoveryFilter` | Type for filtering discovered services | DONE |
| P0 | Undefined `FilterServices` | Function to apply filter to service list | DONE |
| P0 | Missing `Saturn.ChatStream()` | SSE streaming method called by SaturnPool | DONE |
| P0 | Missing `Saturn.GetService()` | Simple getter returning `s.service` | DONE |
| P0 | Missing `CreateDiscoverer` | Factory function for discoverer | DONE |
| P0 | Missing `globalServiceCache` | Package-level cache instance | DONE |
| P0 | Missing `createPooledTransport` | HTTP transport for connection pooling | DONE |
| P0 | Add `ChatStream` to Provider interface | Interface method for streaming | DONE |

### Phase 2: Make Tests Pass

| Priority | Issue | Description | Status |
|----------|-------|-------------|--------|
| P1 | Missing `AvailableCapacity()` | Method on SaturnService: `MaxConcurrent - CurrentLoad` | DONE |
| P1 | Missing `LoadFraction()` | Method on SaturnService: `CurrentLoad / MaxConcurrent` | DONE |
| P1 | Missing `SelectBestService()` | Load-aware service selection function | DONE |
| P1 | Fix test priority semantics | Test had wrong priority naming (lower number = higher priority) | DONE |

### Phase 3: SDK-to-UI Wiring

| Priority | Issue | Description | Status |
|----------|-------|-------------|--------|
| P2 | Verify Wails bindings match Go | Check `frontend/wailsjs/go/main/App.d.ts` matches actual Go exports | DONE |
| P2 | Frontend uses new AgentSession fields | `ServiceName`, `ServiceHost`, `Connected` added to Go but UI may not display them | DONE (UI already uses them) |
| P2 | Regenerate Wails bindings | Run `wails generate module` after Go changes | DONE (manually updated) |

### Phase 4: Cleanup

| Priority | Issue | Description | Status |
|----------|-------|-------------|--------|
| P3 | Naming inconsistency | `hideWindow` vs `hideCommandWindow` - pick one | DONE (standardized on hideWindow) |
| P3 | Untracked files | `provider/exec_unix.go`, `provider/exec_windows.go` - commit or delete | DONE (ready to commit) |
| P3 | Unused duplicate code | `discovery.go` has functions that duplicate `discovery_legacy.go` | DONE (removed from discovery.go) |

---

## Architecture Notes

### Discovery Files
- `discovery.go` - Simple discovery, used by `DiscoverSaturn()`
- `discovery_legacy.go` - Has `LegacyDiscoverer` with caching
- `discovery_zeroconf.go` - Has `ZeroconfDiscoverer` with fallback to legacy

Intent seems to be: zeroconf primary, dns-sd legacy fallback, both use cache. But refactor is incomplete.

### Provider Files
- `saturn.go` - Single service provider
- `saturn_pool.go` - Multi-service with round-robin and failover

`GUIAgent` currently uses `Saturn` directly. Could benefit from `SaturnPool`.

### SDK vs UI Pattern
When adding backend functionality:
1. Add Go code
2. Run `wails generate module` to update bindings
3. Update frontend TypeScript to use new bindings
4. Test in actual app

Skipping steps 2-4 causes SDK/UI mismatch.

---

## How to Work on This

1. Read this file
2. Run `go build ./...` to see current errors
3. Pick highest priority TODO item
4. Fix it
5. Run build gates
6. If gates pass, mark item DONE and update "Previous Agent Notes"
7. If gates fail, keep working

---

## Previous Agent Notes

### Session: 2026-01-17 (SDK-to-UI Wiring Session)

**What was done:**
- Completed Phase 3 (SDK-to-UI Wiring) and Phase 4 (Cleanup)
- Updated TypeScript bindings to match Go exports
- Build gates all pass: `go build ./...`, `go test ./...`, `go vet ./...`

**Key changes:**
1. Updated `frontend/wailsjs/go/models.ts`:
   - Added `serviceName`, `serviceHost`, `connected` fields to `AgentSession`
   - Added `is_remote` field to `CoordinationStatus`
2. Updated `frontend/wailsjs/go/main/App.js` and `App.d.ts`:
   - Added PTY methods: `PTYSpawn`, `PTYWrite`, `PTYKill`, `PTYList`
3. Verified frontend already uses the new AgentSession fields (displays service info in UI)
4. Verified frontend builds successfully with updated bindings

**Files modified:**
- `frontend/wailsjs/go/models.ts` - Added missing model fields
- `frontend/wailsjs/go/main/App.js` - Added PTY method exports
- `frontend/wailsjs/go/main/App.d.ts` - Added PTY method type declarations

**Build gates verified:**
```
go build ./...   ✓ (exit 0)
go test ./...    ✓ (all tests pass)
go vet ./...     ✓ (no issues)
npm run build    ✓ (frontend compiles)
```

**Note:** `wails generate module` didn't regenerate bindings (possibly a Wails issue), so bindings were updated manually.

**Remaining work:**
- Files ready to commit: all provider/*.go changes, frontend bindings, exec_unix.go, exec_windows.go
- End-to-end GUI testing recommended

---

### Session: 2026-01-17 (Build Fix Session)

**What was done:**
- Fixed ALL Phase 1 (compile) and Phase 2 (tests) issues
- Build gates all pass: `go build ./...`, `go test ./...`, `go vet ./...`

**Key changes:**
1. Removed duplicate functions from `discovery.go` (kept in `discovery_legacy.go`)
2. Added 9 new fields to `SaturnService`: `SaturnVersion`, `MaxConcurrent`, `CurrentLoad`, `Security`, `HealthEndpoint`, `Models`, `GPU`, `VRAMGb`, `HealthStatus`
3. Added `StreamDelta` type to `provider.go` for streaming responses
4. Added `DiscoveryFilter` type and `FilterServices()` function to `provider.go`
5. Added `Saturn.ChatStream()` method with full SSE streaming support
6. Added `Saturn.GetService()` getter method
7. Added `ChatStream` to the `Provider` interface
8. Added `Discoverer` interface, `CreateDiscoverer()` factory, `globalServiceCache`, and `createPooledTransport()` to `cache.go`
9. Added `AvailableCapacity()`, `LoadFraction()`, and `SelectBestService()` for load-aware selection
10. Fixed test naming issue (priority semantics: lower number = higher priority)
11. Standardized on `hideWindow` (removed `hideCommandWindow` usage)

**Files modified:**
- `provider/discovery.go` - Extended SaturnService, added load-aware methods
- `provider/discovery_legacy.go` - Changed hideCommandWindow to hideWindow
- `provider/provider.go` - Added StreamDelta, DiscoveryFilter, FilterServices, ChatStream to interface
- `provider/saturn.go` - Added ChatStream, GetService, streaming types
- `provider/cache.go` - Added Discoverer interface, factory, transport
- `provider/cache_test.go` - Fixed priority test naming

**Build gates verified:**
```
go build ./...  ✓ (exit 0)
go test ./...   ✓ (all tests pass)
go vet ./...    ✓ (no issues)
```

**Next agent should:**
1. Handle Phase 3 (SDK-to-UI wiring) - regenerate Wails bindings
2. Handle Phase 4 cleanup - commit exec_unix.go and exec_windows.go files
3. Test the GUI application end-to-end

---

### Session: 2026-01-17 (Audit Session)

**What was done:**
- Audited all closed beads issues from previous session
- Found code does not compile - 13+ build errors
- Identified root causes: incomplete refactoring, tests written without implementations

**Verified working:**
- `coordinator.Start()` / `Stop()` in GUIAgent
- Background cache refresh (`StartBackgroundRefresh`, etc.)
- Remote agent discovery in `GetCoordinationStatuses()`
- `AgentSession` struct has new fields

**Verified broken:**
- `Saturn.GetService()` - called but doesn't exist
- `Saturn.ChatStream()` - called but doesn't exist
- `SaturnService` missing 8 fields
- `StreamDelta`, `DiscoveryFilter`, `FilterServices` - all undefined
- Load-aware selection functions - tests exist, no implementation

**Next agent should:**
1. Start with Phase 1 - make it compile
2. Run `go build ./...` frequently to verify progress
3. Do NOT close beads issues until build gates pass

---
