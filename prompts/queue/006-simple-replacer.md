# Subtask 006: SimpleReplacer (Fuzzy Edit 1/9)

## Goal
Implement the SimpleReplacer - exact string matching (first of 9 fuzzy replacers).

## Research Reference
`research/opencode-edit-tool/FUZZY-REPLACEMENT-SPEC.md` - Replacer 1

## Create File
`internal/fuzzy/replacer.go`

## Code to Write

```go
package fuzzy

type Replacer func(content, find string) []string

func SimpleReplacer(content, find string) []string {
	return []string{find}
}
```

## Update File
Add to `internal/fuzzy/replacer.go`:

```go
var Replacers = []Replacer{
	SimpleReplacer,
	// More replacers will be added in subsequent subtasks
}
```

## Create Test File
`internal/fuzzy/replacer_test.go`

```go
package fuzzy

import (
	"testing"
)

func TestSimpleReplacer(t *testing.T) {
	content := "hello world"
	find := "world"

	results := SimpleReplacer(content, find)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0] != "world" {
		t.Errorf("Expected 'world', got %q", results[0])
	}
}
```

## Verification
```bash
# internal/fuzzy/ should already exist from subtask 002
go test ./internal/fuzzy/...
```

## Done When
- [ ] `internal/fuzzy/replacer.go` exists with SimpleReplacer
- [ ] `internal/fuzzy/replacer_test.go` exists
- [ ] `go test ./internal/fuzzy/...` passes

## Then
Delete this file and exit.
