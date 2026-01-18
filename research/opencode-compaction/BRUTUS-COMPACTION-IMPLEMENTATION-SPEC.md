# BRUTUS Compaction Implementation Specification

## Overview

This document specifies how to implement OpenCode's compaction subsystem in Go for BRUTUS. The implementation should be minimal but complete, supporting:

1. Token overflow detection
2. Tool output pruning
3. Summary generation via a dedicated agent
4. Message filtering for compacted context

## Package Structure

```
brutus/
├── compaction/
│   ├── compaction.go    # Main compaction logic
│   ├── prune.go         # Pruning algorithm
│   ├── filter.go        # Message filtering
│   └── config.go        # Configuration
└── token/
    └── estimate.go      # Token estimation utilities
```

## Core Types

### Token Counts

```go
package compaction

type TokenCounts struct {
    Input     int `json:"input"`
    Output    int `json:"output"`
    Reasoning int `json:"reasoning"`
    Cache     struct {
        Read  int `json:"read"`
        Write int `json:"write"`
    } `json:"cache"`
}
```

### Model Limits

```go
type ModelLimits struct {
    Context int // Total context window
    Input   int // Max input tokens (0 = derived from context)
    Output  int // Max output tokens
}
```

### Configuration

```go
type Config struct {
    Auto  bool // Enable auto-compaction (default: true)
    Prune bool // Enable output pruning (default: true)
}

func DefaultConfig() Config {
    return Config{
        Auto:  true,
        Prune: true,
    }
}
```

### Part State

```go
type ToolPartState struct {
    Status   string                 `json:"status"` // pending|running|completed|error
    Input    map[string]interface{} `json:"input"`
    Output   string                 `json:"output"`
    Time     ToolPartTime           `json:"time"`
    Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type ToolPartTime struct {
    Start     int64 `json:"start"`
    End       int64 `json:"end,omitempty"`
    Compacted int64 `json:"compacted,omitempty"` // Marks as pruned
}
```

## Token Estimation

### Implementation

```go
package token

const CharsPerToken = 4

func Estimate(s string) int {
    if len(s) == 0 {
        return 0
    }
    estimate := len(s) / CharsPerToken
    if len(s)%CharsPerToken != 0 {
        estimate++
    }
    return estimate
}
```

## Overflow Detection

### Interface

```go
func IsOverflow(tokens TokenCounts, limits ModelLimits) bool
```

### Implementation

```go
const OutputTokenMax = 32_000

func IsOverflow(tokens TokenCounts, limits ModelLimits) bool {
    if limits.Context == 0 {
        return false // Unlimited context
    }

    count := tokens.Input + tokens.Cache.Read + tokens.Output

    outputCeiling := limits.Output
    if outputCeiling == 0 || outputCeiling > OutputTokenMax {
        outputCeiling = OutputTokenMax
    }

    usable := limits.Input
    if usable == 0 {
        usable = limits.Context - outputCeiling
    }

    return count > usable
}
```

## Pruning Algorithm

### Constants

```go
const (
    PruneMinimum = 20_000  // Minimum tokens to justify pruning
    PruneProtect = 40_000  // Protect last 40K tokens
)

var PruneProtectedTools = map[string]bool{
    "skill": true,
}
```

### Interface

```go
type Message struct {
    ID       string
    Role     string // "user" | "assistant"
    Summary  bool   // True if this is a compaction summary
    Parts    []Part
}

type Part interface {
    GetType() string
}

type ToolPart struct {
    ID    string
    Tool  string
    State ToolPartState
}

func (p *ToolPart) GetType() string { return "tool" }

type PruneResult struct {
    Pruned int        // Tokens reclaimed
    Total  int        // Total tokens scanned
    Parts  []*ToolPart // Parts that were pruned
}

func Prune(messages []Message, config Config) (PruneResult, error)
```

### Implementation

