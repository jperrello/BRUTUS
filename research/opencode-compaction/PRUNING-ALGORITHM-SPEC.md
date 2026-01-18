# OpenCode Pruning Algorithm Specification

## Purpose

The pruning algorithm performs "lazy deletion" of old tool outputs to reclaim token budget without losing the conversation structure. It marks outputs as compacted rather than deleting them, allowing the message filter to exclude their content from future context windows.

## Core Insight

Tool outputs (especially from read, bash, glob, grep) are the largest token consumers. By marking old outputs as compacted, the system can:

1. Keep the tool call structure intact (for debugging/history)
2. Exclude the actual output content from context
3. Preserve recent outputs for active work context

## Algorithm Visualization

```
Timeline of messages (newest at bottom):

┌─────────────────────────────────────────┐
│ [PRUNE ZONE - Candidates for pruning]   │
│                                         │
│ msg: read_file → output: 5000 tokens    │ ← PRUNE (if total > 40K)
│ msg: bash → output: 2000 tokens         │ ← PRUNE (if total > 40K)
│ msg: grep → output: 8000 tokens         │ ← PRUNE (if total > 40K)
│                                         │
├─────────────────────────────────────────┤
│ [PROTECTION ZONE - Last 40K tokens]     │
│                                         │
│ msg: read_file → output: 3000 tokens    │ ← PROTECTED
│ msg: edit_file → output: 500 tokens     │ ← PROTECTED
│                                         │
├─────────────────────────────────────────┤
│ [ACTIVE ZONE - Last 2 user turns]       │
│                                         │
│ user: "Please edit main.go"             │ ← SKIP (active)
│ assistant: tool calls                   │ ← SKIP (active)
│ user: "Fix the bug"                     │ ← SKIP (active)
│                                         │
└─────────────────────────────────────────┘
```

## State Machine

```
                    ┌────────────────────────────────────┐
                    │                                    │
                    ▼                                    │
              ┌──────────┐                               │
              │  START   │                               │
              └────┬─────┘                               │
                   │                                     │
          config.prune?                                  │
              FALSE ─────────────────► EXIT              │
                   │                                     │
                 TRUE                                    │
                   │                                     │
                   ▼                                     │
         ┌────────────────┐                              │
         │ Load Messages  │                              │
         │ (reversed)     │                              │
         └───────┬────────┘                              │
                 │                                       │
                 ▼                                       │
         ┌────────────────┐                              │
    ┌───►│ Next Message   │◄──────────────────────┐     │
    │    └───────┬────────┘                       │     │
    │            │                                │     │
    │   msg.role == "user"?                       │     │
    │            │                                │     │
    │          YES: turns++                       │     │
    │            │                                │     │
    │   turns < 2?                                │     │
    │            │                                │     │
    │          YES ───────────────────────────────┘     │
    │            │                                      │
    │           NO                                      │
    │            │                                      │
    │   msg.summary == true?                            │
    │            │                                      │
    │          YES ─────────────────► COMMIT PRUNES ────┘
    │            │
    │           NO
    │            │
    │            ▼
    │    ┌───────────────┐
    │    │ Scan Parts    │
    │    │ (reversed)    │
    │    └───────┬───────┘
    │            │
    │            ▼
    │    ┌───────────────┐
    └────┤ Analyze Part  │
         └───────────────┘
```

## Part Analysis Detail

```
For each part in message.parts (reversed):

    part.type == "tool"?
         │
        NO ────────────────► next part
         │
       YES
         │
    part.state.status == "completed"?
         │
        NO ────────────────► next part
         │
       YES
         │
    part.tool in PROTECTED_TOOLS?
         │
       YES ────────────────► next part
         │
        NO
         │
    part.state.time.compacted exists?
         │
       YES ────────────────► STOP (hit boundary)
         │
        NO
         │
    estimate = len(output) / 4
    total += estimate
         │
    total > PRUNE_PROTECT (40K)?
         │
        NO ────────────────► next part (protected)
         │
       YES
         │
    pruned += estimate
    toPrune.push(part)
         │
         ▼
    next part
```

## Constants Rationale

### PRUNE_PROTECT = 40,000 tokens

Protects the last 40K tokens of tool outputs. This ensures:
- Recent file reads are available for editing
- Recent command outputs are available for debugging
- The model has sufficient context for current work

### PRUNE_MINIMUM = 20,000 tokens

Only commit pruning if we can save at least 20K tokens. This avoids:
- Excessive database writes for trivial savings
- Partial pruning that fragments context
- Churn when near the threshold

### CHARS_PER_TOKEN = 4

Rough estimate based on GPT tokenization of English text:
- Typical words: 5-6 chars → 1-2 tokens
- Code: ~4 chars per token average
- This is a heuristic, not precise

## Protected Tools

```typescript
const PRUNE_PROTECTED_TOOLS = ["skill"];
```

The `skill` tool outputs are never pruned because:
- Skills often contain critical configuration or context
- Skills may be referenced later in the conversation
- Skills are typically invoked intentionally, not speculatively

## Pruning Effect

When a part is pruned:

```typescript
// Before
part.state = {
  status: "completed",
  output: "... 50KB of file content ...",
  time: { start: 1234, end: 1235 }
}

// After
part.state = {
  status: "completed",
  output: "... 50KB of file content ...",
  time: { start: 1234, end: 1235, compacted: 1705512345678 }
}
```

The output is NOT deleted. The `compacted` timestamp signals to downstream systems that this output should be excluded from context.

## Message Filter Behavior

When building context, `filterCompacted` uses the compacted timestamp:

```typescript
// In message-v2.ts (conceptual)
function buildContextForLLM(messages) {
  for (const msg of messages) {
    for (const part of msg.parts) {
      if (part.type === "tool" && part.state.time?.compacted) {
        // Exclude output from context
        yield { ...part, state: { ...part.state, output: "[compacted]" } };
      } else {
        yield part;
      }
    }
  }
}
```

## Iteration Order

The algorithm iterates **backwards** through messages because:

1. Most recent content is most valuable
2. Oldest content is most expendable
3. We need to find the "protection boundary" from the end
4. We stop at previous compaction checkpoints

```
messages: [oldest ... ... ... newest]
                               ← iterate this way
```

## Turn Counting

"Turns" are user messages. The algorithm skips the last 2 turns:

```
Turn 0 (current): User just asked something, assistant responding
Turn 1 (previous): Last complete exchange
Turn 2+: Candidates for pruning
```

This ensures the immediate conversation context is never pruned, even if outputs are large.

## Edge Cases

### Empty Session
```
messages.length === 0
→ No pruning possible, returns immediately
```

### No Tool Outputs
```
All parts are text, no tool calls
→ total remains 0, pruned remains 0
→ No pruning happens
```

### All Protected
```
All tool outputs are from protected tools
→ toPrune array remains empty
→ No pruning happens
```

### Previous Compaction Exists
```
Encounters part with time.compacted set
→ Immediately breaks the loop
→ Only prunes content after that point
```

### Below Minimum Threshold
```
pruned < PRUNE_MINIMUM (20K)
→ Does not commit changes
→ Waits until more content accumulates
```

## Performance Characteristics

| Operation | Complexity | Notes |
|-----------|------------|-------|
| Message scan | O(n) | n = message count |
| Part scan | O(m) | m = parts per message |
| Token estimate | O(k) | k = output length |
| DB updates | O(p) | p = parts to prune |

Worst case: O(n × m × k) + O(p) database writes

In practice:
- Messages scanned: ~100s (typical session)
- Parts per message: ~1-5
- Output strings: pre-computed length
- DB updates: batched in practice
