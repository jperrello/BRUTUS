# OpenCode Tool Registry Specification

## Overview

The ToolRegistry namespace manages tool discovery, registration, and resolution. It combines built-in tools with plugins and custom user-defined tools.

## Registry State

```typescript
export const state = Instance.state(async () => {
  const custom = [] as Tool.Info[]
  // ... discovery logic
  return { custom }
})
```

State is lazily initialized per project instance and contains dynamically discovered custom tools.

## Tool Discovery Sources

### 1. Built-in Tools (Hardcoded)

Core tools registered directly in `all()`:

```typescript
return [
  InvalidTool,
  BashTool,
  ReadTool,
  GlobTool,
  GrepTool,
  EditTool,
  WriteTool,
  TaskTool,
  WebFetchTool,
  TodoWriteTool,
  TodoReadTool,
  WebSearchTool,
  CodeSearchTool,
  SkillTool,
  // ... conditionals
]
```

### 2. Config Directory Tools

Scanned from user/project config directories:

```typescript
const glob = new Bun.Glob("{tool,tools}/*.{js,ts}")

for (const dir of await Config.directories()) {
  for await (const match of glob.scan({
    cwd: dir,
    absolute: true,
    followSymlinks: true,
    dot: true,
  })) {
    const namespace = path.basename(match, path.extname(match))
    const mod = await import(match)
    for (const [id, def] of Object.entries<ToolDefinition>(mod)) {
      custom.push(fromPlugin(id === "default" ? namespace : `${namespace}_${id}`, def))
    }
  }
}
```

Paths scanned:
- `~/.config/opencode/tool/*.ts`
- `~/.config/opencode/tools/*.ts`
- `.opencode/tool/*.ts`
- `.opencode/tools/*.ts`

Naming:
- Default export → file basename (e.g., `mytool.ts` → `mytool`)
- Named export → `{basename}_{export}` (e.g., `foo.ts` export `bar` → `foo_bar`)

### 3. Plugin Tools

From MCP and other plugins:

```typescript
const plugins = await Plugin.list()
for (const plugin of plugins) {
  for (const [id, def] of Object.entries(plugin.tool ?? {})) {
    custom.push(fromPlugin(id, def))
  }
}
```

## Plugin Tool Adapter

Converts external ToolDefinition format to internal Tool.Info:

```typescript
function fromPlugin(id: string, def: ToolDefinition): Tool.Info {
  return {
    id,
    init: async (initCtx) => ({
      parameters: z.object(def.args),
      description: def.description,
      execute: async (args, ctx) => {
        const result = await def.execute(args as any, ctx)
        const out = await Truncate.output(result, {}, initCtx?.agent)
        return {
          title: "",
          output: out.truncated ? out.content : result,
          metadata: {
            truncated: out.truncated,
            outputPath: out.truncated ? out.outputPath : undefined,
          },
        }
      },
    }),
  }
}
```

Notes:
- Plugin tools always get truncation applied
- Plugin tools have empty title by default
- Args schema must be `z.object()` compatible

## Tool Registration API

Dynamic registration after initialization:

```typescript
export async function register(tool: Tool.Info) {
  const { custom } = await state()
  const idx = custom.findIndex((t) => t.id === tool.id)
  if (idx >= 0) {
    custom.splice(idx, 1, tool)  // Replace existing
    return
  }
  custom.push(tool)  // Add new
}
```

Use case: MCP servers connecting mid-session.

## Tool Resolution

### List All Tool IDs

```typescript
export async function ids() {
  return all().then((x) => x.map((t) => t.id))
}
```

### Get Initialized Tools

```typescript
export async function tools(providerID: string, agent?: Agent.Info) {
  const tools = await all()
  const result = await Promise.all(
    tools
      .filter((t) => {
        // Provider-specific filtering
        if (t.id === "codesearch" || t.id === "websearch") {
          return providerID === "opencode" || Flag.OPENCODE_ENABLE_EXA
        }
        return true
      })
      .map(async (t) => {
        using _ = log.time(t.id)  // Performance tracking
        return {
          id: t.id,
          ...(await t.init({ agent })),
        }
      }),
  )
  return result
}
```

Returns fully initialized tools with:
- `id`
- `description`
- `parameters` (Zod schema)
- `execute` (function)
- `formatValidationError` (optional)

## Feature Flag Filters

### Client-Specific Tools

```typescript
...(["app", "cli", "desktop"].includes(Flag.OPENCODE_CLIENT) ? [QuestionTool] : [])
```

QuestionTool only available in interactive clients, not API/server.

### Provider-Specific Tools

```typescript
.filter((t) => {
  if (t.id === "codesearch" || t.id === "websearch") {
    return providerID === "opencode" || Flag.OPENCODE_ENABLE_EXA
  }
  return true
})
```

Exa-based tools require either:
- OpenCode provider (Zen users)
- `OPENCODE_ENABLE_EXA` flag set

### Experimental Tools

```typescript
// LSP Tool
...(Flag.OPENCODE_EXPERIMENTAL_LSP_TOOL ? [LspTool] : [])

// Batch Tool
...(config.experimental?.batch_tool === true ? [BatchTool] : [])

// Plan Mode
...(Flag.OPENCODE_EXPERIMENTAL_PLAN_MODE && Flag.OPENCODE_CLIENT === "cli"
    ? [PlanExitTool, PlanEnterTool] : [])
```

## Tool Ordering

Tools returned in this order:
1. `InvalidTool` (always first - handles malformed calls)
2. `QuestionTool` (if client supports)
3. Core tools (bash, read, glob, grep, edit, write, task, etc.)
4. Web tools (webfetch, websearch, codesearch)
5. Todo tools
6. Skill tool
7. Experimental tools (lsp, batch, plan)
8. Custom tools (plugins, config dirs)

Order matters for:
- Tool name collision resolution (earlier wins)
- Tool list presented to model

## Performance Considerations

Each tool initialization is timed:

```typescript
using _ = log.time(t.id)
return { id: t.id, ...(await t.init({ agent })) }
```

Logs help identify slow tool init (e.g., complex setup, network calls).

Tools are initialized in parallel via `Promise.all()`.

## Tool ID Restrictions

### Reserved/Disallowed

The batch tool blocks certain tools:

```typescript
const DISALLOWED = new Set(["batch"])  // Prevent recursion

const FILTERED_FROM_SUGGESTIONS = new Set([
  "invalid",  // Internal error handler
  "patch",    // Deprecated/internal
  ...DISALLOWED
])
```

### MCP Tool Blocking

MCP/external tools cannot be batched:

```typescript
if (!toolMap.get(call.tool)) {
  throw new Error(
    `Tool '${call.tool}' not in registry. External tools (MCP, environment) cannot be batched - call them directly.`
  )
}
```
