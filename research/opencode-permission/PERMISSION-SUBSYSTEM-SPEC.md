# OpenCode Permission Subsystem Specification

## Overview

The permission subsystem is a security layer that controls tool execution through declarative rules. It intercepts tool calls, evaluates them against permission rules, and either allows, denies, or prompts the user for approval.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Tool Execution Request                       │
└────────────────────────────────────┬────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         Permission.ask()                             │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │  1. Build patterns from tool input                            │   │
│  │  2. Check if patterns covered by approved set                 │   │
│  │  3. Evaluate against ruleset (config + stored)                │   │
│  │  4. Return action: allow | deny | ask                         │   │
│  └──────────────────────────────────────────────────────────────┘   │
└────────────────────────────────────┬────────────────────────────────┘
                                     │
              ┌──────────────────────┼──────────────────────┐
              ▼                      ▼                      ▼
        ┌─────────┐           ┌─────────┐           ┌─────────┐
        │  ALLOW  │           │   ASK   │           │  DENY   │
        └────┬────┘           └────┬────┘           └────┬────┘
             │                     │                     │
             │                     ▼                     │
             │              ┌───────────┐                │
             │              │ User Prompt│                │
             │              │  Response  │                │
             │              └─────┬─────┘                │
             │                    │                      │
             │     ┌──────────────┼──────────────┐       │
             │     ▼              ▼              ▼       │
             │  ┌──────┐     ┌────────┐     ┌────────┐   │
             │  │ once │     │ always │     │ reject │   │
             │  └──┬───┘     └───┬────┘     └───┬────┘   │
             │     │             │              │        │
             ▼     ▼             ▼              ▼        ▼
        ┌─────────────┐   ┌──────────────┐   ┌──────────────┐
        │   Execute   │   │   Execute +  │   │    Abort     │
        │    Tool     │   │ Store Pattern│   │   w/Error    │
        └─────────────┘   └──────────────┘   └──────────────┘
```

## Core Data Structures

### Permission Request

```typescript
interface Request {
  id: string                     // "permission_XXXX" ascending ID
  sessionID: string              // Session this request belongs to
  permission: string             // Permission type (e.g., "bash", "edit", "read")
  patterns: string[]             // Specific patterns to check (e.g., ["git status", "npm install"])
  always: string[]               // Patterns to store if "always" is selected
  metadata: Record<string, any>  // Tool-specific context
  tool?: {
    messageID: string
    callID: string
  }
}
```

### Permission Rule

```typescript
interface Rule {
  permission: string    // Permission name (supports wildcards)
  pattern: string       // Pattern to match (supports wildcards)
  action: Action        // "allow" | "deny" | "ask"
}

