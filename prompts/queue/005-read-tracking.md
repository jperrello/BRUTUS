# Subtask 005: File Read Tracking

## Goal
Create a read tracker to ensure files are read before being edited.

## Research Reference
`research/opencode-edit-tool/BRUTUS-EDIT-IMPLEMENTATION-SPEC.md` - see Read Time Tracking

## Create File
`internal/tool/readtrack/tracker.go`

## Code to Write

```go
package readtrack

import (
	"fmt"
	"os"
	"sync"
	"time"
)

var (
	mu        sync.RWMutex
	readTimes = make(map[string]time.Time)
)

func RecordRead(path string) {
	mu.Lock()
	defer mu.Unlock()
	readTimes[path] = time.Now()
}

func WasRead(path string) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, ok := readTimes[path]
	return ok
}

func GetReadTime(path string) (time.Time, bool) {
	mu.RLock()
	defer mu.RUnlock()
	t, ok := readTimes[path]
	return t, ok
}

func AssertReadable(path string) error {
	if !WasRead(path) {
		return fmt.Errorf("file must be read before editing: %s", path)
	}

	readTime, _ := GetReadTime(path)
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot stat file: %w", err)
	}

	if info.ModTime().After(readTime) {
		return fmt.Errorf("file was modified after last read: %s", path)
	}

	return nil
}

func Clear() {
	mu.Lock()
	defer mu.Unlock()
	readTimes = make(map[string]time.Time)
}
```

## Create Test File
`internal/tool/readtrack/tracker_test.go`

```go
package readtrack

import (
	"os"
	"testing"
	"time"
)

func TestReadTracking(t *testing.T) {
	Clear()

	path := "/test/file.go"

	if WasRead(path) {
		t.Error("File should not be marked as read initially")
	}

	RecordRead(path)

	if !WasRead(path) {
		t.Error("File should be marked as read after RecordRead")
	}

	readTime, ok := GetReadTime(path)
	if !ok {
		t.Error("Should get read time")
	}
	if time.Since(readTime) > time.Second {
		t.Error("Read time should be recent")
	}
}

func TestAssertReadable(t *testing.T) {
	Clear()

	// Create temp file
	f, err := os.CreateTemp("", "test-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString("test")
	f.Close()

	// Should fail - not read yet
	if err := AssertReadable(f.Name()); err == nil {
		t.Error("Should fail for unread file")
	}

	// Record read
	RecordRead(f.Name())

	// Should pass now
	if err := AssertReadable(f.Name()); err != nil {
		t.Errorf("Should pass for read file: %v", err)
	}
}
```

## Verification
```bash
mkdir -p internal/tool/readtrack
# Write the files
go test ./internal/tool/readtrack/...
```

## Done When
- [ ] `internal/tool/readtrack/tracker.go` exists
- [ ] `internal/tool/readtrack/tracker_test.go` exists
- [ ] `go test ./internal/tool/readtrack/...` passes

## Then
Delete this file and exit.
