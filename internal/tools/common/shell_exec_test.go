package common

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestShellExecTool(t *testing.T) {
	tool := NewShellExecTool(5 * time.Second)

	if tool.Name() != "shell_exec" {
		t.Errorf("expected name 'shell_exec', got %q", tool.Name())
	}

	input, _ := json.Marshal(map[string]string{"command": "echo hello"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "hello\n" {
		t.Errorf("expected 'hello\\n', got %q", result)
	}
}

func TestShellExecToolTimeout(t *testing.T) {
	tool := NewShellExecTool(100 * time.Millisecond)

	input, _ := json.Marshal(map[string]string{"command": "sleep 1"})
	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected timeout error")
	}
}