```go
func Prune(messages []Message, config Config) (PruneResult, error) {
    if !config.Prune {
        return PruneResult{}, nil
    }

    var (
        total    int
        pruned   int
        toPrune  []*ToolPart
        turns    int
    )

    // Iterate backwards through messages
    for i := len(messages) - 1; i >= 0; i-- {
        msg := messages[i]

        if msg.Role == "user" {
            turns++
        }

        // Skip last 2 user turns (active context)
        if turns < 2 {
            continue
        }

        // Stop at previous summary checkpoint
        if msg.Role == "assistant" && msg.Summary {
            break
        }

        // Scan parts backwards
        for j := len(msg.Parts) - 1; j >= 0; j-- {
            part := msg.Parts[j]

            toolPart, ok := part.(*ToolPart)
            if !ok {
                continue
            }

            if toolPart.State.Status != "completed" {
                continue
            }

            if PruneProtectedTools[toolPart.Tool] {
                continue
            }

            // Stop at already-compacted content
            if toolPart.State.Time.Compacted > 0 {
                goto commit
            }

            estimate := token.Estimate(toolPart.State.Output)
            total += estimate

            // Protect last PRUNE_PROTECT tokens
            if total > PruneProtect {
                pruned += estimate
                toPrune = append(toPrune, toolPart)
            }
        }
    }

commit:
    // Only commit if we exceed minimum threshold
    if pruned <= PruneMinimum {
        return PruneResult{
            Pruned: 0,
            Total:  total,
            Parts:  nil,
        }, nil
    }

    // Mark parts as compacted
    now := time.Now().UnixMilli()
    for _, part := range toPrune {
        part.State.Time.Compacted = now
    }

    return PruneResult{
        Pruned: pruned,
        Total:  total,
        Parts:  toPrune,
    }, nil
}
```

## Message Filtering

### Interface

```go
func FilterCompacted(messages []Message) []Message
```

### Implementation

```go
func FilterCompacted(messages []Message) []Message {
    // Build set of user message IDs with completed summaries
    completed := make(map[string]bool)

    for _, msg := range messages {
        if msg.Role == "assistant" && msg.Summary {
            // Get parent ID (would need to store this)
            completed[msg.ParentID] = true
        }
    }

    // Filter messages, stopping at compaction boundary
    result := make([]Message, 0, len(messages))

    for i := len(messages) - 1; i >= 0; i-- {
        msg := messages[i]
        result = append(result, msg)

        // Check if this user message has a completed compaction
        if msg.Role == "user" && completed[msg.ID] {
            hasCompaction := false
            for _, part := range msg.Parts {
                if compPart, ok := part.(*CompactionPart); ok {
                    hasCompaction = true
                    _ = compPart
                    break
                }
            }
            if hasCompaction {
                break
            }
        }
    }

    // Reverse to chronological order
    for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
        result[i], result[j] = result[j], result[i]
    }

    return result
}
```

## Compaction Part Type

```go
type CompactionPart struct {
    ID        string
    MessageID string
    SessionID string
    Auto      bool // Was this auto-triggered?
}

func (p *CompactionPart) GetType() string { return "compaction" }
```

## Summary Generation

The summary is generated by running the agent loop with a special "compaction" agent. This agent has:

1. A focused system prompt for summarization
2. All prior messages as context
3. No tool access (read-only summarization)

### Agent Definition

```go
var CompactionAgent = Agent{
    Name:   "compaction",
    Mode:   "primary",
    Hidden: true,
    Prompt: `Provide a detailed but concise summary of the conversation.

Capture:
- Completed actions and current work in progress
- Modified files and technical decisions with rationale
- Upcoming tasks and user-specific requirements or constraints
- Persistent preferences affecting the conversation

Balance: Comprehensive enough to provide context but concise enough to be quickly understood.`,
    Permission: DenyAll(),
}
```

### Summary Request Prompt

```go
const DefaultSummaryPrompt = `Provide a detailed prompt for continuing our conversation above.
Focus on information that would be helpful for continuing the conversation,
including what we did, what we're doing, which files we're working on,
and what we're going to do next considering new session will not have
access to our conversation.`
```

## Integration with Agent Loop

### Pre-Loop Check

```go
func (loop *AgentLoop) Run(ctx context.Context) error {
    for {
        // ... get messages ...

        // Check for pending compaction
        task := findPendingTask(messages)
        if task != nil && task.Type == "compaction" {
            if err := loop.processCompaction(ctx, messages, task); err != nil {
                return err
            }
            continue
        }

        // Check for overflow requiring compaction
        if lastAssistant != nil && !lastAssistant.Summary {
            if IsOverflow(lastAssistant.Tokens, model.Limits) {
                if err := loop.createCompaction(ctx, lastUser); err != nil {
                    return err
                }
                continue
            }
        }

        // ... normal processing ...
    }
}
```

### Creating Compaction

```go
func (loop *AgentLoop) createCompaction(ctx context.Context, userMsg Message) error {
    // Create user message marker
    msg := Message{
        ID:        NewMessageID(),
        Role:      "user",
        SessionID: loop.sessionID,
        Model:     userMsg.Model,
        Agent:     userMsg.Agent,
        CreatedAt: time.Now(),
    }

    // Add compaction part
    part := &CompactionPart{
        ID:        NewPartID(),
        MessageID: msg.ID,
        SessionID: loop.sessionID,
        Auto:      true,
    }
    msg.Parts = append(msg.Parts, part)

    return loop.storage.SaveMessage(msg)
}
```

