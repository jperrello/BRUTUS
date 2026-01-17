package coordinator

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

type AgentStatus struct {
	AgentID     string    `json:"agent_id"`
	Status      string    `json:"status"`
	CurrentTask string    `json:"current_task"`
	LastAction  string    `json:"last_action"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type AgentMessage struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

type Coordinator struct {
	agentID        string
	status         AgentStatus
	messages       []AgentMessage
	server         *zeroconf.Server
	mu             sync.RWMutex
	messageHandler func(AgentMessage)
	stopCh         chan struct{}
}

func NewCoordinator(agentID string) *Coordinator {
	return &Coordinator{
		agentID: agentID,
		status: AgentStatus{
			AgentID:     agentID,
			Status:      "idle",
			CurrentTask: "none",
			LastAction:  "none",
			UpdatedAt:   time.Now(),
		},
		messages: make([]AgentMessage, 0),
		stopCh:   make(chan struct{}),
	}
}

func (c *Coordinator) Start(ctx context.Context, port int) error {
	txtRecords := c.buildTXTRecords()

	host, _ := c.getLocalIP()

	server, err := zeroconf.Register(
		fmt.Sprintf("brutus-agent-%s", c.agentID),
		"_brutus-agent._tcp",
		"local.",
		port,
		txtRecords,
		[]net.Interface{},
	)
	if err != nil {
		return fmt.Errorf("failed to register agent: %w", err)
	}

	c.server = server

	go c.listenForAgents(ctx)

	fmt.Printf("[coordinator] Agent %s registered at %s:%d\n", c.agentID, host, port)
	return nil
}

func (c *Coordinator) Stop() {
	close(c.stopCh)
	if c.server != nil {
		c.server.Shutdown()
	}
}

func (c *Coordinator) UpdateStatus(status, task, action string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if status != "" {
		c.status.Status = status
	}
	if task != "" {
		c.status.CurrentTask = task
	}
	if action != "" {
		c.status.LastAction = action
	}
	c.status.UpdatedAt = time.Now()

	if c.server != nil {
		c.server.SetText(c.buildTXTRecords())
	}
}

func (c *Coordinator) Broadcast(msgType, content string) error {
	msg := AgentMessage{
		From:      c.agentID,
		To:        "*",
		Type:      msgType,
		Content:   content,
		Timestamp: time.Now(),
	}

	c.mu.Lock()
	c.messages = append(c.messages, msg)
	c.mu.Unlock()

	if c.server != nil {
		c.server.SetText(c.buildTXTRecords())
	}

	return nil
}

func (c *Coordinator) SendMessage(to, msgType, content string) error {
	msg := AgentMessage{
		From:      c.agentID,
		To:        to,
		Type:      msgType,
		Content:   content,
		Timestamp: time.Now(),
	}

	c.mu.Lock()
	c.messages = append(c.messages, msg)
	c.mu.Unlock()

	if c.server != nil {
		c.server.SetText(c.buildTXTRecords())
	}

	return nil
}

func (c *Coordinator) GetMessages() []AgentMessage {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]AgentMessage, len(c.messages))
	copy(result, c.messages)
	return result
}

func (c *Coordinator) GetStatus() AgentStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

func (c *Coordinator) OnMessage(handler func(AgentMessage)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messageHandler = handler
}

func (c *Coordinator) DiscoverAgents(ctx context.Context, timeout time.Duration) ([]AgentStatus, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry, 10)
	var agents []AgentStatus

	browseCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		for entry := range entries {
			if status := parseAgentEntry(entry); status.AgentID != "" {
				agents = append(agents, status)
			}
		}
		close(done)
	}()

	err = resolver.Browse(browseCtx, "_brutus-agent._tcp", "local.", entries)
	if err != nil {
		return nil, fmt.Errorf("browse failed: %w", err)
	}

	<-browseCtx.Done()
	close(entries)
	<-done

	return agents, nil
}

func (c *Coordinator) DiscoverMessages(ctx context.Context, timeout time.Duration) ([]AgentMessage, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry, 10)
	var allMessages []AgentMessage

	browseCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		for entry := range entries {
			messages := parseAgentMessages(entry)
			for _, msg := range messages {
				if msg.To == "*" || msg.To == c.agentID {
					allMessages = append(allMessages, msg)
				}
			}
		}
		close(done)
	}()

	err = resolver.Browse(browseCtx, "_brutus-agent._tcp", "local.", entries)
	if err != nil {
		return nil, fmt.Errorf("browse failed: %w", err)
	}

	<-browseCtx.Done()
	close(entries)
	<-done

	return allMessages, nil
}

func (c *Coordinator) buildTXTRecords() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	records := []string{
		fmt.Sprintf("agent_id=%s", c.status.AgentID),
		fmt.Sprintf("status=%s", c.status.Status),
		fmt.Sprintf("task=%s", c.status.CurrentTask),
		fmt.Sprintf("action=%s", c.status.LastAction),
		fmt.Sprintf("updated=%d", c.status.UpdatedAt.Unix()),
	}

	for i, msg := range c.messages {
		if i >= 5 {
			break
		}
		msgJSON, _ := json.Marshal(msg)
		records = append(records, fmt.Sprintf("msg%d=%s", i, string(msgJSON)))
	}

	return records
}

func (c *Coordinator) listenForAgents(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			messages, err := c.DiscoverMessages(ctx, 1*time.Second)
			if err != nil {
				continue
			}

			c.mu.RLock()
			handler := c.messageHandler
			c.mu.RUnlock()

			if handler != nil {
				for _, msg := range messages {
					if msg.From != c.agentID {
						handler(msg)
					}
				}
			}
		}
	}
}

func (c *Coordinator) getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1", err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}

	return "127.0.0.1", nil
}

func parseAgentEntry(entry *zeroconf.ServiceEntry) AgentStatus {
	status := AgentStatus{}

	for _, txt := range entry.Text {
		if idx := strings.Index(txt, "="); idx > 0 {
			key := txt[:idx]
			value := txt[idx+1:]

			switch key {
			case "agent_id":
				status.AgentID = value
			case "status":
				status.Status = value
			case "task":
				status.CurrentTask = value
			case "action":
				status.LastAction = value
			case "updated":
				ts, _ := time.Parse(time.RFC3339, value)
				status.UpdatedAt = ts
			}
		}
	}

	return status
}

func parseAgentMessages(entry *zeroconf.ServiceEntry) []AgentMessage {
	var messages []AgentMessage

	for _, txt := range entry.Text {
		if idx := strings.Index(txt, "="); idx > 0 {
			key := txt[:idx]
			value := txt[idx+1:]

			if strings.HasPrefix(key, "msg") {
				var msg AgentMessage
				if err := json.Unmarshal([]byte(value), &msg); err == nil {
					messages = append(messages, msg)
				}
			}
		}
	}

	return messages
}
