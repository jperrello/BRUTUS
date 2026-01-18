# OpenCode Session Subsystem Research

**Status**: Complete
**Date**: 2026-01-17
**Researcher**: Claude Opus 4.5

## Overview

This research documents OpenCode's session management subsystem - the core infrastructure for managing conversational state, message persistence, and the agentic processing loop.

## Files Analyzed

| File | Purpose |
|------|---------|
| `packages/opencode/src/session/index.ts` | Session lifecycle, CRUD operations, forking |
| `packages/opencode/src/session/message-v2.ts` | Message and part type definitions |
| `packages/opencode/src/session/processor.ts` | Stream processing and tool execution loop |
| `packages/opencode/src/session/prompt.ts` | Prompt resolution, tool binding, main loop |
| `packages/opencode/src/storage/storage.ts` | JSON file-based persistence layer |
| `packages/opencode/src/id/id.ts` | Monotonic ID generation (ULID-like) |

## Research Artifacts

- [SESSION-SUBSYSTEM-SPEC.md](./SESSION-SUBSYSTEM-SPEC.md) - Complete session architecture specification
- [ID-GENERATION-SPEC.md](./ID-GENERATION-SPEC.md) - Monotonic ID algorithm details
- [BRUTUS-SESSION-IMPLEMENTATION-SPEC.md](./BRUTUS-SESSION-IMPLEMENTATION-SPEC.md) - Implementation roadmap for BRUTUS
