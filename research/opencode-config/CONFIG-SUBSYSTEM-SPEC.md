# OpenCode Configuration Subsystem Specification

## Overview

OpenCode implements a sophisticated configuration system that loads, merges, and validates configuration from multiple sources with a strict precedence hierarchy. The system supports JSONC (JSON with comments), variable interpolation, and markdown-based extension definitions.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Config.state()                              │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                  Loading Pipeline                         │   │
│  │  ┌────────┐  ┌────────┐  ┌────────┐  ┌────────┐         │   │
│  │  │ Remote │→ │ Global │→ │ Custom │→ │Project │         │   │
│  │  │wellknow│  │ config │  │ path   │  │ config │         │   │
│  │  └────────┘  └────────┘  └────────┘  └────────┘         │   │
│  │       ↓                                    ↓             │   │
│  │  ┌─────────────────────────────────────────────────────┐│   │
│  │  │              mergeConfigConcatArrays()              ││   │
│  │  └─────────────────────────────────────────────────────┘│   │
│  └──────────────────────────────────────────────────────────┘   │
│                              ↓                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │               Extension Loading                           │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────┐  │   │
│  │  │ Commands │  │  Agents  │  │  Modes   │  │ Plugins │  │   │
│  │  │   .md    │  │   .md    │  │   .md    │  │ .ts/.js │  │   │
│  │  └──────────┘  └──────────┘  └──────────┘  └─────────┘  │   │
│  └──────────────────────────────────────────────────────────┘   │
│                              ↓                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                  Zod Schema Validation                    │   │
│  │               Info.safeParse(merged)                      │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## Configuration Precedence (Lowest to Highest)

1. **Remote Well-Known** - `{auth_url}/.well-known/opencode`
2. **Global User Config** - `~/.config/opencode/opencode.json[c]`
3. **Custom Config Path** - `OPENCODE_CONFIG` environment variable
4. **Project Config** - `opencode.json[c]` found via `findUp()`
5. **Inline Content** - `OPENCODE_CONFIG_CONTENT` environment variable
6. **Flag Overrides** - Individual `OPENCODE_*` environment variables

## File Locations

### XDG Base Directories
```typescript
const app = "opencode"
Global.Path = {
  home:   os.homedir(),
  data:   ~/.local/share/opencode/
  cache:  ~/.cache/opencode/
  config: ~/.config/opencode/
  state:  ~/.local/state/opencode/
  bin:    ~/.local/share/opencode/bin/
  log:    ~/.local/share/opencode/log/
}
```

### Search Paths for Project Config
```typescript
// Searched bottom-up from Instance.directory to Instance.worktree
["opencode.jsonc", "opencode.json"]

// Additional directories scanned:
- Global.Path.config
- .opencode/ directories (via Filesystem.up())
- OPENCODE_CONFIG_DIR (if set)
```

## Core Types

### Config.Info (Main Schema)
```typescript
interface Info {
  $schema?: string                           // JSON schema URL
  theme?: string                             // UI theme name
  keybinds?: Keybinds                        // Key binding overrides
  logLevel?: LogLevel                        // Logging verbosity
  tui?: TUI                                  // Terminal UI settings
  server?: Server                            // HTTP server config
  command?: Record<string, Command>          // Slash commands
  watcher?: { ignore?: string[] }            // File watcher config
  plugin?: string[]                          // Plugin specifiers
  snapshot?: boolean                         // Enable snapshots
  share?: "manual" | "auto" | "disabled"     // Sharing behavior
  autoupdate?: boolean | "notify"            // Update behavior
  disabled_providers?: string[]              // Blocked providers
  enabled_providers?: string[]               // Whitelist (exclusive)
  model?: string                             // Default model (provider/model)
  small_model?: string                       // Model for lightweight tasks
  default_agent?: string                     // Default primary agent
  username?: string                          // Display name override
  agent?: Record<string, Agent>              // Agent definitions
  provider?: Record<string, Provider>        // Provider configs
  mcp?: Record<string, Mcp>                  // MCP server configs
  formatter?: Record<string, Formatter>      // Code formatters
  lsp?: Record<string, LSPConfig>            // LSP servers
  instructions?: string[]                    // Additional prompt files
  permission?: Permission                    // Tool permissions
  compaction?: CompactionConfig              // Compaction settings
  experimental?: ExperimentalConfig          // Feature flags
}
```

### Permission System
```typescript
type PermissionAction = "ask" | "allow" | "deny"
type PermissionObject = Record<string, PermissionAction>
type PermissionRule = PermissionAction | PermissionObject

interface Permission {
  read?: PermissionRule
  edit?: PermissionRule
  glob?: PermissionRule
  grep?: PermissionRule
  list?: PermissionRule
  bash?: PermissionRule
  task?: PermissionRule
  external_directory?: PermissionRule
  todowrite?: PermissionAction
  todoread?: PermissionAction
  question?: PermissionAction
  webfetch?: PermissionAction
  websearch?: PermissionAction
  codesearch?: PermissionAction
  lsp?: PermissionRule
  doom_loop?: PermissionAction
  [tool: string]: PermissionRule  // Extensible
}
```

