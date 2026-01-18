# OpenCode Context Compaction Research

**Researcher**: Claude Agent (Opus 4.5)
**Date**: 2026-01-17
**Subject**: Context compaction and pruning subsystem - managing token overflow

---

## What Was Researched

Deep reverse engineering of OpenCode's context compaction system. This is the mechanism that handles long conversations by:
1. Detecting when token limits are exceeded
2. Pruning old tool outputs to reclaim space
3. Generating a summary to preserve context
4. Filtering messages to hide pre-compaction history

## Files in This Directory

| File | Description |
|------|-------------|
| `COMPACTION-SUBSYSTEM-SPEC.md` | Complete specification - architecture, flow, data structures |
| `PRUNING-ALGORITHM-SPEC.md` | Deep dive into the token-aware pruning algorithm |
| `BRUTUS-COMPACTION-IMPLEMENTATION-SPEC.md` | Go implementation specification for BRUTUS |

## Key Findings

### Architecture

The compaction system consists of three cooperating components:

1. **Overflow Detection** (`isOverflow()`) - Checks if token usage exceeds model capacity
2. **Output Pruning** (`prune()`) - Marks old tool outputs as compacted to save tokens
3. **Summary Generation** (`process()`) - Uses the "compaction" agent to create continuation context

### Critical Constants

```typescript
PRUNE_MINIMUM = 20_000      // Minimum tokens to prune (20K)
PRUNE_PROTECT = 40_000      // Protect last 40K tokens from pruning
CHARS_PER_TOKEN = 4         // Estimation: 4 chars = 1 token
OUTPUT_TOKEN_MAX = 32_000   // Default max output tokens
```

### Protected Tools

The `skill` tool is protected from pruning - its outputs are never compacted.

### Overflow Detection Formula

```
count = tokens.input + tokens.cache.read + tokens.output
output = min(model.limit.output, 32000) || 32000
usable = model.limit.input || (context - output)
overflow = count > usable
```

### Message Filtering

After compaction, `filterCompacted()` streams messages backwards and stops when it encounters a user message with a `compaction` part whose corresponding assistant message (with `summary: true`) is complete.

### Configuration Options

```typescript
compaction: {
  auto: boolean   // Enable auto-compaction (default: true)
  prune: boolean  // Enable output pruning (default: true)
}
```

Environment flags: `OPENCODE_DISABLE_AUTOCOMPACT`, `OPENCODE_DISABLE_PRUNE`

## What Was NOT Researched

- Plugin hooks for compaction customization
- UI rendering of compaction state
- Session title generation (separate subsystem)
- Snapshot/diff creation internals

## Next Steps for Future Agents

1. **Implement overflow detection** - Port the token calculation
2. **Implement pruning** - Mark old tool outputs with compaction timestamps
3. **Implement message filtering** - Stream and filter compacted messages
4. **Implement summary agent** - Use a dedicated agent to generate summaries
5. **Add configuration** - Support auto/prune toggles

## Source Files Analyzed

- `packages/opencode/src/session/compaction.ts` (~180 lines)
- `packages/opencode/src/session/summary.ts` (~120 lines)
- `packages/opencode/src/session/prompt.ts` (loop integration)
- `packages/opencode/src/session/message-v2.ts` (filterCompacted)
- `packages/opencode/src/util/token.ts` (token estimation)
- `packages/opencode/src/agent/prompt/compaction.txt` (agent prompt)
- `packages/opencode/src/config/config.ts` (configuration schema)
