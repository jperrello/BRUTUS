# OpenCode Agent Loop Specification

## Overview

The agent loop is the core inference engine that processes user messages, executes tool calls, and manages conversation flow. It runs continuously until a stopping condition is met.

---

## Entry Points

### `SessionPrompt.prompt(input: PromptInput)`

Main entry point for user messages.

```typescript
interface PromptInput {
  sessionID: string           // Session identifier
  messageID?: string          // Optional pre-assigned message ID
  model?: {                   // Model override
    providerID: string
    modelID: string
  }
  agent?: string              // Agent name override
  noReply?: boolean           // Skip assistant response
  tools?: Record<string, boolean>  // Tool enable/disable (deprecated)
  system?: string             // Additional system prompt
  variant?: string            // Model variant (reasoning level)
  parts: PromptPart[]         // Message content parts
}
```

### `SessionPrompt.loop(sessionID: string)`

The actual inference loop. Called internally by `prompt()`.

---

## Loop Structure

### Pseudocode

```
loop(sessionID):
    abort = start(sessionID)  // Get/create abort controller
    if !abort:
        return waitForExisting()  // Another loop is running

    step = 0
    session = getSession(sessionID)

    while true:
        setStatus(sessionID, "busy")
        msgs = filterCompactedMessages(sessionID)

        // Find key messages
        lastUser = findLastUser(msgs)
        lastAssistant = findLastAssistant(msgs)
        lastFinished = findLastFinishedAssistant(msgs)
        tasks = collectPendingTasks(msgs)

        // Exit conditions
        if !lastUser: throw Error
        if lastAssistant.finish && !["tool-calls", "unknown"].includes(finish):
            if lastUser.id < lastAssistant.id:
                break  // Normal completion

        step++

        // First step: trigger title generation
        if step == 1:
            ensureTitle(session, lastUser.model, msgs)

        model = getModel(lastUser.model)
        task = tasks.pop()

        // Handle pending subtask
        if task?.type == "subtask":
            executeSubtask(task)
            continue

        // Handle pending compaction
        if task?.type == "compaction":
            result = processCompaction(msgs, lastUser.id, abort, sessionID)
            if result == "stop": break
            continue

        // Check for context overflow
        if lastFinished && needsCompaction(lastFinished.tokens, model):
            createCompactionTask(sessionID)
            continue

        // Normal processing
        agent = getAgent(lastUser.agent)
        maxSteps = agent.steps ?? Infinity
        isLastStep = step >= maxSteps

        msgs = insertReminders(msgs, agent, session)

        processor = createProcessor({
            assistantMessage: createAssistantMessage(),
            sessionID,
            model,
            abort
        })

        tools = resolveTools(agent, session, model, lastUser.tools, processor)

        result = processor.process({
            user: lastUser,
            agent,
            abort,
            sessionID,
            system: [...environmentPrompts(), ...customPrompts()],
            messages: toModelMessages(msgs),
            tools,
            model
        })

        if result == "stop": break
        if result == "compact":
            createCompactionTask(sessionID)
        continue

    pruneCompaction(sessionID)
    return lastAssistantMessage()
```

---

## Processor Architecture

### `SessionProcessor.create(input)`

Creates a processor instance that handles the actual LLM streaming.

```typescript
interface ProcessorInput {
  assistantMessage: MessageV2.Assistant
  sessionID: string
  model: Provider.Model
  abort: AbortSignal
}
```

### `processor.process(streamInput)`

Processes a single LLM stream, handling all events.

#### Stream Event Types

| Event | Handler |
|-------|---------|
| `start` | Set session status to busy |
| `reasoning-start` | Create reasoning part |
| `reasoning-delta` | Append to reasoning text |
| `reasoning-end` | Finalize reasoning part |
| `tool-input-start` | Create pending tool part |
| `tool-input-delta` | (ignored) |
| `tool-input-end` | (ignored) |
| `tool-call` | Execute tool, check doom loop |
| `tool-result` | Update tool part with result |
| `tool-error` | Update tool part with error |
| `error` | Throw error |
| `start-step` | Take filesystem snapshot |
| `finish-step` | Calculate usage, create patch |
| `text-start` | Create text part |
| `text-delta` | Append to text |
| `text-end` | Finalize text part |
| `finish` | (ignored) |

#### Return Values

- `"continue"` - Normal completion, may have more tool calls
- `"stop"` - Blocked by permission or error
- `"compact"` - Context overflow, needs compaction

---

## Doom Loop Detection

Detects when the LLM is stuck calling the same tool repeatedly.

```typescript
const DOOM_LOOP_THRESHOLD = 3

// In tool-call handler:
const parts = await MessageV2.parts(assistantMessage.id)
const lastThree = parts.slice(-DOOM_LOOP_THRESHOLD)

if (
  lastThree.length === DOOM_LOOP_THRESHOLD &&
  lastThree.every(p =>
    p.type === "tool" &&
    p.tool === value.toolName &&
    p.state.status !== "pending" &&
    JSON.stringify(p.state.input) === JSON.stringify(value.input)
  )
) {
  await PermissionNext.ask({
    permission: "doom_loop",
    patterns: [value.toolName],
    sessionID,
    metadata: { tool: value.toolName, input: value.input },
    always: [value.toolName],
    ruleset: agent.permission
  })
}
```

