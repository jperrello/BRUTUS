# Running Saturn Audits in Parallel

## Overview

Three independent audits that can run simultaneously:

| Audit | Prompt File | Output File | Beads Issue |
|-------|-------------|-------------|-------------|
| Saturn Provider | `prompts/audits/AUDIT-SATURN-PROVIDER.md` | `docs/audits/saturn-provider.md` | BRUTUS-bav |
| Saturn Discovery | `prompts/audits/AUDIT-SATURN-DISCOVERY.md` | `docs/audits/saturn-discovery.md` | BRUTUS-cr2 |
| Saturn Multi-Agent | `prompts/audits/AUDIT-SATURN-MULTIAGENT.md` | `docs/audits/saturn-multiagent.md` | BRUTUS-bak |

## Parallel Execution

These audits are designed to run in parallel with NO conflicts:

- Each reads different files (minimal overlap)
- Each writes to different output files
- No shared state or dependencies
- Can be run by 3 separate agents simultaneously

## How to Run

### Option 1: Three Claude Sessions

Open 3 terminal windows and run:

```bash
# Terminal 1
claude "Read prompts/audits/AUDIT-SATURN-PROVIDER.md and execute the audit"

# Terminal 2
claude "Read prompts/audits/AUDIT-SATURN-DISCOVERY.md and execute the audit"

# Terminal 3
claude "Read prompts/audits/AUDIT-SATURN-MULTIAGENT.md and execute the audit"
```

### Option 2: Subagents (if supported)

```bash
claude "Run these 3 audits in parallel using subagents:
1. prompts/audits/AUDIT-SATURN-PROVIDER.md
2. prompts/audits/AUDIT-SATURN-DISCOVERY.md
3. prompts/audits/AUDIT-SATURN-MULTIAGENT.md"
```

### Option 3: Background Tasks

```bash
# Start all three in background
claude --background "Execute prompts/audits/AUDIT-SATURN-PROVIDER.md"
claude --background "Execute prompts/audits/AUDIT-SATURN-DISCOVERY.md"
claude --background "Execute prompts/audits/AUDIT-SATURN-MULTIAGENT.md"
```

## After Audits Complete

1. Check all outputs exist:
```bash
ls docs/audits/
# Should show: saturn-provider.md, saturn-discovery.md, saturn-multiagent.md
```

2. Update beads issues:
```bash
bd close BRUTUS-bav BRUTUS-cr2 BRUTUS-bak
```

3. Move to next phase:
```bash
bd update BRUTUS-h3g --status=in_progress
# Now: Define Saturn Interface Contract (synthesizes all 3 audits)
```

## Troubleshooting

**Agent can't find files?**
- Make sure working directory is BRUTUS root
- Use absolute paths if needed

**Outputs conflict?**
- They shouldn't - each writes to separate file
- If issues, run sequentially instead

**One audit fails?**
- Others can continue - they're independent
- Fix and re-run failed audit only