type Ruleset = Rule[]
```

### Reply Types

```typescript
type Reply = "once" | "always" | "reject"
```

| Reply | Behavior |
|-------|----------|
| `once` | Allow this specific execution, don't store |
| `always` | Allow and store patterns for future auto-approval |
| `reject` | Abort execution, throw error to agent |

## Permission Types (Built-in)

From config schema:

| Permission | Description | Pattern Examples |
|------------|-------------|------------------|
| `read` | File reading | `*.ts`, `src/**/*` |
| `edit` | File modification (write, edit, patch, multiedit) | `*.md`, `!*.lock` |
| `glob` | File glob operations | `**/*.js` |
| `grep` | Content search | - |
| `list` | Directory listing | - |
| `bash` | Shell command execution | `git *`, `npm install` |
| `task` | Subagent spawning | - |
| `external_directory` | Access outside project | `/etc/*`, `~/.ssh/*` |
| `todowrite` | TODO list writing | - |
| `todoread` | TODO list reading | - |
| `question` | User prompting | - |
| `webfetch` | HTTP fetching | - |
| `websearch` | Web searching | - |
| `codesearch` | Code search | - |
| `lsp` | Language server operations | - |
| `doom_loop` | Repetitive action detection | - |

## Configuration Schema

### Simple Permission (All Patterns)

```json
{
  "permission": {
    "bash": "allow",
    "edit": "ask",
    "webfetch": "deny"
  }
}
```

### Pattern-Specific Permission

```json
{
  "permission": {
    "bash": {
      "git *": "allow",
      "npm install": "allow",
      "rm *": "ask",
      "*": "deny"
    },
    "edit": {
      "*.md": "allow",
      "*.lock": "deny",
      "*": "ask"
    }
  }
}
```

### Agent-Specific Permissions

```json
{
  "agent": {
    "plan": {
      "permission": {
        "edit": "deny",
        "bash": "deny"
      }
    },
    "build": {
      "permission": {
        "edit": "allow",
        "bash": "ask"
      }
    }
  }
}
```

## Rule Evaluation Algorithm

```
function evaluate(permission, pattern, ...rulesets):
  merged = flatten(rulesets)

  # Rules are evaluated in order, last match wins
  # This allows more specific rules to override general ones
  match = merged.findLast(rule =>
    Wildcard.match(permission, rule.permission) AND
    Wildcard.match(pattern, rule.pattern)
  )

  return match ?? { action: "ask", permission, pattern: "*" }
```

Key behaviors:
1. **Last match wins** - Rules are sorted by key length (ascending), then applied in order
2. **Default is ask** - If no rule matches, user is prompted
3. **Wildcard support** - Both permission names and patterns support `*` and `?`

## Bash Tool Permission Flow (Example)

The bash tool demonstrates the complete permission integration:

```
1. Parse command with tree-sitter-bash
   "git checkout main && npm install"

2. Extract command tokens
   ["git", "checkout", "main"]
   ["npm", "install"]

3. Check external directories
   For cd, rm, cp, mv, mkdir, touch, chmod, chown:
   - Resolve paths via realpath
   - If outside Instance.directory → ask "external_directory" permission

4. Build permission patterns
   patterns: ["git checkout main", "npm install"]
   always:   ["git checkout*", "npm install*"]  # Using arity rules

5. Call ctx.ask({ permission: "bash", patterns, always })

6. If approved with "always":
   Store "git checkout*" and "npm install*" for future auto-approval
```

## Command Arity System

The `BashArity` module defines how many tokens constitute a "command" for permission purposes:

```typescript
const ARITY: Record<string, number> = {
  // Simple commands (arity 1)
  cat: 1,     // cat file.txt → "cat"
  ls: 1,      // ls -la → "ls"
  rm: 1,      // rm file.txt → "rm"

  // Two-token commands (arity 2)
  git: 2,     // git checkout main → "git checkout"
  npm: 2,     // npm install → "npm install"
  docker: 2,  // docker run nginx → "docker run"

  // Three-token commands (arity 3)
  "npm run": 3,     // npm run dev → "npm run dev"
  "docker compose": 3,  // docker compose up → "docker compose up"
  "git config": 3,  // git config user.name → "git config user.name"
}
```

The prefix function finds the command portion:
```
prefix(["git", "checkout", "main"]) → ["git", "checkout"]
prefix(["npm", "run", "dev"]) → ["npm", "run", "dev"]
prefix(["ls", "-la", "src"]) → ["ls"]
```

## State Management

### Per-Session State

```typescript
const state = {
  // Pending permission requests awaiting user response
  pending: {
    [sessionID: string]: {
      [permissionID: string]: {
        info: Request
        resolve: () => void
        reject: (e: Error) => void
      }
    }
  },

  // Approved patterns from "always" responses
  approved: Ruleset  // Loaded from Storage.read(["permission", projectID])
}
```

### Event Bus Integration

```typescript
const Event = {
  // Fired when permission request created
  Asked: BusEvent.define("permission.asked", Request),

  // Fired when user responds
  Replied: BusEvent.define("permission.replied", {
    sessionID: string,
    requestID: string,
    reply: Reply
  })
}
```

## Error Types

### RejectedError

Thrown when user explicitly rejects without message:

```typescript
class RejectedError extends Error {
  message = "The user rejected permission to use this specific tool call."
}
```

### CorrectedError

Thrown when user rejects with feedback message:

```typescript
class CorrectedError extends Error {
  message = "The user rejected permission with feedback: ${message}"
}
```

This allows the agent to adjust and retry based on user guidance.

### DeniedError

Thrown when config rule auto-denies:

```typescript
class DeniedError extends Error {
  ruleset: Ruleset  // Rules that caused denial
  message = "Rule prevents this tool call: ${JSON.stringify(ruleset)}"
}
```

## Plugin Integration

Plugins can intercept permission requests:

```typescript
await Plugin.trigger("permission.ask", info, { status: "ask" })
  .then(x => x.status)  // Returns "deny", "allow", or "ask"
```

This allows plugins to:
- Auto-approve certain patterns
- Auto-deny dangerous operations
- Log permission requests
- Implement custom approval flows

## Disabled Tools Detection

```typescript
function disabled(tools: string[], ruleset: Ruleset): Set<string> {
  const result = new Set<string>()
  for (const tool of tools) {
    const permission = EDIT_TOOLS.includes(tool) ? "edit" : tool
    const rule = ruleset.findLast(r =>
      Wildcard.match(permission, r.permission)
    )
    if (rule?.pattern === "*" && rule.action === "deny") {
      result.add(tool)
    }
  }
  return result
}
```

This is used to inform the LLM which tools are completely disabled.

## Tool Context API

Tools receive permission capability via context:

```typescript
interface Context {
  ask(input: {
    permission: string,
    patterns: string[],
    always: string[],
    metadata: Record<string, any>
  }): Promise<void>
}
```

The tool calls `ctx.ask()` which:
1. Returns immediately if patterns already approved
2. Blocks until user responds if prompting needed
3. Throws if denied/rejected
