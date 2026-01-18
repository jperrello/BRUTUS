# OpenCode Share Subsystem Research

This directory contains reverse-engineered specifications for OpenCode's session sharing system.

## Files

| File | Description |
|------|-------------|
| SHARE-SUBSYSTEM-SPEC.md | Complete architecture and protocol |
| SYNC-PROTOCOL-SPEC.md | Real-time WebSocket sync wire protocol |
| BRUTUS-SHARE-IMPLEMENTATION-SPEC.md | Implementation plan for BRUTUS |

## Overview

The Share subsystem enables users to create shareable links for their coding sessions. Others can view conversations in real-time via WebSocket synchronization.

Key characteristics:
- Secret-based authentication (no user accounts required for viewing)
- Real-time updates via Cloudflare Durable Objects + WebSockets
- Hierarchical data model: Session → Messages → Parts
- Queued sync with deduplication and debouncing

## Source Files Analyzed

- `packages/opencode/src/share/share.ts` (legacy)
- `packages/opencode/src/share/share-next.ts` (current)
- `packages/function/src/api.ts` (server-side handlers)
- `packages/opencode/src/session/index.ts` (integration points)
