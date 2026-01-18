# OpenCode Compaction Subsystem Specification

## Overview

The compaction subsystem manages context window overflow through a three-phase approach:
1. **Detection** - Monitor token usage against model limits
2. **Pruning** - Mark old tool outputs as compacted (lazy deletion)
3. **Summarization** - Generate a continuation prompt for the next context window

## Architecture Diagram

```
                                    ┌─────────────────┐
                                    │   Agent Loop    │
                                    │   (prompt.ts)   │
                                    └────────┬────────┘
                                             │
                         ┌───────────────────┼───────────────────┐
                         │                   │                   │
                         ▼                   ▼                   ▼
              ┌──────────────────┐ ┌──────────────────┐ ┌──────────────────┐
              │ isOverflow()    │ │ prune()          │ │ process()        │
              │ Token detection │ │ Mark old outputs │ │ Generate summary │
              └──────────────────┘ └──────────────────┘ └──────────────────┘
                         │                   │                   │
                         ▼                   ▼                   ▼
              ┌──────────────────┐ ┌──────────────────┐ ┌──────────────────┐
              │ SessionCompact   │ │ Session.update   │ │ SessionProcessor │
              │ .create()        │ │ Part()           │ │ .process()       │
              └──────────────────┘ └──────────────────┘ └──────────────────┘
```

## Phase 1: Overflow Detection

### Function: `isOverflow()`

```typescript
async function isOverflow(input: {
  tokens: MessageV2.Assistant["tokens"];
  model: Provider.Model;
}): Promise<boolean>
```

### Algorithm

```
1. Check config: if compaction.auto === false, return false
2. Get model context limit
3. If context === 0, return false (unlimited context)
4. Calculate total: input + cache.read + output
5. Calculate output ceiling: min(model.limit.output, 32000) || 32000
6. Calculate usable: model.limit.input || (context - output)
7. Return: total > usable
```

### Token Structure

```typescript
interface TokenCounts {
  input: number;
  output: number;
  reasoning: number;
  cache: {
    read: number;
    write: number;
  };
}
```

## Phase 2: Pruning

### Function: `prune()`

```typescript
async function prune(input: { sessionID: string }): Promise<void>
```

### Constants

| Constant | Value | Purpose |
|----------|-------|---------|
| `PRUNE_MINIMUM` | 20,000 | Don't prune unless we can save at least 20K tokens |
| `PRUNE_PROTECT` | 40,000 | Protect the last 40K tokens of tool outputs |
| `PRUNE_PROTECTED_TOOLS` | `["skill"]` | Tools whose outputs are never pruned |

### Algorithm (Pseudocode)

```
function prune(sessionID):
    if config.compaction.prune === false:
        return

    messages = Session.messages(sessionID)
    total = 0
    pruned = 0
    toPrune = []
    turns = 0

    # Iterate backwards through messages
    for msg in reversed(messages):
        if msg.role === "user":
            turns++

        # Skip the last 2 user turns (active conversation)
        if turns < 2:
            continue

        # Stop at previous summary checkpoint
        if msg.role === "assistant" AND msg.summary:
            break

        # Find completed tool outputs
        for part in reversed(msg.parts):
            if part.type === "tool" AND part.state.status === "completed":
                # Skip protected tools
                if part.tool in PRUNE_PROTECTED_TOOLS:
                    continue

                # Stop at already-compacted content
                if part.state.time.compacted:
                    break

                estimate = Token.estimate(part.state.output)
                total += estimate

                # Protect last PRUNE_PROTECT tokens
                if total > PRUNE_PROTECT:
                    pruned += estimate
                    toPrune.append(part)

    # Only prune if we can save enough
    if pruned > PRUNE_MINIMUM:
        for part in toPrune:
            part.state.time.compacted = Date.now()
            Session.updatePart(part)
```

### Token Estimation

```typescript
const CHARS_PER_TOKEN = 4;

function estimate(input: string): number {
  return Math.max(0, Math.round((input || "").length / CHARS_PER_TOKEN));
}
```

This is a heuristic, not accurate tokenization. OpenAI models average ~4 chars/token for English.

## Phase 3: Summarization

### Function: `process()`

```typescript
async function process(input: {
  parentID: string;
  messages: MessageV2.WithParts[];
  sessionID: string;
  abort: AbortSignal;
  auto: boolean;
}): Promise<"continue" | "stop">
```

### Process Flow

```
1. Get the user message that triggered compaction
2. Get the "compaction" agent definition
3. Resolve model (agent model or user's model)
4. Create new assistant message with:
   - mode: "compaction"
   - agent: "compaction"
   - summary: true
5. Create SessionProcessor for streaming
6. Trigger plugin hook: "experimental.session.compacting"
7. Build prompt from plugin context or default
8. Stream LLM response with all prior messages + summary request
9. If auto && result === "continue":
   - Create synthetic user message: "Continue if you have next steps"
10. Publish Event.Compacted
11. Return result
```

