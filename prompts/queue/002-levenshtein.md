# Subtask 002: Levenshtein Distance Function

## Goal
Create the Levenshtein distance function needed by fuzzy edit replacers.

## Research Reference
`research/opencode-edit-tool/BRUTUS-EDIT-IMPLEMENTATION-SPEC.md` - see Levenshtein section

## Create File
`internal/fuzzy/levenshtein.go`

## Code to Write

```go
package fuzzy

func Levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	if len(a) > len(b) {
		a, b = b, a
	}

	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)

	for j := 0; j <= len(b); j++ {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			curr[j] = min(
				prev[j]+1,
				curr[j-1]+1,
				prev[j-1]+cost,
			)
		}
		prev, curr = curr, prev
	}

	return prev[len(b)]
}

func Similarity(a, b string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	maxLen := max(len(a), len(b))
	if maxLen == 0 {
		return 1.0
	}
	distance := Levenshtein(a, b)
	return 1.0 - float64(distance)/float64(maxLen)
}
```

## Create Test File
`internal/fuzzy/levenshtein_test.go`

```go
package fuzzy

import "testing"

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		a, b     string
		expected int
	}{
		{"", "", 0},
		{"a", "", 1},
		{"", "a", 1},
		{"a", "a", 0},
		{"a", "b", 1},
		{"ab", "ab", 0},
		{"ab", "ba", 2},
		{"kitten", "sitting", 3},
		{"saturday", "sunday", 3},
	}
	for _, tt := range tests {
		got := Levenshtein(tt.a, tt.b)
		if got != tt.expected {
			t.Errorf("Levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.expected)
		}
	}
}

func TestSimilarity(t *testing.T) {
	if s := Similarity("hello", "hello"); s != 1.0 {
		t.Errorf("Similarity of identical strings should be 1.0, got %f", s)
	}
	if s := Similarity("", ""); s != 1.0 {
		t.Errorf("Similarity of empty strings should be 1.0, got %f", s)
	}
	if s := Similarity("a", "b"); s != 0.0 {
		t.Errorf("Similarity of completely different single chars should be 0.0, got %f", s)
	}
}
```

## Verification
```bash
mkdir -p internal/fuzzy
# Write the files
go test ./internal/fuzzy/...
```

## Done When
- [ ] `internal/fuzzy/levenshtein.go` exists
- [ ] `internal/fuzzy/levenshtein_test.go` exists
- [ ] `go test ./internal/fuzzy/...` passes

## Then
Delete this file and exit.
