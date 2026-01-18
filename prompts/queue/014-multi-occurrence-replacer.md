# Subtask 014: MultiOccurrenceReplacer (Fuzzy Edit 9/9)

## Goal
Implement MultiOccurrenceReplacer - for replaceAll mode.

## Research Reference
`research/opencode-edit-tool/FUZZY-REPLACEMENT-SPEC.md` - Replacer 9

## Update File
Add to `internal/fuzzy/replacer.go`:

```go
func MultiOccurrenceReplacer(content, find string) []string {
	var results []string

	startIndex := 0
	for {
		idx := strings.Index(content[startIndex:], find)
		if idx == -1 {
			break
		}
		results = append(results, find)
		startIndex += idx + len(find)
	}

	return results
}
```

## Update Replacers List (FINAL)
```go
var Replacers = []Replacer{
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
```

## Add Test
```go
func TestMultiOccurrenceReplacer(t *testing.T) {
	content := "hello hello hello"
	find := "hello"

	results := MultiOccurrenceReplacer(content, find)

	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}
}
```

## Verification
```bash
go test ./internal/fuzzy/...
```

## Done When
- [ ] MultiOccurrenceReplacer added
- [ ] All 9 replacers in Replacers list
- [ ] Test passes

## Then
Delete this file and exit.
