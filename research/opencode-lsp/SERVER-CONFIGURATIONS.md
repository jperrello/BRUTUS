# OpenCode LSP Server Configurations

## Overview

OpenCode defines 40+ language server configurations in `server.ts`. Each follows a consistent pattern with auto-download capabilities.

## Server Definition Interface

```typescript
interface LSPServer.Info {
  id: string                              // Unique identifier
  extensions: string[]                    // File extensions this server handles
  root(file: string): string | null       // Project root finder
  spawn(): Promise<Handle | undefined>    // Start server process
}

interface Handle {
  process: ChildProcessWithoutNullStreams
  initialization?: object  // Custom LSP initialization options
}
```

## Root Detection Pattern

The `NearestRoot` utility searches upward for marker files:

```typescript
function NearestRoot(markers: string[], excludePatterns?: string[]) {
  return (file: string): string | null => {
    let dir = path.dirname(file)
    while (dir !== path.dirname(dir)) {  // Until filesystem root
      for (const marker of markers) {
        if (fs.existsSync(path.join(dir, marker))) {
          // Check excludePatterns
          return dir
        }
      }
      dir = path.dirname(dir)
    }
    return null
  }
}
```

## Server Categories

### JavaScript/TypeScript Ecosystem

| ID | Extensions | Root Markers | Binary |
|----|------------|--------------|--------|
| typescript | .ts, .tsx, .js, .jsx, .mjs, .cjs | package.json, tsconfig.json | typescript-language-server |
| deno | .ts, .tsx | deno.json, deno.jsonc | deno lsp |
| vue | .vue | package.json (with vue dep) | @vue/language-server |
| eslint | .ts, .tsx, .js, .jsx | package.json, eslint.config.* | vscode-eslint-language-server |
| biome | .ts, .tsx, .js, .jsx, .json | biome.json, biome.jsonc | biome lsp-proxy |

### Systems Languages

| ID | Extensions | Root Markers | Binary |
|----|------------|--------------|--------|
| rust-analyzer | .rs | Cargo.toml | rust-analyzer |
| clangd | .c, .cpp, .h, .hpp | compile_commands.json, CMakeLists.txt | clangd |
| gopls | .go | go.mod | gopls |
| zls | .zig | build.zig | zls (auto-download) |

### Python

| ID | Extensions | Root Markers | Binary |
|----|------------|--------------|--------|
| pyright | .py | pyproject.toml, requirements.txt | pyright-langserver |
| ty | .py | pyproject.toml | ty server (experimental) |

### JVM Languages

| ID | Extensions | Root Markers | Binary |
|----|------------|--------------|--------|
| jdtls | .java | pom.xml, build.gradle | eclipse.jdt.ls |
| kotlin | .kt, .kts | build.gradle.kts | kotlin-language-server |

### Other Languages

| ID | Extensions | Root Markers | Binary |
|----|------------|--------------|--------|
| elixir-ls | .ex, .exs | mix.exs | elixir-ls |
| ruby-lsp | .rb | Gemfile | ruby-lsp |
| lua-ls | .lua | .luarc.json | lua-language-server |
| haskell | .hs | stack.yaml, cabal.project | haskell-language-server |
| swift | .swift | Package.swift | sourcekit-lsp |
| csharp | .cs | *.csproj, *.sln | OmniSharp |
| php | .php | composer.json | phpactor |
| prisma | .prisma | prisma/schema.prisma | prisma-language-server |
| dart | .dart | pubspec.yaml | dart language-server |
| ocaml | .ml, .mli | dune-project | ocamllsp |
| bash | .sh, .bash | - | bash-language-server |
| terraform | .tf | - | terraform-ls |
| texlab | .tex | - | texlab |
| docker | Dockerfile | - | docker-langserver |
| gleam | .gleam | gleam.toml | gleam lsp |
| clojure | .clj, .cljs | deps.edn | clojure-lsp |
| nix | .nix | flake.nix | nil |
| typst | .typ | - | tinymist |

## Auto-Download Mechanism

