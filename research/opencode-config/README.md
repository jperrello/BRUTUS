# OpenCode Configuration Subsystem Research

This research covers OpenCode's configuration loading, merging, and validation system.

## Files Analyzed
- `packages/opencode/src/config/config.ts` (~1200 lines)
- `packages/opencode/src/config/markdown.ts` (~90 lines)
- `packages/opencode/src/flag/flag.ts` (~50 lines)
- `packages/opencode/src/global/index.ts` (~50 lines)

## Key Findings

1. **Hierarchical Configuration Loading** - 6-layer precedence system
2. **JSONC Support** - Comments allowed in config files
3. **Variable Interpolation** - `{env:VAR}` and `{file:path}` syntax
4. **Markdown-Based Extensions** - Agents, commands, modes loaded from `.md` files
5. **Plugin Deduplication** - Local plugins override global by name
6. **Zod Schema Validation** - Comprehensive type safety with migration support

## Specifications

| File | Description |
|------|-------------|
| [CONFIG-SUBSYSTEM-SPEC.md](./CONFIG-SUBSYSTEM-SPEC.md) | Core configuration architecture |
| [LOADING-ALGORITHM.md](./LOADING-ALGORITHM.md) | Precedence and merge logic |
| [BRUTUS-CONFIG-IMPLEMENTATION-SPEC.md](./BRUTUS-CONFIG-IMPLEMENTATION-SPEC.md) | BRUTUS implementation plan |
