# OpenCode Agent Loop Research

**Researcher**: Claude Agent
**Date**: 2026-01-17
**Subject**: Core inference loop and session processing reverse engineering

---

## What Was Researched

Deep analysis of OpenCode's agent/session subsystem - the beating heart that processes messages, executes tool calls, and manages the inference loop. This is the most critical component for any coding agent.

## Files in This Directory

| File | Description |
|------|-------------|
| `AGENT-LOOP-SPEC.md` | Complete specification of the inference loop architecture |
| `MESSAGE-STRUCTURES.md` | All message and part types with Zod schemas |
| `BRUTUS-IMPLEMENTATION-SPEC.md` | Go implementation specification for BRUTUS |

## Key Findings

### Architecture Overview

OpenCode's agent loop is split across multiple files:
- `session/prompt.ts` - Entry point, message creation, the main `loop()` function
- `session/processor.ts` - Stream event processing, tool execution orchestration
- `session/llm.ts` - LLM API interaction, system prompt construction
- `session/message-v2.ts` - Message and part type definitions

### The Loop Flow

```
User Input → prompt() → loop() → processor.process() → LLM.stream() → Tool Execution → Continue/Stop
```

### Critical Constants

```typescript
DOOM_LOOP_THRESHOLD = 3          // Tool repetition detection
OUTPUT_TOKEN_MAX = 32_000        // Default max output tokens
```

### Built-in Agents

| Agent | Mode | Purpose |
|-------|------|---------|
| build | primary | Full development access |
| plan | primary | Read-only analysis mode |
| general | subagent | Multi-step task execution |
| explore | subagent | Fast codebase exploration |
| compaction | primary/hidden | Context summarization |
| title | primary/hidden | Session title generation |
| summary | primary/hidden | Message summarization |

### Message Part Types (12 total)

1. `text` - Plain text content
2. `file` - File attachments (images, code)
3. `agent` - Agent invocation markers
4. `tool` - Tool call tracking (pending/running/completed/error)
5. `reasoning` - Extended thinking blocks
6. `subtask` - Subtask execution markers
7. `compaction` - Context compaction markers
8. `step-start` / `step-finish` - Step boundary markers
9. `snapshot` / `patch` - File change tracking
10. `retry` - Retry attempt tracking

### Tool State Machine

```
pending → running → completed
                 → error
```

### Key Mechanisms

1. **Doom Loop Detection**: If the same tool with identical input is called 3+ times consecutively, asks for permission to continue

2. **Context Compaction**: When token count exceeds model limits, triggers automatic summarization

3. **Snapshot/Patch Tracking**: Creates filesystem snapshots before tool execution, generates patches after

4. **Permission System**: Flexible rule-based permissions with pattern matching

5. **Plugin Hooks**: Extensible via `Plugin.trigger()` at key points:
   - `tool.execute.before`
   - `tool.execute.after`
   - `experimental.chat.messages.transform`
   - `chat.message`
   - `chat.params`

### Session Status States

```typescript
type SessionStatus =
  | { type: "idle" }
  | { type: "busy" }
  | { type: "retry", attempt: number, message: string, next: number }
```

### Tool Call Repair

OpenCode includes automatic tool call repair - if a tool name doesn't match but a lowercase version exists, it auto-corrects.

## What Was NOT Researched

- Tool implementations (individual tools)
- UI/TUI rendering
- Storage layer internals
- Plugin system internals
- Compaction algorithm details
- Authentication flows

## Next Steps for Future Agents

1. **Port the loop structure**: Implement Go version of the `loop()` → `process()` flow
2. **Message types**: Create Go structs matching MessageV2 schema
3. **Tool state machine**: Implement pending → running → completed/error
4. **Doom loop detection**: Simple counter-based repetition check
5. **Later**: Context compaction, plugin hooks

## Source Files Analyzed

- `packages/opencode/src/session/prompt.ts` (~1800 lines)
- `packages/opencode/src/session/processor.ts` (~400 lines)
- `packages/opencode/src/session/llm.ts` (~250 lines)
- `packages/opencode/src/session/message-v2.ts` (~700 lines)
- `packages/opencode/src/agent/agent.ts` (~250 lines)
