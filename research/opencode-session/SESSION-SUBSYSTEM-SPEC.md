# OpenCode Session Subsystem Specification

## 1. Architecture Overview

The session subsystem is the backbone of OpenCode's conversational state management. It orchestrates:

1. **Session Lifecycle** - Creation, forking, deletion, archival
2. **Message Management** - User/assistant messages with typed parts
3. **Processing Loop** - Agentic inference with tool execution
4. **Persistence** - JSON file-based storage with migrations
5. **Cost Tracking** - Token usage and cost accumulation

```
┌──────────────────────────────────────────────────────────────┐
│                        Session                                │
│  ┌─────────────┐   ┌─────────────┐   ┌─────────────┐        │
│  │   Message   │   │   Message   │   │   Message   │        │
│  │   (User)    │──▶│ (Assistant) │──▶│   (User)    │ ...    │
│  └─────────────┘   └─────────────┘   └─────────────┘        │
│        │                 │                                   │
│        ▼                 ▼                                   │
│  ┌───────────────────────────────────────────────────┐      │
│  │                    Parts                           │      │
│  │  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐   │      │
│  │  │ Text │ │ Tool │ │ File │ │Reason│ │ Snap │   │      │
│  │  └──────┘ └──────┘ └──────┘ └──────┘ └──────┘   │      │
│  └───────────────────────────────────────────────────┘      │
└──────────────────────────────────────────────────────────────┘
```

## 2. Session Entity

### 2.1 Schema Definition

```typescript
Session.Info = {
  id: string           // "ses_" prefix + timestamp + random
  slug: string         // Human-readable URL slug
  projectID: string    // Links to git root commit hash
  directory: string    // Working directory path
  parentID?: string    // For forked/child sessions

  summary?: {
    additions: number  // Lines added across session
    deletions: number  // Lines removed
    files: number      // Files modified
    diffs?: FileDiff[] // Detailed per-file diffs
  }

  share?: { url: string }  // If shared publicly

  title: string        // Auto-generated or user-set
  version: string      // OpenCode version that created it

  time: {
    created: number    // Unix timestamp ms
    updated: number    // Last activity
    compacting?: number
    archived?: number
  }

  permission?: Ruleset // Permission overrides for this session
  revert?: RevertState // Pending revert operation
}
```

### 2.2 Key Operations

| Operation | Function | Behavior |
|-----------|----------|----------|
| `create` | `Session.create(opts?)` | Creates new session with descending ID for listing order |
| `fork` | `Session.fork({sessionID, messageID?})` | Clones messages up to messageID into new session |
| `touch` | `Session.touch(sessionID)` | Updates `time.updated` timestamp |
| `update` | `Session.update(id, editor)` | Atomic read-modify-write with lock |
| `remove` | `Session.remove(sessionID)` | Cascading delete: children, shares, messages, parts |
| `share` | `Session.share(sessionID)` | Creates shareable URL via ShareNext |
| `list` | `Session.list()` | AsyncGenerator yielding all sessions for project |
| `children` | `Session.children(parentID)` | Find all child sessions |

### 2.3 ID Ordering Strategy

Sessions use **descending IDs** so that when listed alphabetically, newest sessions appear first:

```typescript
// Descending ID: bitwise NOT of timestamp
now = descending ? ~now : now
// Result: larger timestamp = smaller ID value
```

## 3. Message System

### 3.1 Message Types

```typescript
// User Message
MessageV2.User = {
  id: string
  sessionID: string
  role: "user"
  time: { created: number }

  agent: string           // Which agent handles this
  model: {                // Model selection
    providerID: string
    modelID: string
  }
  system?: string         // Custom system prompt override
  tools?: Record<string, boolean>  // Tool permissions
  variant?: string        // A/B testing variant
  summary?: {             // Compaction summary
    title?: string
    body?: string
    diffs: FileDiff[]
  }
}

// Assistant Message
MessageV2.Assistant = {
  id: string
  sessionID: string
  role: "assistant"
  parentID: string        // Links to user message that triggered it

  time: {
    created: number
    completed?: number
  }

  agent: string
  modelID: string
  providerID: string

  path: {
    cwd: string           // Current working directory
    root: string          // Project root
  }

  error?: ErrorObject     // If inference failed
  summary?: boolean       // If this is a compaction summary
  finish?: string         // "stop" | "tool-calls" | "unknown" | etc.

  cost: number            // Accumulated cost in USD
  tokens: {
    input: number
    output: number
    reasoning: number
    cache: { read: number, write: number }
  }
}
```

