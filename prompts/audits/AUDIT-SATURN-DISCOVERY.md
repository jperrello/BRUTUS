# Audit: Saturn Discovery System

**Beads Issue**: BRUTUS-cr2
**Output File**: `docs/audits/saturn-discovery.md`
**Parallel Safe**: Yes - no shared dependencies

---

## Context

You are auditing the Saturn discovery system for the BRUTUS v2 rebuild. This is one of BRUTUS's killer features - the ability to automatically find Saturn instances on the local network using mDNS/Zeroconf.

This is MAGIC for users: start BRUTUS, it finds Saturn automatically. No configuration needed.

## Files to Read

Primary:
- `provider/discovery.go` - Main discovery orchestration
- `provider/discovery_zeroconf.go` - mDNS/Zeroconf implementation
- `provider/discovery_legacy.go` - Fallback discovery methods

Supporting:
- `provider/saturn.go` - How discovery results are used

Research (for comparison):
- `research/opencode-server/SERVER-SUBSYSTEM-SPEC.md` - See mDNS section
- Note: OpenCode also uses mDNS for discovery. Compare approaches.

## Questions to Answer

### 1. Discovery Methods
- [ ] What discovery methods are implemented?
- [ ] What's the priority/fallback order?
- [ ] How does Zeroconf discovery work?
- [ ] What's the legacy fallback?
- [ ] Is there manual configuration option?

### 2. Platform Support
- [ ] Does it work on Windows?
- [ ] Does it work on macOS?
- [ ] Does it work on Linux?
- [ ] Are there platform-specific code paths?

### 3. Network Behavior
- [ ] What mDNS service type does Saturn advertise?
- [ ] How long does discovery take?
- [ ] What happens with multiple Saturn instances?
- [ ] How are instances differentiated?

### 4. Error Handling
- [ ] What happens when no Saturn is found?
- [ ] What's the timeout behavior?
- [ ] Are errors user-friendly?
- [ ] Is there retry logic?

### 5. UX Flow
- [ ] How does the user know discovery is happening?
- [ ] How are results presented?
- [ ] Can the user select from multiple instances?

### 6. Reliability
- [ ] How reliable is Zeroconf in practice?
- [ ] What are known failure modes?
- [ ] Are there race conditions?

## Output Format

Create `docs/audits/saturn-discovery.md` with this structure:

```markdown
# Saturn Discovery Audit

**Date**: [today]
**Auditor**: [agent name]
**Issue**: BRUTUS-cr2

## Executive Summary
[2-3 sentences: how discovery works, main strengths/gaps]

## Discovery Architecture

### Methods (in priority order)
1. [Method 1]: [how it works]
2. [Method 2]: [how it works]
...

### Flow Diagram
```
[ASCII diagram of discovery flow]
```

## Platform Support

| Platform | Zeroconf | Legacy | Notes |
|----------|----------|--------|-------|
| Windows  | ?        | ?      |       |
| macOS    | ?        | ?      |       |
| Linux    | ?        | ?      |       |

## What Works Well
- [strengths]

## Gaps Identified

### Reliability Issues
- [ ] [Issue]: [impact]

### UX Issues
- [ ] [Issue]: [impact]

### Missing Features
- [ ] [Feature]: [why it matters]

## Recommendations

### Keep (Don't Touch)
- [working well]

### Polish (Minor Changes)
- [small fixes]

### Rebuild (Significant Changes)
- [rework needed]

## Proposed Discovery Interface

```go
type Discoverer interface {
    // [proposed interface]
}
```

## Demo Script
[How to demonstrate discovery working - for testing]

## Questions for Human
- [decisions needed]
```

## Success Criteria

- [ ] All discovery methods documented
- [ ] Platform matrix completed
- [ ] Flow diagram included
- [ ] Output file created

## Do NOT

- Do not modify any code
- Do not read files outside provider/ directory (except research/)
- Do not wait for other audits
- Do not test network operations (just read code)
