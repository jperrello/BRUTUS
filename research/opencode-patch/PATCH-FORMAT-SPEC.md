# OpenCode Patch Format Specification

## Overview

OpenCode uses a custom patch format distinct from unified diff. The format is designed for LLM-friendliness with explicit markers and human-readable structure.

## Patch Structure

```
*** Begin Patch
*** <Operation> File: <path>
[content]
*** End Patch
```

## Operation Types

### 1. Add File

Creates a new file with specified contents.

```
*** Begin Patch
*** Add File: path/to/new/file.ts
+line 1 of content
+line 2 of content
+line 3 of content
*** End Patch
```

**Rules:**
- Each content line MUST be prefixed with `+`
- Parent directories are created automatically
- Trailing newline is stripped from aggregated content

### 2. Delete File

Removes an existing file.

```
*** Begin Patch
*** Delete File: path/to/remove.ts
*** End Patch
```

**Rules:**
- No content section
- File must exist (error otherwise)

### 3. Update File

Modifies portions of an existing file using context-aware chunks.

```
*** Begin Patch
*** Update File: path/to/modify.ts
@@ optional context line that exists in file
-line to remove
+line to add
 line that stays (context)
-another removal
+replacement line
*** End Patch
```

### 4. Update File with Move

Renames/moves file while applying changes.

```
*** Begin Patch
*** Update File: old/path/file.ts
*** Move to: new/path/file.ts
@@ function declaration
-old implementation
+new implementation
*** End Patch
```

## Line Prefixes

| Prefix | Meaning |
|--------|---------|
| `+` | Add this line (only in new) |
| `-` | Remove this line (only in old) |
| ` ` (space) | Context line (in both old and new) |
| `@@` | Context seek marker |

## Context Seeking

The `@@` marker defines a context line used to locate the change position:

```
@@ export function processData(
```

The algorithm seeks forward from current position to find a line matching this context exactly. Changes are applied immediately after the context match.

## Chunk Structure (Internal)

Each chunk in an update operation produces:

```typescript
interface UpdateFileChunk {
  old_lines: string[]    // Lines expected in original (- and space prefixed)
  new_lines: string[]    // Lines for replacement (+ and space prefixed)
  change_context?: string // Text after @@ marker
  is_end_of_file?: boolean
}
```

## End of File Marker

Special marker for appending at file end:

```
@@ last line of file
+new line at end
*** End of File
```

## Multiple Files

A single patch can contain multiple file operations:

```
*** Begin Patch
*** Add File: src/new.ts
+export const NEW = true
*** Update File: src/index.ts
@@ import { OLD }
-import { OLD } from './old'
+import { OLD } from './old'
+import { NEW } from './new'
*** Delete File: src/deprecated.ts
*** End Patch
```

## Error Cases

| Condition | Error |
|-----------|-------|
| Missing `*** Begin Patch` | ParseError |
| Missing `*** End Patch` | ParseError |
| Context line not found | ComputeReplacements |
| Old lines not found in file | ComputeReplacements |
| File not found (for update/delete) | IoError |

## Parser Implementation Notes

1. Split patch text by newlines
2. Find `*** Begin Patch` and `*** End Patch` markers
3. Iterate lines between markers
4. For each `*** <Op> File:` header:
   - Extract file path
   - Check for `*** Move to:` on next line (updates only)
   - Parse content section based on operation type
5. Aggregate hunks into result array

## Comparison with Unified Diff

| Aspect | OpenCode Patch | Unified Diff |
|--------|---------------|--------------|
| Markers | `*** Begin/End Patch` | `--- a/` / `+++ b/` |
| Context | `@@` single line | `@@ -n,m +n,m @@` range |
| Multi-file | Native support | Concatenated diffs |
| LLM-friendly | High (explicit structure) | Medium |
| Line numbers | Implicit (context-based) | Explicit ranges |
