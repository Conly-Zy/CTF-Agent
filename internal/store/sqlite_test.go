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
