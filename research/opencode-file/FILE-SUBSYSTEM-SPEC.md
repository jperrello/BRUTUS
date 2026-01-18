# OpenCode File Subsystem Specification

## Overview

The File subsystem provides the foundation for all file operations in OpenCode:
- Fast file discovery via ripgrep
- Real-time file watching
- Content reading with encoding detection
- Git-aware status and diffs
- Fuzzy file search

## Module Structure

```
src/file/
├── index.ts      # Main File namespace - read, list, search, status
├── ripgrep.ts    # Ripgrep wrapper - files(), tree(), search()
├── watcher.ts    # FileWatcher using @parcel/watcher
├── ignore.ts     # FileIgnore patterns
└── time.ts       # File modification time utilities
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      File Namespace                          │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐        │
│  │  read   │  │  list   │  │ search  │  │ status  │        │
│  └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘        │
│       │            │            │            │              │
│       ▼            ▼            ▼            ▼              │
│  ┌─────────────────────────────────────────────────┐       │
│  │              Instance State Cache               │       │
│  │   files: string[]    dirs: string[]             │       │
│  └─────────────────────────────────────────────────┘       │
│                          │                                  │
│                          ▼                                  │
│  ┌─────────────────────────────────────────────────┐       │
│  │                   Ripgrep                       │       │
│  │   files() → async generator                     │       │
│  │   tree() → BFS tree rendering                   │       │
│  │   search() → JSON match results                 │       │
│  └─────────────────────────────────────────────────┘       │
│                          │                                  │
│                          ▼                                  │
│  ┌─────────────────────────────────────────────────┐       │
│  │              FileWatcher                        │       │
│  │   @parcel/watcher → Bus events                  │       │
│  └─────────────────────────────────────────────────┘       │
└─────────────────────────────────────────────────────────────┘
```

## Data Types

### File.Info (git status entry)
```typescript
{
  path: string       // Relative file path
  added: number      // Lines added
  removed: number    // Lines removed
  status: "added" | "deleted" | "modified"
}
```

### File.Node (directory listing entry)
```typescript
{
  name: string       // Filename
  path: string       // Relative path from project root
  absolute: string   // Absolute filesystem path
  type: "file" | "directory"
  ignored: boolean   // Matches .gitignore
}
```

### File.Content (file read result)
```typescript
{
  type: "text"
  content: string           // File content (or base64 for binary)
  diff?: string             // Git diff if file changed
  patch?: StructuredPatch   // Parsed unified diff
  encoding?: "base64"       // Set for binary files
  mimeType?: string         // MIME type for binary files
}
```

## Core Operations

### File.read(path)
1. Resolve path relative to Instance.directory
2. Security check: path must not escape project directory
3. Check if file exists (return empty content if not)
4. Detect binary via MIME type → encode as base64
5. Read text content
6. If git project, compute diff from HEAD
7. Return content with optional diff/patch

### File.list(dir?)
1. Resolve directory (default: project root)
2. Security check: must not escape project
3. Load .gitignore and .ignore patterns
4. Read directory entries
5. Filter out .git and .DS_Store
6. Mark ignored files
7. Sort: directories first, then alphabetical

### File.search(query, options)
1. Get cached file/directory list from state
2. If no query, return raw list (limited)
3. Apply fuzzysort against file paths
4. Handle hidden files (.) specially:
   - If query starts with "." → include hidden
   - Otherwise sort hidden to end
5. Return top N matches

### File.status()
1. Run `git diff --numstat HEAD` for modified files
2. Run `git ls-files --others --exclude-standard` for untracked
3. Run `git diff --name-only --diff-filter=D HEAD` for deleted
4. Combine results with line counts

## State Management

Uses Instance.state() for project-scoped caching:
```typescript
const state = Instance.state(async () => {
  let cache = { files: [], dirs: [] }
  let fetching = false

  const fn = async (result) => {
    // Background populate using Ripgrep.files()
    for await (const file of Ripgrep.files({ cwd: Instance.directory })) {
      result.files.push(file)
      // Extract directory paths
    }
    cache = result
    fetching = false
  }

  return {
    async files() {
      if (!fetching) fn({ files: [], dirs: [] })
      return cache
    }
  }
})
```

Key characteristics:
- **Lazy initialization**: First access triggers full scan
- **Background refresh**: Non-blocking updates
- **Stale-while-revalidate**: Returns cached data immediately
- **Per-instance**: Each project has its own cache

## Binary Detection

MIME types that trigger base64 encoding:
- Top-level: image, audio, video, font, model, multipart
- Subtypes containing: zip, gzip, bzip, compressed, binary, pdf, msword, powerpoint, excel, ogg, exe, dmg, iso, rar

Text detection:
- `text/*` → not binary
- Contains `charset=` → not binary
- Everything else checked against binary markers

## Security

Path containment check before any read/list:
```typescript
if (!Instance.containsPath(full)) {
  throw new Error(`Access denied: path escapes project directory`)
}
```

Known limitations documented in source:
- Lexical only - symlinks can escape
- Windows cross-drive paths may bypass

## Events

File.Event.Edited - Published when agent edits a file:
```typescript
{
  type: "file.edited"
  properties: {
    file: string  // Path to edited file
  }
}
```

FileWatcher.Event.Updated - Published on filesystem changes:
```typescript
{
  type: "file.watcher.updated"
  properties: {
    file: string  // Changed file path
    event: "add" | "change" | "unlink"
  }
}
```

## Special Cases

### Global Home Directory
When Instance.directory is the user's home:
- Only scans 2 levels deep
- Ignores platform-specific dirs (Library on macOS, AppData on Windows)
- Ignores common build/dependency folders
- Returns only directories, not files

### Root Directory
If Instance.directory is filesystem root:
- Scanning disabled entirely
- Prevents catastrophic directory walks
