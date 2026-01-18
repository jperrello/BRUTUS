# BRUTUS File Subsystem Implementation Specification

## Overview

Implement OpenCode's file subsystem in Go, adapted for BRUTUS's architecture.

## Component Mapping

| OpenCode | BRUTUS | Notes |
|----------|--------|-------|
| File namespace | `file/` package | Core operations |
| Ripgrep module | `file/ripgrep.go` | Binary management |
| FileWatcher | `file/watcher.go` | Using fsnotify |
| FileIgnore | `file/ignore.go` | Pattern matching |

## Ripgrep Integration

### Binary Management

```go
package file

import (
    "archive/tar"
    "archive/zip"
    "compress/gzip"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "runtime"
)

type Platform struct {
    Arch      string
    Extension string
}

var platforms = map[string]Platform{
    "darwin-arm64": {"aarch64-apple-darwin", "tar.gz"},
    "darwin-amd64": {"x86_64-apple-darwin", "tar.gz"},
    "linux-arm64":  {"aarch64-unknown-linux-gnu", "tar.gz"},
    "linux-amd64":  {"x86_64-unknown-linux-musl", "tar.gz"},
    "windows-amd64": {"x86_64-pc-windows-msvc", "zip"},
}

const ripgrepVersion = "14.1.1"

func RipgrepPath() (string, error) {
    // 1. Check PATH
    if path, err := exec.LookPath("rg"); err == nil {
        return path, nil
    }

    // 2. Check cache
    binDir := filepath.Join(GlobalDataDir(), "bin")
    ext := ""
    if runtime.GOOS == "windows" {
        ext = ".exe"
    }
    cached := filepath.Join(binDir, "rg"+ext)
    if _, err := os.Stat(cached); err == nil {
        return cached, nil
    }

    // 3. Download
    key := runtime.GOOS + "-" + runtime.GOARCH
    platform, ok := platforms[key]
    if !ok {
        return "", fmt.Errorf("unsupported platform: %s", key)
    }

    return downloadRipgrep(binDir, platform)
}
```

### File Listing

```go
type FilesOptions struct {
    Cwd      string
    Glob     []string
    Hidden   bool
    Follow   bool
    MaxDepth int
}

func Files(ctx context.Context, opts FilesOptions) iter.Seq[string] {
    return func(yield func(string) bool) {
        rgPath, err := RipgrepPath()
        if err != nil {
            return
        }

        args := []string{"--files", "--glob=!.git/*"}
        if opts.Follow {
            args = append(args, "--follow")
        }
        if opts.Hidden {
            args = append(args, "--hidden")
        }
        if opts.MaxDepth > 0 {
            args = append(args, fmt.Sprintf("--max-depth=%d", opts.MaxDepth))
        }
        for _, g := range opts.Glob {
            args = append(args, fmt.Sprintf("--glob=%s", g))
        }

        cmd := exec.CommandContext(ctx, rgPath, args...)
        cmd.Dir = opts.Cwd

        stdout, _ := cmd.StdoutPipe()
        cmd.Start()

        scanner := bufio.NewScanner(stdout)
        for scanner.Scan() {
            if !yield(scanner.Text()) {
                cmd.Process.Kill()
                return
            }
        }
        cmd.Wait()
    }
}
```

### Tree Generation

```go
type TreeNode struct {
    Path     []string
    Children []*TreeNode
}

func Tree(ctx context.Context, cwd string, limit int) (string, error) {
    if limit <= 0 {
        limit = 50
    }

    root := &TreeNode{Children: make([]*TreeNode, 0)}

    // Collect files
    for file := range Files(ctx, FilesOptions{Cwd: cwd, Hidden: true, Follow: true}) {
        if strings.Contains(file, ".opencode") {
            continue
        }
        parts := strings.Split(file, string(filepath.Separator))
        insertPath(root, parts)
    }

    // Sort: directories first, then alphabetical
    sortTree(root)

    // BFS truncation
    return renderTree(root, limit), nil
}
```

### JSON Search

