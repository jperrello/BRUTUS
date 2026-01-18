# OpenCode Skill Subsystem Research

Research into OpenCode's Skill discovery, loading, and execution subsystem.

## Files

| File | Purpose |
|------|---------|
| SKILL-SUBSYSTEM-SPEC.md | Complete architecture and data flow |
| SKILL-FILE-FORMAT.md | SKILL.md file format specification |
| BRUTUS-SKILL-IMPLEMENTATION-SPEC.md | Implementation plan for BRUTUS |

## Summary

The Skill subsystem provides a mechanism for users to extend agent behavior through markdown files with YAML frontmatter. Skills are discovered at startup from multiple locations, exposed as a tool to the LLM, and expanded on-demand when invoked.

Key insight: Skills are NOT system prompt injections. They're a **tool** that loads instructions on-demand, reducing context window pressure while enabling extensibility.
