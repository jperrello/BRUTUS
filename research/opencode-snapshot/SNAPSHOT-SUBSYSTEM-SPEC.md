# OpenCode Snapshot Subsystem Specification

## Overview

The Snapshot subsystem provides Git-based state tracking for the working directory, enabling:
- Change detection between agent steps
- File-level revert capabilities
- Unified diff generation
- Full before/after content retrieval

## Core Concepts

### Shadow Git Repository

Unlike traditional Git operations that use the project's `.git` directory, snapshots use a **shadow repository** stored in the application's data directory:

```
~/.local/share/opencode/snapshot/{project-id}/
```

This isolation ensures:
1. Snapshot operations never interfere with user's Git workflow
2. No conflicts with project's `.gitignore` rules
3. Clean separation between user commits and agent tracking

### Tree Hash vs Commits

Snapshots use `git write-tree` rather than full commits. This produces a tree hash representing the exact state of all staged files without creating commit history. Benefits:
- Faster (no commit metadata)
- Lighter (no commit chain)
- Sufficient for diff/restore operations

## API Specification

### `track(): Promise<string | undefined>`

Creates a snapshot of the current working directory state.

**Behavior**:
1. Check if project uses Git VCS and snapshots are enabled
2. Initialize shadow Git repo if first run
3. Configure `core.autocrlf=false` for Windows compatibility
4. Stage all files in worktree (`git add .`)
5. Write staged files to tree object (`git write-tree`)
6. Return tree hash

**Returns**: Tree hash string, or `undefined` if snapshots disabled

**Example**:
```typescript
const hash = await Snapshot.track()
// hash = "a1b2c3d4e5f6..."
```

### `patch(hash: string): Promise<Patch>`

Compares a previous snapshot hash against current state to find changed files.

**Behavior**:
1. Stage current files (`git add .`)
2. Run `git diff --name-only {hash} -- .`
3. Parse output into list of changed file paths
4. Return patch object with hash and absolute file paths

**Returns**:
```typescript
{
  hash: string,
  files: string[]  // Absolute paths relative to worktree
}
```

**Example**:
```typescript
const patch = await Snapshot.patch("a1b2c3d4...")
// { hash: "a1b2c3d4...", files: ["/project/src/main.ts", "/project/README.md"] }
```

### `restore(snapshot: string): Promise<void>`

Restores the working directory to a previous snapshot state.

**Behavior**:
1. Read tree into index (`git read-tree {snapshot}`)
2. Checkout all files from index (`git checkout-index -a -f`)

**Warning**: Destructive operation - overwrites current file contents

### `revert(patches: Patch[]): Promise<void>`

Selectively reverts specific files from multiple patches.

**Behavior**:
For each file in each patch (deduplicated):
1. Attempt `git checkout {hash} -- {file}`
2. If checkout fails, check if file existed in snapshot:
   - If existed: keep current file (checkout issue)
   - If didn't exist: delete file (was created after snapshot)

**Use Case**: Undo changes from specific agent steps without full restore

### `diff(hash: string): Promise<string>`

Generates unified diff between snapshot and current state.

**Returns**: Standard unified diff format string

**Example Output**:
```diff
diff --git a/src/main.ts b/src/main.ts
--- a/src/main.ts
+++ b/src/main.ts
@@ -1,3 +1,4 @@
 import { app } from "./app"
+import { logger } from "./logger"

 app.start()
```

### `diffFull(from: string, to: string): Promise<FileDiff[]>`

Generates detailed per-file diff information between two snapshots.

**Returns**:
```typescript
{
  file: string,      // Relative path
  before: string,    // Full content before
  after: string,     // Full content after
  additions: number, // Lines added
  deletions: number  // Lines deleted
}[]
```

**Binary Handling**: Binary files return `additions=0`, `deletions=0`, empty before/after

## Configuration

### Enable/Disable

Via OpenCode config:
```typescript
{
  snapshot: boolean  // Default: true
}
```

### VCS Requirement

Snapshots only activate when `Instance.project.vcs === "git"`. Non-Git projects skip all snapshot operations gracefully.

## Data Structures

### Patch Schema (Zod)

```typescript
const Patch = z.object({
  hash: z.string(),
  files: z.string().array(),
})
```

### FileDiff Schema (Zod)

```typescript
const FileDiff = z.object({
  file: z.string(),
  before: z.string(),
  after: z.string(),
  additions: z.number(),
  deletions: z.number(),
}).meta({ ref: "FileDiff" })
```

## Error Handling

All Git commands use `.nothrow()` to prevent exceptions:
- Failed `git diff`: Returns empty patch `{ hash, files: [] }`
- Failed `git restore`: Logs error, continues to next file
- Failed checkout during revert: Checks if file existed, handles appropriately

## Platform Considerations

### Windows Line Endings

Critical setting on shadow repo init:
```bash
git config core.autocrlf false
```

This prevents Git from modifying line endings, ensuring diffs accurately reflect actual file changes.

### Path Handling

All file paths in patches are absolute, resolved via `path.join(Instance.worktree, relativePath)`.

## Integration Points

### Session Processor

```
start-step → track() → [tool execution] → patch() → store patch part
```

### Session Revert

```
revert(messageID) → collect patches → Snapshot.revert(patches) → update session
unrevert() → Snapshot.restore(original) → clear revert state
```

## Performance Characteristics

| Operation | Git Commands | Typical Duration |
|-----------|--------------|------------------|
| track() | init?, add, write-tree | 50-200ms |
| patch() | add, diff | 30-100ms |
| restore() | read-tree, checkout-index | 50-150ms |
| diff() | add, diff | 30-100ms |
| diffFull() | diff --numstat, N × show | 100-500ms |

Note: Times vary significantly based on repository size and file count.
