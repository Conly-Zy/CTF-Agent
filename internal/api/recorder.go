package api

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Conly-Zy/CTF-Agent/internal/agent"
	"github.com/Conly-Zy/CTF-Agent/internal/store"
)

type storeRecorder struct {
	store  *store.SQLiteStore
	logger *slog.Logger

	mu        sync.Mutex
	subtasks  map[string]int64
	toolCalls map[string]int64
}

func newStoreRecorder(st *store.SQLiteStore, logger *slog.Logger) *storeRecorder {
	return &storeRecorder{
		store:     st,
		logger:    logger,
		subtasks:  make(map[string]int64),
		toolCalls: make(map[string]int64),
	}
}

func (r *storeRecorder) AgentStarted(ctx context.Context, event agent.AgentEvent) {
	sessionID, ok := r.sessionID(ctx)
	if !ok {
		return
	}

	st := &store.Subtask{
		SessionID:     sessionID,
		TaskID:        event.Task.ID,
		ParentID:      event.Task.ParentID,
		AgentName:     event.AgentName,
		AgentType:     string(event.AgentType),
		ChallengeType: event.Task.Type,
		Title:         titleForTask(event),
		Description:   event.Task.Description,
		Target:        event.Task.Target,
		Status:        "running",
	}
	if err := r.store.CreateSubtask(st); err != nil {
		r.logger.Warn("record subtask start failed", "error", err, "task_id", event.Task.ID, "agent", event.AgentName)
		return
	}

	r.mu.Lock()
	r.subtasks[r.subtaskKey(sessionID, event.Task.ID, event.AgentName)] = st.ID
	r.mu.Unlock()
}

func (r *storeRecorder) AgentCompleted(ctx context.Context, event agent.AgentEvent) {
	sessionID, ok := r.sessionID(ctx)
	if !ok {
		return
	}

	id, ok := r.lookupSubtask(sessionID, event.Task.ID, event.AgentName)
	if !ok {
		return
	}

	completedAt := event.CompletedAt
	if completedAt.IsZero() {
		completedAt = time.Now()
	}
	st := &store.Subtask{ID: id, CompletedAt: &completedAt}
	if event.Error != "" {
		st.Status = "failed"
		st.Error = event.Error
	} else if event.Result != nil {
		if event.Result.Success {
			st.Status = "success"
		} else {
			st.Status = "failed"
		}
		st.Result = event.Result.Summary
		if st.Result == "" {
			st.Result = event.Result.Output
		}
		st.Error = event.Result.Error
	} else {
		st.Status = "finished"
	}

	if err := r.store.UpdateSubtask(st); err != nil {
		r.logger.Warn("record subtask completion failed", "error", err, "subtask_id", id)
	}
}

func (r *storeRecorder) ToolCallStarted(ctx context.Context, event agent.ToolCallEvent) {
	sessionID, ok := r.sessionID(ctx)
	if !ok {
		return
	}

	var subtaskID *int64
	if id, ok := r.lookupSubtask(sessionID, event.TaskID, event.AgentName); ok {
		subtaskID = &id
	}
	call := &store.ToolCall{
		SessionID: sessionID,
		SubtaskID: subtaskID,
		TaskID:    event.TaskID,
		AgentName: event.AgentName,
		AgentType: string(event.AgentType),
		ToolUseID: event.ToolUseID,
		ToolName:  event.ToolName,
		Input:     string(event.Input),
		Status:    "running",
		StartedAt: event.StartedAt,
	}
	if call.StartedAt.IsZero() {
		call.StartedAt = time.Now()
	}
	if err := r.store.CreateToolCall(call); err != nil {
		r.logger.Warn("record tool call start failed", "error", err, "tool", event.ToolName)
		return
	}

	r.mu.Lock()
	r.toolCalls[r.toolCallKey(sessionID, event.TaskID, event.AgentName, event.ToolUseID, event.ToolName)] = call.ID
	r.mu.Unlock()
}

