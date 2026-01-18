# Subtask 021: Doom Loop Detection

## Goal
Implement doom loop detection to prevent infinite tool repetition.

## Research Reference
`research/opencode-agent-loop/AGENT-LOOP-SPEC.md` - Doom Loop section

## Create File
`internal/agent/doom.go`

## Code to Write

```go
package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

const DoomThreshold = 3

type DoomTracker struct {
	history []string
}

func NewDoomTracker() *DoomTracker {
	return &DoomTracker{
		history: make([]string, 0),
	}
}

func (d *DoomTracker) makeKey(toolName string, input json.RawMessage) string {
	h := sha256.New()
	h.Write([]byte(toolName))
	h.Write([]byte(":"))
	h.Write(input)
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func (d *DoomTracker) Track(toolName string, input json.RawMessage) {
	key := d.makeKey(toolName, input)
	d.history = append(d.history, key)

	// Keep only last N entries
	if len(d.history) > DoomThreshold*2 {
		d.history = d.history[len(d.history)-DoomThreshold*2:]
	}
}

func (d *DoomTracker) IsDoomLoop(toolName string, input json.RawMessage) bool {
	if len(d.history) < DoomThreshold {
		return false
	}

	key := d.makeKey(toolName, input)

	// Check if the last N entries are all the same as this key
	consecutive := 0
	for i := len(d.history) - 1; i >= 0 && i >= len(d.history)-DoomThreshold; i-- {
		if d.history[i] == key {
			consecutive++
		} else {
			break
		}
	}

	return consecutive >= DoomThreshold-1 // -1 because current call not yet tracked
}

func (d *DoomTracker) Reset() {
	d.history = make([]string, 0)
}
```

## Create Test
`internal/agent/doom_test.go`:

```go
package agent

import (
	"encoding/json"
	"testing"
)

func TestDoomTracker_NoDoom(t *testing.T) {
	tracker := NewDoomTracker()

	input1 := json.RawMessage(`{"path": "file1.txt"}`)
	input2 := json.RawMessage(`{"path": "file2.txt"}`)

	tracker.Track("read", input1)
	tracker.Track("read", input2)

	if tracker.IsDoomLoop("read", json.RawMessage(`{"path": "file3.txt"}`)) {
		t.Error("Should not detect doom loop for different inputs")
	}
}

func TestDoomTracker_DetectsDoom(t *testing.T) {
	tracker := NewDoomTracker()

	input := json.RawMessage(`{"path": "same.txt"}`)

	tracker.Track("read", input)
	tracker.Track("read", input)

	// Third call with same input should trigger doom detection
	if !tracker.IsDoomLoop("read", input) {
		t.Error("Should detect doom loop after 3 identical calls")
	}
}

func TestDoomTracker_Reset(t *testing.T) {
	tracker := NewDoomTracker()

	input := json.RawMessage(`{"test": true}`)
	tracker.Track("tool", input)
	tracker.Track("tool", input)

	tracker.Reset()

	if tracker.IsDoomLoop("tool", input) {
		t.Error("Should not detect doom after reset")
	}
}
```

## Verification
```bash
mkdir -p internal/agent
go test ./internal/agent/...
```

## Done When
- [ ] `internal/agent/doom.go` exists
- [ ] `internal/agent/doom_test.go` exists
- [ ] Tests pass

## Then
Delete this file and exit.