### Agent Configuration
```typescript
interface Agent {
  model?: string                    // Override model
  temperature?: number              // Sampling temperature
  top_p?: number                    // Nucleus sampling
  prompt?: string                   // System prompt
  disable?: boolean                 // Disable agent
  description?: string              // Agent description
  mode?: "subagent" | "primary" | "all"
  hidden?: boolean                  // Hide from autocomplete
  options?: Record<string, any>     // Custom options
  color?: string                    // Hex color (#RRGGBB)
  steps?: number                    // Max agentic iterations
  permission?: Permission           // Agent-specific permissions
}
```

### MCP Configuration
```typescript
// Local MCP Server
interface McpLocal {
  type: "local"
  command: string[]                         // Command + args
  environment?: Record<string, string>      // Env vars
  enabled?: boolean                         // Auto-start
  timeout?: number                          // Request timeout (ms)
}

// Remote MCP Server
interface McpRemote {
  type: "remote"
  url: string                               // Server URL
  enabled?: boolean
  headers?: Record<string, string>          // HTTP headers
  oauth?: McpOAuth | false                  // OAuth config
  timeout?: number
}

interface McpOAuth {
  clientId?: string                         // OAuth client ID
  clientSecret?: string                     // Client secret
  scope?: string                            // Requested scopes
}
```

## Variable Interpolation

### Environment Variables
```jsonc
{
  "provider": {
    "openai": {
      "options": {
        "apiKey": "{env:OPENAI_API_KEY}"
      }
    }
  }
}
```

Replacement: `text.replace(/\{env:([^}]+)\}/g, (_, varName) => process.env[varName] || "")`

### File Inclusion
```jsonc
{
  "agent": {
    "custom": {
      "prompt": "{file:~/.opencode/prompts/custom.md}"
    }
  }
}
```

Resolution:
1. `~/` expands to `os.homedir()`
2. Relative paths resolve from config file's directory
3. File content is JSON-escaped (newlines/quotes)
4. Commented lines (`//`) skip file expansion

## Merge Behavior

### Default Merge (mergeDeep)
Deep recursive merge where later values override earlier.

### Array Concatenation (mergeConfigConcatArrays)
Special handling for:
- `plugin[]` - Concatenated, then deduplicated by name
- `instructions[]` - Concatenated, deduplicated

```typescript
function mergeConfigConcatArrays(target: Info, source: Info): Info {
  const merged = mergeDeep(target, source)
  if (target.plugin && source.plugin) {
    merged.plugin = Array.from(new Set([...target.plugin, ...source.plugin]))
  }
  if (target.instructions && source.instructions) {
    merged.instructions = Array.from(new Set([...target.instructions, ...source.instructions]))
  }
  return merged
}
```

## Extension Loading

### Commands (Markdown)
```
Location: {command,commands}/**/*.md
Pattern: COMMAND_GLOB = "{command,commands}/**/*.md"

File Format:
---
description: Short description
agent: optional-agent-name
model: optional/model
subtask: true/false
---
Template content with @file references and !`shell` commands
```

### Agents (Markdown)
```
Location: {agent,agents}/**/*.md
Pattern: AGENT_GLOB = "{agent,agents}/**/*.md"

File Format:
---
model: provider/model
temperature: 0.7
description: When to use this agent
mode: subagent | primary | all
color: "#FF5733"
steps: 10
---
System prompt content...
```

### Modes (Markdown - Deprecated)
```
Location: {mode,modes}/*.md
Pattern: MODE_GLOB = "{mode,modes}/*.md"
Note: Migrated to agents with mode: "primary"
```

### Plugins (TypeScript/JavaScript)
```
Location: {plugin,plugins}/*.{ts,js}
Pattern: PLUGIN_GLOB = "{plugin,plugins}/*.{ts,js}"
Resolution: Converted to file:// URL
```

## Plugin Deduplication

```typescript
function getPluginName(plugin: string): string {
  // file:///path/to/plugin/foo.js → "foo"
  if (plugin.startsWith("file://")) {
    return path.parse(new URL(plugin).pathname).name
  }
  // oh-my-opencode@2.4.3 → "oh-my-opencode"
  // @scope/pkg@1.0.0 → "@scope/pkg"
  const lastAt = plugin.lastIndexOf("@")
  if (lastAt > 0) {
    return plugin.substring(0, lastAt)
  }
  return plugin
}

function deduplicatePlugins(plugins: string[]): string[] {
  // Later entries (higher priority) win
  // Reverse → dedupe → reverse back
  const seenNames = new Set<string>()
  const uniqueSpecifiers: string[] = []

  for (const specifier of plugins.toReversed()) {
    const name = getPluginName(specifier)
    if (!seenNames.has(name)) {
      seenNames.add(name)
      uniqueSpecifiers.push(specifier)
    }
  }

  return uniqueSpecifiers.toReversed()
}
```

