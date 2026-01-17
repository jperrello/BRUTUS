# BRUTUS Architecture

BRUTUS is a **local-first AI coding agent** with a desktop GUI.

## Layer Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                    PRESENTATION LAYER                        │
│  ┌─────────────────┐    ┌────────────────────────────────┐  │
│  │   Wails v2 GUI   │    │   CLI (cmd/cli/main.go)        │  │
│  │   (main.go)      │    │   Alternative entry point      │  │
│  └────────┬────────┘    └────────────────────────────────┘  │
│           │                                                  │
│  ┌────────▼────────┐                                        │
│  │   App (app.go)   │  Session management, Wails bindings   │
│  └────────┬────────┘                                        │
└───────────┼─────────────────────────────────────────────────┘
            │
┌───────────▼─────────────────────────────────────────────────┐
│                      AGENT LAYER                             │
│  ┌─────────────────────────────────────────────────────────┐│
│  │              agent/agent.go - THE LOOP                  ││
│  │                                                          ││
│  │   1. Get user input                                      ││
│  │   2. Send to LLM (Provider)                             ││
│  │   3. Check for tool calls                               ││
│  │   4. Execute tools → loop back to step 3               ││
│  │   5. Return response → loop back to step 1             ││
│  └─────────────────────────────────────────────────────────┘│
│  ┌───────────────┐  ┌───────────────┐                       │
│  │ agent/input.go│  │ agent/picker.go│  Input & UI helpers  │
│  └───────────────┘  └───────────────┘                       │
└─────────────────────────────────────────────────────────────┘
            │                           │
            ▼                           ▼
┌─────────────────────────┐   ┌─────────────────────────────┐
│     PROVIDER LAYER       │   │        TOOLS LAYER          │
│                          │   │                             │
│  provider/provider.go    │   │  tools/tool.go (Registry)   │
│  ├── Interface:          │   │  ├── read.go   (read files) │
│  │   Chat()              │   │  ├── list.go   (list dirs)  │
│  │   ChatStream()        │   │  ├── bash.go   (run cmds)   │
│  │   ListModels()        │   │  ├── edit.go   (edit files) │
│  │   SetModel()          │   │  └── search.go (grep/find)  │
│  │                       │   │                             │
│  provider/saturn.go      │   │  Each tool:                 │
│  ├── Saturn provider     │   │  - Input struct             │
│  ├── OpenAI-compat API   │   │  - JSON schema (auto-gen)   │
│  ├── SSE streaming       │   │  - Execute function         │
│  │                       │   │                             │
│  provider/discovery.go   │   └─────────────────────────────┘
│  └── mDNS/DNS-SD         │
│      _saturn._tcp        │
└─────────────────────────┘
            │
            ▼
