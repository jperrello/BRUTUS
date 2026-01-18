# OpenCode Permission Subsystem Research

Research into OpenCode's permission/authorization system for tool execution control.

## Files in This Research

| File | Description |
|------|-------------|
| `PERMISSION-SUBSYSTEM-SPEC.md` | Full specification of permission architecture |
| `WILDCARD-MATCHING-SPEC.md` | Pattern matching algorithm used for permissions |
| `BRUTUS-PERMISSION-IMPLEMENTATION-SPEC.md` | Implementation guide for BRUTUS |

## Key Findings

The permission system is a **hierarchical, pattern-based authorization layer** that:
1. Intercepts tool calls before execution
2. Evaluates against config-defined rules with wildcard patterns
3. Provides three response modes: `allow`, `deny`, `ask`
4. Supports "always" approval for pattern families (command arity)
5. Maintains per-session approval state

## Source Files Analyzed

- `packages/opencode/src/permission/index.ts` - Legacy permission system
- `packages/opencode/src/permission/next.ts` - Current permission system (v2)
- `packages/opencode/src/permission/arity.ts` - Bash command arity dictionary
- `packages/opencode/src/util/wildcard.ts` - Pattern matching
- `packages/opencode/src/config/config.ts` - Permission schema definitions
- `packages/opencode/src/tool/bash.ts` - Permission integration example