```go
type Match struct {
    Path       string
    LineNumber int
    Line       string
    Submatches []Submatch
}

type Submatch struct {
    Text  string
    Start int
    End   int
}

func Search(ctx context.Context, cwd, pattern string, opts SearchOptions) ([]Match, error) {
    rgPath, err := RipgrepPath()
    if err != nil {
        return nil, err
    }

    args := []string{"--json", "--hidden", "--glob=!.git/*"}
    if opts.Follow {
        args = append(args, "--follow")
    }
    for _, g := range opts.Glob {
        args = append(args, fmt.Sprintf("--glob=%s", g))
    }
    if opts.Limit > 0 {
        args = append(args, fmt.Sprintf("--max-count=%d", opts.Limit))
    }
    args = append(args, "--", pattern)

    cmd := exec.CommandContext(ctx, rgPath, args...)
    cmd.Dir = cwd

    output, _ := cmd.Output()

    var matches []Match
    for _, line := range bytes.Split(output, []byte("\n")) {
        if len(line) == 0 {
            continue
        }
        var result ripgrepResult
        json.Unmarshal(line, &result)
        if result.Type == "match" {
            matches = append(matches, parseMatch(result.Data))
        }
    }

    return matches, nil
}
```

## File Watcher

Use fsnotify for cross-platform watching:

```go
package file

import (
    "github.com/fsnotify/fsnotify"
)

type WatchEvent struct {
    Path  string
    Event string // "add", "change", "unlink"
}

type Watcher struct {
    watcher  *fsnotify.Watcher
    events   chan WatchEvent
    ignore   *IgnorePatterns
    done     chan struct{}
}

func NewWatcher(projectDir string, ignore *IgnorePatterns) (*Watcher, error) {
    w, err := fsnotify.NewWatcher()
    if err != nil {
        return nil, err
    }

    watcher := &Watcher{
        watcher: w,
        events:  make(chan WatchEvent, 100),
        ignore:  ignore,
        done:    make(chan struct{}),
    }

    go watcher.loop()

    // Watch project directory
    w.Add(projectDir)

    // Watch .git/HEAD for branch changes
    gitHead := filepath.Join(projectDir, ".git", "HEAD")
    if _, err := os.Stat(gitHead); err == nil {
        w.Add(filepath.Dir(gitHead))
    }

    return watcher, nil
}

func (w *Watcher) loop() {
    for {
        select {
        case event := <-w.watcher.Events:
            if w.ignore.Match(event.Name) {
                continue
            }
            var evtType string
            switch {
            case event.Op&fsnotify.Create != 0:
                evtType = "add"
            case event.Op&fsnotify.Write != 0:
                evtType = "change"
            case event.Op&fsnotify.Remove != 0:
                evtType = "unlink"
            default:
                continue
            }
            w.events <- WatchEvent{Path: event.Name, Event: evtType}

        case <-w.done:
            return
        }
    }
}

func (w *Watcher) Events() <-chan WatchEvent {
    return w.events
}

func (w *Watcher) Close() {
    close(w.done)
    w.watcher.Close()
}
```

## Ignore Patterns

```go
package file

import (
    "path/filepath"
    "strings"
)

var IgnoreFolders = map[string]bool{
    "node_modules":   true,
    "bower_components": true,
    ".pnpm-store":    true,
    "vendor":         true,
    ".npm":           true,
    "dist":           true,
    "build":          true,
    "out":            true,
    ".next":          true,
    "target":         true,
    "bin":            true,
    "obj":            true,
    ".git":           true,
    ".svn":           true,
    ".hg":            true,
    ".vscode":        true,
    ".idea":          true,
    ".turbo":         true,
    ".output":        true,
    ".sst":           true,
    ".cache":         true,
    "__pycache__":    true,
    ".pytest_cache":  true,
    ".gradle":        true,
}

var IgnorePatterns = []string{
    "**/*.swp",
    "**/*.swo",
    "**/*.pyc",
    "**/.DS_Store",
    "**/Thumbs.db",
    "**/logs/**",
    "**/tmp/**",
    "**/temp/**",
    "**/*.log",
    "**/coverage/**",
}

type IgnorePatterns struct {
    folders  map[string]bool
    patterns []glob.Glob
    extra    []glob.Glob
}

func (i *IgnorePatterns) Match(path string) bool {
    parts := strings.Split(path, string(filepath.Separator))
    for _, part := range parts {
        if i.folders[part] {
            return true
        }
    }

    for _, pattern := range append(i.patterns, i.extra...) {
        if pattern.Match(path) {
            return true
        }
    }

    return false
}
```