### 3.2 Part Types (12 types)

Parts are the atomic units within messages:

| Part Type | Purpose | Key Fields |
|-----------|---------|------------|
| `text` | LLM text output | `text`, `synthetic?`, `ignored?`, `time` |
| `reasoning` | Extended thinking | `text`, `time`, `metadata` |
| `tool` | Tool invocation | `callID`, `tool`, `state` (see below) |
| `file` | Attached file/image | `mime`, `filename`, `url`, `source?` |
| `snapshot` | Git state checkpoint | `snapshot` (hash) |
| `patch` | File changes | `hash`, `files[]` |
| `agent` | Agent invocation | `name`, `source?` |
| `compaction` | Context compaction marker | `auto: boolean` |
| `subtask` | Delegated task | `prompt`, `description`, `agent`, `model?` |
| `retry` | API retry record | `attempt`, `error`, `time` |
| `step-start` | Inference step begin | `snapshot?` |
| `step-finish` | Inference step end | `reason`, `tokens`, `cost`, `snapshot?` |

### 3.3 Tool State Machine

```
      ┌─────────┐
      │ pending │  (input received, awaiting execution)
      └────┬────┘
           │ execute()
           ▼
      ┌─────────┐
      │ running │  (execution in progress)
      └────┬────┘
           │
     ┌─────┴─────┐
     │           │
     ▼           ▼
┌──────────┐ ┌───────┐
│completed │ │ error │
└──────────┘ └───────┘
```

```typescript
ToolStatePending = {
  status: "pending"
  input: Record<string, any>
  raw: string
}

ToolStateRunning = {
  status: "running"
  input: Record<string, any>
  title?: string
  metadata?: Record<string, any>
  time: { start: number }
}

ToolStateCompleted = {
  status: "completed"
  input: Record<string, any>
  output: string
  title: string
  metadata: Record<string, any>
  time: { start: number, end: number, compacted?: number }
  attachments?: FilePart[]
}

ToolStateError = {
  status: "error"
  input: Record<string, any>
  error: string
  metadata?: Record<string, any>
  time: { start: number, end: number }
}
```

## 4. Processing Loop

### 4.1 Main Loop (`SessionPrompt.loop`)

```
┌─────────────────────────────────────────────────────────────────┐
│                        Session Loop                              │
│                                                                  │
│  ┌──────────┐    ┌──────────────┐    ┌─────────────┐           │
│  │ Get Last │───▶│ Check State  │───▶│ Process     │           │
│  │ Messages │    │ & Conditions │    │ Response    │           │
│  └──────────┘    └──────────────┘    └──────────────┘          │
│       ▲                                      │                  │
│       │                                      │                  │
│       │         ┌────────────────┐           │                  │
│       └─────────│ Check Result   │◀──────────┘                  │
│                 │ stop/continue/ │                              │
│                 │ compact        │                              │
│                 └────────────────┘                              │
└─────────────────────────────────────────────────────────────────┘
```

Key loop behaviors:

1. **Compaction Trigger**: If `tokens.input > threshold`, auto-compact
2. **Subtask Handling**: Processes `subtask` parts by invoking TaskTool
3. **Doom Loop Detection**: Detects 3+ identical tool calls and prompts user
4. **Step Tracking**: Each LLM round creates `step-start` and `step-finish` parts
5. **Abort Handling**: Clean shutdown on `AbortController.abort()`

### 4.2 Processor Stream Events

The processor handles these stream events from the LLM:

