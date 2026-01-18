# OpenCode Patch Subsystem Specification

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                      Tool Layer                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │ PatchTool   │  │ BashTool    │  │ MultiEditTool       │  │
│  │ (disabled)  │  │ (heredoc)   │  │ (preferred)         │  │
│  └──────┬──────┘  └──────┬──────┘  └─────────────────────┘  │
│         │                │                                   │
│         └───────┬────────┘                                   │
│                 ▼                                            │
│  ┌──────────────────────────────────────────────────────┐   │
│  │              Patch Namespace (patch/index.ts)         │   │
│  │  ┌────────────┐  ┌─────────────┐  ┌──────────────┐   │   │
│  │  │ parsePatch │  │ computeRepl │  │ applyHunks   │   │   │
│  │  └────────────┘  └─────────────┘  └──────────────┘   │   │
│  └──────────────────────────────────────────────────────┘   │
│                           │                                  │
│                           ▼                                  │
│  ┌──────────────────────────────────────────────────────┐   │
│  │              File System + Bus Events                 │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

## Core Types

### Hunk (Discriminated Union)

```typescript
type Hunk =
  | { type: "add"; path: string; contents: string }
  | { type: "delete"; path: string }
  | { type: "update"; path: string; move_path?: string; chunks: UpdateFileChunk[] }
```

### UpdateFileChunk

```typescript
interface UpdateFileChunk {
  old_lines: string[]       // Expected original lines
  new_lines: string[]       // Replacement lines
  change_context?: string   // @@ marker content
  is_end_of_file?: boolean  // True if chunk is at EOF
}
```

### ApplyPatchAction

```typescript
interface ApplyPatchAction {
  changes: Map<string, ApplyPatchFileChange>
  patch: string
  cwd: string
}

type ApplyPatchFileChange =
  | { type: "add"; content: string }
  | { type: "delete"; content: string }
  | { type: "update"; unified_diff: string; move_path?: string; new_content: string }
```

### AffectedPaths

```typescript
interface AffectedPaths {
  added: string[]
  modified: string[]
  deleted: string[]
}
```

## Entry Points

### 1. Direct Patch Application

```typescript
async function applyPatch(patchText: string): Promise<AffectedPaths>
```

Parses patch text and applies all hunks to filesystem.

### 2. Parse-Only

```typescript
function parsePatch(patchText: string): { hunks: Hunk[] }
```

Returns structured hunks without side effects.

### 3. Bash Integration Detection

```typescript
function maybeParseApplyPatch(argv: string[]): MaybeApplyPatch
```

Detects `apply_patch` invocations:
- Direct: `["apply_patch", "<patch>"]`
- Heredoc: `["bash", "-lc", "apply_patch <<<'PATCH'\n...\nPATCH"]`

### 4. Verified Application

```typescript
async function maybeParseApplyPatchVerified(
  argv: string[],
  cwd: string
): Promise<MaybeApplyPatchVerified>
```

Parses, validates file existence, and computes diffs before application.

## Error Types

```typescript
enum ApplyPatchError {
  ParseError = "ParseError",           // Malformed patch syntax
  IoError = "IoError",                 // File read/write failure
  ComputeReplacements = "ComputeReplacements",  // Line matching failure
  ImplicitInvocation = "ImplicitInvocation"     // Raw patch without command
}
```

## Tool Integration (PatchTool)

The `PatchTool` wraps the `Patch` namespace with:

1. **Permission Checks**: Uses `ctx.ask()` for edit permission
2. **External Directory Validation**: Prevents writes outside project
3. **FileTime Tracking**: Coordinates with file watching system
4. **Bus Events**: Publishes `FileWatcher.Event.Updated` after changes
5. **Diff Generation**: Uses `createTwoFilesPatch` from `diff` package

### Tool Schema

```typescript
const PatchParams = z.object({
  patchText: z.string().describe("The full patch text...")
})
```

### Tool Output

```typescript
{
  title: "3 files changed",
  metadata: { diff: "<unified diff>" },
  output: "Patch applied successfully. 3 files changed:\n src/a.ts\n src/b.ts\n src/c.ts"
}
```

## Deprecation Status

The `patch.txt` tool description contains only:
```
do not use
```

This indicates the tool is disabled from LLM selection. Reasons likely include:

1. **Complexity**: Patch format harder for LLMs to generate correctly
2. **Debugging**: Errors harder to diagnose than single-file edits
3. **Preference**: `multiedit` provides similar atomicity with simpler interface

## Relationship to Other Tools

```
┌─────────────────────────────────────────────────────────────┐
│                    File Editing Tools                        │
├───────────────┬───────────────┬────────────────────────────┤
│    edit       │   multiedit   │    patch (deprecated)      │
├───────────────┼───────────────┼────────────────────────────┤
│ Single file   │ Single file   │ Multiple files             │
│ Single change │ Multiple ops  │ Multiple ops               │
│ Substring     │ Substring     │ Context-aware lines        │
│ First match   │ Sequential    │ Forward-seeking            │
│ Simple        │ Medium        │ Complex                    │
│ ACTIVE        │ ACTIVE        │ DISABLED                   │
└───────────────┴───────────────┴────────────────────────────┘
```

## File System Effects

All operations:
1. Validate paths against `Instance.directory`
2. Check/create parent directories
3. Update `FileTime` tracking
4. Publish bus events
5. Return unified diff for UI display

## Thread Safety

Not explicitly thread-safe. Designed for single-agent execution within a session context.

## Configuration

No external configuration. Hardcoded behaviors:
- Timeout: None (inherits from tool timeout)
- Max file size: None
- Binary detection: None (assumes text)
