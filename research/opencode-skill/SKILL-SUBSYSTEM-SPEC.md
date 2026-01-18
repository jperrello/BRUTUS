# OpenCode Skill Subsystem Specification

## Overview

The Skill subsystem enables user-defined, markdown-based extensions that provide specialized instructions to the LLM agent on-demand. Unlike system prompts that consume context at all times, skills are loaded only when explicitly invoked via the `skill` tool.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Startup                                 │
├─────────────────────────────────────────────────────────────────┤
│  1. Skill.state() initializes                                   │
│  2. Scan directories for SKILL.md files                         │
│  3. Parse frontmatter (name, description)                       │
│  4. Build skills registry: Record<name, Info>                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Tool Registration                            │
├─────────────────────────────────────────────────────────────────┤
│  SkillTool.define() creates tool with:                          │
│  - Dynamic description listing available skills                 │
│  - Filtered by agent permissions                                │
│  - Schema: { name: string }                                     │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Runtime Execution                            │
├─────────────────────────────────────────────────────────────────┤
│  LLM calls skill tool with { name: "skill-name" }               │
│  1. Lookup skill by name                                        │
│  2. Permission check via ctx.ask()                              │
│  3. Parse full SKILL.md with ConfigMarkdown.parse()             │
│  4. Return content with base directory context                  │
└─────────────────────────────────────────────────────────────────┘
```

## Data Structures

### Skill.Info

```typescript
const Info = z.object({
  name: z.string(),         // Unique identifier
  description: z.string(),  // Human-readable purpose
  location: z.string(),     // Absolute path to SKILL.md
})
```

### Discovery Globs

```typescript
const OPENCODE_SKILL_GLOB = new Bun.Glob("{skill,skills}/**/SKILL.md")
const CLAUDE_SKILL_GLOB = new Bun.Glob("skills/**/SKILL.md")
```

## Discovery Algorithm

### Directory Search Order

1. **Project-level `.claude/skills/`** - Walk up from `Instance.directory` to `Instance.worktree`
2. **Global `~/.claude/skills/`** - User's home directory
3. **OpenCode config directories** - Via `Config.directories()` (`.opencode/skill/` or `.opencode/skills/`)

### Deduplication

Skills are keyed by `name` from frontmatter. If duplicate names exist:
- Warning is logged
- Later-discovered skill overwrites earlier

### Directory Scan Implementation

```typescript
// Claude Code compatible locations
for (const dir of claudeDirs) {
  for (const match of CLAUDE_SKILL_GLOB.scan({
    cwd: dir,
    absolute: true,
    onlyFiles: true,
    followSymlinks: true,
    dot: true,
  })) {
    await addSkill(match)
  }
}

// OpenCode native locations
for (const dir of await Config.directories()) {
  for await (const match of OPENCODE_SKILL_GLOB.scan({
    cwd: dir,
    absolute: true,
    onlyFiles: true,
    followSymlinks: true,
  })) {
    await addSkill(match)
  }
}
```

## Tool Definition

### Dynamic Description Generation

The skill tool's description is regenerated based on available skills:

```typescript
const description = accessibleSkills.length === 0
  ? "Load a skill to get detailed instructions for a specific task. No skills are currently available."
  : [
      "Load a skill to get detailed instructions for a specific task.",
      "Skills provide specialized knowledge and step-by-step guidance.",
      "Use this when a task matches an available skill's description.",
      "<available_skills>",
      ...accessibleSkills.flatMap((skill) => [
        `  <skill>`,
        `    <name>${skill.name}</name>`,
        `    <description>${skill.description}</description>`,
        `  </skill>`,
      ]),
      "</available_skills>",
    ].join(" ")
```

### Permission Filtering

Skills can be restricted per-agent:

```typescript
const accessibleSkills = agent
  ? skills.filter((skill) => {
      const rule = PermissionNext.evaluate("skill", skill.name, agent.permission)
      return rule.action !== "deny"
    })
  : skills
