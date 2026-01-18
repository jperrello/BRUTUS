# File Watcher Specification

## Overview

FileWatcher provides real-time filesystem change detection using @parcel/watcher, publishing events through the Bus system.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                  FileWatcher                        │
│                                                     │
│  ┌─────────────────────────────────────────────┐   │
│  │           @parcel/watcher                   │   │
│  │  Platform-specific native bindings          │   │
│  │  ├── darwin: fs-events                      │   │
│  │  ├── linux: inotify                         │   │
│  │  └── win32: windows (ReadDirectoryChangesW) │   │
│  └─────────────────────────────────────────────┘   │
│                       │                             │
│                       ▼                             │
│  ┌─────────────────────────────────────────────┐   │
│  │         Subscription Callback               │   │
│  │    create → "add"                           │   │
│  │    update → "change"                        │   │
│  │    delete → "unlink"                        │   │
│  └─────────────────────────────────────────────┘   │
│                       │                             │
│                       ▼                             │
│  ┌─────────────────────────────────────────────┐   │
│  │              Bus.publish()                  │   │
│  │    FileWatcher.Event.Updated                │   │
│  └─────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────┘
```

## Binding Loading

Dynamic native module loading:
```typescript
const binding = require(
  `@parcel/watcher-${process.platform}-${process.arch}` +
  `${process.platform === "linux" ? `-${OPENCODE_LIBC || "glibc"}` : ""}`
)
return createWrapper(binding)
```

Supported combinations:
- darwin-arm64
- darwin-x64
- linux-arm64-glibc
- linux-arm64-musl
- linux-x64-glibc
- linux-x64-musl
- win32-x64

## Subscription Strategy

Two separate watchers:

### 1. Project Directory (Experimental)
```typescript
if (Flag.OPENCODE_EXPERIMENTAL_FILEWATCHER) {
  watcher().subscribe(Instance.directory, callback, {
    ignore: [...FileIgnore.PATTERNS, ...cfgIgnores],
    backend
  })
}
```

Ignored patterns (from FileIgnore.PATTERNS):
- node_modules, vendor, dist, build, target, .git, etc.
- *.swp, *.pyc, *.log, coverage/, tmp/

### 2. Git Directory (Always)
```typescript
const vcsDir = await $`git rev-parse --git-dir`.cwd(Instance.worktree).text()
const ignoreList = gitDirContents.filter(entry => entry !== "HEAD")
watcher().subscribe(vcsDir, callback, { ignore: ignoreList, backend })
```

Only watches .git/HEAD - triggers on:
- Branch switches
- Commits
- Rebases

## Event Flow

```
Filesystem change
      │
      ▼
@parcel/watcher callback
      │
      ▼
Event type mapping:
  create → "add"
  update → "change"
  delete → "unlink"
      │
      ▼
Bus.publish(FileWatcher.Event.Updated, {
  file: evt.path,
  event: eventType
})
```

## Configuration

Via config.watcher.ignore:
```yaml
watcher:
  ignore:
    - "custom-build/"
    - "generated/"
```

## Timeout Handling

10 second timeout for subscription:
```typescript
const SUBSCRIBE_TIMEOUT_MS = 10_000

const sub = await withTimeout(pending, SUBSCRIBE_TIMEOUT_MS).catch(err => {
  log.error("failed to subscribe", { error: err })
  pending.then(s => s.unsubscribe()).catch(() => {})
  return undefined
})
```

Graceful degradation - watcher failure doesn't crash the application.

## Feature Flags

- `OPENCODE_EXPERIMENTAL_FILEWATCHER` - Enable project directory watching
- `OPENCODE_EXPERIMENTAL_DISABLE_FILEWATCHER` - Disable all file watching

## Lifecycle

Initialization:
```typescript
const state = Instance.state(
  async () => { /* setup subscriptions */ },
  async (state) => {
    // Cleanup on instance dispose
    await Promise.all(state.subs.map(sub => sub?.unsubscribe()))
  }
)
```

Per-project lifecycle bound to Instance.state.

## Backend Selection

Automatic platform detection:
```typescript
const backend = (() => {
  if (process.platform === "win32") return "windows"
  if (process.platform === "darwin") return "fs-events"
  if (process.platform === "linux") return "inotify"
})()
```

## Limitations

1. Only for git projects (vcs check on init)
2. Experimental flag for full project watching
3. Git directory only watches HEAD by default
4. Native binding required per platform

## Event Schema

```typescript
FileWatcher.Event.Updated = BusEvent.define(
  "file.watcher.updated",
  z.object({
    file: z.string(),
    event: z.union([
      z.literal("add"),
      z.literal("change"),
      z.literal("unlink")
    ])
  })
)
```
