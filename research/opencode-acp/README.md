# OpenCode ACP (Agent Client Protocol) Research

## Overview

This research reverse-engineers OpenCode's implementation of the Agent Client Protocol (ACP) - a standardized protocol enabling code editors (Zed, Neovim, JetBrains) to communicate with AI coding agents.

## Key Distinction

- **MCP (Model Context Protocol)**: Tools and resources for LLMs
- **ACP (Agent Client Protocol)**: Editor-to-agent communication layer

ACP sits *above* MCP - it defines how an IDE talks to an agent, while MCP defines how that agent talks to external tools/data sources.

## Research Artifacts

| Document | Description |
|----------|-------------|
| [ACP-PROTOCOL-SPEC.md](./ACP-PROTOCOL-SPEC.md) | Core protocol: JSON-RPC methods, message formats, lifecycle |
| [OPENCODE-ACP-IMPLEMENTATION.md](./OPENCODE-ACP-IMPLEMENTATION.md) | How OpenCode implements ACP (agent.ts analysis) |
| [BRUTUS-ACP-IMPLEMENTATION-SPEC.md](./BRUTUS-ACP-IMPLEMENTATION-SPEC.md) | Specification for implementing ACP in BRUTUS |

## Why This Matters for BRUTUS

Implementing ACP would allow BRUTUS to:
1. Integrate with Zed, Neovim, JetBrains, and other ACP clients
2. Provide real-time file access (including unsaved buffer state)
3. Execute commands in the IDE's terminal
4. Stream tool execution progress to the editor UI
5. Support permission requests with user interaction

## Protocol Sources

- Official Spec: https://github.com/agentclientprotocol/agent-client-protocol
- Documentation: https://agentclientprotocol.com
- OpenCode Implementation: `packages/opencode/src/acp/`
