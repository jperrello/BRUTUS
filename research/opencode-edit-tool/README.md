# OpenCode Edit Tool Research

**Researcher**: Claude Agent (Opus 4.5)
**Date**: 2026-01-17
**Subject**: File editing tool with fuzzy string replacement - Deep reverse engineering

---

## What Was Researched

Comprehensive analysis of OpenCode's `edit` tool - the most critical capability for a coding agent. This tool enables precise file modifications through a sophisticated fuzzy string matching system with 9 different replacement strategies.

## Files in This Directory

| File | Description |
|------|-------------|
| `EDIT-TOOL-SPEC.md` | Complete specification of the edit tool architecture, parameters, execution flow |
| `FUZZY-REPLACEMENT-SPEC.md` | Deep dive into the 9 fuzzy replacement algorithms with pseudocode |
| `BRUTUS-EDIT-IMPLEMENTATION-SPEC.md` | Go implementation specification for BRUTUS |

## Key Findings

### Design Philosophy

OpenCode's edit tool is NOT a simple string replace. It employs a **cascade of 9 fuzzy matchers** that try increasingly flexible matching strategies until one succeeds. This compensates for common LLM mistakes:

1. Wrong indentation
2. Extra/missing whitespace
3. Incorrect line endings
4. Escape sequence confusion
5. Partial matches

### Attribution

The tool explicitly credits three sources:
- Cline's diff-apply algorithms
- Google Gemini CLI's editCorrector
- Additional Cline diff refinements

### The 9 Replacement Strategies (in order)

| # | Replacer | What It Does |
|---|----------|--------------|
| 1 | SimpleReplacer | Exact string match |
| 2 | LineTrimmedReplacer | Matches by trimmed line content |
| 3 | BlockAnchorReplacer | Uses first/last lines as anchors, fuzzy middle |
| 4 | WhitespaceNormalizedReplacer | Collapses all whitespace to single spaces |
| 5 | IndentationFlexibleReplacer | Strips common indentation prefix |
| 6 | EscapeNormalizedReplacer | Handles `\n`, `\t`, `\\` etc. |
| 7 | TrimmedBoundaryReplacer | Matches trimmed start/end boundaries |
| 8 | ContextAwareReplacer | First/last line match with 50% middle similarity |
| 9 | MultiOccurrenceReplacer | Finds ALL occurrences (used with replaceAll) |

### Key Parameters

```typescript
{
  filePath: string,    // Absolute path to file
  oldString: string,   // Text to replace (fuzzy matched)
  newString: string,   // Replacement text (exact)
  replaceAll?: boolean // Replace all occurrences (default false)
}
```

### Safety Mechanisms

1. **File Lock** - Promise-based queue prevents concurrent writes
2. **Read Assertion** - Must read file before editing
3. **Modification Detection** - Checks if file changed since read
4. **LSP Diagnostics** - Reports syntax errors after edit
5. **Permission Request** - Asks user before applying changes

### Levenshtein Distance

The BlockAnchorReplacer uses **Levenshtein distance** for fuzzy middle-line matching:
- Single candidate: threshold 0.0 (always accept)
- Multiple candidates: threshold 0.3 (30% similarity required)

### Output Truncation

Tool outputs are capped at:
- 2,000 lines
- 50KB

Full output is written to disk if truncated, with instructions to access it.

## What Was NOT Researched

- Other tools (bash, grep, read, write, etc.)
- The write tool (creates new files)
- Multi-edit batching tool
- Agent loop integration points
- UI rendering of diffs

## Next Steps for Future Agents

1. **Implement SimpleReplacer** - Direct port, trivial
2. **Implement LineTrimmedReplacer** - Line-by-line trim comparison
3. **Implement BlockAnchorReplacer** - Most complex, needs Levenshtein
4. **Add file locking** - Mutex per file path
5. **Add read tracking** - Session-scoped read timestamps
6. **Later**: Remaining replacers, LSP integration

## Source Files Analyzed

- `packages/opencode/src/tool/edit.ts` (~500 lines)
- `packages/opencode/src/tool/tool.ts` (~150 lines)
- `packages/opencode/src/tool/truncation.ts` (~100 lines)
- `packages/opencode/src/file/time.ts` (~80 lines)
- `packages/opencode/src/tool/edit.txt` (description)
