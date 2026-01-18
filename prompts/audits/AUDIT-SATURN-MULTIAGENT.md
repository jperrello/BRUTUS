# Audit: Saturn Pool & Multi-Agent

**Beads Issue**: BRUTUS-bak
**Output File**: `docs/audits/saturn-multiagent.md`
**Parallel Safe**: Yes - no shared dependencies

---

## Context

You are auditing the Saturn pool and multi-agent coordination system for the BRUTUS v2 rebuild. This is a key differentiator - BRUTUS can coordinate multiple Saturn instances for parallel agent execution.

This shows Saturn can do things cloud APIs can't easily do: true local multi-agent with no rate limits.

## Files to Read

Primary:
- `provider/saturn_pool.go` - Pool management for multiple Saturn instances
- `coordinator/coordinator.go` - Multi-agent orchestration

Supporting:
- `provider/saturn.go` - Single instance provider (pool wraps this)
- `sdk/multi_agent.go` - Multi-agent SDK
- `sdk/live_multi_agent.go` - Live multi-agent implementation
- `gui_agent.go` - GUI multi-agent usage

Research (for comparison):
- `research/opencode-agent-loop/AGENT-LOOP-SPEC.md` - See subagent section
- Note: OpenCode spawns subagents on same provider. BRUTUS can use MULTIPLE providers.

## Questions to Answer

### 1. Pool Architecture
- [ ] How does the pool manage multiple Saturn instances?
- [ ] How are instances added to the pool?
- [ ] How are instances selected for work?
- [ ] Is there load balancing?
- [ ] How is instance health tracked?

### 2. Coordinator Architecture
- [ ] What does the coordinator do?
- [ ] How does it orchestrate multiple agents?
- [ ] How do agents communicate/coordinate?
- [ ] Is there shared state between agents?
- [ ] How are tasks distributed?

### 3. Multi-Agent Patterns
- [ ] What multi-agent patterns are supported?
- [ ] Parallel execution (same task, different instances)?
- [ ] Pipeline execution (output of one → input of next)?
- [ ] Collaborative execution (agents working together)?

### 4. Failure Handling
- [ ] What happens when one instance fails?
- [ ] Is there failover?
- [ ] How are partial results handled?
- [ ] Can work be redistributed?

### 5. SDK Usage
- [ ] How do SDK users create multi-agent setups?
- [ ] What's the API surface?
- [ ] Are there good examples?

### 6. GUI Integration
- [ ] How does the GUI show multiple agents?
- [ ] Can users see agent coordination?
- [ ] Is the UX clear?

## Output Format

Create `docs/audits/saturn-multiagent.md` with this structure:

```markdown
# Saturn Multi-Agent Audit

**Date**: [today]
**Auditor**: [agent name]
**Issue**: BRUTUS-bak

## Executive Summary
[2-3 sentences: multi-agent capabilities, main strengths/gaps]

## Architecture Overview

### Pool Manager
```
[ASCII diagram of pool architecture]
```

### Coordinator
```
[ASCII diagram of coordinator flow]
```

## Capabilities

### Supported Patterns
| Pattern | Implemented | Notes |
|---------|-------------|-------|
| Parallel | ?           |       |
| Pipeline | ?           |       |
| Collaborative | ?      |       |

### Instance Management
- How instances are discovered/added
- How instances are selected
- Health checking approach

## What Works Well
- [strengths]

## Gaps Identified

### Architecture Issues
- [ ] [Issue]: [impact]

### Missing Patterns
- [ ] [Pattern]: [why it matters]

### Reliability Issues
- [ ] [Issue]: [impact]

## Recommendations

### Keep (Don't Touch)
- [working well]

### Polish (Minor Changes)
- [small fixes]

### Rebuild (Significant Changes)
- [rework needed]

## Proposed Interfaces

### Pool Interface
```go
type Pool interface {
    // [proposed interface]
}
```

### Coordinator Interface
```go
type Coordinator interface {
    // [proposed interface]
}
```

## Demo Scenario
[How to demonstrate multi-agent working - for testing/demos]

## Questions for Human
- [decisions needed]
```

## Success Criteria

- [ ] Pool architecture documented
- [ ] Coordinator flow documented
- [ ] Multi-agent patterns identified
- [ ] Output file created

## Do NOT

- Do not modify any code
- Do not read files outside provider/, coordinator/, sdk/ directories (except research/)
- Do not wait for other audits
- Do not actually run multi-agent code
