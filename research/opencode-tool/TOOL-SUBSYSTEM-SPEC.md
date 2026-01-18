# OpenCode Tool Subsystem Specification

## Overview

The Tool Subsystem defines the core abstraction for all capabilities in OpenCode. Every action the agent can take - reading files, executing commands, searching code, spawning subagents - is implemented as a Tool.

## Core Types

### Tool.Info

The fundamental tool definition type:

```typescript
interface Info<Parameters extends z.ZodType, M extends Metadata> {
  id: string
  init: (ctx?: InitContext) => Promise<{
    description: string
    parameters: Parameters
    execute(args: z.infer<Parameters>, ctx: Context): Promise<{
      title: string
      metadata: M
      output: string
      attachments?: MessageV2.FilePart[]
    }>
    formatValidationError?(error: z.ZodError): string
  }>
}
```

Key aspects:
- `id`: Unique tool identifier (e.g., "read", "bash", "edit")
- `init`: Async factory that returns tool configuration (allows dynamic descriptions)
- `parameters`: Zod schema for input validation
- `execute`: The actual tool implementation
- `formatValidationError`: Optional custom error formatter

### Tool.Context

The context passed to every tool execution:

```typescript
type Context<M extends Metadata> = {
  sessionID: string
  messageID: string
  agent: string
  abort: AbortSignal
  callID?: string
  extra?: { [key: string]: any }
  metadata(input: { title?: string; metadata?: M }): void
  ask(input: Omit<PermissionNext.Request, "id" | "sessionID" | "tool">): Promise<void>
}
```

Key aspects:
- `sessionID/messageID`: Links tool call to conversation context
- `abort`: Signal for cancellation (user abort, timeout)
- `metadata()`: Push real-time updates to UI (streaming)
- `ask()`: Request user permission for dangerous operations
- `extra`: Bypass flags and additional context

### InitContext

Context available during tool initialization:

```typescript
interface InitContext {
  agent?: Agent.Info
}
```

Purpose: Allows tools to customize behavior based on which agent is using them. The `agent` parameter is used to:
- Adjust truncation hints (e.g., mention Task tool if available)
- Filter descriptions for agent capabilities
- Apply agent-specific permission rules

## Tool.define() Factory

The standard way to create tools:

```typescript
function define<Parameters extends z.ZodType, Result extends Metadata>(
  id: string,
  init: Info<Parameters, Result>["init"] | Awaited<ReturnType<Info<Parameters, Result>["init"]>>
): Info<Parameters, Result>
```

Behavior:
1. Accepts either async init function or static tool config
2. Wraps execute to add automatic validation
3. Wraps execute to add automatic truncation (unless tool opts out)

### Automatic Validation

Before every execution, Tool.define validates arguments:

```typescript
try {
  toolInfo.parameters.parse(args)
} catch (error) {
  if (error instanceof z.ZodError && toolInfo.formatValidationError) {
    throw new Error(toolInfo.formatValidationError(error), { cause: error })
  }
  throw new Error(
    `The ${id} tool was called with invalid arguments: ${error}.\nPlease rewrite the input so it satisfies the expected schema.`,
    { cause: error }
  )
}
```

### Automatic Truncation

After every execution, output is truncated unless tool handles it:

```typescript
if (result.metadata.truncated !== undefined) {
  return result  // Tool handles its own truncation
}
const truncated = await Truncate.output(result.output, {}, initCtx?.agent)
return {
  ...result,
  output: truncated.content,
  metadata: {
    ...result.metadata,
    truncated: truncated.truncated,
    ...(truncated.truncated && { outputPath: truncated.outputPath }),
  },
}
```

## Tool Return Value

Every tool returns:

```typescript
{
  title: string           // Short description for UI (e.g., "src/file.ts")
  metadata: M             // Tool-specific data for UI rendering
  output: string          // Text returned to agent
  attachments?: FilePart[] // Optional binary files (images, PDFs)
}
```

### Attachments

For non-text content like images:

```typescript
{
  id: string
  sessionID: string
  messageID: string
  type: "file"
  mime: string           // e.g., "image/png"
  url: string            // data:mime;base64,... or file path
}
```

