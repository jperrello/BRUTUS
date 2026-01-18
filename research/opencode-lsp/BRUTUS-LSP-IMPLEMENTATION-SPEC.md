# BRUTUS LSP Implementation Specification

## Overview

This document outlines how BRUTUS could implement LSP support based on OpenCode's architecture, adapted for Go.

## Proposed Architecture

```
brutus/
├── lsp/
│   ├── lsp.go           // Main LSP manager (like OpenCode's index.ts)
│   ├── client.go        // LSP client connection (like client.ts)
│   ├── server.go        // Server configurations (like server.ts)
│   ├── language.go      // Extension to language mapping
│   └── download.go      // Auto-download logic for missing servers
├── tools/
│   └── lsp.go           // LSP tool for agent (like lsp.ts)
```

## Core Interfaces

### Server Configuration

```go
package lsp

type ServerInfo struct {
    ID         string
    Extensions []string
    RootMarkers []string
    Spawn      func(root string) (*Handle, error)
}

type Handle struct {
    Process *exec.Cmd
    Stdin   io.WriteCloser
    Stdout  io.ReadCloser
    InitOpts map[string]any
}

var Servers = []ServerInfo{
    {
        ID:         "gopls",
        Extensions: []string{".go"},
        RootMarkers: []string{"go.mod"},
        Spawn: func(root string) (*Handle, error) {
            bin, err := findOrInstall("gopls", "go", "install", "golang.org/x/tools/gopls@latest")
            if err != nil {
                return nil, err
            }
            cmd := exec.Command(bin, "serve")
            cmd.Dir = root
            // ... setup pipes
            return &Handle{Process: cmd, ...}, nil
        },
    },
    // More servers...
}
```

### LSP Client

```go
package lsp

type Client struct {
    conn        *jsonrpc2.Conn
    serverID    string
    root        string
    diagnostics sync.Map  // map[string][]Diagnostic
}

func NewClient(handle *Handle, root string) (*Client, error) {
    // Create JSON-RPC connection over stdio
    stream := jsonrpc2.NewBufferedStream(
        &readWriteCloser{handle.Stdout, handle.Stdin},
        jsonrpc2.VSCodeObjectCodec{},
    )
    conn := jsonrpc2.NewConn(context.Background(), stream, &handler{})

    // Send initialize request
    var result InitializeResult
    ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
    defer cancel()

    err := conn.Call(ctx, "initialize", InitializeParams{
        ProcessID: os.Getpid(),
        RootURI:   "file://" + root,
        Capabilities: ClientCapabilities{
            TextDocument: TextDocumentClientCapabilities{
                Synchronization: TextDocumentSyncClientCapabilities{
                    DidOpen: true,
                    DidChange: true,
                },
            },
        },
    }, &result)
    if err != nil {
        return nil, fmt.Errorf("initialize failed: %w", err)
    }

    // Send initialized notification
    conn.Notify(context.Background(), "initialized", struct{}{})

    return &Client{conn: conn, root: root}, nil
}
```

### LSP Manager

```go
package lsp

type Manager struct {
    clients  []*Client
    spawning sync.Map  // map[string]*sync.WaitGroup
    broken   sync.Map  // map[string]bool
    mu       sync.RWMutex
}

var DefaultManager = &Manager{}

func (m *Manager) TouchFile(file string, waitDiagnostics bool) error {
    ext := filepath.Ext(file)

    for _, server := range Servers {
        if !contains(server.Extensions, ext) {
            continue
        }

        if _, broken := m.broken.Load(server.ID); broken {
            continue
        }

        // Find project root
        root := findRoot(file, server.RootMarkers)
        if root == "" {
            continue
        }

        // Get or spawn client
        client, err := m.getOrSpawnClient(server, root)
        if err != nil {
            m.broken.Store(server.ID, true)
            continue
        }

        // Notify file open
        client.DidOpen(file)

        if waitDiagnostics {
            client.WaitForDiagnostics(file, 3*time.Second)
        }
    }
    return nil
}
```

