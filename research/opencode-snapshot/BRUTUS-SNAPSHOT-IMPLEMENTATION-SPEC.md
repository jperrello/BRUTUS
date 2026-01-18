# BRUTUS Snapshot Implementation Specification

## Overview

This document specifies how to implement OpenCode-compatible snapshot functionality in BRUTUS (Go).

## Architecture Mapping

| OpenCode (TypeScript) | BRUTUS (Go) |
|-----------------------|-------------|
| `Snapshot` namespace | `snapshot` package |
| `Instance.worktree` | `agent.WorkDir` |
| `Instance.project.id` | Hash of working directory |
| `Global.Path.data` | `os.UserConfigDir()` + `/brutus` |
| Bun `$` shell | `exec.Command` |

## Package Structure

```
brutus/
└── snapshot/
    ├── snapshot.go      # Core API
    ├── shadow.go        # Shadow repo management
    └── snapshot_test.go # Tests
```

## Core Types

```go
package snapshot

type Patch struct {
    Hash  string   `json:"hash"`
    Files []string `json:"files"`
}

type FileDiff struct {
    File      string `json:"file"`
    Before    string `json:"before"`
    After     string `json:"after"`
    Additions int    `json:"additions"`
    Deletions int    `json:"deletions"`
}

type Config struct {
    Enabled  bool
    WorkDir  string
    DataDir  string
}
```

## Shadow Repository Path

```go
func shadowDir(cfg Config) string {
    projectID := sha256.Sum256([]byte(cfg.WorkDir))
    return filepath.Join(cfg.DataDir, "snapshot", hex.EncodeToString(projectID[:8]))
}
```

Using first 8 bytes of SHA256 of workdir path as project identifier (shorter, collision-unlikely).

## Core Functions

### Track

```go
func Track(cfg Config) (string, error) {
    if !cfg.Enabled {
        return "", nil
    }

    gitDir := shadowDir(cfg)

    // Initialize if needed
    if _, err := os.Stat(gitDir); os.IsNotExist(err) {
        if err := initShadowRepo(gitDir, cfg.WorkDir); err != nil {
            return "", fmt.Errorf("init shadow repo: %w", err)
        }
    }

    // Stage all files
    if err := gitCmd(gitDir, cfg.WorkDir, "add", "."); err != nil {
        return "", fmt.Errorf("git add: %w", err)
    }

    // Write tree and get hash
    hash, err := gitCmdOutput(gitDir, cfg.WorkDir, "write-tree")
    if err != nil {
        return "", fmt.Errorf("git write-tree: %w", err)
    }

    return strings.TrimSpace(hash), nil
}

func initShadowRepo(gitDir, workDir string) error {
    if err := os.MkdirAll(gitDir, 0755); err != nil {
        return err
    }

    // git init
    cmd := exec.Command("git", "init")
    cmd.Dir = workDir
    cmd.Env = append(os.Environ(),
        "GIT_DIR="+gitDir,
        "GIT_WORK_TREE="+workDir,
    )
    if err := cmd.Run(); err != nil {
        return err
    }

    // Disable autocrlf
    return gitCmd(gitDir, workDir, "config", "core.autocrlf", "false")
}
```

### Patch

```go
func GetPatch(cfg Config, hash string) (Patch, error) {
    gitDir := shadowDir(cfg)

    // Stage current files
    _ = gitCmd(gitDir, cfg.WorkDir, "add", ".")

    // Get changed files
    output, err := gitCmdOutput(gitDir, cfg.WorkDir,
        "-c", "core.autocrlf=false",
        "diff", "--no-ext-diff", "--name-only", hash, "--", ".")

    if err != nil {
        // Return empty patch on error (graceful degradation)
        return Patch{Hash: hash, Files: nil}, nil
    }

    var files []string
    for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
        if line == "" {
            continue
        }
        files = append(files, filepath.Join(cfg.WorkDir, line))
    }

    return Patch{Hash: hash, Files: files}, nil
}
```

### Restore

```go
func Restore(cfg Config, hash string) error {
    gitDir := shadowDir(cfg)

    // Read tree into index
    if err := gitCmd(gitDir, cfg.WorkDir, "read-tree", hash); err != nil {
        return fmt.Errorf("read-tree: %w", err)
    }

    // Checkout all files
    if err := gitCmd(gitDir, cfg.WorkDir, "checkout-index", "-a", "-f"); err != nil {
        return fmt.Errorf("checkout-index: %w", err)
    }

    return nil
}
```

### Revert

```go
func Revert(cfg Config, patches []Patch) error {
    gitDir := shadowDir(cfg)
    reverted := make(map[string]bool)

    for _, patch := range patches {
        for _, file := range patch.Files {
            if reverted[file] {
                continue
            }

            // Try to checkout file from snapshot
            err := gitCmd(gitDir, cfg.WorkDir, "checkout", patch.Hash, "--", file)
            if err != nil {
                // Check if file existed in snapshot
                relPath, _ := filepath.Rel(cfg.WorkDir, file)
                out, _ := gitCmdOutput(gitDir, cfg.WorkDir, "ls-tree", patch.Hash, "--", relPath)

                if strings.TrimSpace(out) == "" {
                    // File didn't exist in snapshot - delete it
                    os.Remove(file)
                }
                // If file existed but checkout failed, keep current version
            }

            reverted[file] = true
        }
    }

    return nil
}
```

### Diff

```go
func Diff(cfg Config, hash string) (string, error) {
    gitDir := shadowDir(cfg)

    _ = gitCmd(gitDir, cfg.WorkDir, "add", ".")

    output, err := gitCmdOutput(gitDir, cfg.WorkDir,
        "-c", "core.autocrlf=false",
        "diff", "--no-ext-diff", hash, "--", ".")

    if err != nil {
        return "", nil // Graceful degradation
    }

    return strings.TrimSpace(output), nil
}
```

