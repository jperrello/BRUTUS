# BRUTUS Patch Implementation Specification

## Recommendation: DO NOT IMPLEMENT

Based on this research, I recommend **not** implementing a dedicated patch tool for BRUTUS. OpenCode itself has deprecated this mechanism in favor of `edit`/`multiedit`. The reasons apply equally to BRUTUS:

1. **LLM Generation Quality**: Patch format is harder for models to generate correctly
2. **Debugging Difficulty**: Multi-file atomic operations obscure which change failed
3. **Simpler Alternatives**: Sequential single-file edits with TodoWrite tracking achieves similar goals

## What to Learn From This Research

### 1. Context-Aware Matching (Useful)

The `seekSequence` algorithm could enhance BRUTUS's edit tool:

```go
// Current BRUTUS edit: first occurrence wins
// Enhanced: context-guided matching

type ContextHint struct {
    NearLine string  // Find changes near this line
    After    bool    // Match must be after context
}

func EditWithContext(file, old, new string, ctx *ContextHint) error
```

### 2. Atomic Multi-File Operations (Maybe Later)

If BRUTUS needs transactional multi-file edits:

```go
type EditBatch struct {
    Edits []Edit
}

func (b *EditBatch) Apply() error {
    // 1. Validate all edits exist
    // 2. Snapshot all original content
    // 3. Apply edits
    // 4. On failure: rollback using snapshots
}
```

### 3. Unified Diff Generation (Useful)

For better tool output, generate diffs:

```go
import "github.com/sergi/go-diff/diffmatchpatch"

func GenerateDiff(original, modified string) string {
    dmp := diffmatchpatch.New()
    diffs := dmp.DiffMain(original, modified, false)
    return dmp.DiffPrettyText(diffs)
}
```

## If You Must Implement: Minimal Spec

### Patch Format (Simplified)

```
*** BRUTUS Patch v1
*** Add: path/to/file.go
+content line 1
+content line 2
*** Update: path/to/existing.go
-old line
+new line
*** Delete: path/to/remove.go
*** End Patch
```

### Go Types

```go
package patch

type Hunk interface {
    Apply(baseDir string) error
}

type AddHunk struct {
    Path     string
    Contents string
}

type DeleteHunk struct {
    Path string
}

type UpdateHunk struct {
    Path   string
    Chunks []Chunk
}

type Chunk struct {
    OldLines []string
    NewLines []string
}

func Parse(text string) ([]Hunk, error)
func Apply(hunks []Hunk, baseDir string) error
```

### Tool Definition

```go
var PatchTool = tools.NewTool[PatchInput](
    "patch",
    "Apply a patch to modify multiple files atomically",
    func(input PatchInput, ctx tools.Context) (string, error) {
        hunks, err := patch.Parse(input.PatchText)
        if err != nil {
            return "", fmt.Errorf("parse: %w", err)
        }
        if err := patch.Apply(hunks, ctx.WorkDir); err != nil {
            return "", err
        }
        return fmt.Sprintf("Applied %d changes", len(hunks)), nil
    },
)

type PatchInput struct {
    PatchText string `json:"patchText"`
}
```

## Priority: Low

This research is primarily archival. Focus implementation effort on:

1. **Edit tool improvements** (context hints, better error messages)
2. **Snapshot/undo system** (from opencode-snapshot research)
3. **MultiEdit wrapper** (sequential Edit calls, not new format)

## Files to Reference

When enhancing BRUTUS edit capabilities, reference:

- `LINE-MATCHING-ALGORITHM.md` - For seekSequence logic
- `PATCH-FORMAT-SPEC.md` - For understanding OpenCode's format choices
- `research/opencode-edit-tool/FUZZY-REPLACEMENT-SPEC.md` - Related algorithm