## Tool Implementation

```go
package tools

var LspTool = NewTool[LspInput]("lsp", `
Interact with Language Server Protocol servers for code intelligence.

Operations:
- goToDefinition: Find where a symbol is defined
- findReferences: Find all references to a symbol
- hover: Get documentation/type info
- documentSymbol: List all symbols in a file
- workspaceSymbol: Search symbols across workspace
- goToImplementation: Find interface implementations
- incomingCalls: Find callers of a function
- outgoingCalls: Find functions called by this function

Parameters:
- operation: The LSP operation to perform
- filePath: Path to the file
- line: Line number (1-based)
- character: Column number (1-based)
`, executeLsp)

type LspInput struct {
    Operation string `json:"operation"`
    FilePath  string `json:"filePath"`
    Line      int    `json:"line"`
    Character int    `json:"character"`
}

func executeLsp(input LspInput, ctx *ToolContext) (*ToolResult, error) {
    // Resolve path
    file := input.FilePath
    if !filepath.IsAbs(file) {
        file = filepath.Join(ctx.WorkDir, file)
    }

    // Check file exists
    if _, err := os.Stat(file); os.IsNotExist(err) {
        return nil, fmt.Errorf("file not found: %s", file)
    }

    // Check LSP available
    if !lsp.HasClients(file) {
        return nil, fmt.Errorf("no LSP server available for this file type")
    }

    // Ensure server ready
    lsp.DefaultManager.TouchFile(file, true)

    // Convert 1-based to 0-based
    position := lsp.Position{
        File:      file,
        Line:      input.Line - 1,
        Character: input.Character - 1,
    }

    // Execute operation
    var result any
    var err error

    switch input.Operation {
    case "goToDefinition":
        result, err = lsp.Definition(position)
    case "findReferences":
        result, err = lsp.References(position)
    case "hover":
        result, err = lsp.Hover(position)
    case "documentSymbol":
        result, err = lsp.DocumentSymbol(file)
    case "workspaceSymbol":
        result, err = lsp.WorkspaceSymbol("")
    case "goToImplementation":
        result, err = lsp.Implementation(position)
    case "incomingCalls":
        result, err = lsp.IncomingCalls(position)
    case "outgoingCalls":
        result, err = lsp.OutgoingCalls(position)
    default:
        return nil, fmt.Errorf("unknown operation: %s", input.Operation)
    }

    if err != nil {
        return nil, err
    }

    output, _ := json.MarshalIndent(result, "", "  ")
    if len(output) == 0 || string(output) == "[]" || string(output) == "null" {
        return &ToolResult{
            Output: fmt.Sprintf("No results found for %s", input.Operation),
        }, nil
    }

    return &ToolResult{
        Output: string(output),
    }, nil
}
```

## Server Auto-Download

```go
package lsp

func findOrDownload(serverID, binaryName string, downloadFunc func() (string, error)) (string, error) {
    // 1. Check PATH
    if bin, err := exec.LookPath(binaryName); err == nil {
        return bin, nil
    }

    // 2. Check cache directory
    cacheDir := filepath.Join(os.UserHomeDir(), ".brutus", "bin")
    cachedBin := filepath.Join(cacheDir, binaryName)
    if runtime.GOOS == "windows" {
        cachedBin += ".exe"
    }

    if _, err := os.Stat(cachedBin); err == nil {
        return cachedBin, nil
    }

    // 3. Check if downloads disabled
    if os.Getenv("BRUTUS_DISABLE_LSP_DOWNLOAD") != "" {
        return "", fmt.Errorf("binary not found and downloads disabled: %s", binaryName)
    }

    // 4. Download
    return downloadFunc()
}

func downloadGithubRelease(owner, repo, assetPattern string) (string, error) {
    // Fetch latest release
    resp, _ := http.Get(fmt.Sprintf(
        "https://api.github.com/repos/%s/%s/releases/latest",
        owner, repo,
    ))
    var release struct {
        Assets []struct {
            Name               string `json:"name"`
            BrowserDownloadURL string `json:"browser_download_url"`
        } `json:"assets"`
    }
    json.NewDecoder(resp.Body).Decode(&release)

    // Find matching asset
    platform := map[string]string{
        "darwin":  "macos",
        "linux":   "linux",
        "windows": "windows",
    }[runtime.GOOS]

    arch := map[string]string{
        "amd64": "x86_64",
        "arm64": "aarch64",
    }[runtime.GOARCH]

    for _, asset := range release.Assets {
        if strings.Contains(asset.Name, platform) &&
           strings.Contains(asset.Name, arch) {
            return downloadAndExtract(asset.BrowserDownloadURL)
        }
    }

    return "", fmt.Errorf("no matching asset found")
}
```