```

### Execution Flow

```typescript
async execute(params, ctx) {
  // 1. Lookup
  const skill = await Skill.get(params.name)
  if (!skill) throw new Error(`Skill "${params.name}" not found`)

  // 2. Permission check
  await ctx.ask({
    permission: "skill",
    patterns: [params.name],
    always: [params.name],
    metadata: {},
  })

  // 3. Parse content
  const parsed = await ConfigMarkdown.parse(skill.location)
  const dir = path.dirname(skill.location)

  // 4. Format output
  return {
    title: `Loaded skill: ${skill.name}`,
    output: [
      `## Skill: ${skill.name}`,
      ``,
      `**Base directory**: ${dir}`,
      ``,
      parsed.content.trim()
    ].join("\n"),
    metadata: { name: skill.name, dir },
  }
}
```

## Content Processing

### ConfigMarkdown.parse()

Handles YAML frontmatter preprocessing:

```typescript
export async function parse(filePath: string) {
  const raw = await Bun.file(filePath).text()
  const template = preprocessFrontmatter(raw)
  const md = matter(template)  // gray-matter library
  return md
}
```

### Frontmatter Preprocessing

Handles edge cases where YAML values contain colons:

```typescript
function preprocessFrontmatter(content: string): string {
  // Convert values with colons to YAML block scalar format
  // e.g., "key: foo:bar" becomes:
  //   key: |
  //     foo:bar
}
```

### File References

Skills can reference files using `@filepath` syntax:

```typescript
const FILE_REGEX = /(?<![\w`])@(\.?[^\s`,.]*(?:\.[^\s`,.]+)*)/g
```

### Shell Commands

Skills can include executable shell commands:

```typescript
const SHELL_REGEX = /!`([^`]+)`/g
```

## Caching Strategy

Skills are loaded once per `Instance` lifecycle via `Instance.state()`:

```typescript
export const state = Instance.state(async () => {
  const skills: Record<string, Info> = {}
  // ... discovery logic
  return skills
})
```

This means:
- Skills are scanned once at startup
- New skills require restart to appear
- Low runtime overhead

## Integration Points

### Session System

Skills are NOT injected into system prompts. They're loaded on-demand:
- LLM sees skill list in tool description
- LLM chooses to invoke skill tool
- Skill content appears as tool result

### Permission System

```typescript
await ctx.ask({
  permission: "skill",
  patterns: [params.name],
  always: [params.name],  // Always prompt for this skill pattern
  metadata: {},
})
```

### Agent System

Agents can configure skill access in their permission ruleset:

```yaml
permission:
  skill:
    "code-review": "allow"
    "deploy-*": "deny"
```

## Error Handling

### Parse Failures

```typescript
const md = await ConfigMarkdown.parse(match).catch((err) => {
  const message = ConfigMarkdown.FrontmatterError.isInstance(err)
    ? err.data.message
    : `Failed to parse skill ${match}`
  Bus.publish(Session.Event.Error, { error: new NamedError.Unknown({ message }).toObject() })
  log.error("failed to load skill", { skill: match, err })
  return undefined
})
```

### Missing Skills

```typescript
if (!skill) {
  const available = await Skill.all().then((x) => Object.keys(x).join(", "))
  throw new Error(`Skill "${params.name}" not found. Available skills: ${available || "none"}`)
}
```

## Feature Flags

```typescript
Flag.OPENCODE_DISABLE_CLAUDE_CODE_SKILLS  // Disable .claude/skills/ scanning
```

## Key Design Decisions

1. **On-demand loading** - Skills don't pollute system prompt, only loaded when needed
2. **Claude Code compatibility** - Supports both `.claude/skills/` and `.opencode/skill/` paths
3. **Hierarchical discovery** - Project > Global > Config directories
4. **Permission-aware** - Skills filtered by agent permissions before listing
5. **Symlink support** - `followSymlinks: true` in glob scanning
6. **Base directory context** - Skills receive their directory path for relative file references
