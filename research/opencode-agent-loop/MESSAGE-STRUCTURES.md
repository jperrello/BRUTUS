# OpenCode Message Structures

## Overview

OpenCode uses a sophisticated message structure to track conversations, tool calls, and system state. Messages are decomposed into typed "parts" for flexibility.

---

## Message Types

### User Message

```typescript
interface UserMessage {
  id: string                    // Identifier.schema("message")
  sessionID: string
  role: "user"
  time: {
    created: number             // Unix timestamp
  }
  summary?: {                   // Optional summary from compaction
    title?: string
    body?: string
    diffs: FileDiff[]
  }
  agent: string                 // Which agent to use
  model: {                      // Model for this message
    providerID: string
    modelID: string
  }
  system?: string               // Additional system prompt
  tools?: Record<string, boolean>  // Tool overrides (deprecated)
  variant?: string              // Reasoning variant
}
```

### Assistant Message

```typescript
interface AssistantMessage {
  id: string
  sessionID: string
  role: "assistant"
  time: {
    created: number
    completed?: number
  }
  error?: Error                 // If message ended in error
  parentID: string              // Links to user message
  modelID: string
  providerID: string
  mode: string                  // @deprecated - use agent
  agent: string                 // Agent that generated this
  path: {
    cwd: string                 // Working directory
    root: string                // Project root
  }
  summary?: boolean             // True if this is a compaction summary
  cost: number                  // Cumulative cost in USD
  tokens: {
    input: number
    output: number
    reasoning: number
    cache: {
      read: number
      write: number
    }
  }
  finish?: string               // Finish reason
}
```

---

## Part Types

### Base Part Structure

All parts share this base:

```typescript
interface PartBase {
  id: string                    // Identifier.schema("part")
  sessionID: string
  messageID: string
}
```

### TextPart

Plain text content (user input or assistant output).

```typescript
interface TextPart extends PartBase {
  type: "text"
  text: string
  synthetic?: boolean           // System-generated, not user input
  ignored?: boolean             // Excluded from model context
  time?: {
    start: number
    end?: number
  }
  metadata?: Record<string, any>  // Provider metadata
}
```

### FilePart

File attachments (images, code files).

```typescript
interface FilePart extends PartBase {
  type: "file"
  mime: string                  // MIME type
  filename?: string
  url: string                   // data: URL or file: URL
  source?: FilePartSource       // Origin info
}

type FilePartSource =
  | { type: "file", path: string, text: SourceText }
  | { type: "symbol", path: string, range: LSPRange, name: string, kind: number, text: SourceText }
  | { type: "resource", clientName: string, uri: string, text: SourceText }

interface SourceText {
  value: string
  start: number
  end: number
}
```

### ToolPart

Tracks tool call lifecycle.

```typescript
interface ToolPart extends PartBase {
  type: "tool"
  callID: string                // Tool call ID from LLM
  tool: string                  // Tool name
  state: ToolState
  metadata?: Record<string, any>
}

type ToolState =
  | ToolStatePending
  | ToolStateRunning
  | ToolStateCompleted
  | ToolStateError

interface ToolStatePending {
  status: "pending"
  input: Record<string, any>    // Partial input while streaming
  raw: string                   // Raw JSON input string
}

interface ToolStateRunning {
  status: "running"
  input: Record<string, any>
  title?: string                // Display title
  metadata?: Record<string, any>
  time: { start: number }
}

interface ToolStateCompleted {
  status: "completed"
  input: Record<string, any>
  output: string                // Tool result string
  title: string
  metadata: Record<string, any>
  time: {
    start: number
    end: number
    compacted?: number          // If output was compacted
  }
  attachments?: FilePart[]      // Return attachments
}

interface ToolStateError {
  status: "error"
  input: Record<string, any>
  error: string
  metadata?: Record<string, any>
  time: {
    start: number
    end: number
  }
}
```

### ReasoningPart

Extended thinking/reasoning blocks.

