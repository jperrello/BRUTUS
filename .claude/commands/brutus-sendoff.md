---
description: End session properly - create issues, update docs, generate handoff
allowed-tools: [Read, Write, Edit, Glob, Grep, Bash, TodoWrite]
---

# BRUTUS Sendoff Agent

You are responsible for properly closing a work session on BRUTUS. Your job is to preserve context so the next agent can pick up seamlessly.

## Sendoff Checklist

### 1. Assess Current State

```bash
git status                    # What changed?
git diff --stat               # Summary of changes
go build -o brutus.exe .      # Does it build?
go vet ./...                  # Any issues?
```

### 2. Create Issues for Remaining Work

For any unfinished work, incomplete features, or discovered problems:

```bash
bd create --title="..." --type=task|bug|feature --priority=2
```

Priority guide:
- 0-1: Critical/urgent
- 2: Normal
- 3-4: Nice to have/backlog

### 3. Update Documentation (if needed)

Check if these need updates based on what changed:
- `CLAUDE.md` - Architecture changed? New commands?
- `README.md` - User-facing changes?
- `LEARNING.md` / `ADDING_TOOLS.md` - New patterns?
- Slash commands in `.claude/commands/` - Outdated info?

### 4. Sync Beads

```bash
bd sync    # Commit beads changes
```

### 5. Generate Handoff

Create a prompt that the next agent can use to continue work:

**Format:**
```markdown
## Context
[What was this session about?]

## Completed
- [What got done]

## In Progress / Remaining
- [What still needs work]

## Key Files Touched
- `path/file.go` - [why]

## Suggested Next Steps
1. [Specific action]
2. [Specific action]

## Recommended Command
[Which /brutus-* command to start with]
```

### 6. Remind Human to Commit

After all the above, remind the user:
> "Run `/commit` to save your changes. Here's the handoff for the next session: [prompt]"

## DO NOT

- Commit code yourself (human does this)
- Push to remote (human decides)
- Close issues that aren't actually done
- Skip the build check

---

**Session context (if provided):** $ARGUMENTS
