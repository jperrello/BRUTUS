# Multi-Agent Coordination Demo

This directory demonstrates BRUTUS's multi-agent coordination capabilities where multiple agents work concurrently on shared resources using Saturn for discovery and communication.

## Quick Start

**Easiest way** - use the demo runner:

```bash
# Windows PowerShell
.\testdata\multi-agent\run-demo.ps1

# Linux/Mac
./testdata/multi-agent/run-demo.sh

# Live mode (requires Saturn beacon)
.\testdata\multi-agent\run-demo.ps1 -Live -Verbose
./testdata/multi-agent/run-demo.sh --live --verbose
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    MULTI-AGENT WORKSPACE                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   AGENT-1 (Editor)              AGENT-2 (Editor)                │
│   ┌──────────────┐              ┌──────────────┐                │
│   │ Edits        │              │ Edits        │                │
│   │ mock1.txt    │              │ mock2.txt    │                │
│   │              │              │              │                │
│   │ Broadcasts   │              │ Broadcasts   │                │
│   │ via TXT      │              │ via TXT      │                │
│   └──────────────┘              └──────────────┘                │
│          │                             │                        │
│          └─────────────┬───────────────┘                        │
│                        │                                        │
│                        ▼                                        │
│              AGENT-3 (Observer)                                 │
│              ┌──────────────────────────┐                       │
│              │ Discovers agents via     │                       │
│              │ Saturn mDNS TXT records  │                       │
│              │                          │                       │
│              │ Reads workspace files    │                       │
│              │ Provides suggestions     │                       │
│              └──────────────────────────┘                       │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Files

| File | Purpose |
|------|---------|
| mock1.txt | Agent 1's workspace file |
| mock2.txt | Agent 2's workspace file |
| status/agent-1.md | Agent 1's status (file-based fallback) |
| status/agent-2.md | Agent 2's status (file-based fallback) |
| multi-scenario.json | Mocked multi-agent scenario |
| live-scenario.json | Live multi-agent scenario (real LLM) |
| run-demo.ps1 | Demo runner (Windows PowerShell) |
| run-demo.sh | Demo runner (Linux/Mac) |
| reset.ps1 | Reset files to initial state (Windows) |
| reset.sh | Reset files to initial state (Linux/Mac) |

## Running the Demo

### Using Demo Runner Scripts (Recommended)

The demo runners automatically reset files, build brutus-test, and run the scenario.

**Windows PowerShell:**
```powershell
# Mocked mode (no network required)
.\testdata\multi-agent\run-demo.ps1

# Live mode with real LLM via Saturn
.\testdata\multi-agent\run-demo.ps1 -Live

# All options
.\testdata\multi-agent\run-demo.ps1 -Live -Verbose -MaxTurns 15 -Timeout 10 -Model "gpt-4"
```

**Linux/Mac:**
```bash
# Mocked mode
./testdata/multi-agent/run-demo.sh

# Live mode
./testdata/multi-agent/run-demo.sh --live

# All options
./testdata/multi-agent/run-demo.sh --live --verbose --max-turns 15 --timeout 10 --model gpt-4
```

### Manual Execution

**Step 1: Reset demo files**
```bash
# Windows
.\testdata\multi-agent\reset.ps1

# Linux/Mac
./testdata/multi-agent/reset.sh
```

**Step 2: Run with brutus-test**

Mocked (no network required):
```bash
./brutus-test multi-agent testdata/multi-agent/multi-scenario.json
```

Live (requires Saturn beacon):
```bash
./brutus-test live-multi-agent -v testdata/multi-agent/live-scenario.json
```

### Command-Line Options

| Option | Description |
|--------|-------------|
| -concurrent | Run agents concurrently (default: true) |
| -v | Verbose output |
| -timeout N | Saturn discovery timeout in seconds (default: 5) |
| -max-turns N | Maximum turns per agent (default: 10) |
| -model NAME | Model to use (optional) |

## Communication Methods

### 1. File-based coordination (default)
- Agents write status to JSON files in temp directory
- Works without network
- Good for testing and offline use

### 2. Saturn TXT Record coordination (use_txt=true)
- Agents broadcast status via mDNS TXT records
- Uses `_brutus-agent._tcp` service type
- Real-time discovery on local network
- Other agents can discover status without reading files

**How TXT Record Communication Works:**

1. When an agent calls `agent_broadcast` with `use_txt=true`:
   - A zeroconf mDNS service is registered
   - TXT records contain: agent_id, status, task, action, updated timestamp
   - Service type: `_brutus-agent._tcp`

2. When an observer calls `observe_agents` with `use_txt=true`:
   - mDNS browse discovers all `_brutus-agent._tcp` services
   - TXT records are parsed to get agent statuses
   - Returns JSON array of discovered agents

## Agent Tools

| Tool | Description |
|------|-------------|
| agent_broadcast | Broadcast agent status (file or TXT record) |
| observe_agents | Discover and read other agent statuses |

### agent_broadcast Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| agent_id | string | Your agent identifier (required) |
| status | string | Current status: idle/working/done (required) |
| task | string | Current task description |
| action | string | Last action taken |
| message | string | Optional message to other agents |
| use_txt | bool | Use Saturn TXT records (default: false) |

### observe_agents Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| status_dir | string | Directory containing agent status files |
| use_txt | bool | Use Saturn TXT records (default: false) |
| timeout | int | Discovery timeout in seconds (default: 2) |

## Testing

Run the multi-agent tests:
```bash
go test ./sdk/... -v -run Multi
```

## Troubleshooting

**"no saturn services found"**
- Ensure a Saturn beacon is running on the network
- Try increasing the timeout: `-timeout 10`

**Agents not discovering each other via TXT records**
- Verify mDNS/Bonjour is working on your network
- Check firewall allows UDP port 5353
- Try file-based mode as fallback

**Files not resetting**
- Run the reset script manually before the demo
- Check file permissions
