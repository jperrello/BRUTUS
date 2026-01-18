# BRUTUS Skill Implementation Specification

## Overview

This document specifies how to implement the Skill subsystem in BRUTUS based on reverse-engineering OpenCode's implementation.

## Required Components

### 1. Skill Discovery Module

**File:** `skill/skill.go`

```go
package skill

type Info struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    Location    string `json:"location"`
}

type Registry struct {
    skills map[string]Info
    mu     sync.RWMutex
}

func NewRegistry() *Registry
func (r *Registry) Scan(dirs []string) error
func (r *Registry) Get(name string) (Info, bool)
func (r *Registry) All() []Info
```

### 2. Skill File Parser

**File:** `skill/parser.go`

Parse SKILL.md files with YAML frontmatter:

```go
package skill

type ParsedSkill struct {
    Name        string
    Description string
    Content     string  // Markdown body
    Dir         string  // Parent directory
}

func ParseSkillFile(path string) (*ParsedSkill, error)
```

Dependencies:
- YAML parser (e.g., `gopkg.in/yaml.v3`)
- Frontmatter extraction

### 3. Skill Tool

**File:** `tools/skill.go`

```go
package tools

type SkillInput struct {
    Name string `json:"name" description:"The skill identifier"`
}

type SkillOutput struct {
    Title    string            `json:"title"`
    Output   string            `json:"output"`
    Metadata map[string]string `json:"metadata"`
}

var SkillTool = NewTool[SkillInput]("skill", description, skillExecute)
```

## Directory Scanning

### Scan Paths (Priority Order)

1. Project `.claude/skills/`
2. User `~/.claude/skills/`
3. Project `.opencode/skill/` or `.opencode/skills/`
4. User `~/.config/opencode/skill/`

### Glob Patterns

```go
patterns := []string{
    "skills/**/SKILL.md",      // .claude directory pattern
    "{skill,skills}/**/SKILL.md", // .opencode pattern
}
```

### Walk Algorithm

```go
func (r *Registry) Scan(dirs []string) error {
    for _, dir := range dirs {
        filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
            if d.Name() == "SKILL.md" {
                skill, err := ParseSkillFile(path)
                if err != nil {
                    log.Warn("failed to parse skill", "path", path, "error", err)
                    return nil // Continue scanning
                }
                r.mu.Lock()
                if existing, ok := r.skills[skill.Name]; ok {
                    log.Warn("duplicate skill name",
                        "name", skill.Name,
                        "existing", existing.Location,
                        "duplicate", path)
                }
                r.skills[skill.Name] = Info{
                    Name:        skill.Name,
                    Description: skill.Description,
                    Location:    path,
                }
                r.mu.Unlock()
            }
            return nil
        })
    }
    return nil
}
```

## YAML Frontmatter Parsing

### Preprocessing

Handle values with colons by converting to block scalar:

```go
func preprocessFrontmatter(content string) string {
    // Match YAML frontmatter
    re := regexp.MustCompile(`(?s)^---\r?\n(.*?)\r?\n---`)
    match := re.FindStringSubmatch(content)
    if match == nil {
        return content
    }

    frontmatter := match[1]
    lines := strings.Split(frontmatter, "\n")
    var result []string

    for _, line := range lines {
        // Skip comments, empty lines, indented lines
        trimmed := strings.TrimSpace(line)
        if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
            result = append(result, line)
            continue
        }

        // Match key: value pattern
        kvRe := regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*)\s*:\s*(.*)$`)
        kvMatch := kvRe.FindStringSubmatch(line)
        if kvMatch == nil {
            result = append(result, line)
            continue
        }

        key := kvMatch[1]
        value := strings.TrimSpace(kvMatch[2])

        // Skip if already handled
        if value == "" || value == ">" || value == "|" ||
           strings.HasPrefix(value, "\"") || strings.HasPrefix(value, "'") {
            result = append(result, line)
            continue
        }

        // Convert values with colons to block scalar
        if strings.Contains(value, ":") {
            result = append(result, key+": |")
            result = append(result, "  "+value)
            continue
        }

        result = append(result, line)
    }

    processed := strings.Join(result, "\n")
    return re.ReplaceAllString(content, "---\n"+processed+"\n---")
}
```

### Parsing

```go
func ParseSkillFile(path string) (*ParsedSkill, error) {
    content, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    preprocessed := preprocessFrontmatter(string(content))

    // Extract frontmatter
    re := regexp.MustCompile(`(?s)^---\r?\n(.*?)\r?\n---\r?\n(.*)$`)
    match := re.FindStringSubmatch(preprocessed)
    if match == nil {
        return nil, fmt.Errorf("no frontmatter found")
    }

    var frontmatter struct {
        Name        string `yaml:"name"`
        Description string `yaml:"description"`
    }

    if err := yaml.Unmarshal([]byte(match[1]), &frontmatter); err != nil {
        return nil, fmt.Errorf("invalid frontmatter: %w", err)
    }

    if frontmatter.Name == "" {
        return nil, fmt.Errorf("skill name is required")
    }

    return &ParsedSkill{
        Name:        frontmatter.Name,
        Description: frontmatter.Description,
        Content:     strings.TrimSpace(match[2]),
        Dir:         filepath.Dir(path),
    }, nil
}
```

## Tool Implementation

### Dynamic Description

```go
func buildDescription(skills []Info) string {
    if len(skills) == 0 {
        return "Load a skill to get detailed instructions for a specific task. No skills are currently available."
    }

    var sb strings.Builder
    sb.WriteString("Load a skill to get detailed instructions for a specific task. ")
    sb.WriteString("Skills provide specialized knowledge and step-by-step guidance. ")
    sb.WriteString("Use this when a task matches an available skill's description. ")
    sb.WriteString("<available_skills>")

    for _, skill := range skills {
        sb.WriteString(fmt.Sprintf(
            " <skill> <name>%s</name> <description>%s</description> </skill>",
            skill.Name, skill.Description))
    }

    sb.WriteString("</available_skills>")
    return sb.String()
}
```

### Execution

```go
func skillExecute(input SkillInput, ctx Context) (*SkillOutput, error) {
    skill, ok := registry.Get(input.Name)
    if !ok {
        available := registry.All()
        names := make([]string, len(available))
        for i, s := range available {
            names[i] = s.Name
        }
        return nil, fmt.Errorf("skill %q not found. Available skills: %s",
            input.Name, strings.Join(names, ", "))
    }

    parsed, err := ParseSkillFile(skill.Location)
    if err != nil {
        return nil, fmt.Errorf("failed to load skill: %w", err)
    }

    output := fmt.Sprintf("## Skill: %s\n\n**Base directory**: %s\n\n%s",
        skill.Name, parsed.Dir, parsed.Content)

    return &SkillOutput{
        Title:  fmt.Sprintf("Loaded skill: %s", skill.Name),
        Output: output,
        Metadata: map[string]string{
            "name": skill.Name,
            "dir":  parsed.Dir,
        },
    }, nil
}
```

## Integration Points

### main.go

```go
func main() {
    // Initialize skill registry
    skillRegistry := skill.NewRegistry()

    // Discover skills from standard paths
    dirs := skill.DiscoveryPaths(workdir)
    if err := skillRegistry.Scan(dirs); err != nil {
        log.Warn("skill discovery failed", "error", err)
    }

    // Build skill tool with current registry
    skillTool := tools.NewSkillTool(skillRegistry)

    // Register tool
    registry.Register(skillTool)
}
```

### Discovery Paths

```go
func DiscoveryPaths(workdir string) []string {
    var paths []string

    // Project-level .claude/skills
    claudeDir := filepath.Join(workdir, ".claude")
    if exists(claudeDir) {
        paths = append(paths, claudeDir)
    }

    // Global ~/.claude/skills
    home, _ := os.UserHomeDir()
    globalClaude := filepath.Join(home, ".claude")
    if exists(globalClaude) {
        paths = append(paths, globalClaude)
    }

    // Project-level .opencode
    opencodeDir := filepath.Join(workdir, ".opencode")
    if exists(opencodeDir) {
        paths = append(paths, opencodeDir)
    }

    return paths
}
```

## File Reference Processing (Optional)

Extract `@filepath` references for additional context:

```go
var fileRefRe = regexp.MustCompile(`(?<![` + "`" + `\w])@(\.?[^\s` + "`" + `,.]*(?:\.[^\s` + "`" + `,.]+)*)`)

func ExtractFileRefs(content string) []string {
    matches := fileRefRe.FindAllStringSubmatch(content, -1)
    var refs []string
    seen := make(map[string]bool)
    for _, m := range matches {
        ref := m[1]
        if !seen[ref] {
            refs = append(refs, ref)
            seen[ref] = true
        }
    }
    return refs
}
```

## Shell Command Extraction (Optional)

Extract `` !`command` `` patterns:

```go
var shellRe = regexp.MustCompile("!`([^`]+)`")

func ExtractShellCommands(content string) []string {
    matches := shellRe.FindAllStringSubmatch(content, -1)
    var cmds []string
    for _, m := range matches {
        cmds = append(cmds, m[1])
    }
    return cmds
}
```

## Testing Strategy

### Unit Tests

1. **Parser tests** - Various frontmatter formats, edge cases
2. **Discovery tests** - Multiple directories, duplicates, symlinks
3. **Tool tests** - Missing skill, permission denied, successful load

### Integration Tests

1. Create test skills in temp directory
2. Verify discovery finds them
3. Verify tool returns correct content

### Test Fixtures

```
testdata/
├── skills/
│   ├── valid-skill/
│   │   └── SKILL.md
│   ├── colon-in-value/
│   │   └── SKILL.md
│   ├── missing-name/
│   │   └── SKILL.md
│   └── nested/
│       └── deep/
│           └── SKILL.md
```

## Error Handling

| Error | Handling |
|-------|----------|
| File not found | Skip, log warning |
| Invalid YAML | Skip, log warning |
| Missing name field | Skip, log warning |
| Duplicate name | Overwrite, log warning |
| Permission denied | Return tool error |

## Performance Considerations

1. **Cache skill registry** - Scan once at startup
2. **Lazy content loading** - Only parse full content on invocation
3. **Parallel scanning** - Use goroutines for directory walking

## Configuration Options

```yaml
# .opencode/config.yaml
skill:
  disable_claude_paths: false  # Disable .claude/skills scanning
  additional_paths:            # Extra skill directories
    - /shared/team-skills
```