## Dependency Installation

Each config directory gets automatic dependency management:

```typescript
async function installDependencies(dir: string) {
  // Create package.json if missing
  const pkg = path.join(dir, "package.json")
  if (!(await Bun.file(pkg).exists())) {
    await Bun.write(pkg, "{}")
  }

  // Create .gitignore
  const gitignore = path.join(dir, ".gitignore")
  if (!hasGitIgnore) {
    await Bun.write(gitignore, [
      "node_modules",
      "package.json",
      "bun.lock",
      ".gitignore"
    ].join("\n"))
  }

  // Install OpenCode plugin SDK
  await BunProc.run([
    "add",
    "@opencode-ai/plugin@" + Installation.VERSION,
    "--exact"
  ], { cwd: dir })

  // Install other dependencies
  await BunProc.run(["install"], { cwd: dir })
}
```

## Backwards Compatibility

### Legacy `tools` Field
```typescript
// Old format
{ "tools": { "write": true, "bash": false } }

// Migrated to
{ "permission": { "edit": "allow", "bash": "deny" } }

// Migration logic:
if (result.tools) {
  const perms: Record<string, PermissionAction> = {}
  for (const [tool, enabled] of Object.entries(result.tools)) {
    const action = enabled ? "allow" : "deny"
    if (["write", "edit", "patch", "multiedit"].includes(tool)) {
      perms.edit = action
    } else {
      perms[tool] = action
    }
  }
  result.permission = mergeDeep(perms, result.permission ?? {})
}
```

### Legacy `autoshare` Field
```typescript
// Old: autoshare: true
// New: share: "auto"
if (result.autoshare === true && !result.share) {
  result.share = "auto"
}
```

### Legacy `mode` Field
```typescript
// Old: mode.build, mode.plan
// New: agent.build, agent.plan with mode: "primary"
for (const [name, mode] of Object.entries(result.mode)) {
  result.agent[name] = { ...mode, mode: "primary" }
}
```

### Legacy `maxSteps` Field
```typescript
// Old: maxSteps: 10
// New: steps: 10
const steps = agent.steps ?? agent.maxSteps
```

## Environment Flags

```typescript
// Config sources
OPENCODE_CONFIG        // Path to custom config file
OPENCODE_CONFIG_DIR    // Additional config directory
OPENCODE_CONFIG_CONTENT // Inline JSON config

// Feature toggles
OPENCODE_DISABLE_AUTOCOMPACT  // Disable auto-compaction
OPENCODE_DISABLE_PRUNE        // Disable output pruning
OPENCODE_PERMISSION           // JSON permission overrides

// Experimental
OPENCODE_EXPERIMENTAL         // Enable all experimental features
```

## Error Types

```typescript
// JSON parsing errors
ConfigJsonError = NamedError.create("ConfigJsonError", {
  path: string,
  message?: string
})

// Validation errors
ConfigInvalidError = NamedError.create("ConfigInvalidError", {
  path: string,
  issues?: ZodIssue[],
  message?: string
})

// Frontmatter parsing errors
ConfigFrontmatterError = NamedError.create("ConfigFrontmatterError", {
  path: string,
  message: string
})
```

## JSONC Parsing

```typescript
import { parse as parseJsonc, printParseErrorCode } from "jsonc-parser"

const errors: JsoncParseError[] = []
const data = parseJsonc(text, errors, { allowTrailingComma: true })

if (errors.length) {
  // Format detailed error messages with line/column numbers
  const errorDetails = errors.map(e => {
    const beforeOffset = text.substring(0, e.offset).split("\n")
    const line = beforeOffset.length
    const column = beforeOffset[beforeOffset.length - 1].length + 1
    return `${printParseErrorCode(e.error)} at line ${line}, column ${column}`
  }).join("\n")

  throw new JsonError({ path: filepath, message: errorDetails })
}
```

## State Management

```typescript
// Instance-scoped lazy state
export const state = Instance.state(async () => {
  // ... loading logic ...
  return {
    config: result,
    directories: directories
  }
})

// Accessors
export async function get() {
  return state().then(x => x.config)
}

export async function directories() {
  return state().then(x => x.directories)
}

// Mutation
export async function update(config: Info) {
  const filepath = path.join(Instance.directory, "config.json")
  const existing = await loadFile(filepath)
  await Bun.write(filepath, JSON.stringify(mergeDeep(existing, config), null, 2))
  await Instance.dispose()  // Invalidate cache
}
```
