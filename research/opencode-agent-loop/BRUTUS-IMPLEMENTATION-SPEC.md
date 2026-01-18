# BRUTUS Agent Loop Implementation Spec

## Overview

This document specifies how to implement OpenCode's agent loop patterns in BRUTUS's Go codebase.

---

## Core Types

### Message Types

```go
package agent

import "time"

type MessageRole string

const (
    RoleUser      MessageRole = "user"
    RoleAssistant MessageRole = "assistant"
)

type Message struct {
    ID        string      `json:"id"`
    SessionID string      `json:"sessionID"`
    Role      MessageRole `json:"role"`
    CreatedAt time.Time   `json:"createdAt"`

    // User-specific
    Agent   string `json:"agent,omitempty"`
    Model   Model  `json:"model,omitempty"`
    System  string `json:"system,omitempty"`
    Variant string `json:"variant,omitempty"`

    // Assistant-specific
    ParentID    string         `json:"parentID,omitempty"`
    CompletedAt *time.Time     `json:"completedAt,omitempty"`
    Finish      string         `json:"finish,omitempty"`
    Error       *MessageError  `json:"error,omitempty"`
    Cost        float64        `json:"cost"`
    Tokens      TokenUsage     `json:"tokens"`
    Path        *PathInfo      `json:"path,omitempty"`
}

type Model struct {
    ProviderID string `json:"providerID"`
    ModelID    string `json:"modelID"`
}

type PathInfo struct {
    Cwd  string `json:"cwd"`
    Root string `json:"root"`
}

type TokenUsage struct {
    Input     int `json:"input"`
    Output    int `json:"output"`
    Reasoning int `json:"reasoning"`
    CacheRead int `json:"cacheRead"`
    CacheWrite int `json:"cacheWrite"`
}
```

### Part Types

```go
type PartType string

const (
    PartTypeText       PartType = "text"
    PartTypeFile       PartType = "file"
    PartTypeTool       PartType = "tool"
    PartTypeReasoning  PartType = "reasoning"
    PartTypeAgent      PartType = "agent"
    PartTypeSubtask    PartType = "subtask"
    PartTypeCompaction PartType = "compaction"
    PartTypeStepStart  PartType = "step-start"
    PartTypeStepFinish PartType = "step-finish"
    PartTypeSnapshot   PartType = "snapshot"
    PartTypePatch      PartType = "patch"
)

type Part struct {
    ID        string   `json:"id"`
    SessionID string   `json:"sessionID"`
    MessageID string   `json:"messageID"`
    Type      PartType `json:"type"`

    // TextPart
    Text      string `json:"text,omitempty"`
    Synthetic bool   `json:"synthetic,omitempty"`
    Ignored   bool   `json:"ignored,omitempty"`

    // ToolPart
    CallID    string     `json:"callID,omitempty"`
    Tool      string     `json:"tool,omitempty"`
    ToolState *ToolState `json:"state,omitempty"`

    // FilePart
    Mime     string `json:"mime,omitempty"`
    Filename string `json:"filename,omitempty"`
    URL      string `json:"url,omitempty"`

    // ReasoningPart
    Metadata map[string]any `json:"metadata,omitempty"`

    // SubtaskPart
    Prompt      string `json:"prompt,omitempty"`
    Description string `json:"description,omitempty"`
    Agent       string `json:"agent,omitempty"`

    // CompactionPart
    Auto bool `json:"auto,omitempty"`

    // StepFinishPart
    Reason string     `json:"reason,omitempty"`
    StepCost float64  `json:"stepCost,omitempty"`
    StepTokens *TokenUsage `json:"stepTokens,omitempty"`

    // SnapshotPart / PatchPart
    Snapshot string   `json:"snapshot,omitempty"`
    Hash     string   `json:"hash,omitempty"`
    Files    []string `json:"files,omitempty"`

    // Timing
    StartTime *time.Time `json:"startTime,omitempty"`
    EndTime   *time.Time `json:"endTime,omitempty"`
}

type ToolStatus string

const (
    ToolStatusPending   ToolStatus = "pending"
    ToolStatusRunning   ToolStatus = "running"
    ToolStatusCompleted ToolStatus = "completed"
    ToolStatusError     ToolStatus = "error"
)

type ToolState struct {
    Status      ToolStatus     `json:"status"`
    Input       map[string]any `json:"input"`
    Output      string         `json:"output,omitempty"`
    Error       string         `json:"error,omitempty"`
    Title       string         `json:"title,omitempty"`
    Metadata    map[string]any `json:"metadata,omitempty"`
    StartTime   time.Time      `json:"startTime,omitempty"`
    EndTime     *time.Time     `json:"endTime,omitempty"`
    Attachments []*Part        `json:"attachments,omitempty"`
}
```

