# BRUTUS Edit Tool Implementation Specification

## Overview

This document provides a Go implementation specification for porting OpenCode's edit tool to BRUTUS. The edit tool is the most critical capability for a coding agent.

## Package Structure

```
tools/
  edit.go           # Main tool definition and execute function
  edit_replacers.go # The 9 fuzzy replacement algorithms
  edit_test.go      # Unit tests for replacers
```

## Core Types

### Input Struct

```go
type EditInput struct {
    FilePath   string `json:"filePath" description:"The absolute path to the file to modify"`
    OldString  string `json:"oldString" description:"The text to replace"`
    NewString  string `json:"newString" description:"The text to replace it with"`
    ReplaceAll bool   `json:"replaceAll,omitempty" description:"Replace all occurrences"`
}
```

### Output Struct

```go
type EditOutput struct {
    Success    bool              `json:"success"`
    Message    string            `json:"message"`
    Diff       string            `json:"diff,omitempty"`
    FileDiff   *FileDiff         `json:"fileDiff,omitempty"`
    Errors     []DiagnosticError `json:"errors,omitempty"`
}

type FileDiff struct {
    File      string `json:"file"`
    Before    string `json:"before"`
    After     string `json:"after"`
    Additions int    `json:"additions"`
    Deletions int    `json:"deletions"`
}

type DiagnosticError struct {
    Line    int    `json:"line"`
    Column  int    `json:"column"`
    Message string `json:"message"`
}
```

## Tool Definition

```go
var EditTool = NewTool[EditInput](
    "edit",
    editDescription, // From embedded edit.txt
    EditExecute,
)

//go:embed edit.txt
var editDescription string
```

## Execute Function

```go
func EditExecute(ctx context.Context, input EditInput) (string, error) {
    // 1. Validate input
    if input.FilePath == "" {
        return "", fmt.Errorf("filePath is required")
    }
    if input.OldString == input.NewString {
        return "", fmt.Errorf("oldString and newString must be different")
    }

    // 2. Resolve path
    filePath := input.FilePath
    if !filepath.IsAbs(filePath) {
        filePath = filepath.Join(WorkingDirectory, filePath)
    }

    // 3. Check workspace boundaries
    if err := assertWithinWorkspace(filePath); err != nil {
        return "", err
    }

    // 4. Acquire file lock
    unlock := fileLock.Lock(filePath)
    defer unlock()

    // 5. Handle empty oldString (file creation)
    if input.OldString == "" {
        if err := os.WriteFile(filePath, []byte(input.NewString), 0644); err != nil {
            return "", fmt.Errorf("failed to write file: %w", err)
        }
        return "File created successfully.", nil
    }

    // 6. Validate file exists and was read
    info, err := os.Stat(filePath)
    if os.IsNotExist(err) {
        return "", fmt.Errorf("file %s not found", filePath)
    }
    if info.IsDir() {
        return "", fmt.Errorf("path is a directory, not a file: %s", filePath)
    }
    if err := assertFileWasRead(sessionID, filePath); err != nil {
        return "", err
    }

    // 7. Read content
    contentBytes, err := os.ReadFile(filePath)
    if err != nil {
        return "", fmt.Errorf("failed to read file: %w", err)
    }
    contentOld := string(contentBytes)

    // 8. Apply fuzzy replacement
    contentNew, err := Replace(contentOld, input.OldString, input.NewString, input.ReplaceAll)
    if err != nil {
        return "", err
    }

    // 9. Write content
    if err := os.WriteFile(filePath, []byte(contentNew), info.Mode()); err != nil {
        return "", fmt.Errorf("failed to write file: %w", err)
    }

    // 10. Update read timestamp
    recordFileRead(sessionID, filePath)

    // 11. Generate diff and return
    diff := generateDiff(filePath, contentOld, contentNew)
    fileDiff := calculateFileDiff(filePath, contentOld, contentNew)

    output := EditOutput{
        Success:  true,
        Message:  "Edit applied successfully.",
        Diff:     diff,
        FileDiff: fileDiff,
    }

    return formatOutput(output), nil
}
```

## File Locking

```go
// Simple per-file mutex map
var fileLocks = struct {
    sync.Mutex
    locks map[string]*sync.Mutex
}{
    locks: make(map[string]*sync.Mutex),
}

type FileLock struct{}

var fileLock FileLock

func (FileLock) Lock(path string) func() {
    fileLocks.Lock()
    lock, ok := fileLocks.locks[path]
    if !ok {
        lock = &sync.Mutex{}
        fileLocks.locks[path] = lock
    }
    fileLocks.Unlock()

    lock.Lock()
    return lock.Unlock
}
```

## Read Time Tracking

