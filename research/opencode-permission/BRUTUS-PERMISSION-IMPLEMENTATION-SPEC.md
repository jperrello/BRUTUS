# BRUTUS Permission Implementation Specification

## Overview

This document specifies how to implement OpenCode's permission system in BRUTUS (Go).

## Core Types

### permission/types.go

```go
package permission

type Action string

const (
    ActionAllow Action = "allow"
    ActionDeny  Action = "deny"
    ActionAsk   Action = "ask"
)

type Rule struct {
    Permission string
    Pattern    string
    Action     Action
}

type Ruleset []Rule

type Request struct {
    ID         string
    SessionID  string
    Permission string
    Patterns   []string
    Always     []string           // Patterns to store on "always" response
    Metadata   map[string]any
    ToolCall   *ToolCallRef       // Optional
}

type ToolCallRef struct {
    MessageID string
    CallID    string
}

type Reply string

const (
    ReplyOnce   Reply = "once"
    ReplyAlways Reply = "always"
    ReplyReject Reply = "reject"
)
```

## Wildcard Matching

### wildcard/wildcard.go

```go
package wildcard

import (
    "regexp"
    "strings"
)

func Match(str, pattern string) bool {
    // Escape regex special chars
    escaped := regexp.QuoteMeta(pattern)

    // Convert wildcards
    escaped = strings.ReplaceAll(escaped, `\*`, `.*`)
    escaped = strings.ReplaceAll(escaped, `\?`, `.`)

    // Special case: trailing " *" becomes optional
    if strings.HasSuffix(escaped, ` .*`) {
        escaped = escaped[:len(escaped)-3] + `( .*)?`
    }

    re := regexp.MustCompile(`^` + escaped + `$`)
    return re.MatchString(str)
}

func Evaluate(permission, pattern string, rulesets ...Ruleset) Rule {
    merged := mergeRulesets(rulesets...)

    // Sort by key length ascending, then alphabetically
    sort.Slice(merged, func(i, j int) bool {
        if len(merged[i].Permission) != len(merged[j].Permission) {
            return len(merged[i].Permission) < len(merged[j].Permission)
        }
        return merged[i].Permission < merged[j].Permission
    })

    var result *Rule
    for _, rule := range merged {
        if Match(permission, rule.Permission) && Match(pattern, rule.Pattern) {
            result = &rule
        }
    }

    if result != nil {
        return *result
    }

    return Rule{
        Permission: permission,
        Pattern:    "*",
        Action:     ActionAsk,
    }
}
```

## Permission Manager

### permission/manager.go