---

## Loop Implementation

### Main Loop

```go
package agent

import (
    "context"
    "errors"
    "fmt"
)

const DoomLoopThreshold = 3

type LoopResult string

const (
    LoopContinue LoopResult = "continue"
    LoopStop     LoopResult = "stop"
    LoopCompact  LoopResult = "compact"
)

type Agent struct {
    sessions  map[string]*Session
    providers ProviderRegistry
    tools     ToolRegistry
}

type Session struct {
    ID       string
    Messages []*Message
    Parts    map[string][]*Part  // messageID -> parts
    Status   SessionStatus
    Cancel   context.CancelFunc
}

type SessionStatus struct {
    Type    string // "idle", "busy", "retry"
    Attempt int
    Message string
    NextAt  *time.Time
}

func (a *Agent) Loop(ctx context.Context, sessionID string) (*Message, error) {
    session, err := a.getOrCreateSession(sessionID)
    if err != nil {
        return nil, err
    }

    // Check if already running
    if session.Status.Type == "busy" {
        return nil, errors.New("session is busy")
    }

    ctx, cancel := context.WithCancel(ctx)
    session.Cancel = cancel
    defer cancel()

    step := 0

    for {
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        default:
        }

        session.Status = SessionStatus{Type: "busy"}

        // Get filtered messages (exclude compacted)
        msgs := a.filterCompactedMessages(session)

        // Find key messages
        lastUser := a.findLastUser(msgs)
        lastAssistant := a.findLastAssistant(msgs)
        tasks := a.collectPendingTasks(msgs)

        if lastUser == nil {
            return nil, errors.New("no user message found")
        }

        // Check exit conditions
        if lastAssistant != nil && lastAssistant.Finish != "" {
            if lastAssistant.Finish != "tool-calls" && lastAssistant.Finish != "unknown" {
                if lastUser.ID < lastAssistant.ID {
                    break // Normal completion
                }
            }
        }

        step++

        // Handle pending tasks
        if len(tasks) > 0 {
            task := tasks[len(tasks)-1]
            switch task.Type {
            case PartTypeSubtask:
                if err := a.executeSubtask(ctx, session, task); err != nil {
                    return nil, err
                }
                continue
            case PartTypeCompaction:
                result, err := a.processCompaction(ctx, session, msgs, lastUser.ID)
                if err != nil {
                    return nil, err
                }
                if result == LoopStop {
                    break
                }
                continue
            }
        }

        // Check context overflow
        if lastAssistant != nil && a.needsCompaction(lastAssistant.Tokens) {
            a.createCompactionTask(session)
            continue
        }

        // Normal processing
        result, err := a.processStep(ctx, session, lastUser, step)
        if err != nil {
            return nil, err
        }

        switch result {
        case LoopStop:
            goto done
        case LoopCompact:
            a.createCompactionTask(session)
        case LoopContinue:
            // continue loop
        }
    }

done:
    session.Status = SessionStatus{Type: "idle"}
    return a.getLastAssistant(session), nil
}
```

### Step Processing

