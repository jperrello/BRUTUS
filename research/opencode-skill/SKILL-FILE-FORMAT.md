# SKILL.md File Format Specification

## Overview

Skills are markdown files with YAML frontmatter. The filename must be exactly `SKILL.md` (case-sensitive).

## File Structure

```markdown
---
name: skill-identifier
description: Brief description shown in tool listing
---

# Skill Title

Instructions for the LLM...
```

## Required Frontmatter Fields

### name

**Type:** `string`

**Purpose:** Unique identifier used to invoke the skill

**Constraints:**
- Must be unique across all discovered skills
- Used in tool invocation: `skill({ name: "value" })`
- Can include path-like segments for organization: `"code-review"`, `"deploy/staging"`

**Example:**
```yaml
name: code-review
```

### description

**Type:** `string`

**Purpose:** Human-readable description shown to LLM in tool listing

**Best Practices:**
- Keep concise (1-2 sentences)
- Describe when to use this skill
- Include key capabilities

**Example:**
```yaml
description: Review code changes for bugs, style issues, and improvements
```

## Markdown Content

Everything after the frontmatter closing `---` is the skill body.

### File References

Use `@filepath` syntax to reference files:

```markdown
Review the configuration in @config/settings.yaml and ensure it follows @docs/config-guide.md
```

**Regex Pattern:**
```regex
/(?<![\w`])@(\.?[^\s`,.]*(?:\.[^\s`,.]+)*)/g
```

**Matching Examples:**
- `@package.json` - root file
- `@src/main.ts` - nested file
- `@./relative/path` - explicit relative
- `@~/global/file` - home directory (expanded later)

**Non-matching (escaped by preceding characters):**
- `email@example.com` - preceded by word char
- `` `@code` `` - inside backticks

### Shell Commands

Use `!`backticks`` syntax for executable shell commands:

```markdown
First, check the current branch with !`git branch --show-current`
```

**Regex Pattern:**
```regex
/!`([^`]+)`/g
```

## YAML Preprocessing

OpenCode preprocesses YAML frontmatter to handle edge cases.

### Colon Handling

Values containing colons are converted to block scalar format:

**Input:**
```yaml
name: my-skill:v2
description: Deploy to https://example.com endpoint
```

**Preprocessed:**
```yaml
name: |
  my-skill:v2
description: |
  Deploy to https://example.com endpoint
```

### Preserved Patterns

These patterns are NOT converted:
- Already quoted: `name: "value:with:colons"`
- Already block scalar: `name: |` or `name: >`
- Empty values: `name:`
- Comments: `# comment`
- Indented continuations

## Directory Structure Examples

### Project-Level Skills

```
project/
├── .claude/
│   └── skills/
│       ├── code-review/
│       │   └── SKILL.md
│       └── deploy/
│           ├── staging/
│           │   └── SKILL.md
│           └── production/
│           │   └── SKILL.md
├── .opencode/
│   └── skill/
│       └── custom-tool/
│           └── SKILL.md
└── src/
```

### Global Skills

```
~/.claude/
└── skills/
    ├── git-workflow/
    │   └── SKILL.md
    └── general-review/
        └── SKILL.md
```

## Complete Example

### .claude/skills/code-review/SKILL.md

```markdown
---
name: code-review
description: Comprehensive code review following team standards
---

# Code Review Skill

You are performing a code review. Follow these steps:

## 1. Understand Context

Read the changed files and understand the purpose of the changes.
Reference the style guide at @docs/STYLE_GUIDE.md

## 2. Check for Issues

Look for:
- Logic errors
- Security vulnerabilities
- Performance issues
- Missing error handling

## 3. Verify Tests

Run the test suite with !`npm test`

Check test coverage with !`npm run coverage`

## 4. Provide Feedback

Format your review as:
- Summary of changes
- Issues found (with severity)
- Suggestions for improvement
- Approval status
```

## Validation

Skills are validated using Zod schema during discovery:

```typescript
const Info = z.object({
  name: z.string(),
  description: z.string(),
})

const parsed = Info.pick({ name: true, description: true }).safeParse(md.data)
if (!parsed.success) return  // Skill is skipped
```

## Output Format

When a skill is loaded, it's returned in this format:

```markdown
## Skill: code-review

**Base directory**: /path/to/.claude/skills/code-review

[Content of SKILL.md body]
```

The base directory allows skills to reference relative files.

## Naming Conventions

| Pattern | Example | Use Case |
|---------|---------|----------|
| `verb-noun` | `code-review` | Action-oriented skills |
| `noun/verb` | `deploy/staging` | Grouped/hierarchical skills |
| `tool-name` | `docker-compose` | Tool-specific skills |
| `category/name` | `review/security` | Organized collections |

## Anti-Patterns

### Don't Include Computed Values

```yaml
# BAD - frontmatter should be static
name: skill-${version}
```

### Don't Duplicate Tool Logic

```markdown
# BAD - this should be a tool, not a skill
Run `bash` with the following commands...
[massive shell script]
```

### Don't Include Secrets

```markdown
# BAD - secrets should come from environment
Set API_KEY=sk-xxxxx
```