```go
package permission

import (
    "context"
    "sync"
)

type Manager struct {
    mu       sync.Mutex
    pending  map[string]map[string]*pendingRequest
    approved map[string]Ruleset
    config   Ruleset
}

type pendingRequest struct {
    Request  Request
    Response chan Reply
}

func NewManager(config Ruleset) *Manager {
    return &Manager{
        pending:  make(map[string]map[string]*pendingRequest),
        approved: make(map[string]Ruleset),
        config:   config,
    }
}

func (m *Manager) Ask(ctx context.Context, req Request) error {
    m.mu.Lock()

    // Build merged ruleset: config + session approved
    merged := append(m.config, m.approved[req.SessionID]...)

    // Check each pattern
    for _, pattern := range req.Patterns {
        rule := Evaluate(req.Permission, pattern, merged)

        switch rule.Action {
        case ActionAllow:
            continue
        case ActionDeny:
            m.mu.Unlock()
            return &DeniedError{Rules: merged}
        case ActionAsk:
            // Need to prompt user
            m.mu.Unlock()
            return m.prompt(ctx, req)
        }
    }

    m.mu.Unlock()
    return nil // All patterns allowed
}

func (m *Manager) prompt(ctx context.Context, req Request) error {
    m.mu.Lock()

    // Generate ID if not set
    if req.ID == "" {
        req.ID = generateID("permission")
    }

    // Create pending entry
    if m.pending[req.SessionID] == nil {
        m.pending[req.SessionID] = make(map[string]*pendingRequest)
    }

    pending := &pendingRequest{
        Request:  req,
        Response: make(chan Reply, 1),
    }
    m.pending[req.SessionID][req.ID] = pending
    m.mu.Unlock()

    // Emit event for UI
    EmitAsked(req)

    // Wait for response
    select {
    case <-ctx.Done():
        m.cleanup(req.SessionID, req.ID)
        return ctx.Err()
    case reply := <-pending.Response:
        return m.handleReply(req, reply)
    }
}

func (m *Manager) Reply(sessionID, requestID string, reply Reply, message string) {
    m.mu.Lock()
    defer m.mu.Unlock()

    pending := m.pending[sessionID][requestID]
    if pending == nil {
        return
    }

    delete(m.pending[sessionID], requestID)

    // Emit event
    EmitReplied(sessionID, requestID, reply)

    pending.Response <- reply

    // If rejected, cancel all other pending for this session
    if reply == ReplyReject {
        for id, p := range m.pending[sessionID] {
            EmitReplied(sessionID, id, ReplyReject)
            p.Response <- ReplyReject
            delete(m.pending[sessionID], id)
        }
    }

    // If always, store patterns and resolve matching pending
    if reply == ReplyAlways {
        for _, pattern := range pending.Request.Always {
            m.approved[sessionID] = append(m.approved[sessionID], Rule{
                Permission: pending.Request.Permission,
                Pattern:    pattern,
                Action:     ActionAllow,
            })
        }

        // Check other pending requests
        for id, p := range m.pending[sessionID] {
            if m.checkApproved(p.Request) {
                EmitReplied(sessionID, id, ReplyAlways)
                p.Response <- ReplyAlways
                delete(m.pending[sessionID], id)
            }
        }
    }
}

func (m *Manager) handleReply(req Request, reply Reply) error {
    switch reply {
    case ReplyOnce, ReplyAlways:
        return nil
    case ReplyReject:
        return &RejectedError{}
    default:
        return &RejectedError{}
    }
}
```

## Error Types

### permission/errors.go

```go
package permission

import "fmt"

type RejectedError struct {
    Message string
}

func (e *RejectedError) Error() string {
    if e.Message != "" {
        return fmt.Sprintf("Permission rejected: %s", e.Message)
    }
    return "The user rejected permission to use this specific tool call."
}

type DeniedError struct {
    Rules Ruleset
}

func (e *DeniedError) Error() string {
    return fmt.Sprintf("Permission denied by rule: %v", e.Rules)
}

type CorrectedError struct {
    Feedback string
}

func (e *CorrectedError) Error() string {
    return fmt.Sprintf("Permission rejected with feedback: %s", e.Feedback)
}
```

## Bash Arity

### permission/arity.go

```go
package permission

var bashArity = map[string]int{
    // Simple commands (arity 1)
    "cat": 1, "cd": 1, "chmod": 1, "chown": 1, "cp": 1,
    "echo": 1, "grep": 1, "kill": 1, "ls": 1, "mkdir": 1,
    "mv": 1, "pwd": 1, "rm": 1, "rmdir": 1, "touch": 1,

    // Two-token commands (arity 2)
    "aws": 3, "brew": 2, "cargo": 2, "docker": 2, "git": 2,
    "go": 2, "helm": 2, "kubectl": 2, "make": 2, "npm": 2,
    "pip": 2, "pnpm": 2, "poetry": 2, "python": 2, "yarn": 2,

    // Three-token commands (arity 3)
    "bun run": 3, "docker compose": 3, "git config": 3,
    "git remote": 3, "git stash": 3, "npm run": 3,
    "pnpm run": 3, "yarn run": 3,
}

func BashArityPrefix(tokens []string) []string {
    for length := len(tokens); length > 0; length-- {
        prefix := strings.Join(tokens[:length], " ")
        if arity, ok := bashArity[prefix]; ok {
            if arity <= len(tokens) {
                return tokens[:arity]
            }
        }
    }

    if len(tokens) == 0 {
        return []string{}
    }
    return tokens[:1]
}
```

