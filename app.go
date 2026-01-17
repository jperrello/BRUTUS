package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"brutus/coordinator"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx        context.Context
	sessions   map[string]*AgentSession
	guiAgents  map[string]*GUIAgent
	sessionsMu sync.RWMutex
	ptyManager *PTYManager
}

type AgentSession struct {
	ID          string        `json:"id"`
	Model       string        `json:"model"`
	Status      string        `json:"status"`
	Cost        float64       `json:"cost"`
	Messages    []ChatMessage `json:"messages"`
	ServiceName string        `json:"serviceName"`
	ServiceHost string        `json:"serviceHost"`
	Connected   bool          `json:"connected"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func NewApp() *App {
	return &App{
		sessions:   make(map[string]*AgentSession),
		guiAgents:  make(map[string]*GUIAgent),
		ptyManager: NewPTYManager(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.ptyManager.SetContext(ctx)
	a.startCoordinationBroadcast()
}

func (a *App) startCoordinationBroadcast() {
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
				statuses := a.GetCoordinationStatuses()
				runtime.EventsEmit(a.ctx, "coordination:status", statuses)
			}
		}
	}()
}

type CoordinationStatus struct {
	AgentID     string `json:"agent_id"`
	Status      string `json:"status"`
	CurrentTask string `json:"current_task"`
	LastAction  string `json:"last_action"`
	IsRemote    bool   `json:"is_remote"`
}

func (a *App) GetCoordinationStatuses() []CoordinationStatus {
	a.sessionsMu.RLock()
	defer a.sessionsMu.RUnlock()

	localIDs := make(map[string]bool)
	var statuses []CoordinationStatus
	var discoveryCoord *coordinator.Coordinator

	for id, agent := range a.guiAgents {
		localIDs[id] = true
		coordStatus := agent.GetCoordinatorStatus()
		statuses = append(statuses, CoordinationStatus{
			AgentID:     id,
			Status:      coordStatus.Status,
			CurrentTask: coordStatus.CurrentTask,
			LastAction:  coordStatus.LastAction,
			IsRemote:    false,
		})
		if discoveryCoord == nil {
			discoveryCoord = agent.GetCoordinator()
		}
	}

	if discoveryCoord != nil {
		ctx, cancel := context.WithTimeout(a.ctx, 500*time.Millisecond)
		defer cancel()

		remoteAgents, err := discoveryCoord.DiscoverAgents(ctx, 400*time.Millisecond)
		if err == nil {
			for _, remote := range remoteAgents {
				if !localIDs[remote.AgentID] {
					statuses = append(statuses, CoordinationStatus{
						AgentID:     remote.AgentID,
						Status:      remote.Status,
						CurrentTask: remote.CurrentTask,
						LastAction:  remote.LastAction,
						IsRemote:    true,
					})
				}
			}
		}
	}

	return statuses
}

func (a *App) GetVersion() string {
	return "0.1.0"
}

func (a *App) NewAgent(model string) (string, error) {
	return a.NewNamedAgent("", model)
}

func (a *App) NewNamedAgent(name string, model string) (string, error) {
	a.sessionsMu.Lock()
	defer a.sessionsMu.Unlock()

	id := name
	if id == "" {
		id = fmt.Sprintf("agent-%d", len(a.sessions)+1)
	}

	if _, exists := a.sessions[id]; exists {
		return "", fmt.Errorf("agent with id '%s' already exists", id)
	}

	guiAgent, err := NewGUIAgent(a.ctx, id, model)
	if err != nil {
		return "", err
	}

	session := &AgentSession{
		ID:       id,
		Model:    model,
		Status:   "idle",
		Cost:     0,
		Messages: []ChatMessage{},
	}

	if svc := guiAgent.GetServiceInfo(); svc != nil {
		session.ServiceName = svc.Name
		session.ServiceHost = svc.Host
		session.Connected = true
	}

	a.sessions[id] = session
	a.guiAgents[id] = guiAgent

	runtime.EventsEmit(a.ctx, "agent:created", id)
	return id, nil
}

func (a *App) GetAgents() []*AgentSession {
	a.sessionsMu.RLock()
	defer a.sessionsMu.RUnlock()

	result := make([]*AgentSession, 0, len(a.sessions))
	for _, session := range a.sessions {
		result = append(result, session)
	}
	return result
}

func (a *App) SendMessage(agentID, message string) error {
	a.sessionsMu.Lock()
	session, ok := a.sessions[agentID]
	guiAgent := a.guiAgents[agentID]
	if !ok || guiAgent == nil {
		a.sessionsMu.Unlock()
		return fmt.Errorf("agent not found: %s", agentID)
	}

	session.Messages = append(session.Messages, ChatMessage{
		Role:    "user",
		Content: message,
	})
	session.Status = "running"
	a.sessionsMu.Unlock()

	runtime.EventsEmit(a.ctx, "agent:status", map[string]string{
		"id":     agentID,
		"status": "running",
	})

	go func() {
		err := guiAgent.SendMessage(message)

		a.sessionsMu.Lock()
		session.Status = "idle"
		if err != nil {
			errMsg := fmt.Sprintf("Error: %s", err)
			session.Messages = append(session.Messages, ChatMessage{
				Role:    "assistant",
				Content: errMsg,
			})
			a.sessionsMu.Unlock()

			runtime.EventsEmit(a.ctx, "agent:error", map[string]string{
				"id":    agentID,
				"error": errMsg,
			})
		} else {
			a.sessionsMu.Unlock()
		}

		runtime.EventsEmit(a.ctx, "agent:status", map[string]string{
			"id":     agentID,
			"status": "idle",
		})
	}()

	return nil
}

func (a *App) StopAgent(agentID string) error {
	a.sessionsMu.Lock()
	defer a.sessionsMu.Unlock()

	guiAgent, ok := a.guiAgents[agentID]
	if !ok {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	guiAgent.Stop()
	if session, exists := a.sessions[agentID]; exists {
		session.Status = "stopped"
	}

	runtime.EventsEmit(a.ctx, "agent:status", map[string]string{
		"id":     agentID,
		"status": "stopped",
	})

	return nil
}

func (a *App) RespondToApproval(agentID, approvalID string, approved bool, reason string) error {
	a.sessionsMu.RLock()
	guiAgent, ok := a.guiAgents[agentID]
	a.sessionsMu.RUnlock()

	if !ok {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	guiAgent.RespondToApproval(approvalID, approved, reason)
	return nil
}

func (a *App) LaunchMultiAgentDemo() ([]string, error) {
	ids := []string{}

	id1, err := a.NewNamedAgent("Editor-1", "")
	if err != nil {
		return nil, err
	}
	ids = append(ids, id1)

	id2, err := a.NewNamedAgent("Editor-2", "")
	if err != nil {
		return nil, err
	}
	ids = append(ids, id2)

	id3, err := a.NewNamedAgent("Observer", "")
	if err != nil {
		return nil, err
	}
	ids = append(ids, id3)

	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = a.SendMessage(id1, "Edit the file mock1.txt and add a greeting function that returns 'Hello, World!'")
	}()

	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = a.SendMessage(id2, "Edit the file mock2.txt and add a farewell function that returns 'Goodbye!'")
	}()

	go func() {
		time.Sleep(500 * time.Millisecond)
		_ = a.SendMessage(id3, "Use the observe_agents tool to discover other agents on the network, then summarize their activity.")
	}()

	return ids, nil
}

func (a *App) PTYSpawn(shell string) (string, error) {
	return a.ptyManager.Spawn(shell)
}

func (a *App) PTYWrite(id string, data string) error {
	return a.ptyManager.Write(id, data)
}

func (a *App) PTYKill(id string) error {
	return a.ptyManager.Kill(id)
}

func (a *App) PTYList() []string {
	return a.ptyManager.List()
}