func (r *storeRecorder) ToolCallCompleted(ctx context.Context, event agent.ToolCallEvent) {
	sessionID, ok := r.sessionID(ctx)
	if !ok {
		return
	}

	id, ok := r.lookupToolCall(sessionID, event.TaskID, event.AgentName, event.ToolUseID, event.ToolName)
	if !ok {
		// Some callers may only emit a completion event (for example, tool not found).
		r.ToolCallStarted(ctx, event)
		id, ok = r.lookupToolCall(sessionID, event.TaskID, event.AgentName, event.ToolUseID, event.ToolName)
		if !ok {
			return
		}
	}

	completedAt := event.CompletedAt
	if completedAt.IsZero() {
		completedAt = time.Now()
	}
	status := "finished"
	if event.Error != "" {
		status = "failed"
	}
	call := &store.ToolCall{
		ID:          id,
		Output:      event.Output,
		Status:      status,
		Error:       event.Error,
		StartedAt:   event.StartedAt,
		CompletedAt: &completedAt,
	}
	if !event.StartedAt.IsZero() {
		call.DurationMs = completedAt.Sub(event.StartedAt).Milliseconds()
	}
	if err := r.store.CompleteToolCall(call); err != nil {
		r.logger.Warn("record tool call completion failed", "error", err, "tool_call_id", id)
	}
}

func (r *storeRecorder) Thinking(ctx context.Context, event agent.ThinkingEvent) {
	sessionID, ok := r.sessionID(ctx)
	if !ok || event.Content == "" {
		return
	}
	msg := &store.ConversationMessage{
		SessionID: sessionID,
		Role:      "assistant",
		Content:   fmt.Sprintf("[%s] %s", event.AgentName, event.Content),
	}
	if err := r.store.AddConversationMessage(msg); err != nil {
		r.logger.Warn("record thinking failed", "error", err)
	}
}

func (r *storeRecorder) FlagFound(ctx context.Context, event agent.FlagEvent) {
	sessionID, ok := r.sessionID(ctx)
	if !ok || event.Flag == "" {
		return
	}
	msg := &store.ConversationMessage{
		SessionID: sessionID,
		Role:      "assistant",
		Content:   fmt.Sprintf("[%s] Flag: %s", event.AgentName, event.Flag),
	}
	if err := r.store.AddConversationMessage(msg); err != nil {
		r.logger.Warn("record flag failed", "error", err)
	}
}

func (r *storeRecorder) sessionID(ctx context.Context) (int64, bool) {
	scope, ok := agent.ExecutionScopeFromContext(ctx)
	if !ok || scope.SessionID == 0 {
		return 0, false
	}
	return scope.SessionID, true
}

func (r *storeRecorder) subtaskKey(sessionID int64, taskID, agentName string) string {
	return fmt.Sprintf("%d:%s:%s", sessionID, taskID, agentName)
}

func (r *storeRecorder) toolCallKey(sessionID int64, taskID, agentName, toolUseID, toolName string) string {
	return fmt.Sprintf("%d:%s:%s:%s:%s", sessionID, taskID, agentName, toolUseID, toolName)
}

func (r *storeRecorder) lookupSubtask(sessionID int64, taskID, agentName string) (int64, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.subtasks[r.subtaskKey(sessionID, taskID, agentName)]
	return id, ok
}

func (r *storeRecorder) lookupToolCall(sessionID int64, taskID, agentName, toolUseID, toolName string) (int64, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.toolCalls[r.toolCallKey(sessionID, taskID, agentName, toolUseID, toolName)]
	return id, ok
}

func titleForTask(event agent.AgentEvent) string {
	if event.Task.Description == "" {
		return event.AgentName
	}
	desc := event.Task.Description
	if len(desc) > 80 {
		desc = desc[:80] + "..."
	}
	return fmt.Sprintf("%s: %s", event.AgentName, desc)
}