## Tool Integration

### tools/bash.go (integration example)

```go
func (b *BashTool) Execute(ctx context.Context, args BashArgs) (*Result, error) {
    // Parse command with tree-sitter or simple tokenization
    commands := parseCommands(args.Command)

    var patterns, always []string
    var externalDirs []string

    for _, cmd := range commands {
        tokens := tokenize(cmd)

        // Check for path-modifying commands
        if isPathCommand(tokens[0]) {
            for _, arg := range tokens[1:] {
                if !strings.HasPrefix(arg, "-") {
                    resolved := resolvePath(arg, args.Workdir)
                    if !isInsideProject(resolved) {
                        externalDirs = append(externalDirs, resolved)
                    }
                }
            }
        }

        // Build permission patterns
        fullCmd := strings.Join(tokens, " ")
        patterns = append(patterns, fullCmd)

        prefix := permission.BashArityPrefix(tokens)
        always = append(always, strings.Join(prefix, " ")+"*")
    }

    // Ask for external directory permission
    if len(externalDirs) > 0 {
        alwaysDirs := make([]string, len(externalDirs))
        for i, d := range externalDirs {
            alwaysDirs[i] = filepath.Dir(d) + "*"
        }

        err := ctx.Ask(permission.Request{
            Permission: "external_directory",
            Patterns:   externalDirs,
            Always:     alwaysDirs,
            Metadata:   map[string]any{},
        })
        if err != nil {
            return nil, err
        }
    }

    // Ask for bash permission
    if len(patterns) > 0 {
        err := ctx.Ask(permission.Request{
            Permission: "bash",
            Patterns:   unique(patterns),
            Always:     unique(always),
            Metadata:   map[string]any{},
        })
        if err != nil {
            return nil, err
        }
    }

    // Execute command...
    return b.exec(ctx, args)
}
```

## Configuration Loading

### config/permission.go

```go
type PermissionConfig map[string]PermissionRule

type PermissionRule interface{} // string | map[string]string

func ParsePermissionConfig(cfg PermissionConfig) permission.Ruleset {
    var rules permission.Ruleset

    for perm, rule := range cfg {
        switch v := rule.(type) {
        case string:
            // Simple: "bash": "allow"
            rules = append(rules, permission.Rule{
                Permission: perm,
                Pattern:    "*",
                Action:     permission.Action(v),
            })
        case map[string]string:
            // Pattern-specific: "bash": {"git *": "allow"}
            for pattern, action := range v {
                rules = append(rules, permission.Rule{
                    Permission: perm,
                    Pattern:    pattern,
                    Action:     permission.Action(action),
                })
            }
        }
    }

    return rules
}
```

## UI Integration

The permission system needs UI hooks to:

1. **Display pending requests** - Show user what tool is asking for permission
2. **Capture user response** - once/always/reject with optional message
3. **Show approval status** - Indicate when patterns are auto-approved

### Event emission

```go
func EmitAsked(req Request) {
    bus.Publish("permission.asked", req)
}

func EmitReplied(sessionID, requestID string, reply Reply) {
    bus.Publish("permission.replied", map[string]any{
        "sessionID":    sessionID,
        "requestID":    requestID,
        "reply":        reply,
    })
}
```

## Testing Considerations

1. **Wildcard matching** - Comprehensive pattern tests
2. **Rule evaluation order** - Verify last-match-wins with sorted rules
3. **Concurrent requests** - Multiple tools asking simultaneously
4. **Session isolation** - Approvals don't leak between sessions
5. **Always propagation** - Other pending requests resolved on always
6. **Reject cascade** - All pending cancelled on reject
