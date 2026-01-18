# OpenCode Output Truncation Algorithm

## Overview

The Truncate namespace provides automatic output truncation to prevent overwhelming the LLM context while preserving full output for user access.

## Constants

```typescript
export const MAX_LINES = 2000
export const MAX_BYTES = 50 * 1024  // 50KB
export const DIR = path.join(Global.Path.data, "tool-output")
export const GLOB = path.join(DIR, "*")
const RETENTION_MS = 7 * 24 * 60 * 60 * 1000  // 7 days
```

## Truncation Result Types

```typescript
export type Result =
  | { content: string; truncated: false }
  | { content: string; truncated: true; outputPath: string }
```

When truncated:
- `content`: Truncated output with hint message
- `outputPath`: Path to full output file

## Options

```typescript
export interface Options {
  maxLines?: number    // Default: 2000
  maxBytes?: number    // Default: 50KB
  direction?: "head" | "tail"  // Default: "head"
}
```

## Algorithm

### Step 1: Early Return Check

```typescript
const lines = text.split("\n")
const totalBytes = Buffer.byteLength(text, "utf-8")

if (lines.length <= maxLines && totalBytes <= maxBytes) {
  return { content: text, truncated: false }
}
```

If both limits satisfied, return unmodified.

### Step 2: Collect Lines Within Limits

**Head Direction (default):**
```typescript
for (i = 0; i < lines.length && i < maxLines; i++) {
  const size = Buffer.byteLength(lines[i], "utf-8") + (i > 0 ? 1 : 0)  // +1 for newline
  if (bytes + size > maxBytes) {
    hitBytes = true
    break
  }
  out.push(lines[i])
  bytes += size
}
```

**Tail Direction:**
```typescript
for (i = lines.length - 1; i >= 0 && out.length < maxLines; i--) {
  const size = Buffer.byteLength(lines[i], "utf-8") + (out.length > 0 ? 1 : 0)
  if (bytes + size > maxBytes) {
    hitBytes = true
    break
  }
  out.unshift(lines[i])  // Prepend to maintain order
  bytes += size
}
```

### Step 3: Save Full Output

```typescript
await init()  // Ensure cleanup has run once
const id = Identifier.ascending("tool")
const filepath = path.join(DIR, id)
await Bun.write(Bun.file(filepath), text)
```

File naming: `tool_<timestamp-based-id>`

### Step 4: Generate Hint Message

Hint varies based on agent capabilities:

```typescript
function hasTaskTool(agent?: Agent.Info): boolean {
  if (!agent?.permission) return false
  const rule = PermissionNext.evaluate("task", "*", agent.permission)
  return rule.action !== "deny"
}
```

**With Task Tool:**
```
The tool call succeeded but the output was truncated. Full output saved to: {path}
Use the Task tool to have explore agent process this file with Grep and Read
(with offset/limit). Do NOT read the full file yourself - delegate to save context.
```

**Without Task Tool:**
```
The tool call succeeded but the output was truncated. Full output saved to: {path}
Use Grep to search the full content or Read with offset/limit to view specific sections.
```

### Step 5: Format Message

**Head direction:**
```
{preview}

...{removed} {unit} truncated...

{hint}
```

**Tail direction:**
```
...{removed} {unit} truncated...

{hint}

{preview}
```

Unit is either "lines" or "bytes" depending on which limit was hit.

## Cleanup Process

Old output files are automatically cleaned:

```typescript
export async function cleanup() {
  const cutoff = Identifier.timestamp(
    Identifier.create("tool", false, Date.now() - RETENTION_MS)
  )
  const glob = new Bun.Glob("tool_*")
  const entries = await Array.fromAsync(
    glob.scan({ cwd: DIR, onlyFiles: true })
  ).catch(() => [] as string[])

  for (const entry of entries) {
    if (Identifier.timestamp(entry) >= cutoff) continue
    await fs.unlink(path.join(DIR, entry)).catch(() => {})
  }
}
```

Cleanup runs lazily (once per session) via:
```typescript
const init = lazy(cleanup)
// ...
await init()  // Called before saving new file
```

## Integration with Tools

### Default Behavior (via Tool.define)

```typescript
if (result.metadata.truncated !== undefined) {
  return result  // Tool handles its own truncation
}
const truncated = await Truncate.output(result.output, {}, initCtx?.agent)
```

### Tool Opt-Out

Tools can handle truncation themselves by setting `metadata.truncated`:

```typescript
return {
  title: "...",
  output: myTruncatedOutput,
  metadata: {
    truncated: true,  // Signals: don't auto-truncate
    outputPath: myCustomPath,
  },
}
```

### Read Tool Example

Read tool has its own limit handling:

```typescript
const DEFAULT_READ_LIMIT = 2000
const MAX_LINE_LENGTH = 2000
const MAX_BYTES = 50 * 1024

// Custom truncation with offset/limit support
for (let i = offset; i < Math.min(lines.length, offset + limit); i++) {
  const line = lines[i].length > MAX_LINE_LENGTH
    ? lines[i].substring(0, MAX_LINE_LENGTH) + "..."
    : lines[i]
  // ...
}

// Returns truncated: true in metadata to skip auto-truncation
return {
  metadata: { preview, truncated: hasMoreLines || truncatedByBytes },
  // ...
}
```

## Bash Tool Streaming

Bash tool truncates metadata differently (for UI, not agent):

```typescript
const MAX_METADATA_LENGTH = 30_000

ctx.metadata({
  metadata: {
    output: output.length > MAX_METADATA_LENGTH
      ? output.slice(0, MAX_METADATA_LENGTH) + "\n\n..."
      : output,
    description: params.description,
  },
})
```

Full output still goes to agent; metadata is just for UI display.
