# Configuration Loading Algorithm Specification

## Overview

This document specifies the exact algorithm OpenCode uses to load, merge, and validate configuration. The algorithm is deterministic and produces consistent results given the same inputs.

## Algorithm Pseudocode

```
FUNCTION Config.state():
    result := {}
    directories := []

    // Phase 1: Remote Well-Known Configs
    FOR each (url, auth) IN Auth.all():
        IF auth.type == "wellknown":
            SET process.env[auth.key] = auth.token
            response := FETCH "{url}/.well-known/opencode"
            IF response.ok:
                wellknown := response.json()
                remoteConfig := wellknown.config ?? {}
                result := mergeConfigConcatArrays(result, load(remoteConfig, url))

    // Phase 2: Global User Config
    result := mergeConfigConcatArrays(result, global())

    // Phase 3: Custom Config Path (env var)
    IF Flag.OPENCODE_CONFIG:
        result := mergeConfigConcatArrays(result, loadFile(Flag.OPENCODE_CONFIG))

    // Phase 4: Project Config (findUp)
    FOR file IN ["opencode.jsonc", "opencode.json"]:
        found := Filesystem.findUp(file, Instance.directory, Instance.worktree)
        FOR resolved IN found.toReversed():  // Bottom-up order
            result := mergeConfigConcatArrays(result, loadFile(resolved))

    // Phase 5: Inline Config Content (env var)
    IF Flag.OPENCODE_CONFIG_CONTENT:
        result := mergeConfigConcatArrays(result, JSON.parse(Flag.OPENCODE_CONFIG_CONTENT))

    // Phase 6: Initialize defaults
    result.agent := result.agent ?? {}
    result.mode := result.mode ?? {}
    result.plugin := result.plugin ?? []

    // Phase 7: Collect extension directories
    directories := [
        Global.Path.config,
        ...Filesystem.up({targets: [".opencode"], start: Instance.directory, stop: Instance.worktree}),
        ...Filesystem.up({targets: [".opencode"], start: Global.Path.home, stop: Global.Path.home})
    ]
    IF Flag.OPENCODE_CONFIG_DIR:
        directories.push(Flag.OPENCODE_CONFIG_DIR)
    directories := unique(directories)

    // Phase 8: Load extensions from each directory
    FOR dir IN directories:
        IF dir.endsWith(".opencode") OR dir == Flag.OPENCODE_CONFIG_DIR:
            FOR file IN ["opencode.jsonc", "opencode.json"]:
                result := mergeConfigConcatArrays(result, loadFile(path.join(dir, file)))

        // Install dependencies if node_modules missing
        IF NOT exists(path.join(dir, "node_modules")):
            AWAIT installDependencies(dir)

        // Load markdown extensions
        result.command := mergeDeep(result.command ?? {}, loadCommand(dir))
        result.agent := mergeDeep(result.agent, loadAgent(dir))
        result.agent := mergeDeep(result.agent, loadMode(dir))
        result.plugin.push(...loadPlugin(dir))

    // Phase 9: Migrate deprecated mode field
    FOR (name, mode) IN result.mode:
        result.agent[name] := {...mode, mode: "primary"}

    // Phase 10: Flag overrides
    IF Flag.OPENCODE_PERMISSION:
        result.permission := mergeDeep(result.permission ?? {}, JSON.parse(Flag.OPENCODE_PERMISSION))

    // Phase 11: Backwards compatibility migrations
    result := migrateTools(result)
    result := migrateAutoshare(result)

    // Phase 12: Apply defaults
    IF NOT result.username:
        result.username := os.userInfo().username
    IF NOT result.keybinds:
        result.keybinds := Keybinds.parse({})

    // Phase 13: Apply compaction flag overrides
    IF Flag.OPENCODE_DISABLE_AUTOCOMPACT:
        result.compaction := {...result.compaction, auto: false}
    IF Flag.OPENCODE_DISABLE_PRUNE:
        result.compaction := {...result.compaction, prune: false}

    // Phase 14: Plugin deduplication
    result.plugin := deduplicatePlugins(result.plugin ?? [])

    RETURN { config: result, directories: directories }
```

## Phase Details

### Phase 1: Remote Well-Known Configs