```go
func (a *Agent) processStep(ctx context.Context, session *Session, lastUser *Message, step int) (LoopResult, error) {
    // Get agent config
    agentCfg := a.getAgentConfig(lastUser.Agent)

    // Create assistant message
    assistant := &Message{
        ID:        generateID("message"),
        SessionID: session.ID,
        Role:      RoleAssistant,
        ParentID:  lastUser.ID,
        Agent:     agentCfg.Name,
        CreatedAt: time.Now(),
        Model:     lastUser.Model,
        Tokens:    TokenUsage{},
        Path: &PathInfo{
            Cwd:  a.currentDir,
            Root: a.projectRoot,
        },
    }
    session.Messages = append(session.Messages, assistant)

    // Resolve tools
    tools := a.resolveTools(agentCfg, lastUser)

    // Build messages for LLM
    modelMessages := a.toModelMessages(session.Messages)

    // Create processor
    processor := &Processor{
        assistant:  assistant,
        session:    session,
        toolCalls:  make(map[string]*Part),
        doomCount:  make(map[string]int),
    }

    // Stream from LLM
    return processor.Process(ctx, a, ProcessInput{
        User:     lastUser,
        Agent:    agentCfg,
        System:   a.buildSystemPrompt(agentCfg),
        Messages: modelMessages,
        Tools:    tools,
    })
}
```

---

## Processor Implementation

```go
type Processor struct {
    assistant  *Message
    session    *Session
    toolCalls  map[string]*Part
    doomCount  map[string]int
    blocked    bool
}

type ProcessInput struct {
    User     *Message
    Agent    *AgentConfig
    System   []string
    Messages []ModelMessage
    Tools    map[string]Tool
}

func (p *Processor) Process(ctx context.Context, agent *Agent, input ProcessInput) (LoopResult, error) {
    stream, err := agent.providers.Stream(ctx, StreamInput{
        Model:    input.User.Model,
        System:   input.System,
        Messages: input.Messages,
        Tools:    input.Tools,
    })
    if err != nil {
        return LoopStop, err
    }

    for event := range stream.Events() {
        select {
        case <-ctx.Done():
            return LoopStop, ctx.Err()
        default:
        }

        switch e := event.(type) {
        case *TextStartEvent:
            part := &Part{
                ID:        generateID("part"),
                SessionID: p.session.ID,
                MessageID: p.assistant.ID,
                Type:      PartTypeText,
                Text:      "",
                StartTime: timePtr(time.Now()),
            }
            p.addPart(part)

        case *TextDeltaEvent:
            p.appendToLastText(e.Text)

        case *TextEndEvent:
            p.finishLastText()

        case *ToolCallStartEvent:
            part := &Part{
                ID:        generateID("part"),
                SessionID: p.session.ID,
                MessageID: p.assistant.ID,
                Type:      PartTypeTool,
                CallID:    e.CallID,
                Tool:      e.ToolName,
                ToolState: &ToolState{
                    Status:    ToolStatusPending,
                    Input:     make(map[string]any),
                    StartTime: time.Now(),
                },
            }
            p.toolCalls[e.CallID] = part
            p.addPart(part)

        case *ToolCallEvent:
            part := p.toolCalls[e.CallID]
            if part == nil {
                continue
            }

            part.ToolState.Status = ToolStatusRunning
            part.ToolState.Input = e.Input

            // Doom loop detection
            doomKey := fmt.Sprintf("%s:%s", e.ToolName, jsonString(e.Input))
            p.doomCount[doomKey]++
            if p.doomCount[doomKey] >= DoomLoopThreshold {
                // Ask for permission to continue
                allowed, err := agent.askPermission(ctx, "doom_loop", e.ToolName)
                if err != nil || !allowed {
                    p.blocked = true
                    continue
                }
                p.doomCount[doomKey] = 0
            }

            // Execute tool
            result, err := agent.executeTool(ctx, e.ToolName, e.Input)
            if err != nil {
                part.ToolState.Status = ToolStatusError
                part.ToolState.Error = err.Error()
                part.ToolState.EndTime = timePtr(time.Now())

                if isPermissionError(err) {
                    p.blocked = true
                }
            } else {
                part.ToolState.Status = ToolStatusCompleted
                part.ToolState.Output = result.Output
                part.ToolState.Title = result.Title
                part.ToolState.Metadata = result.Metadata
                part.ToolState.EndTime = timePtr(time.Now())
                part.ToolState.Attachments = result.Attachments
            }
            delete(p.toolCalls, e.CallID)

        case *ReasoningStartEvent:
            part := &Part{
                ID:        generateID("part"),
                SessionID: p.session.ID,
                MessageID: p.assistant.ID,
                Type:      PartTypeReasoning,
                Text:      "",
                Metadata:  e.Metadata,
                StartTime: timePtr(time.Now()),
            }
            p.addPart(part)

        case *ReasoningDeltaEvent:
            p.appendToLastReasoning(e.Text)

        case *ReasoningEndEvent:
            p.finishLastReasoning()

        case *FinishStepEvent:
            p.assistant.Tokens = e.Tokens
            p.assistant.Cost += e.Cost
            p.assistant.Finish = e.Reason
            p.assistant.CompletedAt = timePtr(time.Now())

            // Check for overflow
            if agent.needsCompaction(e.Tokens) {
                return LoopCompact, nil
            }

        case *ErrorEvent:
            return LoopStop, e.Error
        }
    }

    // Mark incomplete tools as errored
    for _, part := range p.toolCalls {
        if part.ToolState.Status == ToolStatusPending || part.ToolState.Status == ToolStatusRunning {
            part.ToolState.Status = ToolStatusError
            part.ToolState.Error = "Tool execution aborted"
            part.ToolState.EndTime = timePtr(time.Now())
        }
    }

    if p.blocked {
        return LoopStop, nil
    }
    if p.assistant.Error != nil {
        return LoopStop, nil
    }
    return LoopContinue, nil
}

func (p *Processor) addPart(part *Part) {
    parts := p.session.Parts[p.assistant.ID]
    p.session.Parts[p.assistant.ID] = append(parts, part)
}
```

