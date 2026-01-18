# BRUTUS Configuration Implementation Specification

## Overview

This document specifies how BRUTUS should implement configuration loading based on the OpenCode configuration subsystem. BRUTUS, being a Go application, will adapt the TypeScript patterns to idiomatic Go while preserving the core semantics.

## Implementation Priority

### Must Have (Phase 1)
1. JSONC parsing with comments
2. Hierarchical config loading (project > user > global)
3. Environment variable interpolation `{env:VAR}`
4. Basic schema validation
5. Permission configuration

### Should Have (Phase 2)
1. File inclusion `{file:path}`
2. Agent configuration from JSON
3. Plugin array deduplication
4. Backwards compatibility migrations

### Nice to Have (Phase 3)
1. Markdown-based agent/command loading
2. Remote well-known config fetching
3. Automatic dependency installation

## Go Architecture

### Package Structure

```
config/
├── config.go          // Core Config type and loading
├── schema.go          // Zod-like schema definitions
├── merge.go           // Deep merge utilities
├── jsonc.go           // JSONC parser wrapper
├── interpolate.go     // Variable/file substitution
├── loader.go          // File discovery and loading
├── extensions.go      // Markdown agent/command loading
└── migrate.go         // Backwards compatibility
```

### Core Types

```go
package config

type Config struct {
    Schema           string                    `json:"$schema,omitempty"`
    Theme            string                    `json:"theme,omitempty"`
    LogLevel         string                    `json:"logLevel,omitempty"`
    Model            string                    `json:"model,omitempty"`
    SmallModel       string                    `json:"small_model,omitempty"`
    DefaultAgent     string                    `json:"default_agent,omitempty"`
    Username         string                    `json:"username,omitempty"`
    Share            ShareMode                 `json:"share,omitempty"`
    Snapshot         bool                      `json:"snapshot,omitempty"`
    Plugin           []string                  `json:"plugin,omitempty"`
    Instructions     []string                  `json:"instructions,omitempty"`
    DisabledProviders []string                 `json:"disabled_providers,omitempty"`
    EnabledProviders []string                  `json:"enabled_providers,omitempty"`

    Keybinds    *Keybinds                     `json:"keybinds,omitempty"`
    TUI         *TUIConfig                    `json:"tui,omitempty"`
    Server      *ServerConfig                 `json:"server,omitempty"`
    Watcher     *WatcherConfig                `json:"watcher,omitempty"`
    Compaction  *CompactionConfig             `json:"compaction,omitempty"`

    Command     map[string]*Command           `json:"command,omitempty"`
    Agent       map[string]*Agent             `json:"agent,omitempty"`
    Provider    map[string]*Provider          `json:"provider,omitempty"`
    MCP         map[string]*MCPConfig         `json:"mcp,omitempty"`
    LSP         map[string]*LSPConfig         `json:"lsp,omitempty"`
    Formatter   map[string]*FormatterConfig   `json:"formatter,omitempty"`
    Permission  *Permission                   `json:"permission,omitempty"`

    Experimental *ExperimentalConfig          `json:"experimental,omitempty"`

    // Deprecated fields for migration
    Tools       map[string]bool               `json:"tools,omitempty"`
    Autoshare   *bool                         `json:"autoshare,omitempty"`
    Mode        map[string]*Agent             `json:"mode,omitempty"`
}

type ShareMode string
const (
    ShareManual   ShareMode = "manual"
    ShareAuto     ShareMode = "auto"
    ShareDisabled ShareMode = "disabled"
)

type PermissionAction string
const (
    PermissionAsk   PermissionAction = "ask"
    PermissionAllow PermissionAction = "allow"
    PermissionDeny  PermissionAction = "deny"
)

type Permission struct {
    Read              PermissionRule `json:"read,omitempty"`
    Edit              PermissionRule `json:"edit,omitempty"`
    Glob              PermissionRule `json:"glob,omitempty"`
    Grep              PermissionRule `json:"grep,omitempty"`
    List              PermissionRule `json:"list,omitempty"`
    Bash              PermissionRule `json:"bash,omitempty"`
    Task              PermissionRule `json:"task,omitempty"`
    ExternalDirectory PermissionRule `json:"external_directory,omitempty"`
    TodoWrite         PermissionAction `json:"todowrite,omitempty"`
    TodoRead          PermissionAction `json:"todoread,omitempty"`
    Question          PermissionAction `json:"question,omitempty"`
    WebFetch          PermissionAction `json:"webfetch,omitempty"`
    WebSearch         PermissionAction `json:"websearch,omitempty"`
    CodeSearch        PermissionAction `json:"codesearch,omitempty"`
    LSP               PermissionRule `json:"lsp,omitempty"`
    DoomLoop          PermissionAction `json:"doom_loop,omitempty"`

    Extra map[string]PermissionRule `json:"-"` // Catch-all
}

type PermissionRule struct {
    Action PermissionAction           // Simple action
    Rules  map[string]PermissionAction // Pattern-based rules
}

type Agent struct {
    Model       string            `json:"model,omitempty"`
    Temperature *float64          `json:"temperature,omitempty"`
    TopP        *float64          `json:"top_p,omitempty"`
    Prompt      string            `json:"prompt,omitempty"`
    Disable     bool              `json:"disable,omitempty"`
    Description string            `json:"description,omitempty"`
    Mode        AgentMode         `json:"mode,omitempty"`
    Hidden      bool              `json:"hidden,omitempty"`
    Color       string            `json:"color,omitempty"`
    Steps       *int              `json:"steps,omitempty"`
    Permission  *Permission       `json:"permission,omitempty"`
    Options     map[string]any    `json:"options,omitempty"`

    // Deprecated
    MaxSteps *int                 `json:"maxSteps,omitempty"`
    Tools    map[string]bool      `json:"tools,omitempty"`
}

type AgentMode string
const (
    AgentModeSubagent AgentMode = "subagent"
    AgentModePrimary  AgentMode = "primary"
    AgentModeAll      AgentMode = "all"
)

type Command struct {
    Template    string `json:"template"`
    Description string `json:"description,omitempty"`
    Agent       string `json:"agent,omitempty"`
    Model       string `json:"model,omitempty"`
    Subtask     bool   `json:"subtask,omitempty"`
}

type MCPConfig struct {
    Type        string            `json:"type"` // "local" or "remote"

    // Local
    Command     []string          `json:"command,omitempty"`
    Environment map[string]string `json:"environment,omitempty"`

    // Remote
    URL     string            `json:"url,omitempty"`
    Headers map[string]string `json:"headers,omitempty"`
    OAuth   *MCPOAuth         `json:"oauth,omitempty"`

    // Common
    Enabled *bool `json:"enabled,omitempty"`
    Timeout *int  `json:"timeout,omitempty"`
}

type MCPOAuth struct {
    ClientID     string `json:"clientId,omitempty"`
    ClientSecret string `json:"clientSecret,omitempty"`
    Scope        string `json:"scope,omitempty"`
}
```

