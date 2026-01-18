# Fuzzy Replacement Algorithm Specification

## Overview

The edit tool's power comes from its cascade of 9 fuzzy replacement strategies. When the LLM provides an `oldString`, it's rarely an exact match - indentation might be wrong, whitespace might differ, or escape sequences might be literal instead of interpreted.

The `replace()` function tries each replacer in order. The first one to produce a valid match wins.

## The Replacer Interface

```typescript
type Replacer = (content: string, find: string) => Generator<string, void, unknown>
```

Each replacer is a **generator function** that yields candidate matches. This allows:
1. Multiple potential matches to be considered
2. Early termination when a valid match is found
3. Memory-efficient processing of large files

## Algorithm: replace()

```typescript
function replace(content, oldString, newString, replaceAll = false): string {
  if (oldString === newString) throw Error("must be different")

  let notFound = true

  for (const replacer of REPLACERS) {
    for (const search of replacer(content, oldString)) {
      const index = content.indexOf(search)
      if (index === -1) continue

      notFound = false

      if (replaceAll) {
        return content.replaceAll(search, newString)
      }

      // Check for uniqueness
      const lastIndex = content.lastIndexOf(search)
      if (index !== lastIndex) continue  // Multiple matches, try next

      // Single unique match - apply replacement
      return content.substring(0, index) + newString + content.substring(index + search.length)
    }
  }

  if (notFound) throw Error("oldString not found in content")
  throw Error("Found multiple matches...")
}
```

## Replacer Order (Critical)

```typescript
const REPLACERS = [
  SimpleReplacer,              // 1. Exact match
  LineTrimmedReplacer,         // 2. Line-by-line trim
  BlockAnchorReplacer,         // 3. First/last anchors + fuzzy middle
  WhitespaceNormalizedReplacer,// 4. Collapse whitespace
  IndentationFlexibleReplacer, // 5. Remove common indent
  EscapeNormalizedReplacer,    // 6. Interpret escape sequences
  TrimmedBoundaryReplacer,     // 7. Trim boundaries only
  ContextAwareReplacer,        // 8. Anchor match + 50% middle
  MultiOccurrenceReplacer      // 9. All occurrences (for replaceAll)
]
```

---

## Replacer 1: SimpleReplacer

**Purpose**: Exact string match. No fuzzy logic.

```typescript
function* SimpleReplacer(_content, find) {
  yield find
}
```

**Behavior**: Yields the search string unchanged. The caller's `indexOf()` determines if it exists.

**When it matches**: LLM provided exactly correct text.

---

## Replacer 2: LineTrimmedReplacer

**Purpose**: Match content by trimmed line comparison.

**Algorithm**:
```
1. Split content into lines
2. Split search into lines (pop trailing empty line)
3. For each position in content where search could start:
   a. Compare each line (trimmed) to corresponding search line (trimmed)
   b. If all lines match trimmed, compute exact byte range of match
   c. Yield the exact substring from content
```

**Pseudocode**:
```
for i = 0 to (contentLines.length - searchLines.length):
  match = true
  for j = 0 to searchLines.length:
    if contentLines[i+j].trim() != searchLines[j].trim():
      match = false
      break
  if match:
    yield exact_substring_at_position(i, searchLines.length)
```

**When it matches**: LLM got the content right but indentation wrong.

**Example**:
```
Content:    "    if (x) {\n        return;\n    }"
Search:     "if (x) {\n    return;\n}"
Result:     "    if (x) {\n        return;\n    }"
```

---

## Replacer 3: BlockAnchorReplacer

**Purpose**: Match blocks by first/last line anchors, with fuzzy middle content.

**Algorithm**:
```
1. Need at least 3 lines
2. Find all positions where first line matches (trimmed)
3. For each, find nearest position where last line matches (trimmed)
4. Calculate similarity of middle lines using Levenshtein
5. Single candidate: accept if similarity >= 0.0 (always)
6. Multiple candidates: accept best if similarity >= 0.3
```

**Levenshtein Function**:
```typescript
function levenshtein(a: string, b: string): number {
  // Classic DP implementation
  // Returns minimum edit distance
}
```

**Similarity Calculation**:
```
for each middle line:
  distance = levenshtein(contentLine.trim(), searchLine.trim())
  lineSimilarity = 1 - (distance / max(len1, len2))
  totalSimilarity += lineSimilarity / numMiddleLines
```

**Constants**:
```typescript
SINGLE_CANDIDATE_SIMILARITY_THRESHOLD = 0.0   // Accept anything
MULTIPLE_CANDIDATES_SIMILARITY_THRESHOLD = 0.3 // Need 30% similarity
```

**When it matches**: Middle content has typos/variations but boundaries are correct.

---

## Replacer 4: WhitespaceNormalizedReplacer

**Purpose**: Collapse all whitespace to single spaces before comparison.