```go
var readTimes = struct {
    sync.RWMutex
    times map[string]map[string]time.Time // sessionID -> filePath -> readTime
}{
    times: make(map[string]map[string]time.Time),
}

func recordFileRead(sessionID, filePath string) {
    readTimes.Lock()
    defer readTimes.Unlock()

    if readTimes.times[sessionID] == nil {
        readTimes.times[sessionID] = make(map[string]time.Time)
    }
    readTimes.times[sessionID][filePath] = time.Now()
}

func assertFileWasRead(sessionID, filePath string) error {
    readTimes.RLock()
    defer readTimes.RUnlock()

    sessions, ok := readTimes.times[sessionID]
    if !ok {
        return fmt.Errorf("file must be read before editing: %s", filePath)
    }
    readTime, ok := sessions[filePath]
    if !ok {
        return fmt.Errorf("file must be read before editing: %s", filePath)
    }

    // Check if file was modified after read
    info, err := os.Stat(filePath)
    if err != nil {
        return fmt.Errorf("cannot stat file: %w", err)
    }
    if info.ModTime().After(readTime) {
        return fmt.Errorf("file was modified after last read: %s", filePath)
    }

    return nil
}
```

## Replacer Interface

```go
// Replacer yields candidate strings that might match in the content
type Replacer func(content, find string) []string
```

## Replace Function

```go
var replacers = []Replacer{
    SimpleReplacer,
    LineTrimmedReplacer,
    BlockAnchorReplacer,
    WhitespaceNormalizedReplacer,
    IndentationFlexibleReplacer,
    EscapeNormalizedReplacer,
    TrimmedBoundaryReplacer,
    ContextAwareReplacer,
    MultiOccurrenceReplacer,
}

func Replace(content, oldString, newString string, replaceAll bool) (string, error) {
    if oldString == newString {
        return "", fmt.Errorf("oldString and newString must be different")
    }

    notFound := true

    for _, replacer := range replacers {
        for _, search := range replacer(content, oldString) {
            index := strings.Index(content, search)
            if index == -1 {
                continue
            }

            notFound = false

            if replaceAll {
                return strings.ReplaceAll(content, search, newString), nil
            }

            // Check for uniqueness
            lastIndex := strings.LastIndex(content, search)
            if index != lastIndex {
                continue // Multiple matches, try next replacer
            }

            // Single unique match
            return content[:index] + newString + content[index+len(search):], nil
        }
    }

    if notFound {
        return "", fmt.Errorf("oldString not found in content")
    }
    return "", fmt.Errorf("found multiple matches for oldString; provide more surrounding lines to identify the correct match")
}
```

## Replacer Implementations

### 1. SimpleReplacer

```go
func SimpleReplacer(content, find string) []string {
    return []string{find}
}
```

### 2. LineTrimmedReplacer

```go
func LineTrimmedReplacer(content, find string) []string {
    var results []string
    contentLines := strings.Split(content, "\n")
    searchLines := strings.Split(find, "\n")

    // Remove trailing empty line
    if len(searchLines) > 0 && searchLines[len(searchLines)-1] == "" {
        searchLines = searchLines[:len(searchLines)-1]
    }

    for i := 0; i <= len(contentLines)-len(searchLines); i++ {
        match := true
        for j := 0; j < len(searchLines); j++ {
            if strings.TrimSpace(contentLines[i+j]) != strings.TrimSpace(searchLines[j]) {
                match = false
                break
            }
        }

        if match {
            // Calculate exact byte range
            startIndex := 0
            for k := 0; k < i; k++ {
                startIndex += len(contentLines[k]) + 1
            }

            endIndex := startIndex
            for k := 0; k < len(searchLines); k++ {
                endIndex += len(contentLines[i+k])
                if k < len(searchLines)-1 {
                    endIndex += 1
                }
            }

            results = append(results, content[startIndex:endIndex])
        }
    }

    return results
}
```

### 3. BlockAnchorReplacer

```go
func BlockAnchorReplacer(content, find string) []string {
    var results []string
    contentLines := strings.Split(content, "\n")
    searchLines := strings.Split(find, "\n")

    if len(searchLines) < 3 {
        return results
    }

    // Remove trailing empty line
    if searchLines[len(searchLines)-1] == "" {
        searchLines = searchLines[:len(searchLines)-1]
    }

    firstLineSearch := strings.TrimSpace(searchLines[0])
    lastLineSearch := strings.TrimSpace(searchLines[len(searchLines)-1])

    // Find candidates
    type candidate struct {
        startLine, endLine int
    }
    var candidates []candidate

    for i := 0; i < len(contentLines); i++ {
        if strings.TrimSpace(contentLines[i]) != firstLineSearch {
            continue
        }

        for j := i + 2; j < len(contentLines); j++ {
            if strings.TrimSpace(contentLines[j]) == lastLineSearch {
                candidates = append(candidates, candidate{i, j})
                break
            }
        }
    }

    if len(candidates) == 0 {
        return results
    }

    // Calculate similarity and find best match
    bestMatch := -1
    maxSimilarity := -1.0
    threshold := 0.0 // Single candidate
    if len(candidates) > 1 {
        threshold = 0.3 // Multiple candidates
    }

    for idx, c := range candidates {
        actualBlockSize := c.endLine - c.startLine + 1
        searchBlockSize := len(searchLines)

        var similarity float64
        linesToCheck := min(searchBlockSize-2, actualBlockSize-2)

        if linesToCheck > 0 {
            totalSim := 0.0
            for j := 1; j < searchBlockSize-1 && j < actualBlockSize-1; j++ {
                origLine := strings.TrimSpace(contentLines[c.startLine+j])
                searchLine := strings.TrimSpace(searchLines[j])
                maxLen := max(len(origLine), len(searchLine))
                if maxLen == 0 {
                    continue
                }
                distance := levenshtein(origLine, searchLine)
                totalSim += 1.0 - float64(distance)/float64(maxLen)
            }
            similarity = totalSim / float64(linesToCheck)
        } else {
            similarity = 1.0
        }

        if similarity > maxSimilarity {
            maxSimilarity = similarity
            bestMatch = idx
        }
    }

    if maxSimilarity >= threshold && bestMatch >= 0 {
        c := candidates[bestMatch]

        startIndex := 0
        for k := 0; k < c.startLine; k++ {
            startIndex += len(contentLines[k]) + 1
        }

        endIndex := startIndex
        for k := c.startLine; k <= c.endLine; k++ {
            endIndex += len(contentLines[k])
            if k < c.endLine {
                endIndex += 1
            }
        }

        results = append(results, content[startIndex:endIndex])
    }

    return results
}
```