Fetches configuration from authenticated remote endpoints. This allows organizations to distribute default configurations.

**Input**: `Auth.all()` returns map of `{url: {type, key, token}}`

**Process**:
1. Filter to `type === "wellknown"` entries
2. Set environment variable: `process.env[auth.key] = auth.token`
3. Fetch `{url}/.well-known/opencode`
4. Extract `.config` field from response (default `{}`)
5. Add `$schema` if missing
6. Parse through `load()` function
7. Merge with running result

**Error Handling**: Throws on non-OK HTTP response

### Phase 2: Global User Config

```typescript
const global = lazy(async () => {
  return pipe(
    {},
    mergeDeep(await loadFile(path.join(Global.Path.config, "config.json"))),
    mergeDeep(await loadFile(path.join(Global.Path.config, "opencode.json"))),
    mergeDeep(await loadFile(path.join(Global.Path.config, "opencode.jsonc"))),
  )
})
```

**Files checked** (in order):
1. `~/.config/opencode/config.json`
2. `~/.config/opencode/opencode.json`
3. `~/.config/opencode/opencode.jsonc`

**Legacy TOML Migration**: If `config` (no extension) exists, imports it, merges, writes to `config.json`, deletes original.

### Phase 3: Custom Config Path

**Condition**: `OPENCODE_CONFIG` environment variable is set

**Action**: Load file at that exact path

### Phase 4: Project Config Discovery

Uses `Filesystem.findUp()` to locate config files between project directory and worktree root.

**Algorithm**:
```
FUNCTION findUp(filename, start, stop):
    results := []
    current := start
    WHILE current != stop AND current != parent(current):
        IF exists(path.join(current, filename)):
            results.push(path.join(current, filename))
        current := parent(current)
    RETURN results
```

**Processing Order**: Results are reversed so deeper configs (closer to project) override shallower ones.

### Phase 5: Inline Config Content

**Condition**: `OPENCODE_CONFIG_CONTENT` environment variable is set

**Action**: Parse JSON string directly (no file loading)

### Phase 7: Extension Directory Collection

Collects directories to scan for markdown-based extensions.

**Sources**:
1. `Global.Path.config` (~/.config/opencode)
2. `.opencode/` directories found via `Filesystem.up()` from Instance.directory
3. `.opencode/` directories found via `Filesystem.up()` from home directory
4. `OPENCODE_CONFIG_DIR` (if set)

**Deduplication**: `unique(directories)` removes duplicates while preserving order.

### Phase 8: Extension Loading

For each directory:

1. **Config files**: Load `opencode.jsonc` and `opencode.json` if directory ends with `.opencode` or is OPENCODE_CONFIG_DIR
2. **Dependencies**: Run `installDependencies()` if `node_modules/` missing
3. **Commands**: Glob `{command,commands}/**/*.md`
4. **Agents**: Glob `{agent,agents}/**/*.md`
5. **Modes**: Glob `{mode,modes}/*.md`
6. **Plugins**: Glob `{plugin,plugins}/*.{ts,js}`

### Phase 11: Backwards Compatibility

**Tools Migration**:
```typescript
if (result.tools) {
  const perms = {}
  for (const [tool, enabled] of Object.entries(result.tools)) {
    const action = enabled ? "allow" : "deny"
    if (["write", "edit", "patch", "multiedit"].includes(tool)) {
      perms.edit = action
    } else {
      perms[tool] = action
    }
  }
  result.permission = mergeDeep(perms, result.permission ?? {})
}
```

**Autoshare Migration**:
```typescript
if (result.autoshare === true && !result.share) {
  result.share = "auto"
}
```

## loadFile() Function

```
FUNCTION loadFile(filepath):
    text := READ filepath
    IF ENOENT error:
        RETURN {}
    IF other error:
        THROW JsonError(path=filepath)
    RETURN load(text, filepath)
```

## load() Function

