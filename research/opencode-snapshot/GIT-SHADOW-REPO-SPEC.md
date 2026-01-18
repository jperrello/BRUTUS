# Git Shadow Repository Specification

## Concept

A "shadow repository" is a Git repository that tracks the same working directory as the project but is stored in a completely separate location. This enables independent state tracking without affecting the user's Git workflow.

## Directory Structure

```
~/.local/share/opencode/
└── snapshot/
    └── {project-id}/
        ├── HEAD
        ├── config
        ├── objects/
        │   ├── pack/
        │   └── {hash-prefix}/
        │       └── {hash-suffix}
        └── refs/
```

Each project gets its own isolated shadow repository identified by project ID.

## Initialization Sequence

```bash
# 1. Create shadow git directory
mkdir -p ~/.local/share/opencode/snapshot/{project-id}

# 2. Initialize bare-ish repo with custom directories
GIT_DIR=~/.local/share/opencode/snapshot/{project-id} \
GIT_WORK_TREE=/path/to/project \
git init

# 3. Configure line ending handling (critical for Windows)
git --git-dir {shadow} config core.autocrlf false
```

## Environment Variables

All Git commands must specify both:
- `GIT_DIR`: Path to shadow repository
- `GIT_WORK_TREE`: Path to actual project directory

```typescript
await $`git init`
  .env({
    ...process.env,
    GIT_DIR: shadowPath,
    GIT_WORK_TREE: worktreePath,
  })
```

## Command Patterns

### Alternative: Flag-based (Used in OpenCode)

Instead of environment variables, OpenCode uses flags:

```bash
git --git-dir {shadow} --work-tree {worktree} <command>
```

This is more explicit and debuggable than env vars.

### Staging Files

```bash
git --git-dir {shadow} --work-tree {worktree} add .
```

Stages all files from worktree into shadow repo's index.

### Creating Snapshot

```bash
git --git-dir {shadow} --work-tree {worktree} write-tree
```

Writes current index to a tree object, returns tree hash (40-char hex).

### Comparing States

```bash
git -c core.autocrlf=false \
    --git-dir {shadow} \
    --work-tree {worktree} \
    diff --no-ext-diff --name-only {hash} -- .
```

Compares tree hash against current working directory.

### Reading Tree into Index

```bash
git --git-dir {shadow} --work-tree {worktree} read-tree {hash}
```

Populates index from tree object (doesn't modify working directory).

### Checkout from Index

```bash
git --git-dir {shadow} --work-tree {worktree} checkout-index -a -f
```

Forces all files in index to be written to working directory.

### Retrieve File at Snapshot

```bash
git -c core.autocrlf=false \
    --git-dir {shadow} \
    --work-tree {worktree} \
    show {hash}:{relative-path}
```

Outputs file contents as they existed in snapshot.

### Restore Specific File

```bash
git --git-dir {shadow} --work-tree {worktree} checkout {hash} -- {file}
```

Restores single file to its state in snapshot.

### Check if File Existed

```bash
git --git-dir {shadow} --work-tree {worktree} ls-tree {hash} -- {relative-path}
```

Returns non-empty output if file existed in snapshot.

## Key Differences from Normal Git Usage

| Aspect | Normal Git | Shadow Repo |
|--------|------------|-------------|
| Location | `.git/` in project | External data directory |
| Commits | Full commit history | Tree hashes only |
| Remote | Usually has origin | Never (local only) |
| Visibility | User sees via `git log` | Hidden from user |
| Purpose | Version control | State tracking |
| Lifetime | Permanent | Session-scoped or app-scoped |

## Why Tree Hashes Instead of Commits

Commits include:
- Tree hash
- Parent commit(s)
- Author info
- Committer info
- Timestamp
- Message

Tree objects include:
- File/directory entries
- Modes (permissions)
- Hashes of blobs/subtrees

For snapshot purposes, only the tree matters. Using `write-tree` directly:
1. Skips commit creation overhead
2. Avoids accumulating commit history
3. Still enables full diff/restore via tree hash
4. Produces same 40-char hash for diffing

## Autocrlf Configuration

**Critical on Windows**: Must set `core.autocrlf=false`

Why:
- Windows default may convert `\n` to `\r\n` on checkout
- This would show spurious diffs for every file
- Setting to `false` preserves exact file contents

Applied:
- Once during repo init (`git config core.autocrlf false`)
- On every diff command (`-c core.autocrlf=false`)

## File System Considerations

### Ignored Files

Shadow repo stages **all** files via `git add .` regardless of project's `.gitignore`. This ensures:
- Files the project ignores are still tracked for revert
- Build outputs can be snapshot/reverted
- Node_modules, venv, etc. are captured

**Implication**: Shadow repo can grow large for projects with many generated files.

### Binary Files

Binary files are:
- Staged and tracked normally
- Detected during diff via `--numstat` (shows `-` for additions/deletions)
- Content retrieval skipped (before/after return empty strings)

### Symbolic Links

Tracked as symlinks in Git's index. Restoration preserves link structure.

## Cleanup and Maintenance

### Manual Cleanup

Shadow repos accumulate loose objects over time. Could periodically run:
```bash
git --git-dir {shadow} gc --aggressive
```

### Project Removal

When project is deleted/removed from OpenCode, shadow repo should be deleted:
```bash
rm -rf ~/.local/share/opencode/snapshot/{project-id}
```

### Disk Usage Monitoring

Consider monitoring `~/.local/share/opencode/snapshot/` total size and warning users if it grows excessively.

## Security Considerations

1. **Sensitive Files**: Shadow repo captures ALL files, including `.env`, secrets, credentials that might be in `.gitignore`
2. **Storage Location**: User's data directory - same security as other app data
3. **No Encryption**: Files stored as plain Git objects
4. **No Remote Sync**: Data stays local, never pushed anywhere

## Error Recovery

If shadow repo becomes corrupted:
1. Delete `~/.local/share/opencode/snapshot/{project-id}`
2. Next `track()` call reinitializes
3. Only lose ability to revert to previous snapshots from this session

No user data loss since shadow repo only tracks state, doesn't store original content.