### Default Summary Prompt

```
Provide a detailed prompt for continuing our conversation above.
Focus on information that would be helpful for continuing the conversation,
including what we did, what we're doing, which files we're working on,
and what we're going to do next considering new session will not have
access to our conversation.
```

### Compaction Agent Prompt

The compaction agent is a hidden primary agent with the following directive:

```
Provide a detailed but concise summary of the conversation.

Capture:
- Completed actions and current work in progress
- Modified files and technical decisions with rationale
- Upcoming tasks and user-specific requirements or constraints
- Persistent preferences affecting the conversation

Balance: Comprehensive enough to provide context but concise
enough to be quickly understood.
```

### Assistant Message Schema (Compaction)

```typescript
interface CompactionAssistantMessage {
  id: string;              // Ascending identifier
  role: "assistant";
  parentID: string;        // Links to user message
  sessionID: string;
  mode: "compaction";
  agent: "compaction";
  summary: true;           // Marks this as a summary message
  path: {
    cwd: string;
    root: string;
  };
  cost: number;
  tokens: TokenCounts;
  modelID: string;
  providerID: string;
  time: {
    created: number;
  };
}
```

## Message Filtering

### Function: `filterCompacted()`

After compaction, the loop must exclude pre-compaction messages. This is done by streaming messages and stopping at the compaction boundary.

```typescript
async function filterCompacted(
  stream: AsyncIterable<MessageV2.WithParts>
): Promise<MessageV2.WithParts[]>
```

### Algorithm

```
1. Collect messages from stream
2. Track which user messages have completed summaries
3. When a user message with:
   - A compaction part
   - A corresponding completed summary (assistant with summary: true)
   - Is encountered → STOP iteration
4. Reverse the collected messages (they came in reverse order)
5. Return filtered list
```

### CompactionPart Type

```typescript
interface CompactionPart {
  id: string;
  messageID: string;
  sessionID: string;
  type: "compaction";
  auto: boolean;          // Was this auto-triggered?
}
```

## Entry Point: `create()`

Creates the user message that initiates compaction.

```typescript
const create = fn(
  z.object({
    sessionID: Identifier.schema("session"),
    agent: z.string(),
    model: z.object({
      providerID: z.string(),
      modelID: z.string(),
    }),
    auto: z.boolean(),
  }),
  async (input) => {
    // Create user message
    const msg = await Session.updateMessage({
      id: Identifier.ascending("message"),
      role: "user",
      model: input.model,
      sessionID: input.sessionID,
      agent: input.agent,
      time: { created: Date.now() },
    });

    // Add compaction part
    await Session.updatePart({
      id: Identifier.ascending("part"),
      messageID: msg.id,
      sessionID: msg.sessionID,
      type: "compaction",
      auto: input.auto,
    });
  }
);
```

## Loop Integration

In `prompt.ts`, the main loop checks for compaction in this order:

```typescript
// 1. Check for pending compaction task
if (task?.type === "compaction") {
  const result = await SessionCompaction.process({
    messages: msgs,
    parentID: lastUser.id,
    abort,
    sessionID,
    auto: task.auto,
  });
  if (result === "stop") break;
  continue;
}

// 2. Check for context overflow requiring compaction
if (
  lastFinished &&
  lastFinished.summary !== true &&
  (await SessionCompaction.isOverflow({ tokens: lastFinished.tokens, model }))
) {
  await SessionCompaction.create({
    sessionID,
    agent: lastUser.agent,
    model: lastUser.model,
    auto: true,
  });
  continue;
}
```

## Events

### Compacted Event

```typescript
const Event = {
  Compacted: BusEvent.define(
    "session.compacted",
    z.object({
      sessionID: z.string(),
    }),
  ),
};
```

Published after successful compaction to notify UI and other systems.

## Session Summary (Related)

The `SessionSummary` namespace handles two related functions:

1. **Session-level diff** - Aggregates all file changes across the session
2. **Message-level summary** - Generates titles for user messages

### Diff Tracking

```typescript
interface FileDiff {
  file: string;
  before: string;
  after: string;
  additions: number;
  deletions: number;
}
```

Diffs are computed between snapshot hashes captured at `step-start` and `step-finish` parts.

## Configuration

```typescript
// In config schema
compaction: z.object({
  auto: z.boolean().optional(),   // Default: true
  prune: z.boolean().optional(),  // Default: true
}).optional()
```

### Environment Overrides

- `OPENCODE_DISABLE_AUTOCOMPACT` - Disables auto compaction
- `OPENCODE_DISABLE_PRUNE` - Disables output pruning
