# OpenCode Snapshot Subsystem Research

This research focuses on OpenCode's **snapshot subsystem** - a Git-based state tracking mechanism that enables file change detection, revert capabilities, and diff generation within agent sessions.

## Research Scope

- **Primary Source**: `packages/opencode/src/snapshot/index.ts` (6.5KB)
- **Integration Points**: Session processor, revert system
- **Purpose**: Understand how OpenCode tracks filesystem state for undo/revert operations

## Key Findings

1. **Shadow Git Repository**: Snapshots use an isolated Git repo separate from the project's VCS
2. **Tree-hash Based**: Uses `git write-tree` instead of commits for lightweight snapshots
3. **Per-Step Tracking**: Snapshots are taken at the start of each agent "step" (tool execution boundary)
4. **Patch Generation**: Changes are captured as patches linking hash→files for revert operations

## Specifications

| Document | Description |
|----------|-------------|
| [SNAPSHOT-SUBSYSTEM-SPEC.md](./SNAPSHOT-SUBSYSTEM-SPEC.md) | Core architecture and API |
| [GIT-SHADOW-REPO-SPEC.md](./GIT-SHADOW-REPO-SPEC.md) | Shadow Git repository mechanics |
| [BRUTUS-SNAPSHOT-IMPLEMENTATION-SPEC.md](./BRUTUS-SNAPSHOT-IMPLEMENTATION-SPEC.md) | Implementation guide for BRUTUS |

## Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                    Session Processor                     │
│  ┌─────────┐   ┌─────────┐   ┌─────────┐   ┌─────────┐ │
│  │  start  │──▶│ track() │──▶│ execute │──▶│ patch() │ │
│  │  step   │   │         │   │  tools  │   │         │ │
│  └─────────┘   └────┬────┘   └─────────┘   └────┬────┘ │
└──────────────────────│──────────────────────────│──────┘
                       │                          │
                       ▼                          ▼
              ┌────────────────┐         ┌──────────────┐
              │ Shadow Git Repo │         │ Patch Record │
              │ ~/.local/share/ │         │ {hash,files} │
              │ opencode/       │         └──────────────┘
              │ snapshot/{id}   │
              └────────────────┘
```
