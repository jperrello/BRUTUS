package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

type BroadcastInput struct {
	AgentID string `json:"agent_id" jsonschema:"description=Your agent identifier,required"`
	Status  string `json:"status" jsonschema:"description=Current status (idle/working/done),required"`
	Task    string `json:"task" jsonschema:"description=Current task description"`
	Action  string `json:"action" jsonschema:"description=Last action taken"`
	Message string `json:"message" jsonschema:"description=Optional message to other agents"`
	UseTXT  bool   `json:"use_txt" jsonschema:"description=Use Saturn TXT records for real-time broadcast (requires network)"`
}

type ObserveInput struct {
	StatusDir string `json:"status_dir" jsonschema:"description=Directory containing agent status files"`
	UseTXT    bool   `json:"use_txt" jsonschema:"description=Use Saturn TXT records to discover agents on network"`
	Timeout   int    `json:"timeout" jsonschema:"description=Discovery timeout in seconds (default 2)"`
}

var (
	broadcastDir  = filepath.Join(os.TempDir(), "brutus-agents")
	broadcastLock sync.Mutex

	activeServers     = make(map[string]*zeroconf.Server)
	activeServersLock sync.Mutex
	nextPort          = 9100
)

func broadcastFunc(input json.RawMessage) (string, error) {
	var params BroadcastInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	if params.AgentID == "" {
		return "", fmt.Errorf("agent_id is required")
	}
	if params.Status == "" {
		return "", fmt.Errorf("status is required")
	}

	if params.UseTXT {
		return broadcastViaTXT(params)
	}
	return broadcastViaFile(params)
}

func broadcastViaFile(params BroadcastInput) (string, error) {
	broadcastLock.Lock()
	defer broadcastLock.Unlock()

	if err := os.MkdirAll(broadcastDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create broadcast directory: %w", err)
	}

	statusData := map[string]interface{}{
		"agent_id":   params.AgentID,
		"status":     params.Status,
		"task":       params.Task,
		"action":     params.Action,
		"message":    params.Message,
		"updated_at": time.Now().Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(statusData, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal status: %w", err)
	}

	statusFile := filepath.Join(broadcastDir, fmt.Sprintf("agent-%s.json", params.AgentID))
	if err := os.WriteFile(statusFile, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write status file: %w", err)
	}

	return fmt.Sprintf("Status broadcast (file): agent=%s status=%s task=%s",
		params.AgentID, params.Status, params.Task), nil
}

func broadcastViaTXT(params BroadcastInput) (string, error) {
	activeServersLock.Lock()
	defer activeServersLock.Unlock()

	if existing, ok := activeServers[params.AgentID]; ok {
		existing.Shutdown()
		delete(activeServers, params.AgentID)
	}

	txtRecords := []string{
		fmt.Sprintf("agent_id=%s", params.AgentID),
		fmt.Sprintf("status=%s", params.Status),
		fmt.Sprintf("task=%s", params.Task),
		fmt.Sprintf("action=%s", params.Action),
		fmt.Sprintf("updated=%d", time.Now().Unix()),
	}
	if params.Message != "" {
		msgJSON, _ := json.Marshal(map[string]string{
			"from":    params.AgentID,
			"content": params.Message,
			"time":    time.Now().Format(time.RFC3339),
		})
		txtRecords = append(txtRecords, fmt.Sprintf("msg=%s", string(msgJSON)))
	}

	port := nextPort
	nextPort++

	server, err := zeroconf.Register(
		fmt.Sprintf("brutus-agent-%s", params.AgentID),
		"_brutus-agent._tcp",
		"local.",
		port,
		txtRecords,
		[]net.Interface{},
	)
	if err != nil {
		return broadcastViaFile(params)
	}

	activeServers[params.AgentID] = server

	return fmt.Sprintf("Status broadcast (TXT): agent=%s status=%s task=%s (port %d)",
		params.AgentID, params.Status, params.Task, port), nil
}

func observeAgentsFunc(input json.RawMessage) (string, error) {
	var params ObserveInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	if params.UseTXT {
		return observeViaTXT(params)
	}
	return observeViaFile(params)
}

func observeViaFile(params ObserveInput) (string, error) {
	searchDir := broadcastDir
	if params.StatusDir != "" {
		searchDir = params.StatusDir
	}

	entries, err := os.ReadDir(searchDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "No agent broadcasts found", nil
		}
		return "", fmt.Errorf("failed to read directory: %w", err)
	}

	var agents []map[string]interface{}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if filepath.Ext(entry.Name()) == ".json" || filepath.Ext(entry.Name()) == ".md" {
			filePath := filepath.Join(searchDir, entry.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			if filepath.Ext(entry.Name()) == ".json" {
				var agentData map[string]interface{}
				if err := json.Unmarshal(data, &agentData); err == nil {
					agents = append(agents, agentData)
				}
			} else {
				agents = append(agents, map[string]interface{}{
					"file":    entry.Name(),
					"content": string(data),
				})
			}
		}
	}

	if len(agents) == 0 {
		return "No agent status files found", nil
	}

	result, err := json.MarshalIndent(agents, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal results: %w", err)
	}

	return string(result), nil
}