---

## Tool Resolution

Tools are resolved per-request based on:

1. **ToolRegistry** - Built-in tools for the provider/agent
2. **MCP tools** - External tools from MCP servers
3. **Permission filtering** - Tools disabled by agent permissions
4. **User overrides** - Per-message tool enable/disable

```typescript
async function resolveTools(input) {
  const tools = {}

  // Add registry tools
  for (const item of await ToolRegistry.tools(providerID, agent)) {
    tools[item.id] = wrapTool(item, context)
  }

  // Add MCP tools
  for (const [key, item] of Object.entries(await MCP.tools())) {
    tools[key] = wrapMCPTool(item, context)
  }

  return tools
}
```

---

## LLM Stream Configuration

### System Prompt Assembly

```typescript
const system = SystemPrompt.header(providerID)  // Provider-specific header

system.push([
  // Agent prompt OR provider default prompt
  ...(agent.prompt ? [agent.prompt] : isCodex ? [] : SystemPrompt.provider(model)),
  // Custom prompts from this call
  ...input.system,
  // User's custom system prompt
  ...(user.system ? [user.system] : [])
].filter(x => x).join("\n"))
```

### Options Layering

```typescript
const options = pipe(
  base,                          // Provider defaults
  mergeDeep(model.options),      // Model-specific options
  mergeDeep(agent.options),      // Agent options
  mergeDeep(variant)             // Reasoning variant options
)
```

### Tool Call Repair

When a tool call fails, OpenCode attempts repair:

```typescript
async experimental_repairToolCall(failed) {
  const lower = failed.toolCall.toolName.toLowerCase()

  // Try lowercase version
  if (lower !== failed.toolCall.toolName && tools[lower]) {
    return { ...failed.toolCall, toolName: lower }
  }

  // Mark as invalid
  return {
    ...failed.toolCall,
    input: JSON.stringify({
      tool: failed.toolCall.toolName,
      error: failed.error.message
    }),
    toolName: "invalid"
  }
}
```

---

## Context Compaction

### Overflow Detection

```typescript
async function isOverflow(input: { tokens: TokenUsage, model: Model }) {
  const limit = model.limit.context
  const threshold = 0.85  // 85% of context window
  const used = tokens.input + tokens.output + tokens.reasoning
  return used > limit * threshold
}
```

### Compaction Task Creation

When overflow is detected, a compaction task is added:

```typescript
await Session.updatePart({
  type: "compaction",
  auto: true,
  sessionID,
  messageID,
  id: Identifier.ascending("part")
})
```

The next loop iteration detects this part and triggers compaction processing.

---

## Stopping Conditions

The loop exits when:

1. **Normal completion**: Last assistant message has a finish reason other than `tool-calls` or `unknown`, and was created after the last user message

2. **Blocked**: Permission denied or question rejected (returns `"stop"`)

3. **Error**: Unrecoverable error (returns `"stop"`)

4. **Abort**: Signal aborted externally

---

## Session State Machine

```
                  ┌─────────────────────────────────┐
                  │                                 │
                  ▼                                 │
    ┌──────────────────┐                           │
    │      idle        │◄──────────────────────────┤
    └────────┬─────────┘                           │
             │ prompt()                            │
             ▼                                     │
    ┌──────────────────┐                           │
    │      busy        │───────────────────────────┤
    └────────┬─────────┘                           │
             │ API error                           │
             ▼                                     │
    ┌──────────────────┐                           │
    │      retry       │───────────────────────────┘
    └──────────────────┘       success/abort/max retries
```

---

## Retry Logic

### Retryable Errors

From `session/retry.ts`:

```typescript
function retryable(error: APIError): string | undefined {
  if (error.statusCode === 529) return "API overloaded"
  if (error.statusCode === 529) return "API overloaded"
  if (error.statusCode === 503) return "Service unavailable"
  if (error.statusCode === 502) return "Bad gateway"
  if (error.statusCode === 500) return "Internal server error"
  if (error.isRetryable) return error.message
  return undefined  // Not retryable
}
```

### Delay Calculation

Exponential backoff with jitter:

```typescript
function delay(attempt: number, error?: APIError): number {
  const baseDelay = 1000
  const maxDelay = 60000
  const retryAfter = error?.responseHeaders?.["retry-after"]

  if (retryAfter) {
    const parsed = parseInt(retryAfter)
    if (!isNaN(parsed)) return parsed * 1000
  }

  const exponential = baseDelay * Math.pow(2, attempt - 1)
  const jitter = Math.random() * 1000
  return Math.min(exponential + jitter, maxDelay)
}
```