### Loading Function

```go
package config

import (
    "os"
    "path/filepath"
    "sync"
)

type State struct {
    Config      *Config
    Directories []string
}

var (
    state     *State
    stateOnce sync.Once
    stateMu   sync.RWMutex
)

func Load(projectDir string) (*State, error) {
    stateMu.Lock()
    defer stateMu.Unlock()

    result := &Config{}
    directories := []string{}

    // Phase 1: Global config
    globalConfig, err := loadGlobal()
    if err != nil {
        return nil, err
    }
    result = mergeConfig(result, globalConfig)

    // Phase 2: Custom config path
    if envConfig := os.Getenv("BRUTUS_CONFIG"); envConfig != "" {
        custom, err := loadFile(envConfig)
        if err != nil {
            return nil, err
        }
        result = mergeConfig(result, custom)
    }

    // Phase 3: Project config (findUp)
    projectConfigs, err := findUp(projectDir, []string{"brutus.jsonc", "brutus.json"})
    if err != nil {
        return nil, err
    }
    for i := len(projectConfigs) - 1; i >= 0; i-- { // Bottom-up
        cfg, err := loadFile(projectConfigs[i])
        if err != nil {
            return nil, err
        }
        result = mergeConfig(result, cfg)
    }

    // Phase 4: Inline config
    if content := os.Getenv("BRUTUS_CONFIG_CONTENT"); content != "" {
        inline, err := parseJSONC([]byte(content))
        if err != nil {
            return nil, err
        }
        result = mergeConfig(result, inline)
    }

    // Phase 5: Apply defaults
    applyDefaults(result)

    // Phase 6: Migrations
    migrateDeprecatedFields(result)

    // Phase 7: Flag overrides
    applyFlagOverrides(result)

    // Phase 8: Deduplicate plugins
    result.Plugin = deduplicatePlugins(result.Plugin)

    state = &State{
        Config:      result,
        Directories: directories,
    }

    return state, nil
}

func Get() *Config {
    stateMu.RLock()
    defer stateMu.RUnlock()

    if state == nil {
        return &Config{}
    }
    return state.Config
}

func Reload() error {
    stateMu.Lock()
    state = nil
    stateMu.Unlock()

    _, err := Load(getCurrentProjectDir())
    return err
}
```