---

## Tool Resolution

```go
type Tool interface {
    ID() string
    Description() string
    Parameters() json.RawMessage  // JSON Schema
    Execute(ctx context.Context, input map[string]any) (*ToolResult, error)
}

type ToolResult struct {
    Output      string
    Title       string
    Metadata    map[string]any
    Attachments []*Part
}

func (a *Agent) resolveTools(agentCfg *AgentConfig, user *Message) map[string]Tool {
    tools := make(map[string]Tool)

    // Get tools from registry
    for _, t := range a.tools.ForAgent(agentCfg.Name) {
        // Check permissions
        if a.isToolDenied(agentCfg, t.ID()) {
            continue
        }
        tools[t.ID()] = t
    }

    // Add MCP tools if available
    for id, t := range a.mcpTools {
        if a.isToolDenied(agentCfg, id) {
            continue
        }
        tools[id] = t
    }

    return tools
}
```

---

## Helper Functions

```go
func generateID(prefix string) string {
    return fmt.Sprintf("%s_%s", prefix, ulid.Make().String())
}

func timePtr(t time.Time) *time.Time {
    return &t
}

func jsonString(v any) string {
    b, _ := json.Marshal(v)
    return string(b)
}

func isPermissionError(err error) bool {
    var permErr *PermissionError
    return errors.As(err, &permErr)
}
```

---

## Integration with Existing BRUTUS Code

### Modify `agent/agent.go`

The existing BRUTUS agent loop can be enhanced to match OpenCode's patterns:

1. **Add message/part storage** - Currently BRUTUS keeps conversation in memory; add persistence

2. **Add tool state tracking** - Track pending → running → completed/error

3. **Add doom loop detection** - Simple counter check

4. **Add step boundaries** - Track when LLM completes a generation step

### Suggested Incremental Steps

1. **Phase 1**: Add Part types alongside existing Message type
2. **Phase 2**: Add tool state machine to existing tool execution
3. **Phase 3**: Add doom loop detection
4. **Phase 4**: Add session persistence
5. **Phase 5**: Add context compaction (later)

---

## Key Differences from Current BRUTUS

| Aspect | Current BRUTUS | OpenCode Pattern |
|--------|----------------|------------------|
| Messages | Single struct | User/Assistant discriminated |
| Tool tracking | In message | Separate Part struct |
| Tool state | Boolean | Enum (pending/running/completed/error) |
| Context | In-memory | Persistent storage |
| Retry | Simple | Exponential backoff |
| Loop control | Continue/break | Return LoopResult enum |
| Doom detection | None | 3-call threshold |
