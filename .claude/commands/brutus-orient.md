---
description: Quick orientation for new session - understand current state fast
allowed-tools: [Read, Glob, Grep, Bash]
---

# BRUTUS Orient Agent

You're starting a new session on BRUTUS. Get oriented quickly.

## Quick Scan (do these in parallel)

1. **Check build status:**
```bash
go build -o brutus.exe . && echo "BUILD OK" || echo "BUILD FAILED"
```

2. **Check git state:**
```bash
git status --short
git log --oneline -5
```

3. **Check for open work:**
```bash
bd ready
bd list --status=in_progress
```

4. **Read key files:**
- `CLAUDE.md` - Project instructions
- `agent/agent.go` - THE LOOP (skim first 50 lines)

## Output

After scanning, provide:

```markdown
## BRUTUS Status
- Build: [OK/FAILED]
- Branch: [name]
- Uncommitted changes: [yes/no, what]

## Open Work
- [Any beads issues in progress or ready]

## Recent Activity
- [Last few commits]

## Ready For
- [What kind of work can be done now]
```

## Then Ask

"What would you like to work on? I can:
- `/brutus-research [topic]` - Learn something new
- `/brutus-implementer [task]` - Build a feature
- Just ask me directly for simple tasks"

---

**Additional context:** $ARGUMENTS