| Event | Handler Action |
|-------|----------------|
| `start` | Set session status to "busy" |
| `reasoning-start` | Create reasoning part |
| `reasoning-delta` | Append to reasoning text |
| `reasoning-end` | Finalize reasoning part |
| `text-start` | Create text part |
| `text-delta` | Append to text, publish delta |
| `text-end` | Trim and finalize text |
| `tool-input-start` | Create pending tool part |
| `tool-call` | Transition to running, check doom loop |
| `tool-result` | Transition to completed |
| `tool-error` | Transition to error |
| `start-step` | Take snapshot, create step-start |
| `finish-step` | Calculate usage, create step-finish, patch |
| `error` | Convert to typed error, potentially retry |
| `finish` | No-op, handled by loop |

### 4.3 Automatic Retries

```typescript
SessionRetry.retryable(error) // Returns retry message or undefined
SessionRetry.delay(attempt, apiError?) // Exponential backoff

// Retryable conditions:
// - APIError with isRetryable: true
// - ECONNRESET
// - 429/5xx status codes (rate limit, server error)
```

## 5. Storage Layer

### 5.1 File Structure

```
~/.opencode/storage/
├── migration          # Current migration version (integer)
├── project/
│   └── {projectID}.json
├── session/
│   └── {projectID}/
│       └── {sessionID}.json
├── message/
│   └── {sessionID}/
│       └── {messageID}.json
├── part/
│   └── {messageID}/
│       └── {partID}.json
├── session_diff/
│   └── {sessionID}.json
└── share/
    └── {sessionID}.json
```

### 5.2 Storage Operations

```typescript
Storage.write(key: string[], content: T)   // Write with lock
Storage.read<T>(key: string[])             // Read with lock
Storage.update<T>(key: string[], fn)       // Atomic read-modify-write
Storage.remove(key: string[])              // Delete file
Storage.list(prefix: string[])             // Glob for files under prefix
```

### 5.3 Locking

Uses file-based read/write locks:
- Read operations acquire shared locks
- Write operations acquire exclusive locks
- Prevents concurrent modification corruption

### 5.4 Migrations

```typescript
const MIGRATIONS: Migration[] = [
  async (dir) => { /* v0->v1: Restructure project/session hierarchy */ },
  async (dir) => { /* v1->v2: Extract diffs to separate files */ },
]
// Runs on first access, stores version in `migration` file
```

## 6. ID Generation

### 6.1 Algorithm

```typescript
Identifier.ascending(prefix, given?)   // Chronological order
Identifier.descending(prefix, given?)  // Reverse chronological

// Format: {prefix}_{timestamp_hex}{random_base62}
// Example: msg_01932a4b5c6d7eABCDEFghijklmn

// Components:
// - Prefix: "ses" | "msg" | "prt" | "per" | "que" | "usr" | "pty" | "tool"
// - Timestamp: 6 bytes (48 bits), includes counter for same-ms ordering
// - Random: 14 chars base62 for uniqueness
```

### 6.2 Monotonic Guarantee

```typescript
let lastTimestamp = 0
let counter = 0

function create(prefix, descending, timestamp?) {
  const currentTimestamp = timestamp ?? Date.now()

  if (currentTimestamp !== lastTimestamp) {
    lastTimestamp = currentTimestamp
    counter = 0
  }
  counter++

  // Pack timestamp + counter into 48 bits
  let now = BigInt(currentTimestamp) * BigInt(0x1000) + BigInt(counter)

  // Invert for descending order
  now = descending ? ~now : now

  // Encode to hex
  const timeBytes = Buffer.alloc(6)
  for (let i = 0; i < 6; i++) {
    timeBytes[i] = Number((now >> BigInt(40 - 8 * i)) & BigInt(0xff))
  }

  return prefix + "_" + timeBytes.toString("hex") + randomBase62(14)
}
```

### 6.3 Timestamp Extraction

```typescript
// Only works for ascending IDs
Identifier.timestamp(id: string): number {
  const prefix = id.split("_")[0]
  const hex = id.slice(prefix.length + 1, prefix.length + 13)
  const encoded = BigInt("0x" + hex)
  return Number(encoded / BigInt(0x1000))
}
```

