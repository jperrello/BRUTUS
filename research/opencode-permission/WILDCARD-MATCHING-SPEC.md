# OpenCode Wildcard Pattern Matching Specification

## Overview

OpenCode uses a custom wildcard matching system for permission patterns. This is NOT glob matching - it's a simpler pattern language optimized for command and path matching.

## Pattern Syntax

| Syntax | Meaning | Regex Equivalent |
|--------|---------|------------------|
| `*` | Match any characters (zero or more) | `.*` |
| `?` | Match any single character | `.` |
| Other | Literal character (regex-escaped) | Escaped |

## Core Algorithm

```typescript
function match(str: string, pattern: string): boolean {
  let escaped = pattern
    .replace(/[.+^${}()|[\]\\]/g, "\\$&")  // Escape regex special chars
    .replace(/\*/g, ".*")                    // * → .*
    .replace(/\?/g, ".")                     // ? → .

  // Special case: trailing " *" becomes optional
  // "ls *" matches both "ls" and "ls -la"
  if (escaped.endsWith(" .*")) {
    escaped = escaped.slice(0, -3) + "( .*)?"
  }

  return new RegExp("^" + escaped + "$", "s").test(str)
}
```

## Pattern Examples

### Basic Wildcards

| Pattern | Input | Match |
|---------|-------|-------|
| `git *` | `git status` | ✓ |
| `git *` | `git` | ✓ (trailing space-star is optional) |
| `git *` | `gitfoo` | ✗ (space required before *) |
| `*.ts` | `main.ts` | ✓ |
| `*.ts` | `.ts` | ✓ |
| `src/*` | `src/main.go` | ✓ |
| `src/*` | `src/sub/file.go` | ✓ |

### Single Character Wildcard

| Pattern | Input | Match |
|---------|-------|-------|
| `file?.ts` | `file1.ts` | ✓ |
| `file?.ts` | `file12.ts` | ✗ |
| `?.md` | `a.md` | ✓ |

### Trailing Space-Star Special Case

This is a key optimization for command matching:

| Pattern | Input | Match | Reason |
|---------|-------|-------|--------|
| `git checkout *` | `git checkout` | ✓ | Trailing " *" optional |
| `git checkout *` | `git checkout main` | ✓ | With argument |
| `npm install *` | `npm install` | ✓ | No packages = OK |
| `npm install *` | `npm install react` | ✓ | With package |

## Rule Evaluation Order

When multiple rules might match, OpenCode uses "last match wins" with length-based sorting:

```typescript
function all(input: string, patterns: Record<string, any>) {
  const sorted = pipe(patterns, Object.entries,
    sortBy(
      [([key]) => key.length, "asc"],  // Shorter patterns first
      [([key]) => key, "asc"]           // Alphabetical tiebreaker
    )
  )

  let result = undefined
  for (const [pattern, value] of sorted) {
    if (match(input, pattern)) {
      result = value  // Last match wins
    }
  }
  return result
}
```

### Evaluation Example

Given rules:
```json
{
  "*": "deny",
  "git *": "allow",
  "git checkout *": "ask"
}
```

Sorted order: `*`, `git *`, `git checkout *`

| Input | Matches | Result |
|-------|---------|--------|
| `ls` | `*` | deny |
| `git status` | `*`, `git *` | allow |
| `git checkout main` | `*`, `git *`, `git checkout *` | ask |

## Structured Pattern Matching

For bash commands with complex argument structures:

```typescript
function allStructured(
  input: { head: string; tail: string[] },
  patterns: Record<string, any>
)
```

This splits the pattern and input:
- Pattern: `"git remote add *"` → head: `"git"`, tail: `["remote", "add", "*"]`
- Input: `{ head: "git", tail: ["remote", "add", "origin", "..."] }`

### Sequence Matching

The tail matching is position-independent for wildcards:

```typescript
function matchSequence(items: string[], patterns: string[]): boolean {
  if (patterns.length === 0) return true
  const [pattern, ...rest] = patterns

  // Wildcard consumes nothing, try rest
  if (pattern === "*") return matchSequence(items, rest)

  // Try matching pattern at each position
  for (let i = 0; i < items.length; i++) {
    if (match(items[i], pattern) && matchSequence(items.slice(i + 1), rest)) {
      return true
    }
  }
  return false
}
```

## Differences from Glob

| Feature | OpenCode Wildcard | Standard Glob |
|---------|-------------------|---------------|
| `**` recursive | No | Yes |
| `{a,b}` alternation | No | Yes |
| `[abc]` character class | No | Yes |
| Trailing ` *` optional | Yes | No |
| Anchored by default | Yes (^ and $) | No |

## Implementation Notes

### Regex Safety

All special regex characters are escaped before conversion:
```typescript
.replace(/[.+^${}()|[\]\\]/g, "\\$&")
```

This ensures patterns like `file.ts` match literally (not "file" + any char + "ts").

### DOTALL Mode

The regex uses `s` flag (DOTALL), meaning `.` matches newlines. This is relevant for multi-line inputs.

### Case Sensitivity

Matching is case-sensitive. No `i` flag is used.

## Permission Pattern Best Practices

### Bash Commands

```json
{
  "bash": {
    "git *": "allow",           // All git commands
    "git push *": "ask",        // But ask for push
    "npm *": "allow",
    "npm publish *": "deny",    // Never auto-publish
    "*": "ask"                  // Default: prompt
  }
}
```

### File Paths

```json
{
  "edit": {
    "*.md": "allow",
    "*.lock": "deny",
    "node_modules/*": "deny",
    ".env*": "deny",
    "*": "ask"
  }
}
```

### External Directories

```json
{
  "external_directory": {
    "/tmp/*": "allow",
    "~/.config/*": "ask",
    "*": "deny"
  }
}
```