### JSONC Parser

```go
package config

import (
    "regexp"
    "strings"
    "encoding/json"
)

func parseJSONC(data []byte) (*Config, error) {
    // Remove single-line comments
    singleLineRe := regexp.MustCompile(`//.*$`)

    // Remove multi-line comments
    multiLineRe := regexp.MustCompile(`/\*[\s\S]*?\*/`)

    text := string(data)

    // Process line by line for single-line comments
    // But preserve strings
    lines := strings.Split(text, "\n")
    for i, line := range lines {
        // Simple heuristic: remove // not inside strings
        // More robust: use a state machine
        lines[i] = removeLineComment(line)
    }
    text = strings.Join(lines, "\n")

    // Remove multi-line comments
    text = multiLineRe.ReplaceAllString(text, "")

    // Remove trailing commas before } or ]
    trailingCommaRe := regexp.MustCompile(`,\s*([\]}])`)
    text = trailingCommaRe.ReplaceAllString(text, "$1")

    var cfg Config
    if err := json.Unmarshal([]byte(text), &cfg); err != nil {
        return nil, fmt.Errorf("jsonc parse error: %w", err)
    }

    return &cfg, nil
}

func removeLineComment(line string) string {
    inString := false
    escape := false

    for i := 0; i < len(line); i++ {
        if escape {
            escape = false
            continue
        }

        c := line[i]

        if c == '\\' && inString {
            escape = true
            continue
        }

        if c == '"' {
            inString = !inString
            continue
        }

        if !inString && i+1 < len(line) && line[i:i+2] == "//" {
            return line[:i]
        }
    }

    return line
}
```

### Variable Interpolation

```go
package config

import (
    "os"
    "path/filepath"
    "regexp"
    "strings"
)

var (
    envVarRe  = regexp.MustCompile(`\{env:([^}]+)\}`)
    fileRefRe = regexp.MustCompile(`\{file:([^}]+)\}`)
)

func interpolate(text string, configDir string) (string, error) {
    // Phase 1: Environment variables
    text = envVarRe.ReplaceAllStringFunc(text, func(match string) string {
        varName := envVarRe.FindStringSubmatch(match)[1]
        return os.Getenv(varName)
    })

    // Phase 2: File references
    matches := fileRefRe.FindAllStringSubmatch(text, -1)
    for _, match := range matches {
        fullMatch := match[0]
        filePath := match[1]

        // Skip if in a comment
        if isInComment(text, fullMatch) {
            continue
        }

        // Expand ~
        if strings.HasPrefix(filePath, "~/") {
            home, _ := os.UserHomeDir()
            filePath = filepath.Join(home, filePath[2:])
        }

        // Resolve relative paths
        if !filepath.IsAbs(filePath) {
            filePath = filepath.Join(configDir, filePath)
        }

        content, err := os.ReadFile(filePath)
        if err != nil {
            return "", fmt.Errorf("file reference %s: %w", fullMatch, err)
        }

        // JSON escape the content
        escaped := jsonEscape(strings.TrimSpace(string(content)))
        text = strings.Replace(text, fullMatch, escaped, 1)
    }

    return text, nil
}

func jsonEscape(s string) string {
    b, _ := json.Marshal(s)
    // Remove surrounding quotes
    return string(b[1 : len(b)-1])
}

func isInComment(text, substr string) bool {
    idx := strings.Index(text, substr)
    if idx == -1 {
        return false
    }

    // Check if there's // before it on the same line
    lineStart := strings.LastIndex(text[:idx], "\n") + 1
    linePrefix := text[lineStart:idx]

    return strings.Contains(linePrefix, "//")
}
```

### Deep Merge

```go
package config

import (
    "reflect"
)

func mergeConfig(base, overlay *Config) *Config {
    if base == nil {
        return overlay
    }
    if overlay == nil {
        return base
    }

    result := &Config{}

    // Copy base
    *result = *base

    // Merge with overlay
    mergeStruct(reflect.ValueOf(result).Elem(), reflect.ValueOf(overlay).Elem())

    // Special handling for arrays that should concatenate
    result.Plugin = deduplicateStrings(append(base.Plugin, overlay.Plugin...))
    result.Instructions = deduplicateStrings(append(base.Instructions, overlay.Instructions...))

    return result
}

