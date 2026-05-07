package agent

import (
	"context"
	"encoding/json"
	"time"
)

type executionScopeKey struct{}

// ExecutionScope carries runtime identifiers used by recorders to correlate
// agent events with persisted sessions/subtasks/tool calls.
type ExecutionScope struct {
	SessionID int64  `json:"session_id"`
	TaskID    string `json:"task_id,omitempty"`
}

// WithExecutionScope attaches session/task metadata to a context.
func WithExecutionScope(ctx context.Context, scope ExecutionScope) context.Context {
	return context.WithValue(ctx, executionScopeKey{}, scope)
}

// ExecutionScopeFromContext returns recorder metadata if present.
func ExecutionScopeFromContext(ctx context.Context) (ExecutionScope, bool) {
	scope, ok := ctx.Value(executionScopeKey{}).(ExecutionScope)
	return scope, ok
}

// ExecutionRecorder is the small observability contract used by agents.
// Implementations should be best-effort and must not panic on persistence errors.
type ExecutionRecorder interface {
	AgentStarted(ctx context.Context, event AgentEvent)
	AgentCompleted(ctx context.Context, event AgentEvent)
	ToolCallStarted(ctx context.Context, event ToolCallEvent)
	ToolCallCompleted(ctx context.Context, event ToolCallEvent)
	Thinking(ctx context.Context, event ThinkingEvent)
	FlagFound(ctx context.Context, event FlagEvent)
}

type AgentEvent struct {
	Task        Task      `json:"task"`
	AgentName   string    `json:"agent_name"`
	AgentType   AgentType `json:"agent_type"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
	Result      *Result   `json:"result,omitempty"`
	Error       string    `json:"error,omitempty"`
}

type ToolCallEvent struct {
	TaskID      string          `json:"task_id"`
	AgentName   string          `json:"agent_name"`
	AgentType   AgentType       `json:"agent_type"`
	ToolUseID   string          `json:"tool_use_id"`
	ToolName    string          `json:"tool_name"`
	Input       json.RawMessage `json:"input,omitempty"`
	Output      string          `json:"output,omitempty"`
	Error       string          `json:"error,omitempty"`
	StartedAt   time.Time       `json:"started_at,omitempty"`
	CompletedAt time.Time       `json:"completed_at,omitempty"`
}

type ThinkingEvent struct {
	TaskID    string    `json:"task_id"`
	AgentName string    `json:"agent_name"`
	AgentType AgentType `json:"agent_type"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type FlagEvent struct {
	TaskID    string    `json:"task_id"`
	AgentName string    `json:"agent_name"`
	AgentType AgentType `json:"agent_type"`
	Flag      string    `json:"flag"`
	CreatedAt time.Time `json:"created_at"`
}
