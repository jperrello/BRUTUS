# OpenCode File Subsystem Research

This research covers OpenCode's file handling infrastructure:
- Ripgrep integration for fast file listing and code search
- File watcher for real-time change detection
- Ignore pattern matching
- File content reading with binary detection
- Git-aware diff/status integration
- Fuzzy file search

## Files

| File | Description |
|------|-------------|
| `FILE-SUBSYSTEM-SPEC.md` | Architecture overview and data flows |
| `RIPGREP-INTEGRATION-SPEC.md` | Binary download, file listing, tree, search |
| `WATCHER-SPEC.md` | File watcher with @parcel/watcher |
| `BRUTUS-FILE-IMPLEMENTATION-SPEC.md` | How to implement in BRUTUS |

## Key Insights

1. **Ripgrep is bundled** - OpenCode downloads platform-specific ripgrep binaries at runtime
2. **Tree generation uses BFS** - Breadth-first tree rendering with truncation limits
3. **Binary detection** - MIME type inspection to decide base64 encoding
4. **Fuzzy search** - Uses `fuzzysort` library for file path matching
5. **Git-aware** - Automatic diff computation for changed files