## File Reading

```go
type FileContent struct {
    Type     string  `json:"type"`
    Content  string  `json:"content"`
    Diff     string  `json:"diff,omitempty"`
    Encoding string  `json:"encoding,omitempty"`
    MimeType string  `json:"mimeType,omitempty"`
}

func ReadFile(projectDir, relPath string) (*FileContent, error) {
    fullPath := filepath.Join(projectDir, relPath)

    // Security: path containment check
    if !strings.HasPrefix(filepath.Clean(fullPath), filepath.Clean(projectDir)) {
        return nil, errors.New("access denied: path escapes project directory")
    }

    data, err := os.ReadFile(fullPath)
    if os.IsNotExist(err) {
        return &FileContent{Type: "text", Content: ""}, nil
    }
    if err != nil {
        return nil, err
    }

    // Binary detection
    mimeType := http.DetectContentType(data)
    if isBinary(mimeType) {
        return &FileContent{
            Type:     "text",
            Content:  base64.StdEncoding.EncodeToString(data),
            Encoding: "base64",
            MimeType: mimeType,
        }, nil
    }

    content := strings.TrimSpace(string(data))

    // Git diff if available
    diff := getGitDiff(projectDir, relPath)

    return &FileContent{
        Type:    "text",
        Content: content,
        Diff:    diff,
    }, nil
}

func isBinary(mimeType string) bool {
    if strings.HasPrefix(mimeType, "text/") {
        return false
    }
    if strings.Contains(mimeType, "charset=") {
        return false
    }

    binaryTypes := []string{
        "image/", "audio/", "video/", "font/", "model/",
        "application/zip", "application/gzip", "application/pdf",
        "application/octet-stream",
    }

    for _, t := range binaryTypes {
        if strings.Contains(mimeType, t) {
            return true
        }
    }

    return false
}
```

## Fuzzy Search

Use go-fuzzyfinder or similar:

```go
import (
    "github.com/sahilm/fuzzy"
)

type FileCache struct {
    mu    sync.RWMutex
    files []string
    dirs  []string
}

func (c *FileCache) Search(query string, limit int, kind string) []string {
    c.mu.RLock()
    defer c.mu.RUnlock()

    var items []string
    switch kind {
    case "file":
        items = c.files
    case "directory":
        items = c.dirs
    default:
        items = append(c.files, c.dirs...)
    }

    if query == "" {
        if len(items) > limit {
            return items[:limit]
        }
        return items
    }

    matches := fuzzy.Find(query, items)
    result := make([]string, 0, limit)
    for i, m := range matches {
        if i >= limit {
            break
        }
        result = append(result, m.Str)
    }

    return result
}
```

## Integration Points

### With Agent Loop
- File read tool uses ReadFile
- List tool uses List
- Search tool uses Search

### With Event System
- FileWatcher publishes to bus
- Agent can subscribe for file changes

### With Session
- File cache is per-project
- Disposed when session ends

## Testing Strategy

1. **Ripgrep binary management**
   - Mock HTTP for download tests
   - Test extraction on all platforms
   - Test PATH detection

2. **File operations**
   - Path escape prevention
   - Binary detection
   - Git diff integration

3. **Watcher**
   - Create/modify/delete events
   - Ignore pattern filtering
   - Graceful shutdown

4. **Search**
   - Fuzzy matching accuracy
   - Hidden file handling
   - Performance with large repos
