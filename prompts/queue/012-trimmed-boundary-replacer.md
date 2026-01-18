# Subtask 012: TrimmedBoundaryReplacer (Fuzzy Edit 7/9)

## Goal
Implement TrimmedBoundaryReplacer - trims boundaries only.

## Research Reference
`research/opencode-edit-tool/FUZZY-REPLACEMENT-SPEC.md` - Replacer 7

## Update File
Add to `internal/fuzzy/replacer.go`:

```go
func TrimmedBoundaryReplacer(content, find string) []string {
	var results []string

	trimmedFind := strings.TrimSpace(find)

	// Skip if already trimmed (SimpleReplacer would handle)
	if trimmedFind == find {
		return results
	}

	// Direct match
	if strings.Contains(content, trimmedFind) {
		results = append(results, trimmedFind)
		return results
	}

	// Block match
	contentLines := strings.Split(content, "\n")
	findLines := strings.Split(trimmedFind, "\n")

	if len(findLines) > 0 && findLines[len(findLines)-1] == "" {
		findLines = findLines[:len(findLines)-1]
	}

	for i := 0; i <= len(contentLines)-len(findLines); i++ {
		block := strings.Join(contentLines[i:i+len(findLines)], "\n")
		if strings.TrimSpace(block) == trimmedFind {
			results = append(results, block)
		}
	}

	return results
}
```

## Update Replacers List
```go
var Replacers = []Replacer{
	SimpleReplacer,
	LineTrimmedReplacer,
	BlockAnchorReplacer,
	WhitespaceNormalizedReplacer,
	IndentationFlexibleReplacer,
	EscapeNormalizedReplacer,
	TrimmedBoundaryReplacer,
}
```

## Add Test
```go
func TestTrimmedBoundaryReplacer(t *testing.T) {
	content := "hello world"
	find := "  hello world  " // Extra whitespace at boundaries

	results := TrimmedBoundaryReplacer(content, find)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0] != "hello world" {
		t.Errorf("Expected 'hello world', got %q", results[0])
	}
}
```

## Verification
```bash
go test ./internal/fuzzy/...
```

## Done When
- [ ] TrimmedBoundaryReplacer added
- [ ] Test passes

## Then
Delete this file and exit.
