# Subtask 013: ContextAwareReplacer (Fuzzy Edit 8/9)

## Goal
Implement ContextAwareReplacer - anchors + 50% middle line match.

## Research Reference
`research/opencode-edit-tool/FUZZY-REPLACEMENT-SPEC.md` - Replacer 8

## Update File
Add to `internal/fuzzy/replacer.go`:

```go
func ContextAwareReplacer(content, find string) []string {
	var results []string

	contentLines := strings.Split(content, "\n")
	findLines := strings.Split(find, "\n")

	if len(findLines) < 3 {
		return results
	}

	// Remove trailing empty line
	if findLines[len(findLines)-1] == "" {
		findLines = findLines[:len(findLines)-1]
	}

	if len(findLines) < 3 {
		return results
	}

	firstLine := strings.TrimSpace(findLines[0])
	lastLine := strings.TrimSpace(findLines[len(findLines)-1])

	for i := 0; i < len(contentLines); i++ {
		if strings.TrimSpace(contentLines[i]) != firstLine {
			continue
		}

		for j := i + len(findLines) - 1; j < len(contentLines); j++ {
			if strings.TrimSpace(contentLines[j]) != lastLine {
				continue
			}

			blockLen := j - i + 1
			if blockLen != len(findLines) {
				continue // Must be exact line count
			}

			// Check 50% of middle lines match
			middleCount := len(findLines) - 2
			if middleCount <= 0 {
				// No middle lines, anchors match is enough
				startIndex := 0
				for k := 0; k < i; k++ {
					startIndex += len(contentLines[k]) + 1
				}
				endIndex := startIndex
				for k := i; k <= j; k++ {
					endIndex += len(contentLines[k])
					if k < j {
						endIndex++
					}
				}
				if endIndex <= len(content) {
					results = append(results, content[startIndex:endIndex])
				}
				return results
			}

			matchingLines := 0
			for k := 1; k < len(findLines)-1; k++ {
				if strings.TrimSpace(contentLines[i+k]) == strings.TrimSpace(findLines[k]) {
					matchingLines++
				}
			}

			matchRatio := float64(matchingLines) / float64(middleCount)
			if matchRatio >= 0.5 {
				startIndex := 0
				for k := 0; k < i; k++ {
					startIndex += len(contentLines[k]) + 1
				}
				endIndex := startIndex
				for k := i; k <= j; k++ {
					endIndex += len(contentLines[k])
					if k < j {
						endIndex++
					}
				}
				if endIndex <= len(content) {
					results = append(results, content[startIndex:endIndex])
				}
				return results
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
	BlockAnchorReplacer,
	WhitespaceNormalizedReplacer,
	IndentationFlexibleReplacer,
	EscapeNormalizedReplacer,
	TrimmedBoundaryReplacer,
	ContextAwareReplacer,
}
```

## Add Test
```go
func TestContextAwareReplacer(t *testing.T) {
	content := "func start() {\n    line1\n    line2\n    line3\n    line4\n}"
	find := "func start() {\n    line1\n    different\n    line3\n    different\n}"

	results := ContextAwareReplacer(content, find)

	// 2 of 4 middle lines match = 50%, should work
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
- [ ] ContextAwareReplacer added
- [ ] Test passes

## Then
Delete this file and exit.