## 7. Event Bus Integration

The session subsystem publishes events for UI updates:

```typescript
Session.Event.Created   // New session created
Session.Event.Updated   // Session metadata changed
Session.Event.Deleted   // Session removed
Session.Event.Diff      // File changes detected
Session.Event.Error     // Error occurred

MessageV2.Event.Updated     // Message info changed
MessageV2.Event.Removed     // Message deleted
MessageV2.Event.PartUpdated // Part content changed (with optional delta)
MessageV2.Event.PartRemoved // Part deleted
```

## 8. Model Message Conversion

`MessageV2.toModelMessage()` converts stored messages to LLM-compatible format:

```typescript
// Conversion rules:
// - User text parts → user content
// - User file parts → file content (except text/plain, directories)
// - User compaction → "What did we do so far?"
// - User subtask → "The following tool was executed by the user"
// - Assistant text → assistant content
// - Assistant tool (completed) → tool call + result
// - Assistant tool (error) → tool call + error result
// - Assistant tool (pending/running) → tool call + "[interrupted]"
// - Assistant reasoning → reasoning content
// - Errored assistant messages → skipped (unless aborted with content)
```

## 9. Cost Calculation

```typescript
Session.getUsage({ model, usage, metadata }) {
  // Handle provider-specific caching semantics
  const cachedInputTokens = usage.cachedInputTokens ?? 0
  const excludesCachedTokens = metadata?.["anthropic"] || metadata?.["bedrock"]

  const adjustedInputTokens = excludesCachedTokens
    ? usage.inputTokens
    : usage.inputTokens - cachedInputTokens

  // Check for experimental >200K pricing
  const costInfo = (tokens.input + tokens.cache.read > 200_000)
    ? model.cost?.experimentalOver200K
    : model.cost

  // Calculate cost
  return new Decimal(0)
    .add(tokens.input * costInfo.input / 1_000_000)
    .add(tokens.output * costInfo.output / 1_000_000)
    .add(tokens.cache.read * costInfo.cache.read / 1_000_000)
    .add(tokens.cache.write * costInfo.cache.write / 1_000_000)
    .add(tokens.reasoning * costInfo.output / 1_000_000)  // reasoning = output rate
    .toNumber()
}
```

## 10. Session Forking

Forking creates a new session with copied message history:

```typescript
Session.fork({ sessionID, messageID? }) {
  // 1. Create new empty session
  const session = await createNext({ directory: Instance.directory })

  // 2. Get all messages from source
  const msgs = await messages({ sessionID })

  // 3. Build ID mapping for parent references
  const idMap = new Map<string, string>()

  // 4. Copy messages up to messageID (if specified)
  for (const msg of msgs) {
    if (messageID && msg.info.id >= messageID) break

    const newID = Identifier.ascending("message")
    idMap.set(msg.info.id, newID)

    // Update parent references for assistant messages
    const parentID = msg.info.role === "assistant" && msg.info.parentID
      ? idMap.get(msg.info.parentID)
      : undefined

    // Clone message with new ID
    const cloned = await updateMessage({
      ...msg.info,
      sessionID: session.id,
      id: newID,
      ...(parentID && { parentID }),
    })

    // Clone all parts with new IDs
    for (const part of msg.parts) {
      await updatePart({
        ...part,
        id: Identifier.ascending("part"),
        messageID: cloned.id,
        sessionID: session.id,
      })
    }
  }

  return session
}
```

## 11. Plan Mode

Special handling for plan mode sessions:

```typescript
// Plan file location
Session.plan(session) {
  const base = Instance.project.vcs
    ? path.join(Instance.worktree, ".opencode", "plans")
    : path.join(Global.Path.data, "plans")
  return path.join(base, [session.time.created, session.slug].join("-") + ".md")
}

// Plan mode reminders injected when:
// 1. Agent is "plan" → inject plan workflow instructions
// 2. Switching from "plan" to "build" → inject build switch reminder
// 3. Plan file exists → tell agent to execute it
```
