package store

import (
	"os"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) *SQLiteStore {
	tmpFile, err := os.CreateTemp("", "ctf-agent-test-*.db")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	tmpFile.Close()

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	t.Cleanup(func() {
		store.Close()
		os.Remove(tmpFile.Name())
	})

	return store
}

func TestCreateAndGetSession(t *testing.T) {
	store := setupTestDB(t)

	sess := &Session{
		ChallengeType: "web",
		Description:   "Test challenge",
		Target:        "http://example.com",
		Status:        "pending",
		CreatedAt:     time.Now(),
	}

	if err := store.CreateSession(sess); err != nil {
		t.Fatalf("create session: %v", err)
	}

	if sess.ID == 0 {
		t.Fatal("expected non-zero ID")
	}

	got, err := store.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}

	if got.ChallengeType != "web" {
		t.Errorf("expected type 'web', got %q", got.ChallengeType)
	}

	if got.Description != "Test challenge" {
		t.Errorf("expected description 'Test challenge', got %q", got.Description)
	}
}

func TestUpdateSession(t *testing.T) {
	store := setupTestDB(t)

	sess := &Session{
		ChallengeType: "web",
		Status:        "pending",
		CreatedAt:     time.Now(),
	}
	store.CreateSession(sess)

	sess.Status = "success"
	sess.Flag = "flag{test}"
	sess.Iterations = 5
	completedAt := time.Now()
	sess.CompletedAt = &completedAt

	if err := store.UpdateSession(sess); err != nil {
		t.Fatalf("update session: %v", err)
	}

	got, _ := store.GetSession(sess.ID)
	if got.Status != "success" {
		t.Errorf("expected status 'success', got %q", got.Status)
	}
	if got.Flag != "flag{test}" {
		t.Errorf("expected flag 'flag{test}', got %q", got.Flag)
	}
}

func TestListSessions(t *testing.T) {
	store := setupTestDB(t)

	for i := 0; i < 5; i++ {
		store.CreateSession(&Session{
			ChallengeType: "web",
			Status:        "success",
			CreatedAt:     time.Now(),
		})
	}

	sessions, err := store.ListSessions(3, 0)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}

	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(sessions))
	}
}

func TestKnowledgeOperations(t *testing.T) {
	store := setupTestDB(t)

	sess := &Session{
		ChallengeType: "web",
		Status:        "success",
		CreatedAt:     time.Now(),
	}
	store.CreateSession(sess)

	k := &Knowledge{
		SessionID: sess.ID,
		Title:     "SQL Injection",
		Content:   "# SQL Injection\n\nThis is a test.",
		Type:      "vulnerability",
		CreatedAt: time.Now(),
	}

	if err := store.CreateKnowledge(k); err != nil {
		t.Fatalf("create knowledge: %v", err)
	}

	got, err := store.GetKnowledge(k.ID)
	if err != nil {
		t.Fatalf("get knowledge: %v", err)
	}

	if got.Title != "SQL Injection" {
		t.Errorf("expected title 'SQL Injection', got %q", got.Title)
	}

	items, err := store.ListKnowledge("", 10, 0)
	if err != nil {
		t.Fatalf("list knowledge: %v", err)
	}

	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
}

func TestSearchKnowledge(t *testing.T) {
	store := setupTestDB(t)

	sess := &Session{ChallengeType: "web", Status: "success", CreatedAt: time.Now()}
	store.CreateSession(sess)

	store.CreateKnowledge(&Knowledge{
		SessionID: sess.ID,
		Title:     "SQL Injection Tutorial",
		Content:   "How to perform SQL injection",
		Type:      "technique",
		CreatedAt: time.Now(),
	})

	store.CreateKnowledge(&Knowledge{
		SessionID: sess.ID,
		Title:     "XSS Attack",
		Content:   "Cross-site scripting basics",
		Type:      "vulnerability",
		CreatedAt: time.Now(),
	})

	items, err := store.SearchKnowledge("SQL", 10)
	if err != nil {
		t.Fatalf("search knowledge: %v", err)
	}

	if len(items) != 1 {
		t.Errorf("expected 1 result, got %d", len(items))
	}
}

func TestTagOperations(t *testing.T) {
	store := setupTestDB(t)

	tag, err := store.CreateTag("web")
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}

	if tag.Name != "web" {
		t.Errorf("expected name 'web', got %q", tag.Name)
	}

	tags, err := store.ListTags()
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}

	if len(tags) != 1 {
		t.Errorf("expected 1 tag, got %d", len(tags))
	}
}

func TestConversationMessages(t *testing.T) {
	store := setupTestDB(t)

	sess := &Session{ChallengeType: "web", Status: "success", CreatedAt: time.Now()}
	store.CreateSession(sess)

	msg := &ConversationMessage{
		SessionID: sess.ID,
		Role:      "assistant",
		Content:   "I'll analyze this challenge",
		CreatedAt: time.Now(),
	}

	if err := store.AddConversationMessage(msg); err != nil {
		t.Fatalf("add message: %v", err)
	}

	messages, err := store.GetConversationMessages(sess.ID)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}

	if len(messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(messages))
	}
}

func TestGetStats(t *testing.T) {
	store := setupTestDB(t)

	store.CreateSession(&Session{ChallengeType: "web", Status: "success", CreatedAt: time.Now()})
	store.CreateSession(&Session{ChallengeType: "web", Status: "failed", CreatedAt: time.Now()})
	store.CreateSession(&Session{ChallengeType: "pwn", Status: "success", CreatedAt: time.Now()})

	stats, err := store.GetStats()
	if err != nil {
		t.Fatalf("get stats: %v", err)
	}

	if stats.TotalSessions != 3 {
		t.Errorf("expected 3 total sessions, got %d", stats.TotalSessions)
	}

	if stats.SuccessSessions != 2 {
		t.Errorf("expected 2 success sessions, got %d", stats.SuccessSessions)
	}

	if stats.ByType["web"] != 2 {
		t.Errorf("expected 2 web sessions, got %d", stats.ByType["web"])
	}

	if stats.ByType["pwn"] != 1 {
		t.Errorf("expected 1 pwn session, got %d", stats.ByType["pwn"])
	}
}

