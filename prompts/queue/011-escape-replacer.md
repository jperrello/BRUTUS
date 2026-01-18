# Subtask 011: EscapeNormalizedReplacer (Fuzzy Edit 6/9)

## Goal
Implement EscapeNormalizedReplacer - handles literal escape sequences.

## Research Reference
`research/opencode-edit-tool/FUZZY-REPLACEMENT-SPEC.md` - Replacer 6

## Update File
Add to `internal/fuzzy/replacer.go`:

```go
func unescapeString(s string) string {
	replacements := map[string]string{
		`\n`:  "\n",
		`\t`:  "\t",
		`\r`:  "\r",
		`\'`:  "'",
		`\"`:  "\"",
		"\\`": "`",
		`\\`: "\\",
		`\$`:  "$",
	}

	result := s
	for escaped, unescaped := range replacements {
		result = strings.ReplaceAll(result, escaped, unescaped)
	}
	return result
}

func EscapeNormalizedReplacer(content, find string) []string {
	var results []string

	unescapedFind := unescapeString(find)

	// Direct match with unescaped
	if strings.Contains(content, unescapedFind) {
		results = append(results, unescapedFind)
		return results
	}

	// Block match
	contentLines := strings.Split(content, "\n")
	findLines := strings.Split(unescapedFind, "\n")

	if len(findLines) > 0 && findLines[len(findLines)-1] == "" {
		findLines = findLines[:len(findLines)-1]
	}

	for i := 0; i <= len(contentLines)-len(findLines); i++ {
		block := strings.Join(contentLines[i:i+len(findLines)], "\n")
		if unescapeString(block) == unescapedFind {
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
}
```

## Add Test
```go
func TestEscapeNormalizedReplacer(t *testing.T) {
	content := "line1\nline2"
	find := "line1\\nline2" // Literal \n

	results := EscapeNormalizedReplacer(content, find)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
}
```

## Verification
```bash
go test ./internal/fuzzy/...
```

## Done When
- [ ] EscapeNormalizedReplacer added
- [ ] Test passes

## Then
Delete this file and exit.
