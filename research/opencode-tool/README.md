# OpenCode Tool Subsystem Research

This research covers the **Tool Subsystem** - the core abstraction layer that defines how all tools are structured, registered, invoked, and managed in OpenCode.

## Documents

| Document | Purpose |
|----------|---------|
| `TOOL-SUBSYSTEM-SPEC.md` | Complete spec of the tool abstraction and lifecycle |
| `TOOL-REGISTRY-SPEC.md` | How tools are discovered, registered, and resolved |
| `TRUNCATION-ALGORITHM.md` | Output truncation system for large results |
| `FUZZY-REPLACEMENT-ALGORITHM.md` | Edit tool's multi-strategy fuzzy matching |
| `BRUTUS-TOOL-IMPLEMENTATION-SPEC.md` | Implementation guidance for BRUTUS |

## Key Findings

1. **Lazy Initialization**: Tools use async `init()` for deferred setup, allowing agent-context-aware descriptions
2. **Zod Validation**: All parameters validated with Zod schemas, custom error formatters supported
3. **Automatic Truncation**: Output auto-truncated at 2000 lines / 50KB unless tool handles it
4. **Permission Model**: Tools request permissions via `ctx.ask()` with pattern/always rules
5. **Plugin System**: Custom tools loaded from `{tool,tools}/*.{js,ts}` in config directories
6. **Batch Execution**: Experimental tool allows parallel execution of up to 10 tools
7. **Metadata Streaming**: Tools can push real-time metadata updates via `ctx.metadata()`

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     ToolRegistry                            │
├─────────────────────────────────────────────────────────────┤
│  ┌────────────┐  ┌────────────┐  ┌────────────┐            │
│  │ Built-in   │  │ Plugin     │  │ Custom     │            │
│  │ Tools      │  │ Tools      │  │ Tools      │            │
│  └─────┬──────┘  └─────┬──────┘  └─────┬──────┘            │
│        │               │               │                    │
│        └───────────────┼───────────────┘                    │
│                        ▼                                    │
│                 ┌────────────┐                              │
│                 │ Tool.Info  │ ← Lazy init + Zod schema     │
│                 └─────┬──────┘                              │
│                       │                                     │
│                       ▼                                     │
│                 ┌────────────┐                              │
│                 │ execute()  │ → Permission → Run → Result  │
│                 └─────┬──────┘                              │
│                       │                                     │
│                       ▼                                     │
│                 ┌────────────┐                              │
│                 │ Truncate   │ → Output processing          │
│                 └────────────┘                              │
└─────────────────────────────────────────────────────────────┘
```

## Source Files Analyzed

- `packages/opencode/src/tool/tool.ts` - Core abstraction
- `packages/opencode/src/tool/registry.ts` - Tool discovery and registration
- `packages/opencode/src/tool/truncation.ts` - Output truncation
- `packages/opencode/src/tool/edit.ts` - Fuzzy replacement algorithms
- `packages/opencode/src/tool/bash.ts` - Complex permission handling
- `packages/opencode/src/tool/task.ts` - Subagent spawning
- `packages/opencode/src/tool/batch.ts` - Parallel tool execution
- `packages/opencode/src/tool/read.ts` - File reading with binary detection
- `packages/opencode/src/tool/external-directory.ts` - Path security checks