## Priority Server Configurations

For initial implementation, focus on these high-value servers:

| Priority | Server | Why |
|----------|--------|-----|
| 1 | gopls | Go project, essential for self-improvement |
| 2 | typescript-language-server | Most common web dev language |
| 3 | rust-analyzer | Systems programming, well-supported |
| 4 | pyright | Python very common, good LSP |
| 5 | clangd | C/C++ for systems work |

## Go JSON-RPC Library

Use `github.com/sourcegraph/jsonrpc2` for the LSP connection:

```go
import "github.com/sourcegraph/jsonrpc2"

// Or use the official LSP types from:
import "go.lsp.dev/protocol"
```

## Diagnostic Integration

```go
type DiagnosticPublisher interface {
    PublishDiagnostics(file string, diagnostics []Diagnostic)
}

func (c *Client) handleNotification(ctx context.Context, conn *jsonrpc2.Conn, r *jsonrpc2.Request) {
    if r.Method == "textDocument/publishDiagnostics" {
        var params PublishDiagnosticsParams
        json.Unmarshal(*r.Params, &params)

        file := uriToPath(params.URI)
        c.diagnostics.Store(file, params.Diagnostics)

        // Publish to bus for UI/agent consumption
        bus.Publish("lsp.diagnostics", DiagnosticEvent{
            ServerID:    c.serverID,
            File:        file,
            Diagnostics: params.Diagnostics,
        })
    }
}
```

## Key Differences from OpenCode

| Aspect | OpenCode | BRUTUS |
|--------|----------|--------|
| Language | TypeScript/Bun | Go |
| JSON-RPC | vscode-jsonrpc | sourcegraph/jsonrpc2 |
| Async | Promise-based | goroutine + channel |
| Config | JSON file | Go struct |
| Package manager | npm/bun | go install |

## Testing Strategy

```go
func TestLspTool(t *testing.T) {
    // Create temp Go file
    tmpDir := t.TempDir()
    file := filepath.Join(tmpDir, "main.go")
    os.WriteFile(file, []byte(`package main

func hello() string {
    return "world"
}

func main() {
    x := hello()
    println(x)
}
`), 0644)

    // Initialize go.mod
    exec.Command("go", "mod", "init", "test").Dir(tmpDir).Run()

    // Test goToDefinition
    result, err := executeLsp(LspInput{
        Operation: "goToDefinition",
        FilePath:  file,
        Line:      9,       // x := hello()
        Character: 7,       // cursor on "hello"
    }, &ToolContext{WorkDir: tmpDir})

    require.NoError(t, err)
    require.Contains(t, result.Output, "line\": 3")  // hello() defined on line 3
}
```

## Implementation Phases

### Phase 1: Core Infrastructure
- [ ] LSP client with JSON-RPC
- [ ] gopls integration (single server)
- [ ] Basic tool with goToDefinition

### Phase 2: Full Tool
- [ ] All 9 operations
- [ ] Multiple server support
- [ ] Diagnostic aggregation

### Phase 3: Auto-Download
- [ ] GitHub release downloads
- [ ] go install integration
- [ ] Platform detection

### Phase 4: Polish
- [ ] Server health monitoring
- [ ] Graceful shutdown
- [ ] Configuration file support
