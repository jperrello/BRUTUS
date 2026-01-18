# Subtask 015: Replace Orchestrator

## Goal
Create the Replace function that orchestrates all 9 replacers.

## Research Reference
`research/opencode-edit-tool/FUZZY-REPLACEMENT-SPEC.md` - see replace() algorithm

## Update File
Add to `internal/fuzzy/replacer.go`:

```go
import "fmt"

func Replace(content, oldString, newString string, replaceAll bool) (string, error) {
	if oldString == newString {
		return "", fmt.Errorf("oldString and newString must be different")
	}

	notFound := true

	for _, replacer := range Replacers {
		for _, search := range replacer(content, oldString) {
			idx := strings.Index(content, search)
			if idx == -1 {
				continue
			}

			notFound = false

			if replaceAll {
				return strings.ReplaceAll(content, search, newString), nil
			}

			// Check for uniqueness
			lastIdx := strings.LastIndex(content, search)
			if idx != lastIdx {
				continue // Multiple matches, try next replacer
			}

			// Single unique match - apply replacement
			return content[:idx] + newString + content[idx+len(search):], nil
		}
	}

	if notFound {
		return "", fmt.Errorf("oldString not found in content")
	}
	return "", fmt.Errorf("found multiple matches for oldString; provide more context to identify unique match")
}
```

## Add Tests
```go
func TestReplace_ExactMatch(t *testing.T) {
	content := "hello world"
	result, err := Replace(content, "world", "universe", false)
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello universe" {
		t.Errorf("Expected 'hello universe', got %q", result)
	}
}

func TestReplace_NotFound(t *testing.T) {
	_, err := Replace("hello world", "xyz", "abc", false)
	if err == nil {
		t.Error("Expected error for not found")
	}
}

func TestReplace_MultipleMatches(t *testing.T) {
	_, err := Replace("hello hello", "hello", "world", false)
	if err == nil {
		t.Error("Expected error for multiple matches")
	}
}

func TestReplace_ReplaceAll(t *testing.T) {
	result, err := Replace("hello hello", "hello", "world", true)
	if err != nil {
		t.Fatal(err)
	}
	if result != "world world" {
		t.Errorf("Expected 'world world', got %q", result)
	}
}

func TestReplace_FuzzyIndentation(t *testing.T) {
	content := "    if (x) {\n        return;\n    }"
	find := "if (x) {\n    return;\n}"
	replace := "if (y) {\n    return;\n}"

	result, err := Replace(content, find, replace, false)
	if err != nil {
		t.Fatalf("Fuzzy match should work: %v", err)
	}
	if !strings.Contains(result, "if (y)") {
		t.Errorf("Replacement should have been applied")
	}
}
```

## Verification
```bash
go test ./internal/fuzzy/... -v
```

## Done When
- [ ] Replace function added
- [ ] All tests pass
- [ ] Fuzzy matching works

## Then
Delete this file and exit.
