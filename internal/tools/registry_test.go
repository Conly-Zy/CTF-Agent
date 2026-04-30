package tools

import (
	"context"
	"encoding/json"
	"testing"
)

type mockTool struct {
	name string
}

func (m *mockTool) Name() string {
	return m.name
}

func (m *mockTool) Description() string {
	return "mock tool"
}

func (m *mockTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	return "mock result", nil
}

func (m *mockTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"input": map[string]any{
				"type": "string",
			},
		},
	}
}

func TestRegistry(t *testing.T) {
	registry := NewRegistry()

	tool := &mockTool{name: "test_tool"}
	if err := registry.Register(tool); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := registry.Register(tool); err == nil {
		t.Error("expected error for duplicate registration")
	}

	got, ok := registry.Get("test_tool")
	if !ok {
		t.Fatal("expected to find tool")
	}
	if got.Name() != "test_tool" {
		t.Errorf("expected 'test_tool', got %q", got.Name())
	}

	_, ok = registry.Get("nonexistent")
	if ok {
		t.Error("expected not to find nonexistent tool")
	}

	names := registry.Names()
	if len(names) != 1 || names[0] != "test_tool" {
		t.Errorf("expected ['test_tool'], got %v", names)
	}

	claudeTools := registry.ToClaudeTools()
	if len(claudeTools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(claudeTools))
	}
	if claudeTools[0]["name"] != "test_tool" {
		t.Errorf("expected 'test_tool', got %q", claudeTools[0]["name"])
	}
}
