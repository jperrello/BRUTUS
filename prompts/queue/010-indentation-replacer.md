# Subtask 010: IndentationFlexibleReplacer (Fuzzy Edit 5/9)

## Goal
Implement IndentationFlexibleReplacer - strips common indentation before comparison.

## Research Reference
`research/opencode-edit-tool/FUZZY-REPLACEMENT-SPEC.md` - Replacer 5

## Update File
Add to `internal/fuzzy/replacer.go`:

```go
func removeCommonIndentation(text string) string {
	lines := strings.Split(text, "\n")

	// Find minimum indentation of non-empty lines
	minIndent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if minIndent == -1 || indent < minIndent {
			minIndent = indent
		}
	}

	if minIndent <= 0 {
		return text
	}

	// Remove common indentation
	var result []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			result = append(result, line)
		} else if len(line) >= minIndent {
			result = append(result, line[minIndent:])
		} else {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

func IndentationFlexibleReplacer(content, find string) []string {
	var results []string

	normalizedFind := removeCommonIndentation(find)
	if normalizedFind == "" {
		return results
	}

	contentLines := strings.Split(content, "\n")
	findLines := strings.Split(normalizedFind, "\n")

	// Remove trailing empty line
	if len(findLines) > 0 && findLines[len(findLines)-1] == "" {
		findLines = findLines[:len(findLines)-1]
	}

	for i := 0; i <= len(contentLines)-len(findLines); i++ {
		block := strings.Join(contentLines[i:i+len(findLines)], "\n")
		if removeCommonIndentation(block) == normalizedFind {
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
}
```

## Add Test
```go
func TestIndentationFlexibleReplacer(t *testing.T) {
	content := "        function foo() {\n            return;\n        }"
	find := "function foo() {\n    return;\n}"

	results := IndentationFlexibleReplacer(content, find)

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
- [ ] IndentationFlexibleReplacer added
- [ ] Test passes

## Then
Delete this file and exit.
