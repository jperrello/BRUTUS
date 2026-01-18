# Ripgrep Integration Specification

## Overview

OpenCode bundles ripgrep for fast file discovery and code search. The binary is downloaded at runtime if not found in PATH.

## Binary Management

### Platform Detection
```typescript
const PLATFORM = {
  "arm64-darwin": { platform: "aarch64-apple-darwin", extension: "tar.gz" },
  "arm64-linux":  { platform: "aarch64-unknown-linux-gnu", extension: "tar.gz" },
  "x64-darwin":   { platform: "x86_64-apple-darwin", extension: "tar.gz" },
  "x64-linux":    { platform: "x86_64-unknown-linux-musl", extension: "tar.gz" },
  "x64-win32":    { platform: "x86_64-pc-windows-msvc", extension: "zip" },
}
```

Key: `${process.arch}-${process.platform}`

### Download Flow
```
1. Check Bun.which("rg") for system ripgrep
   ├── Found → Use system binary
   └── Not found → Continue

2. Check Global.Path.bin/rg(.exe)
   ├── Exists → Use cached binary
   └── Not exists → Download

3. Download from GitHub releases
   URL: github.com/BurntSushi/ripgrep/releases/download/{version}/{filename}
   Version: 14.1.1 (hardcoded)

4. Extract based on extension
   ├── tar.gz → tar -xzf with --strip-components=1
   │            macOS: --include=*/rg
   │            Linux: --wildcards */rg
   └── zip → ZipReader extract rg.exe only

5. Set permissions (chmod 0o755 on non-Windows)
```

### Error Types
- `UnsupportedPlatformError` - Unknown arch/platform combination
- `DownloadFailedError` - HTTP request failed
- `ExtractionFailedError` - Archive extraction failed

## API

### Ripgrep.files(options)
Async generator yielding file paths.

```typescript
interface FilesOptions {
  cwd: string        // Directory to scan
  glob?: string[]    // Include/exclude patterns
  hidden?: boolean   // Include hidden files (default: true)
  follow?: boolean   // Follow symlinks (default: true)
  maxDepth?: number  // Limit directory depth
}
```

Implementation:
```
rg --files --glob=!.git/* [--follow] [--hidden] [--max-depth=N] [--glob=pattern...]
```

Returns:
- Streaming line reader
- Handles both Unix (\n) and Windows (\r\n) line endings
- 20MB max buffer

### Ripgrep.tree(options)
Generates a truncated directory tree.

```typescript
interface TreeOptions {
  cwd: string      // Directory to scan
  limit?: number   // Max entries (default: 50)
}
```

Algorithm (BFS with breadth-first truncation):
```
1. Collect all files via Ripgrep.files()
2. Build tree structure from paths
3. Sort each level:
   - Directories before files
   - Alphabetical within type
4. BFS traversal:
   - Process level by level
   - Round-robin across directories at same depth
   - Stop at limit
5. Add "[N truncated]" markers for omitted entries
6. Render as indented text
```

Output format:
```
src/
    agent/
        agent.go
        loop.go
    tools/
        bash.go
        read.go
        [3 truncated]
main.go
```

### Ripgrep.search(options)
Pattern search with JSON output.

```typescript
interface SearchOptions {
  cwd: string        // Directory to search
  pattern: string    // Regex pattern
  glob?: string[]    // File filters
  limit?: number     // Max matches per file
  follow?: boolean   // Follow symlinks (default: true)
}
```

Implementation:
```
rg --json --hidden --glob='!.git/*' [--follow] [--glob=pattern...] [--max-count=N] -- {pattern}
```

Returns:
```typescript
interface Match {
  path: { text: string }
  lines: { text: string }
  line_number: number
  absolute_offset: number
  submatches: Array<{
    match: { text: string }
    start: number
    end: number
  }>
}
```

## JSON Output Parsing

Ripgrep --json emits newline-delimited JSON:
```typescript
const Result = z.union([Begin, Match, End, Summary])

// Begin - start of file matches
{ type: "begin", data: { path: { text: "..." } } }

// Match - individual match
{ type: "match", data: { ... } }

// End - end of file, includes stats
{ type: "end", data: { path: {...}, stats: {...} } }

// Summary - overall search stats
{ type: "summary", data: { elapsed_total: {...}, stats: {...} } }
```

Only Match results are returned from search().

## Performance Characteristics

File listing:
- 20MB buffer prevents memory issues on huge repos
- Streaming iterator avoids full collection in memory
- Excludes .git/ by default

Tree generation:
- O(n) file collection
- O(n log n) sorting per level
- O(limit) output generation
- Filters .opencode directories

Search:
- Uses ripgrep's native JSON mode
- Respects .gitignore by default
- Hidden files included by default
- Symlinks followed by default

## Windows Considerations

- Line ending normalization: split on /\r?\n/
- ZIP extraction uses @zip.js/zip.js
- No chmod call after extraction
- Extension is .exe
