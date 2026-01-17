package sdk

import (
	"context"
	"testing"

	"brutus/tools"
)

func TestToolRunner_Execute(t *testing.T) {
	runner := NewToolRunner()
	runner.Register(tools.ReadFileTool)

	result, err := runner.Execute("read_file", `{"path": "../main.go"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
	if len(runner.GetCalls()) != 1 {
		t.Errorf("expected 1 call, got %d", len(runner.GetCalls()))
	}
}

func TestToolRunner_ExecuteUnknownTool(t *testing.T) {
	runner := NewToolRunner()

	_, err := runner.Execute("unknown_tool", `{}`)
	if err == nil {
		t.Error("expected error for unknown tool")
	}
}

func TestMockProvider_QueueAndChat(t *testing.T) {
	ctx := context.Background()
	mock := NewMockProvider()

	mock.QueueTextResponse("Hello!")
	mock.QueueTextResponse("How can I help?")

	resp1, err := mock.Chat(ctx, "", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp1.Content != "Hello!" {
		t.Errorf("expected 'Hello!', got '%s'", resp1.Content)
	}

	resp2, err := mock.Chat(ctx, "", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp2.Content != "How can I help?" {
		t.Errorf("expected 'How can I help?', got '%s'", resp2.Content)
	}
}

func TestMockProvider_QueueToolCall(t *testing.T) {
	ctx := context.Background()
	mock := NewMockProvider()

	mock.QueueToolCall("read_file", map[string]interface{}{"path": "test.go"})

	resp, err := mock.Chat(ctx, "", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "read_file" {
		t.Errorf("expected 'read_file', got '%s'", resp.ToolCalls[0].Name)
	}
}

func TestHarness_BasicFlow(t *testing.T) {
	ctx := context.Background()
	harness := NewHarness().
		WithDefaultTools().
		QueueToolCall("list_files", map[string]interface{}{"path": ".", "recursive": false}).
		QueueTextResponse("I found several files.")

	harness.SendUserMessage("List files in current directory")

	if err := harness.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !harness.ToolWasCalled("list_files") {
		t.Error("expected list_files to be called")
	}

	if harness.LastAssistantMessage() != "I found several files." {
		t.Errorf("unexpected last message: %s", harness.LastAssistantMessage())
	}
}

func TestHarness_ToolCallCount(t *testing.T) {
	ctx := context.Background()
	harness := NewHarness().
		WithDefaultTools().
		QueueToolCall("read_file", map[string]interface{}{"path": "main.go"}).
		QueueToolCall("read_file", map[string]interface{}{"path": "app.go"}).
		QueueTextResponse("Done reading both files.")

	harness.SendUserMessage("Read main.go and app.go")

	if err := harness.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if harness.ToolCallCount("read_file") != 2 {
		t.Errorf("expected 2 read_file calls, got %d", harness.ToolCallCount("read_file"))
	}
}

func TestDefaultToolRunner(t *testing.T) {
	runner := DefaultToolRunner()
	
	expectedTools := []string{"read_file", "list_files", "edit_file", "bash", "code_search"}
	for _, name := range expectedTools {
		if _, ok := runner.GetRegistry().Get(name); !ok {
			t.Errorf("expected tool '%s' to be registered", name)
		}
	}
}