func observeViaTXT(params ObserveInput) (string, error) {
	timeout := 2 * time.Second
	if params.Timeout > 0 {
		timeout = time.Duration(params.Timeout) * time.Second
	}

	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return observeViaFile(params)
	}

	entries := make(chan *zeroconf.ServiceEntry, 10)
	var agents []map[string]interface{}
	var mu sync.Mutex

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	go func() {
		for entry := range entries {
			agent := parseAgentTXTRecords(entry)
			if agent["agent_id"] != nil {
				mu.Lock()
				agents = append(agents, agent)
				mu.Unlock()
			}
		}
	}()

	err = resolver.Browse(ctx, "_brutus-agent._tcp", "local.", entries)
	if err != nil {
		return observeViaFile(params)
	}

	<-ctx.Done()
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	agentsCopy := make([]map[string]interface{}, len(agents))
	copy(agentsCopy, agents)
	mu.Unlock()

	if len(agentsCopy) == 0 {
		return observeViaFile(params)
	}

	result, err := json.MarshalIndent(agentsCopy, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal results: %w", err)
	}

	return fmt.Sprintf("Discovered %d agents via TXT records:\n%s", len(agentsCopy), string(result)), nil
}

func parseAgentTXTRecords(entry *zeroconf.ServiceEntry) map[string]interface{} {
	agent := make(map[string]interface{})
	agent["service_name"] = entry.Instance
	agent["host"] = entry.HostName
	agent["port"] = entry.Port

	for _, txt := range entry.Text {
		if idx := strings.Index(txt, "="); idx > 0 {
			key := txt[:idx]
			value := txt[idx+1:]

			switch key {
			case "agent_id":
				agent["agent_id"] = value
			case "status":
				agent["status"] = value
			case "task":
				agent["task"] = value
			case "action":
				agent["action"] = value
			case "updated":
				agent["updated"] = value
			case "msg":
				var msg map[string]string
				if err := json.Unmarshal([]byte(value), &msg); err == nil {
					agent["message"] = msg
				}
			}
		}
	}

	return agent
}

func ShutdownAllBroadcasts() {
	activeServersLock.Lock()
	defer activeServersLock.Unlock()

	for id, server := range activeServers {
		server.Shutdown()
		delete(activeServers, id)
	}
}

var BroadcastTool = NewTool[BroadcastInput](
	"agent_broadcast",
	"Broadcast your agent status to other agents in the multi-agent system. Set use_txt=true for real-time network broadcast via Saturn mDNS TXT records, or use_txt=false (default) for file-based broadcast.",
	broadcastFunc,
)

var ObserveAgentsTool = NewTool[ObserveInput](
	"observe_agents",
	"Observe the status of other agents in the multi-agent system. Set use_txt=true to discover agents via Saturn mDNS TXT records on the network, or use_txt=false (default) to read from status files.",
	observeAgentsFunc,
)
