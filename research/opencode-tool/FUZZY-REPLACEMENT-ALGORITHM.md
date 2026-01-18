# OpenCode Edit Tool Fuzzy Replacement Algorithm

## Overview

The Edit tool uses a multi-strategy fuzzy matching system to handle imprecise oldString inputs from the LLM. This makes edits more robust against whitespace variations, indentation differences, and minor formatting inconsistencies.

## Replacer Interface

Each strategy is a generator function:

```typescript
export type Replacer = (content: string, find: string) => Generator<string, void, unknown>
```

Yields candidate strings that could match the `find` parameter.

## Replacement Cascade

Strategies are tried in order until one succeeds:

```typescript
for (const replacer of [
  SimpleReplacer,           // Exact match
  LineTrimmedReplacer,      // Trimmed line comparison
  BlockAnchorReplacer,      // First/last line anchors with fuzzy middle
  WhitespaceNormalizedReplacer,  // Collapse whitespace
  IndentationFlexibleReplacer,   // Remove indentation
  EscapeNormalizedReplacer,      // Handle escape sequences
  TrimmedBoundaryReplacer,       // Trim leading/trailing whitespace
  ContextAwareReplacer,          // Context anchors (similar to BlockAnchor)
  MultiOccurrenceReplacer,       // Find all exact matches
]) {
  for (const search of replacer(content, oldString)) {
    const index = content.indexOf(search)
    if (index === -1) continue
    // ... handle match
  }
}
```

## Strategy Details

### 1. SimpleReplacer (Exact Match)

```typescript
export const SimpleReplacer: Replacer = function* (_content, find) {
  yield find
}
```

Just yields the search string unchanged. Relies on `content.indexOf(search)` for exact match.

### 2. LineTrimmedReplacer

Matches lines by trimmed content, preserving original whitespace:

```typescript
for (let i = 0; i <= originalLines.length - searchLines.length; i++) {
  let matches = true
  for (let j = 0; j < searchLines.length; j++) {
    const originalTrimmed = originalLines[i + j].trim()
    const searchTrimmed = searchLines[j].trim()
    if (originalTrimmed !== searchTrimmed) {
      matches = false
      break
    }
  }
  if (matches) {
    yield content.substring(matchStartIndex, matchEndIndex)
  }
}
```

Use case: LLM adds/removes leading spaces in oldString.

### 3. BlockAnchorReplacer

Uses first and last lines as anchors, with Levenshtein similarity for middle content:

```typescript
const SINGLE_CANDIDATE_SIMILARITY_THRESHOLD = 0.0
const MULTIPLE_CANDIDATES_SIMILARITY_THRESHOLD = 0.3

// Find blocks where first and last lines match (trimmed)
const firstLineSearch = searchLines[0].trim()
const lastLineSearch = searchLines[searchLines.length - 1].trim()

// For each candidate, compute similarity of middle lines
for (let j = 1; j < searchBlockSize - 1 && j < actualBlockSize - 1; j++) {
  const distance = levenshtein(originalLine, searchLine)
  similarity += (1 - distance / maxLen) / linesToCheck
}
```

Key points:
- Requires at least 3 lines
- Single candidate: accepts with 0% similarity (just anchors)
- Multiple candidates: requires 30% middle similarity
- Returns best match for multiple candidates

### 4. WhitespaceNormalizedReplacer

Collapses all whitespace to single spaces:

```typescript
const normalizeWhitespace = (text: string) => text.replace(/\s+/g, " ").trim()
```

Handles:
- Multiple spaces → single space
- Tabs → space
- Newlines in unexpected places

For single-line matches, also tries regex pattern matching:
```typescript
const pattern = words.map((word) =>
  word.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")
).join("\\s+")
const match = line.match(regex)
```

### 5. IndentationFlexibleReplacer

Removes common indentation from both sides:

```typescript
const removeIndentation = (text: string) => {
  const lines = text.split("\n")
  const nonEmptyLines = lines.filter((line) => line.trim().length > 0)
  const minIndent = Math.min(
    ...nonEmptyLines.map((line) => line.match(/^(\s*)/)[1].length)
  )
  return lines.map((line) =>
    line.trim().length === 0 ? line : line.slice(minIndent)
  ).join("\n")
}
```

Use case: LLM consistently wrong about indentation level.

### 6. EscapeNormalizedReplacer

Handles escape sequence variations:

```typescript
const unescapeString = (str: string): string => {
  return str.replace(/\\(n|t|r|'|"|`|\\|\n|\$)/g, (match, capturedChar) => {
    switch (capturedChar) {
      case "n": return "\n"
      case "t": return "\t"
      // ...
    }
  })
}
```

Tries both:
1. Unescaping the search string
2. Finding escaped content that matches unescaped search

### 7. TrimmedBoundaryReplacer

Just trims the search string:

```typescript
const trimmedFind = find.trim()
if (trimmedFind === find) return  // Already trimmed
if (content.includes(trimmedFind)) yield trimmedFind
```

Also matches blocks where trimmed content equals trimmed search.

### 8. ContextAwareReplacer

Similar to BlockAnchor but with 50% middle-line matching requirement:

```typescript
for (let k = 1; k < blockLines.length - 1; k++) {
  const blockLine = blockLines[k].trim()
  const findLine = findLines[k].trim()
  if (blockLine.length > 0 || findLine.length > 0) {
    totalNonEmptyLines++
    if (blockLine === findLine) matchingLines++
  }
}
if (totalNonEmptyLines === 0 || matchingLines / totalNonEmptyLines >= 0.5) {
  yield block
}
```

Stricter than BlockAnchor for multi-candidate scenarios.

### 9. MultiOccurrenceReplacer

Finds ALL exact matches (for `replaceAll: true`):

```typescript
while (true) {
  const index = content.indexOf(find, startIndex)
  if (index === -1) break
  yield find
  startIndex = index + find.length
}
```

## Replace Function Logic

```typescript
export function replace(
  content: string,
  oldString: string,
  newString: string,
  replaceAll = false
): string
```

### Single Match Logic (replaceAll: false)

```typescript
for (const search of replacer(content, oldString)) {
  const index = content.indexOf(search)
  if (index === -1) continue
  notFound = false

  // Check for ambiguity
  const lastIndex = content.lastIndexOf(search)
  if (index !== lastIndex) continue  // Multiple matches - try next replacer

  // Single match - do replacement
  return content.substring(0, index) + newString + content.substring(index + search.length)
}
```

### Replace All Logic

```typescript
if (replaceAll) {
  return content.replaceAll(search, newString)
}
```

### Error Cases

```typescript
if (notFound) {
  throw new Error("oldString not found in content")
}
throw new Error(
  "Found multiple matches for oldString. Provide more surrounding lines in oldString to identify the correct match."
)
```

## Levenshtein Distance

Standard dynamic programming implementation:

```typescript
function levenshtein(a: string, b: string): number {
  if (a === "" || b === "") {
    return Math.max(a.length, b.length)
  }
  const matrix = Array.from({ length: a.length + 1 }, (_, i) =>
    Array.from({ length: b.length + 1 }, (_, j) =>
      (i === 0 ? j : j === 0 ? i : 0))
  )
  for (let i = 1; i <= a.length; i++) {
    for (let j = 1; j <= b.length; j++) {
      const cost = a[i - 1] === b[j - 1] ? 0 : 1
      matrix[i][j] = Math.min(
        matrix[i - 1][j] + 1,      // deletion
        matrix[i][j - 1] + 1,      // insertion
        matrix[i - 1][j - 1] + cost // substitution
      )
    }
  }
  return matrix[a.length][b.length]
}
```

## Diff Trimming

Visual diff output is trimmed for display:

```typescript
export function trimDiff(diff: string): string {
  // Find minimum indentation of content lines
  const contentLines = lines.filter(line =>
    (line.startsWith("+") || line.startsWith("-") || line.startsWith(" ")) &&
    !line.startsWith("---") && !line.startsWith("+++")
  )
  const min = Math.min(...nonEmptyLines.map(line => indentLength))

  // Remove common indentation from all content lines
  return lines.map(line => {
    if (isContentLine(line)) {
      const prefix = line[0]
      const content = line.slice(1)
      return prefix + content.slice(min)
    }
    return line
  }).join("\n")
}
```

Makes diffs more readable by removing unnecessary leading whitespace.

## Attribution

The edit strategies are sourced from:
- Cline: `cline/evals/diff-edits/diff-apply/diff-06-23-25.ts`
- Gemini CLI: `google-gemini/gemini-cli/packages/core/src/utils/editCorrector.ts`
- Cline (updated): `cline/evals/diff-edits/diff-apply/diff-06-26-25.ts`