```
FUNCTION load(text, configFilepath):
    // Step 1: Environment variable substitution
    text := text.replace(/\{env:([^}]+)\}/g, (_, varName) => process.env[varName] || "")

    // Step 2: File inclusion
    fileMatches := text.match(/\{file:[^}]+\}/g)
    IF fileMatches:
        configDir := path.dirname(configFilepath)
        lines := text.split("\n")

        FOR match IN fileMatches:
            lineIndex := lines.findIndex(line => line.includes(match))
            IF lines[lineIndex].trim().startsWith("//"):
                CONTINUE  // Skip commented lines

            filePath := match.replace(/^\{file:/, "").replace(/\}$/, "")
            IF filePath.startsWith("~/"):
                filePath := path.join(os.homedir(), filePath.slice(2))
            resolvedPath := path.isAbsolute(filePath) ? filePath : path.resolve(configDir, filePath)

            fileContent := READ resolvedPath
            IF ENOENT:
                THROW InvalidError(path=configFilepath, message="bad file reference")
            IF other error:
                THROW InvalidError(path=configFilepath, message="bad file reference")

            // Escape for JSON embedding
            escaped := JSON.stringify(fileContent.trim()).slice(1, -1)
            text := text.replace(match, escaped)

    // Step 3: JSONC parsing
    errors := []
    data := parseJsonc(text, errors, {allowTrailingComma: true})

    IF errors.length > 0:
        errorDetails := formatErrors(errors, text)
        THROW JsonError(path=configFilepath, message=errorDetails)

    // Step 4: Schema validation
    parsed := Info.safeParse(data)
    IF NOT parsed.success:
        THROW InvalidError(path=configFilepath, issues=parsed.error.issues)

    // Step 5: Auto-add schema and write back
    IF NOT parsed.data.$schema:
        parsed.data.$schema := "https://opencode.ai/config.json"
        WRITE configFilepath, JSON.stringify(parsed.data, null, 2)

    // Step 6: Resolve plugin paths
    IF parsed.data.plugin:
        FOR i IN range(parsed.data.plugin.length):
            TRY:
                parsed.data.plugin[i] := import.meta.resolve(parsed.data.plugin[i], configFilepath)
            CATCH:
                // Keep original if resolution fails

    RETURN parsed.data
```

## loadCommand() Function

```
FUNCTION loadCommand(dir):
    result := {}
    GLOB := "{command,commands}/**/*.md"

    FOR item IN GLOB.scan(dir):
        md := ConfigMarkdown.parse(item)
        IF parse error:
            Bus.publish(Session.Event.Error, {error})
            CONTINUE

        patterns := ["/.opencode/command/", "/.opencode/commands/", "/command/", "/commands/"]
        file := rel(item, patterns) ?? path.basename(item)
        name := trim(file)  // Remove extension

        config := {
            name: name,
            ...md.data,         // Frontmatter
            template: md.content.trim()
        }

        parsed := Command.safeParse(config)
        IF parsed.success:
            result[config.name] := parsed.data
        ELSE:
            THROW InvalidError(path=item, issues=parsed.error.issues)

    RETURN result
```

## loadAgent() Function

```
FUNCTION loadAgent(dir):
    result := {}
    GLOB := "{agent,agents}/**/*.md"

    FOR item IN GLOB.scan(dir):
        md := ConfigMarkdown.parse(item)
        IF parse error:
            Bus.publish(Session.Event.Error, {error})
            CONTINUE

        patterns := ["/.opencode/agent/", "/.opencode/agents/", "/agent/", "/agents/"]
        file := rel(item, patterns) ?? path.basename(item)
        agentName := trim(file)

        config := {
            name: agentName,
            ...md.data,
            prompt: md.content.trim()
        }

        parsed := Agent.safeParse(config)
        IF parsed.success:
            result[config.name] := parsed.data
        ELSE:
            THROW InvalidError(path=item, issues=parsed.error.issues)

    RETURN result
```

## loadPlugin() Function

```
FUNCTION loadPlugin(dir):
    plugins := []
    GLOB := "{plugin,plugins}/*.{ts,js}"

    FOR item IN GLOB.scan(dir):
        plugins.push(pathToFileURL(item).href)

    RETURN plugins
```

## mergeConfigConcatArrays() Function