func mergeStruct(base, overlay reflect.Value) {
    for i := 0; i < overlay.NumField(); i++ {
        overlayField := overlay.Field(i)
        baseField := base.Field(i)

        if !overlayField.IsValid() || isZero(overlayField) {
            continue
        }

        switch overlayField.Kind() {
        case reflect.Ptr:
            if !overlayField.IsNil() {
                if baseField.IsNil() {
                    baseField.Set(overlayField)
                } else if overlayField.Elem().Kind() == reflect.Struct {
                    mergeStruct(baseField.Elem(), overlayField.Elem())
                } else {
                    baseField.Set(overlayField)
                }
            }
        case reflect.Map:
            if !overlayField.IsNil() {
                if baseField.IsNil() {
                    baseField.Set(reflect.MakeMap(overlayField.Type()))
                }
                for _, key := range overlayField.MapKeys() {
                    baseField.SetMapIndex(key, overlayField.MapIndex(key))
                }
            }
        case reflect.Slice:
            // Don't overwrite slices here - handled specially above
            if baseField.IsNil() {
                baseField.Set(overlayField)
            }
        default:
            if !isZero(overlayField) {
                baseField.Set(overlayField)
            }
        }
    }
}

func isZero(v reflect.Value) bool {
    return reflect.DeepEqual(v.Interface(), reflect.Zero(v.Type()).Interface())
}

func deduplicateStrings(s []string) []string {
    seen := make(map[string]bool)
    result := make([]string, 0, len(s))
    for _, v := range s {
        if !seen[v] {
            seen[v] = true
            result = append(result, v)
        }
    }
    return result
}
```

### Plugin Deduplication

```go
package config

import (
    "net/url"
    "path/filepath"
    "strings"
)

func getPluginName(plugin string) string {
    // file:///path/to/plugin/foo.js → "foo"
    if strings.HasPrefix(plugin, "file://") {
        u, err := url.Parse(plugin)
        if err == nil {
            base := filepath.Base(u.Path)
            ext := filepath.Ext(base)
            return strings.TrimSuffix(base, ext)
        }
    }

    // oh-my-brutus@2.4.3 → "oh-my-brutus"
    // @scope/pkg@1.0.0 → "@scope/pkg"
    lastAt := strings.LastIndex(plugin, "@")
    if lastAt > 0 {
        return plugin[:lastAt]
    }

    return plugin
}

func deduplicatePlugins(plugins []string) []string {
    seenNames := make(map[string]bool)
    unique := make([]string, 0)

    // Process in reverse order (highest priority first)
    for i := len(plugins) - 1; i >= 0; i-- {
        name := getPluginName(plugins[i])
        if !seenNames[name] {
            seenNames[name] = true
            unique = append([]string{plugins[i]}, unique...)
        }
    }

    return unique
}
```

### Migrations

```go
package config

import "os"

func migrateDeprecatedFields(cfg *Config) {
    // Migrate tools → permission
    if cfg.Tools != nil && cfg.Permission == nil {
        cfg.Permission = &Permission{}
    }
    if cfg.Tools != nil {
        for tool, enabled := range cfg.Tools {
            action := PermissionDeny
            if enabled {
                action = PermissionAllow
            }

            switch tool {
            case "write", "edit", "patch", "multiedit":
                cfg.Permission.Edit = PermissionRule{Action: action}
            default:
                if cfg.Permission.Extra == nil {
                    cfg.Permission.Extra = make(map[string]PermissionRule)
                }
                cfg.Permission.Extra[tool] = PermissionRule{Action: action}
            }
        }
    }

    // Migrate autoshare → share
    if cfg.Autoshare != nil && *cfg.Autoshare && cfg.Share == "" {
        cfg.Share = ShareAuto
    }

    // Migrate mode → agent
    if cfg.Mode != nil {
        if cfg.Agent == nil {
            cfg.Agent = make(map[string]*Agent)
        }
        for name, mode := range cfg.Mode {
            mode.Mode = AgentModePrimary
            cfg.Agent[name] = mode
        }
    }

    // Migrate agent.maxSteps → agent.steps
    for _, agent := range cfg.Agent {
        if agent.Steps == nil && agent.MaxSteps != nil {
            agent.Steps = agent.MaxSteps
        }
    }
}

func applyDefaults(cfg *Config) {
    if cfg.Username == "" {
        cfg.Username = os.Getenv("USER")
        if cfg.Username == "" {
            cfg.Username = "user"
        }
    }

    if cfg.Agent == nil {
        cfg.Agent = make(map[string]*Agent)
    }

    if cfg.Plugin == nil {
        cfg.Plugin = []string{}
    }
}