### DiffFull

```go
func DiffFull(cfg Config, from, to string) ([]FileDiff, error) {
    gitDir := shadowDir(cfg)

    // Get numstat for additions/deletions
    output, err := gitCmdOutput(gitDir, cfg.WorkDir,
        "-c", "core.autocrlf=false",
        "diff", "--no-ext-diff", "--no-renames", "--numstat", from, to, "--", ".")

    if err != nil {
        return nil, err
    }

    var diffs []FileDiff
    for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
        if line == "" {
            continue
        }

        parts := strings.SplitN(line, "\t", 3)
        if len(parts) != 3 {
            continue
        }

        file := parts[2]
        isBinary := parts[0] == "-" && parts[1] == "-"

        var before, after string
        var additions, deletions int

        if !isBinary {
            before, _ = gitCmdOutput(gitDir, cfg.WorkDir,
                "-c", "core.autocrlf=false", "show", from+":"+file)
            after, _ = gitCmdOutput(gitDir, cfg.WorkDir,
                "-c", "core.autocrlf=false", "show", to+":"+file)
            additions, _ = strconv.Atoi(parts[0])
            deletions, _ = strconv.Atoi(parts[1])
        }

        diffs = append(diffs, FileDiff{
            File:      file,
            Before:    before,
            After:     after,
            Additions: additions,
            Deletions: deletions,
        })
    }

    return diffs, nil
}
```

## Helper Functions

```go
func gitCmd(gitDir, workDir string, args ...string) error {
    fullArgs := append([]string{"--git-dir", gitDir, "--work-tree", workDir}, args...)
    cmd := exec.Command("git", fullArgs...)
    cmd.Dir = workDir
    return cmd.Run()
}

func gitCmdOutput(gitDir, workDir string, args ...string) (string, error) {
    fullArgs := append([]string{"--git-dir", gitDir, "--work-tree", workDir}, args...)
    cmd := exec.Command("git", fullArgs...)
    cmd.Dir = workDir
    out, err := cmd.Output()
    return string(out), err
}
```

## Integration with Agent Loop

```go
// In agent loop, at step boundaries:

func (a *Agent) executeStep(step Step) error {
    // Track state before tools run
    hash, err := snapshot.Track(a.snapshotConfig)
    if err != nil {
        a.log.Warn("snapshot track failed", "err", err)
    }

    // Execute tools
    result, err := a.executeTool(step.Tool)

    // Capture changes after tools
    if hash != "" {
        patch, _ := snapshot.GetPatch(a.snapshotConfig, hash)
        if len(patch.Files) > 0 {
            a.recordPatch(step.ID, patch)
        }
    }

    return err
}
```

## Session Revert Integration

```go
type RevertState struct {
    MessageID string
    PartID    string
    Snapshot  string // Original snapshot for unrevert
    Diff      string // Diff showing what was reverted
}

func (s *Session) Revert(messageID, partID string) error {
    // Collect patches from target message forward
    patches := s.collectPatchesFrom(messageID, partID)

    // Save current state for unrevert
    originalSnapshot, _ := snapshot.Track(s.snapshotConfig)

    // Revert files
    if err := snapshot.Revert(s.snapshotConfig, patches); err != nil {
        return err
    }

    // Generate diff for display
    diff, _ := snapshot.Diff(s.snapshotConfig, originalSnapshot)

    s.revertState = &RevertState{
        MessageID: messageID,
        PartID:    partID,
        Snapshot:  originalSnapshot,
        Diff:      diff,
    }

    return nil
}

func (s *Session) Unrevert() error {
    if s.revertState == nil || s.revertState.Snapshot == "" {
        return nil
    }

    if err := snapshot.Restore(s.snapshotConfig, s.revertState.Snapshot); err != nil {
        return err
    }

    s.revertState = nil
    return nil
}
```

## Testing Strategy

```go
func TestTrackAndPatch(t *testing.T) {
    // Create temp directory
    tmpDir := t.TempDir()
    cfg := snapshot.Config{
        Enabled: true,
        WorkDir: tmpDir,
        DataDir: t.TempDir(),
    }

    // Create initial file
    os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("hello"), 0644)

    // Track
    hash, err := snapshot.Track(cfg)
    require.NoError(t, err)
    require.NotEmpty(t, hash)

    // Modify file
    os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("world"), 0644)

    // Get patch
    patch, err := snapshot.GetPatch(cfg, hash)
    require.NoError(t, err)
    require.Len(t, patch.Files, 1)
    require.Contains(t, patch.Files[0], "test.txt")
}
```

## Error Handling Philosophy

Following OpenCode's approach:
- All git commands use graceful degradation
- Failures return empty results rather than errors
- Logging captures issues for debugging
- Agent continues functioning even if snapshots fail

This ensures snapshot failures never block the agent's primary task.

## Performance Considerations

1. **Large Repos**: `git add .` can be slow for large directories
   - Consider using `git add --update` for subsequent tracks
   - Or tracking only specific paths

2. **Binary Files**: Large binaries bloat shadow repo
   - Consider `.gitattributes` to exclude large binaries
   - Or size limits on tracked files

3. **Disk Space**: Shadow repos accumulate objects
   - Implement periodic `git gc`
   - Or delete shadow repos after session ends

## Future Enhancements

1. **Partial Tracking**: Only track files modified by tools
2. **Compression**: Compress old snapshots
3. **Expiration**: Auto-delete snapshots older than N sessions
4. **Exclude Patterns**: Configure paths to exclude from tracking