┌─────────────────────────────────────────────────────────────┐
│                 MULTI-AGENT COORDINATION                     │
│                                                              │
│  coordinator/coordinator.go    tools/broadcast.go            │
│  ├── AgentStatus               ├── agent_broadcast (tool)    │
│  ├── AgentMessage              ├── observe_agents (tool)     │
│  ├── Broadcast()/SendMessage() └── File + TXT record modes   │
│  └── DiscoverAgents()                                        │
│                                                              │
│  sdk/multi_agent.go            sdk/live_multi_agent.go       │
│  ├── MultiAgentHarness         ├── LiveMultiAgentHarness     │
│  ├── RunSequential()           ├── Real Saturn LLM calls     │
│  ├── RunConcurrent()           └── Concurrent agent runs     │
│  └── Mock LLM responses                                      │
│                                                              │
│  Communication via:                                          │
│  • File-based: /tmp/brutus-agents/*.json (default)          │
│  • Network: _brutus-agent._tcp TXT records (use_txt=true)   │
└─────────────────────────────────────────────────────────────┘
```

## Key Architectural Patterns

| Pattern | Implementation | Purpose |
|---------|----------------|---------|
| **ReAct Loop** | `agent/agent.go` | Reason-Act cycle: think → tool → observe → repeat |
| **Provider Abstraction** | `provider.Provider` interface | Swap backends without changing agent logic |
| **Zero-Config Discovery** | `discovery.go` via mDNS | Find Saturn services on LAN automatically |
| **Tool Registry** | `tools.Registry` | Plug-in capability system with JSON schema |
| **Wails Hybrid** | `main.go` + `frontend/` | Go backend + embedded web frontend |
| **Multi-Agent Coord** | `coordinator/` + `tools/broadcast.go` | Agent-to-agent status sharing via files or mDNS |

## What Makes It Interesting

1. **Saturn Discovery**: Uses `dns-sd` (Bonjour) to find AI services on the local network. Services advertise via `_saturn._tcp` with TXT records containing ephemeral API keys. No config files needed.

2. **OpenAI-Compatible Wire Protocol**: The Saturn provider speaks OpenAI's `/v1/chat/completions` format, so it works with any compatible backend (Ollama, LM Studio, OpenRouter, etc.).

3. **Streaming First**: `ChatStream()` returns a channel for real-time token streaming via SSE.

4. **Type-Safe Tools**: Uses Go generics + reflection to auto-generate JSON schemas from structs:
   ```go
   var ReadTool = NewTool[ReadInput]("read_file", "...", readFunc)
   ```

5. **Desktop-Native**: Wails embeds a webview with the frontend, giving you a native window with web UI flexibility.

## Data Flow Example

```
User types "read main.go"
       │
       ▼
   App.SendMessage()  →  GUIAgent.SendMessage()
       │
       ▼
   Agent.Run()  →  provider.ChatStream()
       │
       ▼
   Saturn discovers service via mDNS
   Saturn calls POST /v1/chat/completions
       │
       ▼
   LLM returns: { tool_calls: [{ name: "read_file", input: {path: "main.go"} }] }
       │
       ▼
   Agent executes: tools.Registry.Get("read_file").Function(input)
       │
       ▼
   File contents sent back to LLM as tool result
       │
       ▼
   LLM responds with summary
       │
       ▼
   Streamed to GUI via Wails events
```

## Core Files

| File | Purpose |
|------|---------|
| `agent/agent.go` | THE LOOP - start here |
| `tools/tool.go` | Tool abstraction + registry |
| `tools/broadcast.go` | Multi-agent communication tools |
| `provider/provider.go` | Provider interface |
| `provider/saturn.go` | Saturn + OpenAI-compat client |
| `provider/discovery.go` | mDNS service discovery |
| `coordinator/coordinator.go` | Multi-agent coordination via mDNS |
| `sdk/multi_agent.go` | Test harness for multi-agent scenarios |
| `app.go` | Wails app bindings |
| `main.go` | Entry point, wires everything |

## Multi-Agent Coordination

BRUTUS supports multiple agents working concurrently on shared resources:

```
AGENT-1 (Editor)              AGENT-2 (Editor)
┌──────────────┐              ┌──────────────┐
│ Edits        │              │ Edits        │
│ mock1.txt    │              │ mock2.txt    │
│              │              │              │
│ Broadcasts   │              │ Broadcasts   │
│ via TXT      │              │ via TXT      │
└──────────────┘              └──────────────┘
       │                             │
       └─────────────┬───────────────┘
                     │
                     ▼
           AGENT-3 (Observer)
           ┌──────────────────────────┐
           │ Discovers agents via     │
           │ Saturn mDNS TXT records  │
           │                          │
           │ Reads workspace files    │
           │ Provides suggestions     │
           └──────────────────────────┘
```

**Communication Methods:**

1. **File-based** (default): Agents write status to `/tmp/brutus-agents/*.json`
2. **Network-based** (use_txt=true): Agents broadcast via `_brutus-agent._tcp` TXT records

**Tools:**
- `agent_broadcast`: Announce status to other agents
- `observe_agents`: Discover and read other agent statuses

**Testing:**
```bash
# Mocked scenario (no network required)
./brutus-test multi-agent testdata/multi-agent/multi-scenario.json

# Live scenario (requires Saturn beacon)
./brutus-test live-multi-agent testdata/multi-agent/live-scenario.json
```

## Summary

BRUTUS is a thin ReAct loop wired to a zero-config provider discovery system, with a Wails desktop shell and multi-agent coordination via mDNS. The core is ~300 lines in `agent/agent.go`. Everything else is plumbing.
