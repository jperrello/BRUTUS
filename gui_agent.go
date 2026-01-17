package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"brutus/coordinator"
	"brutus/provider"
	"brutus/tools"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var guiAgentPortCounter int32 = 9000

type ToolApprovalRequest struct {
	ID        string `json:"id"`
	AgentID   string `json:"agentId"`
	Tool      string `json:"tool"`
	Arguments string `json:"arguments"`
}

type ToolApprovalResponse struct {
	Approved bool   `json:"approved"`
	Reason   string `json:"reason"`
}

var autoApproveTools = map[string]bool{
	"read_file":       true,
	"list_files":      true,
	"code_search":     true,
	"agent_broadcast": true,
	"observe_agents":  true,
}

type GUIAgent struct {
	id              string
	provider        provider.Provider
	tools           *tools.Registry
	systemPrompt    string
	conversation    []provider.Message
	ctx             context.Context
	appCtx          context.Context
	cancel          context.CancelFunc
	mu              sync.Mutex
	pendingApproval map[string]chan ToolApprovalResponse
	approvalMu      sync.Mutex
	coordinator     *coordinator.Coordinator
}

func NewGUIAgent(appCtx context.Context, id string, model string) (*GUIAgent, error) {
	systemPrompt, err := os.ReadFile("BRUTUS.md")
	if err != nil {
		systemPrompt = []byte("You are BRUTUS, a coding agent.")
	}

	ctx, cancel := context.WithCancel(context.Background())

	prov, err := provider.NewSaturn(ctx, provider.SaturnConfig{
		Model:     model,
		MaxTokens: 4096,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to Saturn: %w", err)
	}

	registry := tools.NewRegistry()
	registry.Register(tools.ReadFileTool)
	registry.Register(tools.ListFilesTool)
	registry.Register(tools.EditFileTool)
	registry.Register(tools.BashTool)
	registry.Register(tools.CodeSearchTool)
	registry.Register(tools.BroadcastTool)
	registry.Register(tools.ObserveAgentsTool)

	coord := coordinator.NewCoordinator(id)

	port := int(atomic.AddInt32(&guiAgentPortCounter, 1))
	if err := coord.Start(ctx, port); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start coordinator: %w", err)
	}

	return &GUIAgent{
		id:              id,
		provider:        prov,
		tools:           registry,
		systemPrompt:    string(systemPrompt),
		appCtx:          appCtx,
		ctx:             ctx,
		cancel:          cancel,
		pendingApproval: make(map[string]chan ToolApprovalResponse),
		coordinator:     coord,
	}, nil
}

func (g *GUIAgent) Stop() {
	g.updateStatusWithBroadcast("stopped", "", "Agent stopped")
	g.coordinator.Stop()
	g.cancel()
}

func (g *GUIAgent) GetCoordinatorStatus() coordinator.AgentStatus {
	return g.coordinator.GetStatus()
}

func (g *GUIAgent) GetServiceInfo() *provider.SaturnService {
	if saturn, ok := g.provider.(*provider.Saturn); ok {
		return saturn.GetService()
	}
	return nil
}

func (g *GUIAgent) GetCoordinator() *coordinator.Coordinator {
	return g.coordinator
}

func (g *GUIAgent) updateStatusWithBroadcast(status, task, action string) {
	g.coordinator.UpdateStatus(status, task, action)

	broadcastInput := tools.BroadcastInput{
		AgentID: g.id,
		Status:  status,
		Task:    task,
		Action:  action,
		UseTXT:  false,
	}
	inputJSON, err := json.Marshal(broadcastInput)
	if err != nil {
		return
	}
	_, _ = tools.BroadcastTool.Function(inputJSON)
}

func (g *GUIAgent) SendMessage(message string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.conversation = append(g.conversation, provider.Message{
		Role:    "user",
		Content: message,
	})

	return g.runInferenceLoop()
}

