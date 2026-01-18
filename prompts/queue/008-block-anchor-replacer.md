# Subtask 008: BlockAnchorReplacer (Fuzzy Edit 3/9)

## Goal
Implement BlockAnchorReplacer - matches by first/last line anchors with fuzzy middle.

## Research Reference
`research/opencode-edit-tool/FUZZY-REPLACEMENT-SPEC.md` - Replacer 3

## Update File
Add to `internal/fuzzy/replacer.go`:

```go
func BlockAnchorReplacer(content, find string) []string {
	var results []string
	contentLines := strings.Split(content, "\n")
	searchLines := strings.Split(find, "\n")

	// Need at least 3 lines for anchor matching
	if len(searchLines) < 3 {
		return results
	}

	// Remove trailing empty line
	if searchLines[len(searchLines)-1] == "" {
		searchLines = searchLines[:len(searchLines)-1]
	}

	if len(searchLines) < 3 {
		return results
	}

	firstLineSearch := strings.TrimSpace(searchLines[0])
	lastLineSearch := strings.TrimSpace(searchLines[len(searchLines)-1])

	type candidate struct {
		startLine, endLine int
		similarity         float64
	}
	var candidates []candidate

	// Find candidate blocks
	for i := 0; i < len(contentLines); i++ {
		if strings.TrimSpace(contentLines[i]) != firstLineSearch {
			continue
		}

		for j := i + 2; j < len(contentLines); j++ {
			if strings.TrimSpace(contentLines[j]) == lastLineSearch {
				// Calculate similarity of middle lines
				actualBlockSize := j - i + 1
				searchBlockSize := len(searchLines)
				linesToCheck := min(searchBlockSize-2, actualBlockSize-2)

				var similarity float64
				if linesToCheck > 0 {
					totalSim := 0.0
					for k := 1; k < searchBlockSize-1 && k < actualBlockSize-1; k++ {
						origLine := strings.TrimSpace(contentLines[i+k])
						searchLine := strings.TrimSpace(searchLines[k])
						totalSim += Similarity(origLine, searchLine)
					}
					similarity = totalSim / float64(linesToCheck)
				} else {
					similarity = 1.0
				}

				candidates = append(candidates, candidate{i, j, similarity})
				break
			}
		}
	}

	if len(candidates) == 0 {
		return results
	}

	// Find best match
	threshold := 0.0 // Single candidate
	if len(candidates) > 1 {
		threshold = 0.3 // Multiple candidates need 30% similarity
	}

	bestIdx := -1
	maxSim := -1.0
	for idx, c := range candidates {
		if c.similarity > maxSim {
			maxSim = c.similarity
			bestIdx = idx
		}
	}

	if maxSim >= threshold && bestIdx >= 0 {
		c := candidates[bestIdx]

		// Calculate byte range
		startIndex := 0
		for k := 0; k < c.startLine; k++ {
			startIndex += len(contentLines[k]) + 1
		}

		endIndex := startIndex
		for k := c.startLine; k <= c.endLine; k++ {
			endIndex += len(contentLines[k])
			if k < c.endLine {
				endIndex++
			}
		}

		if endIndex <= len(content) {
			results = append(results, content[startIndex:endIndex])
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
}
```

## Add Test
```go
func TestBlockAnchorReplacer(t *testing.T) {
	content := "func foo() {\n    // different comment\n    return 1\n}"
	find := "func foo() {\n    // some comment\n    return 1\n}"

	results := BlockAnchorReplacer(content, find)

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
- [ ] BlockAnchorReplacer added
- [ ] Test passes
- [ ] Uses Levenshtein Similarity from subtask 002

## Then
Delete this file and exit.