### Processing Compaction

```go
func (loop *AgentLoop) processCompaction(ctx context.Context, messages []Message, task *CompactionPart) error {
    // First prune old outputs
    result, err := Prune(messages, loop.config.Compaction)
    if err != nil {
        return err
    }
    for _, part := range result.Parts {
        if err := loop.storage.UpdatePart(part); err != nil {
            return err
        }
    }

    // Create assistant message for summary
    assistantMsg := Message{
        ID:        NewMessageID(),
        Role:      "assistant",
        SessionID: loop.sessionID,
        Agent:     "compaction",
        Mode:      "compaction",
        Summary:   true,
        Model:     loop.model,
        CreatedAt: time.Now(),
    }

    // Run compaction agent
    summary, err := loop.runCompactionAgent(ctx, messages)
    if err != nil {
        return err
    }

    // Store summary as text part
    textPart := &TextPart{
        ID:        NewPartID(),
        MessageID: assistantMsg.ID,
        Text:      summary,
    }
    assistantMsg.Parts = append(assistantMsg.Parts, textPart)

    if err := loop.storage.SaveMessage(assistantMsg); err != nil {
        return err
    }

    // Auto-continue if configured
    if task.Auto {
        continueMsg := Message{
            ID:        NewMessageID(),
            Role:      "user",
            SessionID: loop.sessionID,
            Agent:     loop.currentAgent,
            Model:     loop.model,
            CreatedAt: time.Now(),
        }
        continueMsg.Parts = append(continueMsg.Parts, &TextPart{
            ID:        NewPartID(),
            MessageID: continueMsg.ID,
            Text:      "Continue if you have next steps",
            Synthetic: true,
        })
        if err := loop.storage.SaveMessage(continueMsg); err != nil {
            return err
        }
    }

    return nil
}
```

## Testing Strategy

### Unit Tests

```go
func TestTokenEstimate(t *testing.T) {
    tests := []struct {
        input    string
        expected int
    }{
        {"", 0},
        {"a", 1},
        {"abcd", 1},
        {"abcde", 2},
        {strings.Repeat("x", 100), 25},
    }
    for _, tt := range tests {
        got := token.Estimate(tt.input)
        if got != tt.expected {
            t.Errorf("Estimate(%q) = %d, want %d", tt.input, got, tt.expected)
        }
    }
}

func TestIsOverflow(t *testing.T) {
    limits := ModelLimits{Context: 128000, Output: 8000}

    // Under limit
    tokens := TokenCounts{Input: 50000, Output: 5000}
    if IsOverflow(tokens, limits) {
        t.Error("Should not overflow")
    }

    // Over limit
    tokens = TokenCounts{Input: 120000, Output: 10000}
    if !IsOverflow(tokens, limits) {
        t.Error("Should overflow")
    }
}

func TestPruneProtectsRecentContent(t *testing.T) {
    messages := []Message{
        {Role: "user", ID: "1"},
        {Role: "assistant", ID: "2", Parts: []Part{
            &ToolPart{Tool: "read", State: ToolPartState{
                Status: "completed",
                Output: strings.Repeat("x", 10000), // 2500 tokens
            }},
        }},
        {Role: "user", ID: "3"},   // Turn 1
        {Role: "user", ID: "4"},   // Turn 0 (current)
    }

    result, _ := Prune(messages, DefaultConfig())
    if result.Pruned > 0 {
        t.Error("Should not prune within last 2 turns")
    }
}
```

### Integration Tests

Test the full compaction flow:
1. Create messages until overflow
2. Verify compaction is triggered
3. Verify summary is generated
4. Verify message filtering works
5. Verify loop continues correctly

## Migration Path

### Phase 1: Token Estimation
- Implement `token.Estimate()`
- Add token counting to agent loop

### Phase 2: Overflow Detection
- Implement `IsOverflow()`
- Add check to agent loop (log only)

### Phase 3: Pruning
- Implement `Prune()`
- Add compacted timestamp to part storage
- Run after each assistant turn

### Phase 4: Message Filtering
- Implement `FilterCompacted()`
- Apply when loading messages for context

### Phase 5: Summary Generation
- Add compaction agent
- Implement `processCompaction()`
- Wire into agent loop

## Metrics to Track

```go
type CompactionMetrics struct {
    OverflowDetections int       // Times overflow was detected
    PruneOperations    int       // Times prune was called
    TokensPruned       int       // Total tokens pruned
    SummariesGenerated int       // Compaction summaries created
    AverageTokenCount  int       // Running average before compaction
}
```
