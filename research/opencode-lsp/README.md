# OpenCode LSP Subsystem Research

This directory contains reverse-engineered specifications of OpenCode's Language Server Protocol (LSP) integration subsystem.

## What This Research Covers

OpenCode implements a sophisticated LSP management layer that:
- Spawns and manages 40+ language servers automatically
- Downloads missing language servers from GitHub releases
- Exposes LSP capabilities to the AI agent through a unified tool interface
- Aggregates diagnostics from multiple servers

## Documents

| File | Description |
|------|-------------|
| `LSP-SUBSYSTEM-SPEC.md` | Complete architecture of the LSP management layer |
| `SERVER-CONFIGURATIONS.md` | All 40+ language server configs with spawn patterns |
| `LSP-TOOL-SPEC.md` | How OpenCode exposes LSP to the AI agent |
| `BRUTUS-LSP-IMPLEMENTATION-SPEC.md` | Proposed implementation for BRUTUS |

## Key Insights

1. **Auto-installation**: OpenCode automatically downloads missing language servers
2. **Project root detection**: Uses marker files (package.json, Cargo.toml) to find roots
3. **Unified API**: Single tool interface exposes all 9 LSP operations
4. **1-based indexing**: Tool uses editor-style 1-based coordinates
5. **Lazy spawning**: Servers start on first file access for that language

## Relationship to BRUTUS

BRUTUS currently has no LSP integration. This research provides the foundation for adding code intelligence capabilities like go-to-definition, find-references, and hover documentation.
