package sdk

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMultiAgentHarness_RunSequential(t *testing.T) {
	harness := NewMultiAgentHarness()

	harness.AddAgent(AgentConfig{
		ID:           "test-agent-1",
		SystemPrompt: "You are a test agent.",
	})

	harness.AddAgent(AgentConfig{
		ID:           "test-agent-2",
		SystemPrompt: "You are another test agent.",
	})

	harness.QueueResponseForAgent("test-agent-1", MockResponse{Content: "Agent 1 response"})
	harness.QueueResponseForAgent("test-agent-2", MockResponse{Content: "Agent 2 response"})

	messages := map[string][]string{
		"test-agent-1": {"Hello from test"},
		"test-agent-2": {"Hello from test 2"},
	}

	ctx := context.Background()
	results, err := harness.RunSequential(ctx, messages)

	if err != nil {
		t.Fatalf("RunSequential failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	for _, result := range results {
		if !result.Success {
			t.Errorf("Agent %s failed: %v", result.AgentID, result.Error)
		}
	}
}

func TestMultiAgentHarness_RunConcurrent(t *testing.T) {
	harness := NewMultiAgentHarness()

	harness.AddAgent(AgentConfig{
		ID:           "concurrent-1",
		SystemPrompt: "Agent 1",
	})

	harness.AddAgent(AgentConfig{
		ID:           "concurrent-2",
		SystemPrompt: "Agent 2",
	})

	harness.QueueResponseForAgent("concurrent-1", MockResponse{Content: "Concurrent 1 done"})
	harness.QueueResponseForAgent("concurrent-2", MockResponse{Content: "Concurrent 2 done"})

	messages := map[string][]string{
		"concurrent-1": {"Start work"},
		"concurrent-2": {"Start work"},
	}

	ctx := context.Background()
	results, err := harness.RunConcurrent(ctx, messages)

	if err != nil {
		t.Fatalf("RunConcurrent failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	foundAgent1 := false
	foundAgent2 := false
	for _, result := range results {
		if result.AgentID == "concurrent-1" {
			foundAgent1 = true
		}
		if result.AgentID == "concurrent-2" {
			foundAgent2 = true
		}
		if !result.Success {
			t.Errorf("Agent %s failed: %v", result.AgentID, result.Error)
		}
	}

	if !foundAgent1 || !foundAgent2 {
		t.Error("Not all agents returned results")
	}
}

func TestMultiAgentHarness_WithToolCalls(t *testing.T) {
	harness := NewMultiAgentHarness().WithVerbose(false)

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("original content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	harness.AddAgent(AgentConfig{
		ID:           "editor",
		SystemPrompt: "You are an editor agent.",
	})

	harness.QueueResponseForAgent("editor",
		MockResponse{
			ToolCall: "read_file",
			Input:    map[string]interface{}{"path": testFile},
		},
		MockResponse{Content: "I read the file."},
	)

	messages := map[string][]string{
		"editor": {"Read the test file"},
	}

	ctx := context.Background()
	results, err := harness.RunSequential(ctx, messages)

	if err != nil {
		t.Fatalf("RunSequential failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]
	if !result.Success {
		t.Errorf("Agent failed: %v", result.Error)
	}

	if len(result.ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(result.ToolCalls))
	}

	if result.ToolCalls[0].Name != "read_file" {
		t.Errorf("Expected read_file tool call, got %s", result.ToolCalls[0].Name)
	}
}

func TestMultiAgentScenario_Load(t *testing.T) {
	tmpDir := t.TempDir()
	scenarioFile := filepath.Join(tmpDir, "test-scenario.json")

	scenarioJSON := `{
		"name": "Test Scenario",
		"description": "A test scenario",
		"agents": [
			{
				"id": "agent-1",
				"system_prompt": "You are agent 1",
				"user_messages": ["Hello"],
				"mock_responses": [{"content": "Hi there"}]
			}
		],
		"assertions": [
			{"agent_id": "agent-1", "type": "success", "value": ""}
		]
	}`

	if err := os.WriteFile(scenarioFile, []byte(scenarioJSON), 0644); err != nil {
		t.Fatalf("Failed to write scenario file: %v", err)
	}

	scenario, err := LoadMultiAgentScenario(scenarioFile)
	if err != nil {
		t.Fatalf("Failed to load scenario: %v", err)
	}

	if scenario.Name != "Test Scenario" {
		t.Errorf("Expected name 'Test Scenario', got '%s'", scenario.Name)
	}

	if len(scenario.Agents) != 1 {
		t.Errorf("Expected 1 agent, got %d", len(scenario.Agents))
	}

	if scenario.Agents[0].ID != "agent-1" {
		t.Errorf("Expected agent ID 'agent-1', got '%s'", scenario.Agents[0].ID)
	}
}

func TestMultiAgentHarness_ValidateAssertions(t *testing.T) {
	harness := NewMultiAgentHarness()

	harness.AddAgent(AgentConfig{
		ID:           "test-agent",
		SystemPrompt: "Test",
	})

	harness.QueueResponseForAgent("test-agent",
		MockResponse{ToolCall: "read_file", Input: map[string]interface{}{"path": "test.txt"}},
		MockResponse{Content: "The file contains important data"},
	)

	messages := map[string][]string{
		"test-agent": {"Read the file"},
	}

	ctx := context.Background()
	results, _ := harness.RunSequential(ctx, messages)

	assertions := []MultiAgentAssertion{
		{AgentID: "test-agent", Type: "tool_called", Value: "read_file"},
		{AgentID: "test-agent", Type: "contains", Value: "important"},
		{AgentID: "test-agent", Type: "success", Value: ""},
	}

	errors := harness.ValidateAssertions(results, assertions)

	if len(errors) != 0 {
		for _, err := range errors {
			t.Errorf("Assertion failed: %v", err)
		}
	}
}

func TestMultiAgentHarness_Summary(t *testing.T) {
	harness := NewMultiAgentHarness()

	harness.AddAgent(AgentConfig{
		ID:           "summary-agent",
		SystemPrompt: "Test",
	})

	harness.QueueResponseForAgent("summary-agent", MockResponse{Content: "Done"})

	messages := map[string][]string{
		"summary-agent": {"Do something"},
	}

	ctx := context.Background()
	harness.RunSequential(ctx, messages)

	summary := harness.Summary()

	if summary == "" {
		t.Error("Summary should not be empty")
	}

	if !contains(summary, "summary-agent") {
		t.Error("Summary should contain agent ID")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
