# Subtask 009: WhitespaceNormalizedReplacer (Fuzzy Edit 4/9)

## Goal
Implement WhitespaceNormalizedReplacer - collapses whitespace before comparison.

## Research Reference
`research/opencode-edit-tool/FUZZY-REPLACEMENT-SPEC.md` - Replacer 4

## Update File
Add to `internal/fuzzy/replacer.go`:

```go
import "regexp"

var whitespaceRegex = regexp.MustCompile(`\s+`)

func normalizeWhitespace(s string) string {
	return strings.TrimSpace(whitespaceRegex.ReplaceAllString(s, " "))
}

func WhitespaceNormalizedReplacer(content, find string) []string {
	var results []string

	normalizedFind := normalizeWhitespace(find)
	if normalizedFind == "" {
		return results
	}

	contentLines := strings.Split(content, "\n")
	findLines := strings.Split(find, "\n")

	// Single line match
	if len(findLines) == 1 {
		for _, line := range contentLines {
			normalizedLine := normalizeWhitespace(line)
			if normalizedLine == normalizedFind {
				results = append(results, line)
			} else if strings.Contains(normalizedLine, normalizedFind) {
				// Try to extract the matching portion
				words := strings.Fields(find)
				if len(words) > 0 {
					pattern := strings.Join(words, `\s+`)
					re, err := regexp.Compile(pattern)
					if err == nil {
						if match := re.FindString(line); match != "" {
							results = append(results, match)
						}
					}
				}
			}
		}
		return results
	}

	// Multi-line match
	for i := 0; i <= len(contentLines)-len(findLines); i++ {
		block := strings.Join(contentLines[i:i+len(findLines)], "\n")
		if normalizeWhitespace(block) == normalizedFind {
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
}
```

## Add Test
```go
func TestWhitespaceNormalizedReplacer(t *testing.T) {
	content := "function  foo(   a,b  ) {}"
	find := "function foo(a, b) {}"

	results := WhitespaceNormalizedReplacer(content, find)

	if len(results) == 0 {
		t.Fatal("Expected at least 1 result")
	}
}
```

## Verification
```bash
go test ./internal/fuzzy/...
```

## Done When
- [ ] WhitespaceNormalizedReplacer added
- [ ] Test passes

## Then
Delete this file and exit.
