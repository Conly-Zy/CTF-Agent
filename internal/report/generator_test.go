package report

import (
	"strings"
	"testing"
	"time"

	"github.com/Conly-Zy/CTF-Agent/internal/store"
)

func TestGenerateFromEvidence(t *testing.T) {
	started := time.Now().Add(-2 * time.Minute)
	completed := started.Add(time.Minute)
	session := &store.Session{
		ID:            7,
		ChallengeType: "web",
		Description:   "Find the flag",
		Target:        "http://challenge.local",
		Status:        "success",
		Flag:          "flag{test}",
		Iterations:    3,
		DurationMs:    int64((2 * time.Minute).Milliseconds()),
		CreatedAt:     started,
		CompletedAt:   &completed,
	}
	subtasks := []store.Subtask{
		{
			ID:          1,
			AgentName:   "WebAgent",
			AgentType:   "web",
			Title:       "Enumerate routes",
			Description: "Check robots and source",
			Status:      "success",
			Result:      "found /admin",
			CreatedAt:   started,
			CompletedAt: &completed,
		},
	}
	toolCalls := []store.ToolCall{
		{
			ID:          1,
			AgentName:   "WebAgent",
			AgentType:   "web",
			ToolName:    "http_request",
			Status:      "finished",
			Output:      "200 OK",
			StartedAt:   started.Add(10 * time.Second),
			CompletedAt: &completed,
			DurationMs:  100,
		},
	}

	gen := NewReportGenerator()
	report := gen.GenerateFromEvidence(session, subtasks, toolCalls)
	if len(report.Subtasks) != 1 {
		t.Fatalf("expected one subtask, got %d", len(report.Subtasks))
	}
	if len(report.ToolsUsed) != 1 || report.ToolsUsed[0].Name != "http_request" {
		t.Fatalf("unexpected tools: %+v", report.ToolsUsed)
	}
	if len(report.Timeline) < 2 {
		t.Fatalf("expected evidence timeline, got %+v", report.Timeline)
	}

	md := gen.RenderMarkdown(report)
	if !strings.Contains(md, "## Agent 子任务") || !strings.Contains(md, "http_request") {
		t.Fatalf("markdown missing evidence sections:\n%s", md)
	}
}