## Permission System Integration

Tools request permissions via `ctx.ask()`:

```typescript
await ctx.ask({
  permission: "edit",           // Permission type
  patterns: ["src/file.ts"],    // Specific patterns for this call
  always: ["*"],                // Patterns for "always allow" option
  metadata: { diff },           // Context shown to user
})
```

Permission types observed:
- `read` - File read access
- `edit` - File modification
- `bash` - Shell command execution
- `task` - Subagent spawning
- `external_directory` - Access outside project root

## Real-time Metadata Updates

Tools can stream status via `ctx.metadata()`:

```typescript
// Bash tool example - streaming output
const append = (chunk: Buffer) => {
  output += chunk.toString()
  ctx.metadata({
    metadata: {
      output: output.slice(0, MAX_METADATA_LENGTH),
      description: params.description,
    },
  })
}
proc.stdout?.on("data", append)
proc.stderr?.on("data", append)
```

This enables:
- Live command output in UI
- Progress indicators
- Partial results display

## Error Handling

Tools should throw errors with descriptive messages:

```typescript
throw new Error(`File not found: ${filepath}`)
throw new Error("oldString and newString must be different")
throw new Error(`Unknown agent type: ${params.subagent_type}`)
```

The framework catches errors and:
1. Records them in tool state as "error"
2. Returns error text to the agent
3. Allows agent to retry or recover

## Tool State Lifecycle

```
┌─────────┐     ┌─────────┐     ┌───────────┐
│ pending │ ──► │ running │ ──► │ completed │
└─────────┘     └────┬────┘     └───────────┘
                     │
                     ▼
                ┌─────────┐
                │  error  │
                └─────────┘
```

States stored per tool call:
- `pending` - Queued, not yet started
- `running` - Currently executing
- `completed` - Success (has output, title, metadata)
- `error` - Failed (has error message)

## Abort Handling

Tools must respect the abort signal:

```typescript
// Bash tool example
if (ctx.abort.aborted) {
  aborted = true
  await kill()
}

const abortHandler = () => {
  aborted = true
  void kill()
}
ctx.abort.addEventListener("abort", abortHandler, { once: true })

// Cleanup in finally
ctx.abort.removeEventListener("abort", abortHandler)
```

## Built-in Tools

Complete list from registry:

| Tool | Purpose |
|------|---------|
| `invalid` | Error placeholder for malformed calls |
| `question` | Ask user questions (client-only) |
| `bash` | Execute shell commands |
| `read` | Read file contents |
| `glob` | Find files by pattern |
| `grep` | Search file contents |
| `edit` | Modify files |
| `write` | Create/overwrite files |
| `task` | Spawn subagents |
| `webfetch` | Fetch web content |
| `todowrite` | Manage task list |
| `todoread` | Read task list |
| `websearch` | Search the web (Exa) |
| `codesearch` | Semantic code search (Exa) |
| `skill` | Execute skill definitions |
| `lsp` | LSP operations (experimental) |
| `batch` | Parallel tool execution (experimental) |
| `plan_enter/plan_exit` | Plan mode (experimental) |

## Conditional Tool Availability

Tools filtered by feature flags and provider:

```typescript
return [
  // Always available
  BashTool, ReadTool, GlobTool, GrepTool, EditTool, WriteTool, TaskTool, ...

  // Client-specific
  ...(["app", "cli", "desktop"].includes(Flag.OPENCODE_CLIENT) ? [QuestionTool] : []),

  // Provider-specific (Exa requires opencode provider or flag)
  // See tools() function filter

  // Experimental
  ...(Flag.OPENCODE_EXPERIMENTAL_LSP_TOOL ? [LspTool] : []),
  ...(config.experimental?.batch_tool === true ? [BatchTool] : []),
  ...(Flag.OPENCODE_EXPERIMENTAL_PLAN_MODE && Flag.OPENCODE_CLIENT === "cli"
      ? [PlanExitTool, PlanEnterTool] : []),

  // Custom tools from plugins and config dirs
  ...custom,
]
```