func (g *GUIAgent) runInferenceLoop() error {
	g.updateStatusWithBroadcast("working", "Processing request", "Starting inference")
	defer g.updateStatusWithBroadcast("idle", "", "Inference complete")

	for {
		select {
		case <-g.ctx.Done():
			return g.ctx.Err()
		default:
		}

		stream, err := g.provider.ChatStream(g.ctx, g.systemPrompt, g.conversation, g.tools.All())
		if err != nil {
			return fmt.Errorf("inference failed: %w", err)
		}

		var contentBuilder strings.Builder
		var toolCalls []provider.ToolCall

		for delta := range stream {
			if delta.Error != nil {
				return delta.Error
			}

			if delta.Content != "" {
				contentBuilder.WriteString(delta.Content)
				runtime.EventsEmit(g.appCtx, "agent:stream", map[string]string{
					"id":      g.id,
					"content": delta.Content,
				})
			}

			if delta.ToolCall != nil {
				toolCalls = append(toolCalls, *delta.ToolCall)
			}

			if delta.Done {
				break
			}
		}

		response := provider.Message{
			Role:      "assistant",
			Content:   contentBuilder.String(),
			ToolCalls: toolCalls,
		}

		g.conversation = append(g.conversation, response)

		if response.Content != "" {
			runtime.EventsEmit(g.appCtx, "agent:message", map[string]string{
				"id":      g.id,
				"role":    "assistant",
				"content": response.Content,
			})
		}

		if len(response.ToolCalls) == 0 {
			return nil
		}

		var toolResults []provider.ToolResult

		for _, tc := range response.ToolCalls {
			g.updateStatusWithBroadcast("working", fmt.Sprintf("Executing %s", tc.Name), tc.Name)

			runtime.EventsEmit(g.appCtx, "agent:tool", map[string]string{
				"id":   g.id,
				"tool": tc.Name,
			})

			approved, err := g.requestApproval(tc)
			if err != nil {
				return err
			}

			if !approved {
				toolResults = append(toolResults, provider.ToolResult{
					ID:      tc.ID,
					Content: "Tool execution was denied by user.",
					IsError: true,
				})
				continue
			}

			result, toolErr := g.executeTool(tc)

			if toolErr != nil {
				result = toolErr.Error()
			}

			toolResults = append(toolResults, provider.ToolResult{
				ID:      tc.ID,
				Content: result,
				IsError: toolErr != nil,
			})

			runtime.EventsEmit(g.appCtx, "agent:tool_result", map[string]interface{}{
				"id":      g.id,
				"tool":    tc.Name,
				"result":  truncate(result, 500),
				"isError": toolErr != nil,
			})
		}

		g.conversation = append(g.conversation, provider.Message{
			Role:        "user",
			ToolResults: toolResults,
		})
	}
}

func (g *GUIAgent) requestApproval(tc provider.ToolCall) (bool, error) {
	if autoApproveTools[tc.Name] {
		return true, nil
	}

	approvalID := fmt.Sprintf("%s-%s", g.id, tc.ID)
	responseChan := make(chan ToolApprovalResponse, 1)

	g.approvalMu.Lock()
	g.pendingApproval[approvalID] = responseChan
	g.approvalMu.Unlock()

	defer func() {
		g.approvalMu.Lock()
		delete(g.pendingApproval, approvalID)
		g.approvalMu.Unlock()
	}()

	runtime.EventsEmit(g.appCtx, "agent:approval_request", ToolApprovalRequest{
		ID:        approvalID,
		AgentID:   g.id,
		Tool:      tc.Name,
		Arguments: string(tc.Input),
	})

	select {
	case <-g.ctx.Done():
		return false, g.ctx.Err()
	case resp := <-responseChan:
		return resp.Approved, nil
	}
}

func (g *GUIAgent) RespondToApproval(approvalID string, approved bool, reason string) {
	g.approvalMu.Lock()
	ch, ok := g.pendingApproval[approvalID]
	g.approvalMu.Unlock()

	if ok {
		ch <- ToolApprovalResponse{Approved: approved, Reason: reason}
	}
}

func (g *GUIAgent) executeTool(tc provider.ToolCall) (string, error) {
	tool, ok := g.tools.Get(tc.Name)
	if !ok {
		return "", fmt.Errorf("tool '%s' not found", tc.Name)
	}

	return tool.Function(json.RawMessage(tc.Input))
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
