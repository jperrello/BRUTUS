# BRUTUS

A coding agent powered by [Saturn](https://github.com/jperrello/Saturn) - zero-config AI on your network.

Based on [Geoffrey Huntley's "How to Build a Coding Agent"](https://ghuntley.com/agent/) workshop.

## What is This?

BRUTUS is an educational coding agent that demonstrates the core concepts behind tools like Claude Code, Cursor, and Aider. It's designed to be:

- **Understandable**: Clean, well-documented code showing exactly how agents work
- **Extensible**: Easy to add new tools and capabilities
- **Network-native**: Uses Saturn for automatic AI discovery - no API keys needed

## Requirements

- A Saturn server running on your network
- Go 1.21+

That's it. No API keys. No configuration. If you're on a network with Saturn, BRUTUS just works.

## The Core Insight

A coding agent is **300 lines of code running in a loop with LLM tokens**:

```
1. Get user input
2. Send to LLM
3. If LLM wants to use a tool → execute it → go to 2
4. Show response → go to 1
```

That's the entire architecture. Everything else is just tools (what it can do) and prompts (how it behaves).

## Quick Start

```bash
# Make sure you have a Saturn server running on your network
# See: https://github.com/jperrello/Saturn

# Build
go build

# Run
./brutus
```

If no Saturn server is found, BRUTUS will tell you:
```
Error: no saturn services found on network

BRUTUS requires a Saturn server on your network.
Start a Saturn beacon or server, then try again.
See: https://github.com/jperrello/Saturn
```

## Project Structure

```
brutus/
├── agent/           # THE LOOP - the core thing to understand
│   └── agent.go     # ~150 lines, heavily commented
├── tools/           # What BRUTUS can DO
│   ├── tool.go      # Tool abstraction
│   ├── read.go      # Read files
│   ├── list.go      # List directories
│   ├── bash.go      # Execute commands
│   ├── edit.go      # Modify files
│   └── search.go    # Code search (ripgrep)
├── provider/        # Where the LLM comes from
│   ├── provider.go  # Provider interface
│   ├── discovery.go # Saturn mDNS discovery
│   └── saturn.go    # OpenAI-compatible client
├── examples/        # Progressive learning stages
│   ├── 01-chat/     # Simple chatbot
│   ├── 02-read/     # Add first tool
│   └── 03-multi/    # Multiple tools
├── main.go          # Entry point
├── LEARNING.md      # How it all works
└── ADDING_TOOLS.md  # How to extend
```

## How Saturn Works

When BRUTUS starts, it:
1. Searches for `_saturn._tcp.local.` services via mDNS
2. Picks the highest priority (lowest number) server
3. Gets ephemeral credentials from beacon TXT records
4. Uses OpenAI-compatible API to talk to the server

This means **network presence = AI access**. No API keys to manage.

## Learning Path

1. **Start here**: Read `examples/01-chat/main.go` - a simple chatbot
2. **Add a tool**: Read `examples/02-read-tool/main.go` - introducing tools
3. **Multiple tools**: Read `examples/03-multi-tool/main.go` - tools working together
4. **The real thing**: Read `agent/agent.go` - the full implementation
5. **Extend it**: Read `ADDING_TOOLS.md` - add your own capabilities

## Options

| Flag | Description | Default |
|------|-------------|---------|
| `-verbose` | Detailed logging | false |
| `-model` | Model to request | (server default) |
| `-max-tokens` | Max response tokens | 8192 |
| `-timeout` | Discovery timeout | 5s |
| `-version` | Print version | - |

## Why Saturn-Only?

BRUTUS is designed for networks where Saturn provides AI access. Benefits:

- **Zero config**: No API keys to manage
- **Shared access**: Multiple tools share one subscription
- **Ephemeral credentials**: Keys rotate automatically
- **Network-scoped**: Leave the network, lose access

If you need direct API access, look at the original [ghuntley workshop](https://github.com/ghuntley/how-to-build-a-coding-agent).

## Credits

- Original workshop: [ghuntley/how-to-build-a-coding-agent](https://github.com/ghuntley/how-to-build-a-coding-agent)
- Article: [ghuntley.com/agent](https://ghuntley.com/agent/)
- Saturn: [jperrello/Saturn](https://github.com/jperrello/Saturn)

## License

MIT
