# OpenCode Patch Subsystem Research

**Status**: Complete
**Date**: 2026-01-17
**Researcher**: Claude Opus 4.5

## Overview

This research documents OpenCode's patch subsystem - a unified diff-like system for applying multi-file changes atomically. The patch system is a **deprecated but still present** mechanism that predates the current `edit`/`multiedit` tools.

## Files Analyzed

| File | Purpose |
|------|---------|
| `packages/opencode/src/patch/index.ts` | Core patch parsing and application logic |
| `packages/opencode/src/tool/patch.ts` | Tool wrapper exposing patch to LLM |
| `packages/opencode/src/tool/patch.txt` | Tool description (contains only "do not use") |
| `packages/opencode/src/tool/multiedit.ts` | Current preferred multi-edit mechanism |

## Key Finding

The patch tool is **explicitly disabled** (`patch.txt` contains "do not use"). OpenCode has moved to `edit` + `multiedit` as the preferred editing mechanism. However, the patch subsystem remains in the codebase and provides insight into:

1. A structured patch format (non-unified-diff)
2. Context-aware line matching algorithms
3. Atomic multi-file operations with rollback semantics

## Research Artifacts

- [PATCH-SUBSYSTEM-SPEC.md](./PATCH-SUBSYSTEM-SPEC.md) - Complete architecture specification
- [PATCH-FORMAT-SPEC.md](./PATCH-FORMAT-SPEC.md) - Wire format for patch text
- [LINE-MATCHING-ALGORITHM.md](./LINE-MATCHING-ALGORITHM.md) - Fuzzy line matching details
- [BRUTUS-PATCH-IMPLEMENTATION-SPEC.md](./BRUTUS-PATCH-IMPLEMENTATION-SPEC.md) - Implementation roadmap
