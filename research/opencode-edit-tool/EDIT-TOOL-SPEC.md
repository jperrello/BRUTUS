# OpenCode Edit Tool Specification

## Overview

The edit tool performs string replacement operations on files. Unlike naive `string.replace()`, it employs a cascade of 9 fuzzy matching algorithms to tolerate common LLM mistakes like incorrect indentation, whitespace variations, and escape sequence confusion.

## Tool Definition

```typescript
EditTool = Tool.define("edit", {
  description: <from edit.txt>,
  parameters: z.object({
    filePath: z.string(),
    oldString: z.string(),
    newString: z.string(),
    replaceAll: z.boolean().optional()
  }),
  execute(params, ctx) { ... }
})
```

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `filePath` | string | Yes | Absolute path to file (relative paths resolved against `Instance.directory`) |
| `oldString` | string | Yes | Text to find and replace |
| `newString` | string | Yes | Replacement text |
| `replaceAll` | boolean | No | If true, replace all occurrences; default false |

## Execution Flow

### 1. Parameter Validation

```
if (!filePath) throw Error("filePath is required")
if (oldString === newString) throw Error("must be different")
```

### 2. Path Resolution

```
filePath = isAbsolute(filePath)
  ? filePath
  : join(Instance.directory, filePath)
```

### 3. External Directory Check

```
await assertExternalDirectory(ctx, filePath)
```

Verifies the path is within allowed workspace boundaries.

### 4. File Lock Acquisition

```
await FileTime.withLock(filePath, async () => { ... })
```

All operations run inside a promise-based lock to prevent concurrent modifications.

### 5. Special Case: Empty oldString (File Creation)

If `oldString === ""`:
- Write `newString` directly to file (creates or overwrites)
- No file existence check required
- Still requires permission

### 6. Normal Case: String Replacement

#### 6a. File Validation

```
const stats = await file.stat()
if (!stats) throw Error("File not found")
if (stats.isDirectory()) throw Error("Path is a directory")
```

#### 6b. Read Assertion

```
await FileTime.assert(ctx.sessionID, filePath)
```

Fails if:
- File was not previously read by this session
- File was modified externally since last read

#### 6c. Read Content

```
contentOld = await file.text()
```

#### 6d. Fuzzy Replacement

```
contentNew = replace(contentOld, oldString, newString, replaceAll)
```

This calls the cascade of 9 replacers.

### 7. Permission Request

```
await ctx.ask({
  permission: "edit",
  patterns: [relativePath],
  always: ["*"],
  metadata: { filepath, diff }
})
```

User must approve the edit before it's applied.

### 8. Write File

```
await file.write(contentNew)
```

### 9. Post-Write Events

```
await Bus.publish(File.Event.Edited, { file: filePath })
FileTime.read(ctx.sessionID, filePath)  // Update read timestamp
```

### 10. LSP Diagnostics

```
await LSP.touchFile(filePath, true)
const diagnostics = await LSP.diagnostics()
```

Reports any syntax errors introduced by the edit.

### 11. Return Value

```typescript
return {
  metadata: {
    diagnostics,
    diff,
    filediff: {
      file: string,
      before: string,
      after: string,
      additions: number,
      deletions: number
    }
  },
  title: relativePath,
  output: "Edit applied successfully." + diagnosticErrors
}
```

## Error Conditions

| Condition | Error Message |
|-----------|---------------|
| Missing filePath | "filePath is required" |
| oldString === newString | "oldString and newString must be different" |
| File doesn't exist | "File {path} not found" |
| Path is directory | "Path is a directory, not a file: {path}" |
| Never read file | (from FileTime.assert) |
| File modified externally | (from FileTime.assert) |
| No match found | "oldString not found in content" |
| Multiple matches | "Found multiple matches... Provide more surrounding lines" |

## Diff Generation

Uses `diff` library:

```typescript
import { createTwoFilesPatch, diffLines } from "diff"

diff = createTwoFilesPatch(filePath, filePath, contentOld, contentNew)
diff = trimDiff(diff)  // Remove common indentation
```

The `trimDiff()` function strips leading whitespace from diff lines for cleaner display.

## Line Ending Normalization

```typescript
function normalizeLineEndings(text: string): string {
  return text.replaceAll("\r\n", "\n")
}
```

Applied before diff generation to prevent CRLF/LF mismatches from appearing as changes.

## Constants

```typescript
MAX_DIAGNOSTICS_PER_FILE = 20  // Limit LSP errors shown
```

## File Time Tracking

### Read Tracking

```typescript
FileTime.read(sessionID, filePath)  // Record read timestamp
FileTime.get(sessionID, filePath)   // Get recorded timestamp
```

### Assertion

```typescript
FileTime.assert(sessionID, filePath)
```

Checks:
1. File was read by this session
2. File mtime hasn't changed since read

### Locking

```typescript
await FileTime.withLock(filePath, async () => {
  // Exclusive access to file
})
```

Promise-based queue ensures sequential writes.

## Integration Points

### Events Published

- `File.Event.Edited` - After successful write

### LSP Integration

- `LSP.touchFile(path, true)` - Notify LSP of file change
- `LSP.diagnostics()` - Get current diagnostic state
- `LSP.Diagnostic.pretty()` - Format diagnostic for display

### Permission System

- Permission type: `"edit"`
- Pattern matching against relative file path
- Always option: `["*"]` allows "always allow" for all files