Many servers implement automatic binary downloading:

### Pattern: GitHub Release Download

```typescript
spawn: async () => {
  // 1. Check if binary exists in PATH
  let bin = Bun.which("zls")

  // 2. Check if already downloaded to Global.Path.bin
  if (!bin) {
    bin = path.join(Global.Path.bin, "zls" + (isWindows ? ".exe" : ""))
    if (!fs.existsSync(bin)) {
      // 3. Respect disable flag
      if (process.env.OPENCODE_DISABLE_LSP_DOWNLOAD) return undefined

      // 4. Fetch latest release from GitHub API
      const release = await fetch(
        "https://api.github.com/repos/zigtools/zls/releases/latest"
      ).then(r => r.json())

      // 5. Find matching asset for platform/arch
      const asset = release.assets.find(a =>
        a.name.includes(platform) && a.name.includes(arch)
      )

      // 6. Download and extract
      const response = await fetch(asset.browser_download_url)
      await extractTarGz(response.body, Global.Path.bin)
      await fs.chmod(bin, 0o755)
    }
  }

  // 7. Spawn process
  return {
    process: spawn(bin, [], { stdio: ["pipe", "pipe", "pipe"] })
  }
}
```

### Pattern: Package Manager Install

```typescript
// Go tools
spawn: async () => {
  let bin = Bun.which("gopls")
  if (!bin) {
    await execAsync("go install golang.org/x/tools/gopls@latest")
    bin = path.join(process.env.GOPATH || "~/go", "bin", "gopls")
  }
  return { process: spawn(bin, ["serve"]) }
}

// Ruby gems
spawn: async () => {
  let bin = Bun.which("ruby-lsp")
  if (!bin) {
    await execAsync("gem install ruby-lsp")
    bin = Bun.which("ruby-lsp")
  }
  return { process: spawn(bin, []) }
}

// .NET tools
spawn: async () => {
  let bin = Bun.which("csharp-ls")
  if (!bin) {
    await execAsync("dotnet tool install -g csharp-ls")
    bin = path.join(process.env.HOME, ".dotnet/tools/csharp-ls")
  }
  return { process: spawn(bin, []) }
}
```

## Platform-Specific Handling

```typescript
const platform = process.platform  // "darwin", "linux", "win32"
const arch = process.arch          // "x64", "arm64"

// Binary extension
const ext = platform === "win32" ? ".exe" : ""

// Asset name mapping
const platformMap = {
  darwin: "macos",
  linux: "linux",
  win32: "windows"
}

const archMap = {
  x64: "x86_64",
  arm64: "aarch64"
}
```

## Virtual Environment Support (Python)

```typescript
spawn: async () => {
  // 1. Check VIRTUAL_ENV environment variable
  let venvPath = process.env.VIRTUAL_ENV

  // 2. Check local .venv directory
  if (!venvPath) {
    const localVenv = path.join(projectRoot, ".venv")
    if (fs.existsSync(localVenv)) {
      venvPath = localVenv
    }
  }

  // 3. Use venv's python if available
  const python = venvPath
    ? path.join(venvPath, "bin", "python")
    : "python"

  return {
    process: spawn(python, ["-m", "pyright", "--langserver"])
  }
}
```

## Environment Variables

```typescript
// Set for Bun-based tooling
process.env.BUN_BE_BUN = "1"

// Disable all auto-downloads
process.env.OPENCODE_DISABLE_LSP_DOWNLOAD = "1"
```

## Spawn Command Patterns

| Server | Command |
|--------|---------|
| typescript | bun x typescript-language-server --stdio |
| rust-analyzer | rust-analyzer |
| gopls | gopls serve |
| pyright | pyright-langserver --stdio |
| clangd | clangd --background-index |
| deno | deno lsp |
| biome | biome lsp-proxy |
| vue | vue-language-server --stdio |

## Error Recovery

1. If spawn fails → server added to "broken" set
2. If binary not found and downloads disabled → return undefined (skip)
3. If download fails → throw error, mark broken
4. If initialization times out (45s) → mark broken