### 4-9. Remaining Replacers

See `FUZZY-REPLACEMENT-SPEC.md` for detailed algorithms. Each follows the same pattern:
- Accept content and find strings
- Apply normalization/transformation
- Return slice of candidate matches

## Levenshtein Distance

```go
func levenshtein(a, b string) int {
    if a == "" || b == "" {
        return max(len(a), len(b))
    }

    matrix := make([][]int, len(a)+1)
    for i := range matrix {
        matrix[i] = make([]int, len(b)+1)
        matrix[i][0] = i
    }
    for j := 0; j <= len(b); j++ {
        matrix[0][j] = j
    }

    for i := 1; i <= len(a); i++ {
        for j := 1; j <= len(b); j++ {
            cost := 0
            if a[i-1] != b[j-1] {
                cost = 1
            }
            matrix[i][j] = min(
                matrix[i-1][j]+1,      // deletion
                matrix[i][j-1]+1,      // insertion
                matrix[i-1][j-1]+cost, // substitution
            )
        }
    }

    return matrix[len(a)][len(b)]
}
```

## Diff Generation

```go
import "github.com/sergi/go-diff/diffmatchpatch"

func generateDiff(filePath, before, after string) string {
    dmp := diffmatchpatch.New()
    diffs := dmp.DiffMain(before, after, true)
    return dmp.DiffPrettyText(diffs)
}

func calculateFileDiff(filePath, before, after string) *FileDiff {
    beforeLines := strings.Split(before, "\n")
    afterLines := strings.Split(after, "\n")

    // Simple line-based diff count
    // For production, use a proper diff library
    additions := 0
    deletions := 0

    // ... calculate additions/deletions ...

    return &FileDiff{
        File:      filePath,
        Before:    before,
        After:     after,
        Additions: additions,
        Deletions: deletions,
    }
}
```

## Testing Strategy

```go
func TestSimpleReplacer(t *testing.T) {
    content := "hello world"
    find := "world"
    results := SimpleReplacer(content, find)
    assert.Equal(t, []string{"world"}, results)
}

func TestLineTrimmedReplacer(t *testing.T) {
    content := "    if (x) {\n        return;\n    }"
    find := "if (x) {\n    return;\n}"
    results := LineTrimmedReplacer(content, find)
    assert.Len(t, results, 1)
    assert.Equal(t, content, results[0])
}

func TestBlockAnchorReplacer(t *testing.T) {
    content := "func foo() {\n    // different comment\n    return 1\n}"
    find := "func foo() {\n    // some comment\n    return 1\n}"
    results := BlockAnchorReplacer(content, find)
    assert.Len(t, results, 1)
}

func TestReplace_NotFound(t *testing.T) {
    _, err := Replace("hello world", "xyz", "abc", false)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "not found")
}

func TestReplace_MultipleMatches(t *testing.T) {
    _, err := Replace("hello hello", "hello", "world", false)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "multiple matches")
}

func TestReplace_ReplaceAll(t *testing.T) {
    result, err := Replace("hello hello", "hello", "world", true)
    assert.NoError(t, err)
    assert.Equal(t, "world world", result)
}
```

## Dependencies

```go
import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "sync"
    "time"

    "github.com/sergi/go-diff/diffmatchpatch" // For diff generation
)
```

## Implementation Priority

1. **Phase 1**: SimpleReplacer + basic Replace function
2. **Phase 2**: LineTrimmedReplacer + file locking
3. **Phase 3**: BlockAnchorReplacer + Levenshtein
4. **Phase 4**: Remaining replacers
5. **Phase 5**: Read tracking + assertion
6. **Phase 6**: Diff generation + output formatting

## Notes

- Go doesn't have generators; use slices instead
- The replacer order is critical - don't reorder
- Levenshtein can be O(n*m); consider caching for large files
- File locking is simpler in Go with sync.Mutex
- Consider using `golang.org/x/exp/slices` for min/max helpers