**Algorithm**:
```
normalize(text) = text.replace(/\s+/g, ' ').trim()

For single-line matches:
  if normalize(line) == normalize(find):
    yield line
  else if normalize(line).includes(normalize(find)):
    // Build regex from words, match original
    pattern = words.join('\\s+')
    match = line.match(pattern)
    if match: yield match[0]

For multi-line matches:
  for each block of findLines.length:
    if normalize(block.join('\n')) == normalize(find):
      yield block.join('\n')
```

**When it matches**: Extra spaces, tabs vs spaces, newline differences.

**Example**:
```
Content:    "function  foo(   a,b  ) {}"
Search:     "function foo(a, b) {}"
Normalized: "function foo( a,b ) {}"
```

---

## Replacer 5: IndentationFlexibleReplacer

**Purpose**: Strip minimum common indentation before comparison.

**Algorithm**:
```
removeIndentation(text):
  lines = text.split('\n')
  nonEmpty = lines.filter(line => line.trim().length > 0)
  minIndent = min(leading_whitespace for each nonEmpty line)
  return lines.map(line =>
    line.trim().length == 0 ? line : line.slice(minIndent)
  ).join('\n')

For each block in content:
  if removeIndentation(block) == removeIndentation(find):
    yield block
```

**When it matches**: Entire block has wrong base indentation but relative indentation is correct.

**Example**:
```
Content:    "        function foo() {\n            return;\n        }"
Search:     "function foo() {\n    return;\n}"
Dedented:   "function foo() {\n    return;\n}"
```

---

## Replacer 6: EscapeNormalizedReplacer

**Purpose**: Handle literal escape sequences vs interpreted ones.

**Escape Mappings**:
```
\\n  -> \n  (newline)
\\t  -> \t  (tab)
\\r  -> \r  (carriage return)
\\'  -> '
\\"  -> "
\\`  -> `
\\\\  -> \
\\\n -> \n  (escaped actual newline)
\\$  -> $
```

**Algorithm**:
```
unescape(str) = str.replace(/\\(n|t|r|'|"|`|\\|\n|\$)/g, mapper)

unescapedFind = unescape(find)

// Direct match
if content.includes(unescapedFind):
  yield unescapedFind

// Block match
for each block:
  if unescape(block) == unescapedFind:
    yield block
```

**When it matches**: LLM wrote `\\n` when it meant an actual newline.

---

## Replacer 7: TrimmedBoundaryReplacer

**Purpose**: Match when LLM added/removed whitespace at boundaries.

**Algorithm**:
```
trimmedFind = find.trim()

// Skip if already trimmed (SimpleReplacer would handle)
if trimmedFind == find:
  return

// Direct match
if content.includes(trimmedFind):
  yield trimmedFind

// Block match
for each block:
  if block.trim() == trimmedFind:
    yield block
```

**When it matches**: Extra leading/trailing whitespace in search string.

---

## Replacer 8: ContextAwareReplacer

**Purpose**: First/last line anchors with 50% middle line match requirement.

**Algorithm**:
```
// Similar to BlockAnchorReplacer but:
// 1. Requires exact line count match
// 2. Requires 50% of middle lines to match exactly (trimmed)

for i where contentLines[i].trim() == firstLine.trim():
  for j where contentLines[j].trim() == lastLine.trim():
    block = contentLines[i..j+1]
    if block.length != findLines.length:
      break  // Wrong size, next j

    matchingLines = 0
    for k = 1 to block.length - 1:
      if block[k].trim() == findLines[k].trim():
        matchingLines++

    if matchingLines / middleLineCount >= 0.5:
      yield block.join('\n')
```

**When it matches**: Context lines are correct, some middle content varies.

---

## Replacer 9: MultiOccurrenceReplacer

**Purpose**: Find ALL exact occurrences (for `replaceAll` mode).

**Algorithm**:
```
startIndex = 0
while true:
  index = content.indexOf(find, startIndex)
  if index == -1:
    break
  yield find
  startIndex = index + find.length
```

**When it matches**: Used with `replaceAll=true` to replace every occurrence.

---

## Summary Table

| # | Replacer | Matches When... | Complexity |
|---|----------|-----------------|------------|
| 1 | Simple | Exact match | O(n) |
| 2 | LineTrimmed | Indentation differs | O(n*m) |
| 3 | BlockAnchor | Boundaries correct, middle fuzzy | O(n*m) + Levenshtein |
| 4 | WhitespaceNormalized | Spacing differs | O(n*m) |
| 5 | IndentationFlexible | Common indent wrong | O(n*m) |
| 6 | EscapeNormalized | Escape sequences literal | O(n) |
| 7 | TrimmedBoundary | Leading/trailing space | O(n*m) |
| 8 | ContextAware | Anchors + 50% middle match | O(n*m) |
| 9 | MultiOccurrence | All occurrences needed | O(n) |

## Design Implications for BRUTUS

1. **Generator pattern is elegant but optional** - Go can use slices of candidate strings
2. **Levenshtein is required** - Implement or use library
3. **Order matters** - Simple match should always be tried first
4. **Threshold constants are important** - 0.0 for single, 0.3 for multiple
5. **Trimming is everywhere** - Most replacers use trimmed comparison