```
FUNCTION mergeConfigConcatArrays(target, source):
    merged := mergeDeep(target, source)  // Standard deep merge

    // Special handling: concatenate arrays instead of replace
    IF target.plugin AND source.plugin:
        merged.plugin := Array.from(new Set([...target.plugin, ...source.plugin]))

    IF target.instructions AND source.instructions:
        merged.instructions := Array.from(new Set([...target.instructions, ...source.instructions]))

    RETURN merged
```

## deduplicatePlugins() Function

```
FUNCTION deduplicatePlugins(plugins):
    seenNames := Set()
    uniqueSpecifiers := []

    // Process in reverse order (highest priority first)
    FOR specifier IN plugins.toReversed():
        name := getPluginName(specifier)
        IF NOT seenNames.has(name):
            seenNames.add(name)
            uniqueSpecifiers.push(specifier)

    // Restore original order
    RETURN uniqueSpecifiers.toReversed()


FUNCTION getPluginName(plugin):
    // file:///path/to/plugin/foo.js → "foo"
    IF plugin.startsWith("file://"):
        RETURN path.parse(new URL(plugin).pathname).name

    // oh-my-opencode@2.4.3 → "oh-my-opencode"
    // @scope/pkg@1.0.0 → "@scope/pkg"
    lastAt := plugin.lastIndexOf("@")
    IF lastAt > 0:
        RETURN plugin.substring(0, lastAt)

    RETURN plugin
```

## installDependencies() Function

```
FUNCTION installDependencies(dir):
    // Ensure package.json exists
    pkg := path.join(dir, "package.json")
    IF NOT exists(pkg):
        WRITE pkg, "{}"

    // Create .gitignore if missing
    gitignore := path.join(dir, ".gitignore")
    IF NOT exists(gitignore):
        WRITE gitignore, "node_modules\npackage.json\nbun.lock\n.gitignore"

    // Install plugin SDK
    version := Installation.isLocal() ? "latest" : Installation.VERSION
    BunProc.run(["add", "@opencode-ai/plugin@" + version, "--exact"], {cwd: dir})

    // Install other dependencies
    BunProc.run(["install"], {cwd: dir})
```

## ConfigMarkdown.parse() Function

```
FUNCTION ConfigMarkdown.parse(filePath):
    raw := READ filePath
    template := preprocessFrontmatter(raw)

    TRY:
        md := matter(template)  // gray-matter library
        RETURN md
    CATCH err:
        THROW FrontmatterError(path=filePath, message=err.message)


FUNCTION preprocessFrontmatter(content):
    match := content.match(/^---\r?\n([\s\S]*?)\r?\n---/)
    IF NOT match:
        RETURN content

    frontmatter := match[1]
    lines := frontmatter.split("\n")
    result := []

    FOR line IN lines:
        // Skip comments and empty lines
        IF line.trim().startsWith("#") OR line.trim() == "":
            result.push(line)
            CONTINUE

        // Skip continuation lines (indented)
        IF line.match(/^\s+/):
            result.push(line)
            CONTINUE

        // Match key: value pattern
        kvMatch := line.match(/^([a-zA-Z_][a-zA-Z0-9_]*)\s*:\s*(.*)$/)
        IF NOT kvMatch:
            result.push(line)
            CONTINUE

        key := kvMatch[1]
        value := kvMatch[2].trim()

        // Skip if already handled
        IF value == "" OR value == ">" OR value == "|"
           OR value.startsWith('"') OR value.startsWith("'"):
            result.push(line)
            CONTINUE

        // If value contains colon, convert to block scalar
        IF value.includes(":"):
            result.push(key + ": |")
            result.push("  " + value)
            CONTINUE

        result.push(line)

    processed := result.join("\n")
    RETURN content.replace(frontmatter, processed)
```

## Caching

Configuration is cached per-instance via `Instance.state()`:

```typescript
export const state = Instance.state(async () => {
  // ... loading logic ...
  return { config, directories }
})

export async function get() {
  return state().then(x => x.config)
}

export async function update(config: Info) {
  const filepath = path.join(Instance.directory, "config.json")
  const existing = await loadFile(filepath)
  await Bun.write(filepath, JSON.stringify(mergeDeep(existing, config), null, 2))
  await Instance.dispose()  // Invalidate cache
}
```

Cache invalidation occurs when:
- `update()` is called
- `Instance.dispose()` is called
- A new instance is created
