package planner

import (
	"testing"

	"github.com/Conly-Zy/CTF-Agent/internal/store"
)

func TestGenerateFromTemplate(t *testing.T) {
	tpl := &store.FlowTemplate{
		Title: "Web baseline",
		Content: `1. Recon: collect headers
2. Discovery: enumerate paths
- Exploit: validate one primitive`,
	}
	steps := GenerateFromTemplate("web", tpl, "")
	if len(steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(steps))
	}
	if steps[0].Title != "Recon" || steps[0].Description != "collect headers" {
		t.Fatalf("unexpected first step: %+v", steps[0])
	}
	if steps[2].Title != "Exploit" {
		t.Fatalf("unexpected bullet step: %+v", steps[2])
	}
}

func TestGenerateFromTemplateFallback(t *testing.T) {
	steps := GenerateFromTemplate("misc", nil, "inspect files")
	if len(steps) != 1 || steps[0].Description != "inspect files" {
		t.Fatalf("unexpected fallback: %+v", steps)
	}
}

func TestRefineStatus(t *testing.T) {
	sess := &store.Session{Status: "success", Flag: "flag{ok}"}
	st := store.Subtask{Title: "Flag extraction", Description: "recover flag"}
	status, result, errMsg := RefineStatus(sess, st, nil)
	if status != "success" || result == "" || errMsg != "" {
		t.Fatalf("unexpected success refinement: %q %q %q", status, result, errMsg)
	}

	sess.Status = "failed"
	sess.Error = "max iterations"
	status, _, errMsg = RefineStatus(sess, st, nil)
	if status != StatusNeedsReview || errMsg != "max iterations" {
		t.Fatalf("unexpected failed refinement: %q %q", status, errMsg)
	}
}
