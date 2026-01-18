# OpenCode Event Bus Research

This directory contains reverse-engineered specifications for OpenCode's internal event bus system.

## Files

| File | Description |
|------|-------------|
| `BUS-SUBSYSTEM-SPEC.md` | Core bus architecture and message flow |
| `EVENT-CATALOG.md` | Complete catalog of all known events |
| `BRUTUS-BUS-IMPLEMENTATION-SPEC.md` | Implementation spec for BRUTUS |

## Summary

The OpenCode bus system is a typed publish/subscribe event system with:
- Instance-scoped state management
- Global cross-instance event propagation
- Zod schema validation for all events
- SSE (Server-Sent Events) streaming to clients
- Automatic cleanup on instance disposal

## Key Insight

The bus bridges the gap between the internal state changes and external observers (UI, CLI) via SSE streaming. Events are scoped to instances (project directories) but can also propagate globally.
