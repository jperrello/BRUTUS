# Subtask 001: Token Estimation Utility

## Goal
Create a simple token estimation utility used by truncation and compaction.

## Create File
`internal/token/estimate.go`

## Code to Write

```go
package token

const CharsPerToken = 4

func Estimate(s string) int {
	if len(s) == 0 {
		return 0
	}
	return (len(s) + CharsPerToken - 1) / CharsPerToken
}

func EstimateMessages(messages []string) int {
	total := 0
	for _, m := range messages {
		total += Estimate(m)
	}
	return total
}
```

## Create Test File
`internal/token/estimate_test.go`

```go
package token

import "testing"

func TestEstimate(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"a", 1},
		{"abcd", 1},
		{"abcde", 2},
		{"abcdefgh", 2},
		{"abcdefghi", 3},
	}
	for _, tt := range tests {
		got := Estimate(tt.input)
		if got != tt.expected {
			t.Errorf("Estimate(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}
```

## Verification
```bash
mkdir -p internal/token
# Write the files
go test ./internal/token/...
```

## Done When
- [ ] `internal/token/estimate.go` exists
- [ ] `internal/token/estimate_test.go` exists
- [ ] `go test ./internal/token/...` passes

## Then
Delete this file and exit.
