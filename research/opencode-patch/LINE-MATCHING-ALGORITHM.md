# OpenCode Patch Line Matching Algorithm

## Overview

The patch system uses a **sequential seek algorithm** to locate change positions without requiring explicit line numbers. This document specifies the exact matching behavior.

## Algorithm: `seekSequence`

```
FUNCTION seekSequence(lines[], pattern[], startIndex) -> int
  IF pattern is empty THEN RETURN -1

  FOR i FROM startIndex TO (lines.length - pattern.length)
    matches = TRUE

    FOR j FROM 0 TO pattern.length - 1
      IF lines[i + j] != pattern[j] THEN
        matches = FALSE
        BREAK
      END IF
    END FOR

    IF matches THEN RETURN i
  END FOR

  RETURN -1  // Not found
END FUNCTION
```

### Key Properties

1. **Exact String Match**: Lines must match character-for-character
2. **Forward-Only**: Search starts at `startIndex` and moves forward
3. **Contiguous**: All pattern lines must be consecutive in source
4. **First Match**: Returns first occurrence, not closest/best

## Algorithm: `computeReplacements`

```
FUNCTION computeReplacements(originalLines[], filePath, chunks[]) -> replacements[]
  replacements = []
  lineIndex = 0

  FOR EACH chunk IN chunks
    // Phase 1: Context seeking
    IF chunk.change_context EXISTS THEN
      contextIdx = seekSequence(originalLines, [chunk.change_context], lineIndex)
      IF contextIdx == -1 THEN
        THROW "Failed to find context"
      END IF
      lineIndex = contextIdx + 1  // Move past context line
    END IF

    // Phase 2: Handle pure additions
    IF chunk.old_lines is empty THEN
      insertionIdx = IF last line is empty
                     THEN lines.length - 1
                     ELSE lines.length
      replacements.ADD(insertionIdx, 0, chunk.new_lines)
      CONTINUE
    END IF

    // Phase 3: Pattern matching with retry
    pattern = chunk.old_lines
    newSlice = chunk.new_lines
    found = seekSequence(originalLines, pattern, lineIndex)

    // Retry without trailing empty line
    IF found == -1 AND pattern ends with empty string THEN
      pattern = pattern[0..length-2]  // Remove last
      IF newSlice ends with empty string THEN
        newSlice = newSlice[0..length-2]
      END IF
      found = seekSequence(originalLines, pattern, lineIndex)
    END IF

    IF found != -1 THEN
      replacements.ADD(found, pattern.length, newSlice)
      lineIndex = found + pattern.length
    ELSE
      THROW "Failed to find expected lines"
    END IF
  END FOR

  // Sort by index for ordered application
  SORT replacements BY index
  RETURN replacements
END FUNCTION
```

## Algorithm: `applyReplacements`

Replacements are applied in **reverse order** to avoid index shifting:

```
FUNCTION applyReplacements(lines[], replacements[]) -> newLines[]
  result = COPY(lines)

  FOR i FROM replacements.length - 1 DOWNTO 0
    [startIdx, oldLen, newSegment] = replacements[i]

    // Remove old lines
    result.SPLICE(startIdx, oldLen)

    // Insert new lines
    FOR j FROM 0 TO newSegment.length - 1
      result.INSERT(startIdx + j, newSegment[j])
    END FOR
  END FOR

  RETURN result
END FUNCTION
```

## Trailing Newline Handling

Special logic handles the common case of trailing newlines:

1. **On Load**: If original file ends with `\n`, the empty string after split is dropped
2. **On Match Failure**: If pattern ends with empty string, retry without it
3. **On Save**: Ensure output ends with exactly one newline

## Example Walkthrough

**Original File:**
```
1: import { foo } from './foo'
2: import { bar } from './bar'
3:
4: export function main() {
5:   foo()
6:   bar()
7: }
```

**Patch:**
```
*** Update File: example.ts
@@ import { bar } from './bar'
-import { bar } from './bar'
+import { bar } from './bar'
+import { baz } from './baz'
*** End Patch
```

**Execution:**

1. Context seek: Find `import { bar } from './bar'` at line 2
2. Set `lineIndex = 3` (past context)
3. Pattern: `["import { bar } from './bar'"]`
4. seekSequence from index 3... not found (line 2 already passed!)
5. **This is a bug/limitation**: Context and pattern overlap

**Correct Patch:**
```
@@ import { foo } from './foo'
 import { foo } from './foo'
-import { bar } from './bar'
+import { bar } from './bar'
+import { baz } from './baz'
```

Now context points to line BEFORE the change target.

## Edge Cases

### Empty File Addition
```
old_lines: []
new_lines: ["first line", "second line"]
```
Insertion at index 0.

### Complete File Replacement
Match all lines, replace with all new lines.

### Overlapping Changes
Replacements sorted by index; reverse application prevents interference.

### Whitespace Sensitivity
Full whitespace sensitivity - indentation must match exactly.

## Limitations

1. **No Fuzzy Matching**: Exact string equality only
2. **No Best-Match Selection**: First match wins, even if suboptimal
3. **Context Consumption**: Context line advances position, limiting backtrack
4. **Whitespace Rigidity**: Single space difference causes failure

## Comparison with Edit Tool

| Aspect | Patch Line Matching | Edit Tool |
|--------|--------------------|----|
| Match Type | Exact contiguous | Exact substring |
| Context | Explicit `@@` marker | Implicit (first occurrence) |
| Multi-line | Native support | Native support |
| Failure Mode | Exception | Exception |
| Uniqueness | Depends on pattern length | Requires unique match |