func TestSubtaskOperations(t *testing.T) {
	store := setupTestDB(t)
	sess := &Session{ChallengeType: "web", Status: "solving", CreatedAt: time.Now()}
	if err := store.CreateSession(sess); err != nil {
		t.Fatalf("create session: %v", err)
	}

	st := &Subtask{
		SessionID:     sess.ID,
		TaskID:        "task-1",
		AgentName:     "WebAgent",
		AgentType:     "web",
		ChallengeType: "web",
		Description:   "Enumerate routes",
		Status:        "running",
		SortOrder:     2,
	}
	if err := store.CreateSubtask(st); err != nil {
		t.Fatalf("create subtask: %v", err)
	}
	if st.ID == 0 {
		t.Fatal("expected subtask id")
	}

	completedAt := time.Now()
	st.Status = "success"
	st.Result = "found /admin"
	st.CompletedAt = &completedAt
	if err := store.UpdateSubtask(st); err != nil {
		t.Fatalf("update subtask: %v", err)
	}

	items, err := store.ListSubtasks(sess.ID, "success")
	if err != nil {
		t.Fatalf("list subtasks: %v", err)
	}
	if len(items) != 1 || items[0].Result != "found /admin" {
		t.Fatalf("unexpected subtasks: %+v", items)
	}

	plan := &Subtask{
		ID:            st.ID,
		SessionID:     sess.ID,
		TaskID:        "task-1",
		AgentName:     "Planner",
		AgentType:     "planner",
		ChallengeType: "web",
		Title:         "Updated plan",
		Description:   "updated",
		Status:        "planned",
		SortOrder:     1,
	}
	if err := store.UpdateSubtaskPlan(plan); err != nil {
		t.Fatalf("update subtask plan: %v", err)
	}
	got, err := store.GetSubtask(st.ID)
	if err != nil {
		t.Fatalf("get subtask: %v", err)
	}
	if got.Title != "Updated plan" || got.SortOrder != 1 || got.AgentType != "planner" {
		t.Fatalf("unexpected updated plan: %+v", got)
	}
}

func TestToolCallOperations(t *testing.T) {
	store := setupTestDB(t)
	sess := &Session{ChallengeType: "web", Status: "solving", CreatedAt: time.Now()}
	if err := store.CreateSession(sess); err != nil {
		t.Fatalf("create session: %v", err)
	}

	call := &ToolCall{
		SessionID: sess.ID,
		TaskID:    "task-1",
		AgentName: "WebAgent",
		AgentType: "web",
		ToolUseID: "toolu_test",
		ToolName:  "http_request",
		Input:     `{"url":"http://example.com"}`,
		Status:    "running",
		StartedAt: time.Now(),
	}
	if err := store.CreateToolCall(call); err != nil {
		t.Fatalf("create tool call: %v", err)
	}
	completedAt := time.Now()
	call.Output = "HTTP/1.1 200 OK"
	call.Status = "finished"
	call.CompletedAt = &completedAt
	if err := store.CompleteToolCall(call); err != nil {
		t.Fatalf("complete tool call: %v", err)
	}

	calls, err := store.ListToolCalls(sess.ID, 10)
	if err != nil {
		t.Fatalf("list tool calls: %v", err)
	}
	if len(calls) != 1 || calls[0].ToolName != "http_request" || calls[0].Status != "finished" {
		t.Fatalf("unexpected tool calls: %+v", calls)
	}

	stats, err := store.GetToolCallStats(sess.ID)
	if err != nil {
		t.Fatalf("get tool call stats: %v", err)
	}
	if stats.TotalCalls != 1 || stats.SuccessCalls != 1 || stats.FailedCalls != 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if len(stats.ByTool) != 1 || stats.ByTool[0].Name != "http_request" {
		t.Fatalf("unexpected by_tool stats: %+v", stats.ByTool)
	}
	if len(stats.ByAgent) != 1 || stats.ByAgent[0].Name != "WebAgent" {
		t.Fatalf("unexpected by_agent stats: %+v", stats.ByAgent)
	}
}

func TestFlowTemplateOperations(t *testing.T) {
	store := setupTestDB(t)

	defaults, err := store.ListFlowTemplates("web")
	if err != nil {
		t.Fatalf("list default templates: %v", err)
	}
	if len(defaults) == 0 {
		t.Fatal("expected seeded web templates")
	}

	tpl := &FlowTemplate{
		ChallengeType: "misc",
		Title:         "Custom misc playbook",
		Description:   "custom",
		Content:       "1. inspect artifacts\n2. recover flag",
		Tags:          "misc,custom",
	}
	if err := store.CreateFlowTemplate(tpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	if tpl.ID == 0 {
		t.Fatal("expected template id")
	}

	tpl.Title = "Updated misc playbook"
	if err := store.UpdateFlowTemplate(tpl); err != nil {
		t.Fatalf("update template: %v", err)
	}
	got, err := store.GetFlowTemplate(tpl.ID)
	if err != nil {
		t.Fatalf("get template: %v", err)
	}
	if got.Title != "Updated misc playbook" {
		t.Fatalf("unexpected title: %q", got.Title)
	}

	if err := store.DeleteFlowTemplate(tpl.ID); err != nil {
		t.Fatalf("delete template: %v", err)
	}
}