func applyFlagOverrides(cfg *Config) {
    if os.Getenv("BRUTUS_DISABLE_AUTOCOMPACT") != "" {
        if cfg.Compaction == nil {
            cfg.Compaction = &CompactionConfig{}
        }
        cfg.Compaction.Auto = boolPtr(false)
    }

    if os.Getenv("BRUTUS_DISABLE_PRUNE") != "" {
        if cfg.Compaction == nil {
            cfg.Compaction = &CompactionConfig{}
        }
        cfg.Compaction.Prune = boolPtr(false)
    }

    if permJSON := os.Getenv("BRUTUS_PERMISSION"); permJSON != "" {
        var perm Permission
        if err := json.Unmarshal([]byte(permJSON), &perm); err == nil {
            if cfg.Permission == nil {
                cfg.Permission = &perm
            } else {
                // Merge
            }
        }
    }
}

func boolPtr(b bool) *bool {
    return &b
}
```

### File Discovery

```go
package config

import (
    "os"
    "path/filepath"
)

func loadGlobal() (*Config, error) {
    configDir := getConfigDir()

    var result *Config

    // Load in order of precedence
    files := []string{
        filepath.Join(configDir, "config.json"),
        filepath.Join(configDir, "brutus.json"),
        filepath.Join(configDir, "brutus.jsonc"),
    }

    for _, file := range files {
        cfg, err := loadFile(file)
        if err != nil {
            if os.IsNotExist(err) {
                continue
            }
            return nil, err
        }
        result = mergeConfig(result, cfg)
    }

    return result, nil
}

func getConfigDir() string {
    // XDG_CONFIG_HOME or ~/.config
    if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
        return filepath.Join(xdg, "brutus")
    }

    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".config", "brutus")
}

func findUp(start string, targets []string) ([]string, error) {
    var results []string
    current := start

    for {
        for _, target := range targets {
            path := filepath.Join(current, target)
            if _, err := os.Stat(path); err == nil {
                results = append(results, path)
            }
        }

        parent := filepath.Dir(current)
        if parent == current {
            break // Reached root
        }
        current = parent
    }

    return results, nil
}

func loadFile(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    configDir := filepath.Dir(path)
    text := string(data)

    // Interpolate variables
    text, err = interpolate(text, configDir)
    if err != nil {
        return nil, err
    }

    // Parse JSONC
    return parseJSONC([]byte(text))
}
```

## File Format Reference

### brutus.jsonc Example

```jsonc
{
  // BRUTUS Configuration
  "$schema": "https://brutus.dev/config.json",

  // Default model
  "model": "saturn/o1",

  // Sharing disabled
  "share": "disabled",

  // Custom agents
  "agent": {
    "researcher": {
      "description": "Research-focused agent",
      "prompt": "You are a research assistant...",
      "mode": "subagent",
      "steps": 20
    }
  },

  // Permissions
  "permission": {
    "bash": {
      "npm *": "allow",
      "rm -rf *": "deny",
      "*": "ask"
    },
    "edit": "allow",
    "webfetch": "ask"
  },

  // MCP servers
  "mcp": {
    "playwright": {
      "type": "local",
      "command": ["npx", "@anthropic/mcp-playwright"],
      "enabled": true
    }
  },

  // API key from environment
  "provider": {
    "anthropic": {
      "options": {
        "apiKey": "{env:ANTHROPIC_API_KEY}"
      }
    }
  }
}
```

## Testing Strategy

```go
func TestConfigLoading(t *testing.T) {
    tests := []struct {
        name     string
        files    map[string]string
        env      map[string]string
        expected *Config
    }{
        {
            name: "basic project config",
            files: map[string]string{
                "brutus.json": `{"model": "test/model"}`,
            },
            expected: &Config{Model: "test/model"},
        },
        {
            name: "env var interpolation",
            files: map[string]string{
                "brutus.json": `{"model": "{env:TEST_MODEL}"}`,
            },
            env: map[string]string{"TEST_MODEL": "env/model"},
            expected: &Config{Model: "env/model"},
        },
        {
            name: "jsonc comments",
            files: map[string]string{
                "brutus.jsonc": `{
                    // This is a comment
                    "model": "test/model",
                }`,
            },
            expected: &Config{Model: "test/model"},
        },
        {
            name: "merge precedence",
            files: map[string]string{
                "~/.config/brutus/brutus.json": `{"model": "global/model", "theme": "dark"}`,
                "brutus.json":                   `{"model": "project/model"}`,
            },
            expected: &Config{Model: "project/model", Theme: "dark"},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Setup test directory and files
            // Set environment variables
            // Load config
            // Assert equality
        })
    }
}
```
