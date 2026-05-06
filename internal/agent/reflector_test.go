package agent

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
)

func TestReflector_OnNoToolCall(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	reflector := NewReflector(logger, 3)

	tests := []struct {
		name        string
		iteration   int
		expectRetry bool
	}{
		{"first attempt", 0, true},
		{"second attempt", 1, true},
		{"max retries", 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reflector.ReflectOnNoToolCall(context.Background(), "TestAgent", tt.iteration, "test response")
			if result.ShouldRetry != tt.expectRetry {
				t.Errorf("expected ShouldRetry=%v, got %v", tt.expectRetry, result.ShouldRetry)
			}
		})
	}
}

func TestReflector_OnError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	reflector := NewReflector(logger, 3)

	result := reflector.ReflectOnError(context.Background(), "TestAgent", "shell_exec", errors.New("command failed"))
	if !result.ShouldRetry {
		t.Error("expected ShouldRetry=true")
	}
	if result.Suggestion == "" {
		t.Error("expected non-empty suggestion")
	}
}

func TestExecutionMonitor_RecordAndCheck(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	monitor := NewExecutionMonitor(logger, 3, 10, 5)

	// Record some tool calls
	monitor.RecordToolCall("shell_exec")
	monitor.RecordToolCall("shell_exec")
	monitor.RecordToolCall("shell_exec")

	// Should detect loop (same tool 3 times)
	if !monitor.CheckForLoop() {
		t.Error("expected loop detection after 3 same tool calls")
	}
}

func TestExecutionMonitor_NoLoop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	monitor := NewExecutionMonitor(logger, 3, 10, 5)

	// Record different tool calls
	monitor.RecordToolCall("shell_exec")
	monitor.RecordToolCall("file_read")
	monitor.RecordToolCall("http_request")

	// Should not detect loop
	if monitor.CheckForLoop() {
		t.Error("unexpected loop detection")
	}
}

func TestExecutionMonitor_Reset(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	monitor := NewExecutionMonitor(logger, 3, 10, 5)

	monitor.RecordToolCall("shell_exec")
	monitor.RecordToolCall("shell_exec")
	monitor.RecordToolCall("shell_exec")

	monitor.Reset()

	// After reset, should not detect loop
	if monitor.CheckForLoop() {
		t.Error("unexpected loop detection after reset")
	}
}
