# BRUTUS v2 Implementation Plan

**Mode**: Ralph Loop (autonomous subtask execution)
**Status**: Active

---

## How This Works

1. You are running in a ralph loop
2. Look at `prompts/queue/` directory
3. Find the FIRST file (alphabetically by number prefix)
4. Execute that ONE subtask completely
5. When done: DELETE the prompt file you just completed
6. Exit the session
7. Ralph loop restarts, you read this file again, find next subtask

**CRITICAL**: Only work on ONE subtask per session. Delete its prompt when done. Exit.

---

## Your Instructions

```
1. List files in prompts/queue/
2. If empty → ALL DONE! Print "BRUTUS v2 COMPLETE" and exit
3. If files exist → Read the FIRST file (lowest number)
4. Execute the subtask described in that file
5. Verify the subtask is complete (tests pass, file exists, etc.)
6. DELETE the prompt file: rm prompts/queue/XXX-name.md
7. Exit session
```

---

## Current Progress

Check `prompts/queue/` to see remaining tasks. Lower numbers = earlier in sequence.

Files are numbered to ensure correct execution order:
- 001-xxx = Foundation utilities
- 002-xxx = Tool abstraction
- 003-xxx = Individual tools
- 004-xxx = Agent loop
- 005-xxx = Integration

---

## Research Reference

All research is in `research/` directory. Each subtask prompt tells you exactly which research files to read.

Key implementation specs (these have Go code):
- `research/opencode-edit-tool/BRUTUS-EDIT-IMPLEMENTATION-SPEC.md`
- `research/opencode-agent-loop/BRUTUS-IMPLEMENTATION-SPEC.md`
- `research/opencode-tool/BRUTUS-TOOL-IMPLEMENTATION-SPEC.md`
- `research/opencode-compaction/BRUTUS-COMPACTION-IMPLEMENTATION-SPEC.md`

---

## Safety

- Each subtask is small (15-30 min of work)
- Each subtask has clear completion criteria
- If something breaks, you can re-add the prompt to retry
- All code is committed incrementally

---

## When All Done

When `prompts/queue/` is empty:
1. Run `go build ./...` to verify everything compiles
2. Run `go test ./...` to verify tests pass
3. Run `bd sync` to sync beads
4. Print "BRUTUS v2 COMPLETE - EXIT_SIGNAL"

The EXIT_SIGNAL tells ralph loop to stop.
