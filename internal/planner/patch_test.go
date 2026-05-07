package planner

import (
	"testing"

	"github.com/Conly-Zy/CTF-Agent/internal/store"
)

func TestApplyPatch(t *testing.T) {
	plan := []store.Subtask{
		{ID: 1, Title: "Recon", Description: "old", SortOrder: 1, Status: StatusPlanned},
		{ID: 2, Title: "Exploit", Description: "exploit", SortOrder: 2, Status: StatusCovered},
	}
	patch := Patch{Operations: []Operation{
		{Op: OpModify, ID: ptrID(1), Description: "collect headers"},
		{Op: OpAdd, AfterID: ptrID(1), Title: "Discovery", Description: "enumerate paths"},
		{Op: OpReorder, ID: ptrID(2), AfterID: ptrID(0)},
	}}

	updated, err := ApplyPatch(plan, patch, PatchOptions{SessionID: 9, ParentTaskID: "session-9", ChallengeType: "web", Target: "http://x"})
	if err != nil {
		t.Fatalf("apply patch: %v", err)
	}
	if len(updated) != 3 {
		t.Fatalf("expected 3 items, got %d", len(updated))
	}
	if updated[0].ID != 2 || updated[1].ID != 1 || updated[2].Title != "Discovery" {
		t.Fatalf("unexpected order: %+v", updated)
	}
	if updated[1].Description != "collect headers" || updated[1].Status != StatusPlanned || updated[1].Result != "" {
		t.Fatalf("modify did not reset item: %+v", updated[1])
	}
	if updated[2].ID != 0 || updated[2].SessionID != 9 || updated[2].SortOrder != 3 {
		t.Fatalf("unexpected new item: %+v", updated[2])
	}
}

func TestApplyPatchRemove(t *testing.T) {
	plan := []store.Subtask{{ID: 1, Title: "A"}, {ID: 2, Title: "B"}}
	updated, err := ApplyPatch(plan, Patch{Operations: []Operation{{Op: OpRemove, ID: ptrID(1)}}}, PatchOptions{})
	if err != nil {
		t.Fatalf("apply patch: %v", err)
	}
	if len(updated) != 1 || updated[0].ID != 2 || updated[0].SortOrder != 1 {
		t.Fatalf("unexpected remove result: %+v", updated)
	}
}

func ptrID(v int64) *int64 { return &v }