```typescript
interface ReasoningPart extends PartBase {
  type: "reasoning"
  text: string
  metadata?: Record<string, any>  // Provider-specific
  time: {
    start: number
    end?: number
  }
}
```

### AgentPart

Marks when a user explicitly invoked an agent via `@agent_name`.

```typescript
interface AgentPart extends PartBase {
  type: "agent"
  name: string                  // Agent name
  source?: {                    // Position in original text
    value: string
    start: number
    end: number
  }
}
```

### SubtaskPart

Marks a pending subtask execution (for Task tool).

```typescript
interface SubtaskPart extends PartBase {
  type: "subtask"
  prompt: string                // Task prompt
  description: string           // Short description
  agent: string                 // Subagent to use
  model?: {
    providerID: string
    modelID: string
  }
  command?: string              // Originating command
}
```

### CompactionPart

Marks a pending compaction request.

```typescript
interface CompactionPart extends PartBase {
  type: "compaction"
  auto: boolean                 // Auto-triggered vs manual
}
```

### StepStartPart / StepFinishPart

Marks boundaries of LLM generation steps.

```typescript
interface StepStartPart extends PartBase {
  type: "step-start"
  snapshot?: string             // Filesystem snapshot ID
}

interface StepFinishPart extends PartBase {
  type: "step-finish"
  reason: string                // Finish reason
  snapshot?: string
  cost: number                  // Step cost
  tokens: TokenUsage
}
```

### SnapshotPart / PatchPart

Filesystem change tracking.

```typescript
interface SnapshotPart extends PartBase {
  type: "snapshot"
  snapshot: string              // Snapshot ID
}

interface PatchPart extends PartBase {
  type: "patch"
  hash: string                  // Patch hash
  files: string[]               // Changed file paths
}
```

### RetryPart

Records retry attempts.

```typescript
interface RetryPart extends PartBase {
  type: "retry"
  attempt: number
  error: APIError
  time: { created: number }
}
```

---

## Error Types

### APIError

```typescript
interface APIError {
  name: "APIError"
  data: {
    message: string
    statusCode?: number
    isRetryable: boolean
    responseHeaders?: Record<string, string>
    responseBody?: string
    metadata?: Record<string, string>
  }
}
```

### AuthError

```typescript
interface AuthError {
  name: "ProviderAuthError"
  data: {
    providerID: string
    message: string
  }
}
```

### OutputLengthError

```typescript
interface OutputLengthError {
  name: "MessageOutputLengthError"
  data: {}
}
```

### AbortedError

```typescript
interface AbortedError {
  name: "MessageAbortedError"
  data: {
    message: string
  }
}
```

---

## Message to Model Conversion

The `toModelMessage()` function converts internal message format to AI SDK format.

### Key Transformations

1. **User text parts** → `{ type: "text", text: string }`

2. **User file parts** → `{ type: "file", url, mediaType, filename }` (except text/plain and directories)

3. **Compaction parts** → `{ type: "text", text: "What did we do so far?" }`

4. **Subtask parts** → `{ type: "text", text: "The following tool was executed by the user" }`

5. **Assistant text** → `{ type: "text", text, providerMetadata }`

6. **Tool completed** → `{ type: "tool-{name}", state: "output-available", toolCallId, input, output }`

7. **Tool error** → `{ type: "tool-{name}", state: "output-error", toolCallId, input, errorText }`

8. **Tool pending/running** → `{ type: "tool-{name}", state: "output-error", errorText: "[Tool execution was interrupted]" }`

9. **Reasoning** → `{ type: "reasoning", text, providerMetadata }`

10. **Tool attachments** → Injected as separate user message with file parts

### Messages with Errors

- Messages with errors are **excluded** from model context
- Exception: Aborted messages that have some content are included

---

## Storage Keys

Messages and parts are stored with these key patterns:

```
message/{sessionID}/{messageID}
part/{messageID}/{partID}
session/{projectID}/{sessionID}
```

Parts are sorted by ID (ascending) when retrieved.
