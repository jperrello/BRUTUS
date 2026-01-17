# Ralph Loop Agent Instructions

## 🚨 HARD RULES - READ FIRST

1. **YOU WILL WORK ON EXACTLY 3 ISSUES** - no more, no less
2. **AFTER 3 ISSUES, YOU STOP** - do not continue, do not offer to do more
3. **UPDATE THIS FILE** before stopping - next agent depends on your context
4. **NO SCOPE CREEP** - if you discover work, file an issue, don't do it

**This is non-negotiable. Another agent starts immediately after you.**

---

## Context From Previous Agent

> First agent in this loop. No prior context.
>
> Starting fresh with `bd ready` issues.

---

## Your 3 Issues

Run `bd ready` and pick the top 3 issues to work on. Update this section with what you picked.

**Issue 1**: _[to be filled]_
**Issue 2**: _[to be filled]_
**Issue 3**: _[to be filled]_

---

## Work Protocol

For each issue:
1. `bd update <id> --status=in_progress` - claim it
2. Do the work (code, fix, feature)
3. `bd close <id>` - mark complete
4. Note any discoveries/blockers below

---

## Discoveries & New Issues

_If you discover work during your 3 issues, file them into beads.

---

## End Protocol (MANDATORY)

When you finish your 3 issues, you MUST:

### 1. Update the context section above

Replace "Context From Previous Agent" with what you accomplished:
```markdown
## Context From Previous Agent

> Agent completed 3 issues:
> - BRUTUS-xxx: [what was done]
> - BRUTUS-yyy: [what was done]
> - BRUTUS-zzz: [what was done]
>
> Notes for next agent:
> - [any important context]
> - [blockers, discoveries, recommendations]
```

### 2. Pick next 3 issues for the next agent

Run `bd ready` and fill in "Your 3 Issues" section with suggested next issues.

### 3. STOP

Say "Ralph loop complete. Context updated for next agent." and stop.

**DO NOT:**
- Offer to do more work
- Start on issue #4
- Say "I could also..."
- Continue past the 3-issue limit

---

## Current Status

- **Loop iteration**: 1
- **Issues completed this loop**: 0/3
- **Total issues in backlog**: (run `bd ready` to see)
