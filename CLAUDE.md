# BRUTUS - Saturn-Powered Coding Agent

## Quick Architecture
- **THE LOOP**: `agent/agent.go` - The core inference loop (read this first)
- **Tools**: `tools/*.go` - Each file = one capability (read, list, bash, edit, search)
- **Provider**: `provider/saturn.go` - Saturn discovery + OpenAI-compat client
- **Entry**: `main.go` - Wires everything together

## Adding a New Tool
1. Create `tools/mytool.go` with input struct + function
2. Export `var MyTool = NewTool[MyInput]("name", "desc", MyFunc)`
3. Register in `main.go`: `registry.Register(tools.MyTool)`

## Key Files
| File | What You Learn |
|------|----------------|
| `agent/agent.go` | THE LOOP - start here |
| `tools/tool.go` | Tool abstraction |
| `sdk/` | Testing SDK for programmatic testing |
| `LEARNING.md` | Full explanation |
| `ADDING_TOOLS.md` | How to extend |

## Testing SDK (How to Test Features)

The `sdk/` package and `brutus-test` CLI let you test BRUTUS features programmatically without the UI.

### Direct Tool Execution
```bash
# List available tools
./brutus-test.exe tools

# Execute any tool with JSON input
./brutus-test.exe tool read_file '{"path": "main.go"}'
./brutus-test.exe tool list_files '{"path": "sdk", "recursive": false}'
./brutus-test.exe tool code_search '{"pattern": "func New", "path": "."}'
./brutus-test.exe tool bash '{"command": "go version"}'
./brutus-test.exe tool edit_file '{"path": "test.txt", "old_str": "", "new_str": "hello"}'
```

### Run Test Scenarios
```bash
./brutus-test.exe scenario testdata/read-scenario.json
```

Scenario files define mock LLM responses and assertions:
```json
{
  "name": "Test Name",
  "user_messages": ["What's in main.go?"],
  "mock_responses": [
    {"tool_call": "read_file", "input": {"path": "main.go"}},
    {"content": "The file contains..."}
  ],
  "assertions": [
    {"type": "tool_called", "value": "read_file"}
  ]
}
```

### Go Tests
```bash
go test ./sdk/... -v
```

### SDK Components
| Component | Purpose |
|-----------|---------|
| `sdk.ToolRunner` | Execute tools directly with JSON |
| `sdk.MockProvider` | Queue deterministic LLM responses |
| `sdk.TestHarness` | Run full agent loops with mocks |

## Specialized Commands
| Command | Purpose |
|---------|---------|
| `/brutus-orient` | Quick orientation for new session |
| `/brutus-research` | Research external concepts, produce plan |
| `/brutus-implementer` | Implement features with codebase context |
| `/brutus-debug` | Diagnose when something's broken |
| `/brutus-sendoff` | End session, create handoffs |

---

## Beads Quick Reference
```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --status in_progress  # Claim work
bd close <id>         # Complete work
bd sync               # Sync with git
```

## Landing the Plane
When ending a work session ("land the plane"), complete these steps:
1. **File issues** - `bd create` for remaining work
2. **Quality gates** - Build, vet, test if code changed
3. **Update status** - Close finished work
4. **ASK HUMAN TO COMMIT** - Run `/commit`

Offer a handoff prompt for the next agent, or run `/brutus-sendoff`.
