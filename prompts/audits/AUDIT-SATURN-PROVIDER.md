# Audit: Saturn Provider

**Beads Issue**: BRUTUS-bav
**Output File**: `docs/audits/saturn-provider.md`
**Parallel Safe**: Yes - no shared dependencies

---

## Context

You are auditing the Saturn provider for the BRUTUS v2 rebuild. BRUTUS is a coding agent that showcases Saturn (a local LLM inference server). Your job is to document what exists, what works, and what needs to change.

Saturn is the STAR of BRUTUS - we're keeping this code, just polishing it.

## Files to Read

Primary:
- `provider/saturn.go` - Main Saturn provider implementation
- `provider/provider.go` - Provider interface definitions

Supporting (skim for context):
- `provider/cache.go` - Response caching
- `provider/exec_windows.go` / `provider/exec_unix.go` - Process execution

Research (for comparison patterns):
- `research/opencode-provider/PROVIDER-SUBSYSTEM-SPEC.md`
- Note: We are NOT copying OpenCode's provider. Just look for patterns we might be missing.

## Questions to Answer

### 1. API Surface
- [ ] What is the Provider interface?
- [ ] What methods does SaturnProvider implement?
- [ ] What's the signature of Chat()? Does it support streaming?
- [ ] How are models listed and selected?
- [ ] Is there a health check / ping method?

### 2. Connection Lifecycle
- [ ] How does it discover/connect to Saturn?
- [ ] What happens when Saturn isn't available?
- [ ] Is there reconnection logic?
- [ ] How is the base URL configured?

### 3. Request/Response Flow
- [ ] How are messages formatted for Saturn?
- [ ] How are tool definitions passed?
- [ ] How are responses parsed?
- [ ] How are tool calls extracted from responses?

### 4. Error Handling
- [ ] What errors can occur?
- [ ] How are HTTP errors handled?
- [ ] How are parsing errors handled?
- [ ] Are errors actionable for the caller?

### 5. Streaming Support
- [ ] Is streaming implemented?
- [ ] If yes, how does it work?
- [ ] If no, is the infrastructure there to add it?

### 6. Caching
- [ ] What does the cache do?
- [ ] When is it used?
- [ ] Is it useful for the new architecture?

## Output Format

Create `docs/audits/saturn-provider.md` with this structure:

```markdown
# Saturn Provider Audit

**Date**: [today]
**Auditor**: [agent name]
**Issue**: BRUTUS-bav

## Executive Summary
[2-3 sentences: what works, what's the main gap]

## Current Capabilities

### API Surface
[List all public methods with signatures]

### What Works Well
- [bullet points of strengths]

### Connection Management
[How it connects, reconnects, handles failures]

## Gaps Identified

### Missing Features
- [ ] [Feature]: [why it matters]

### Code Quality Issues
- [ ] [Issue]: [recommendation]

### Architecture Concerns
- [ ] [Concern]: [impact]

## Recommendations

### Keep (Don't Touch)
- [things that work well]

### Polish (Minor Changes)
- [things that need small fixes]

### Rebuild (Significant Changes)
- [things that need rework]

## Proposed Interface

```go
type Provider interface {
    // [proposed interface based on findings]
}
```

## Questions for Human
- [any decisions that need human input]
```

## Success Criteria

- [ ] All questions answered with evidence from code
- [ ] Output file created at correct path
- [ ] Recommendations are actionable
- [ ] Interface proposal is concrete Go code

## Do NOT

- Do not modify any code
- Do not read files outside the provider/ directory (except research/)
- Do not wait for other audits to complete
- Do not make changes to beads issues
