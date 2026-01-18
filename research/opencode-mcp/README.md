# OpenCode MCP Research

**Researcher**: Claude Agent
**Date**: 2026-01-17
**Subject**: Model Context Protocol (MCP) client implementation in OpenCode

---

## What Was Researched

Deep analysis of OpenCode's MCP subsystem located at `packages/opencode/src/mcp/`. This is the protocol that enables AI agents to communicate with external tool servers.

## Files in This Directory

| File | Description |
|------|-------------|
| `MCP-SUBSYSTEM-SPEC.md` | Complete specification of OpenCode's MCP implementation - architecture, data structures, state management, OAuth flow |
| `MCP-WIRE-PROTOCOL.md` | Wire protocol analysis - JSON-RPC messages, transport details, content types |
| `BRUTUS-MCP-IMPLEMENTATION-SPEC.md` | Go implementation specification for BRUTUS - types, interfaces, code patterns |

## Key Findings

### Architecture
- Uses `@modelcontextprotocol/sdk` official SDK
- Supports local (stdio) and remote (HTTP/SSE) transports
- Full OAuth 2.0 + PKCE for remote server authentication
- Event-driven with notification handlers

### Key Constants
```
OAuth Callback Port: 19876
OAuth Callback Path: /mcp/oauth/callback
Default Timeout: 30,000ms
OAuth Timeout: 5 minutes
```

### Tool Naming Convention
```
{sanitized_client_name}_{sanitized_tool_name}
```
Sanitization replaces non-alphanumeric (except `_-`) with `_`

### Status States
```
connected | disabled | failed | needs_auth | needs_client_registration
```

## What Was NOT Researched

- OpenCode's agent loop
- OpenCode's tool implementations
- UI/TUI components
- Other packages (console, desktop, etc.)

## Next Steps for Future Agents

1. **Implement basic MCP**: Start with stdio transport only
2. **Add tool discovery**: Parse `tools/list` response
3. **Add tool invocation**: Implement `tools/call`
4. **Test with real server**: Use `@modelcontextprotocol/server-everything`
5. **Later**: Add HTTP transport and OAuth

## Source Verification

All analysis derived from:
- https://github.com/anomalyco/opencode
- Branch: `dev`
- Files: `packages/opencode/src/mcp/*.ts`
