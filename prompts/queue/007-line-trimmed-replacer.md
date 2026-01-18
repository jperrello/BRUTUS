# Subtask 007: LineTrimmedReplacer (Fuzzy Edit 2/9)

## Goal
Implement LineTrimmedReplacer - matches by trimmed line comparison.

## Research Reference
`research/opencode-edit-tool/FUZZY-REPLACEMENT-SPEC.md` - Replacer 2

## Update File
Add to `internal/fuzzy/replacer.go`:

```go
import "strings"

func LineTrimmedReplacer(content, find string) []string {
	var results []string
	contentLines := strings.Split(content, "\n")
	searchLines := strings.Split(find, "\n")

	// Remove trailing empty line from search
	if len(searchLines) > 0 && searchLines[len(searchLines)-1] == "" {
		searchLines = searchLines[:len(searchLines)-1]
	}

	if len(searchLines) == 0 {
		return results
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
					endIndex++
				}
			}

			if endIndex <= len(content) {
				results = append(results, content[startIndex:endIndex])
			}
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
	// More replacers will be added
}
```

## Add Test
Add to `internal/fuzzy/replacer_test.go`:

```go
func TestLineTrimmedReplacer(t *testing.T) {
	content := "    if (x) {\n        return;\n    }"
	find := "if (x) {\n    return;\n}"

	results := LineTrimmedReplacer(content, find)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	// Should return the actual content with original indentation
	if results[0] != content {
		t.Errorf("Expected original content, got %q", results[0])
	}
}
```

## Verification
```bash
go test ./internal/fuzzy/...
```

## Done When
- [ ] LineTrimmedReplacer added to `internal/fuzzy/replacer.go`
- [ ] Test added to `internal/fuzzy/replacer_test.go`
- [ ] `go test ./internal/fuzzy/...` passes

## Then
Delete this file and exit.
