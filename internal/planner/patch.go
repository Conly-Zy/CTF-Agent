package planner

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Conly-Zy/CTF-Agent/internal/store"
)

const (
	OpAdd     = "add"
	OpRemove  = "remove"
	OpModify  = "modify"
	OpReorder = "reorder"
)

type Patch struct {
	Message    string      `json:"message,omitempty"`
	Operations []Operation `json:"operations"`
}

type Operation struct {
	Op          string `json:"op"`
	ID          *int64 `json:"id,omitempty"`
	AfterID     *int64 `json:"after_id,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

type PatchOptions struct {
	SessionID     int64
	ParentTaskID  string
	ChallengeType string
	Target        string
}

// ApplyPatch applies PentAGI-style delta operations to a plan in memory and
// returns the resulting ordered plan. New subtasks have ID=0 and should be
// inserted by the caller.
func ApplyPatch(plan []store.Subtask, patch Patch, opts PatchOptions) ([]store.Subtask, error) {
	result := append([]store.Subtask(nil), plan...)
	idToIdx := buildIDIndex(result)
	removed := map[int64]bool{}

	for i, op := range patch.Operations {
		switch op.Op {
		case OpRemove:
			if op.ID == nil {
				return nil, fmt.Errorf("operation %d: remove requires id", i)
			}
			if _, ok := idToIdx[*op.ID]; !ok {
				return nil, fmt.Errorf("operation %d: subtask id %d not found", i, *op.ID)
			}
			removed[*op.ID] = true
		case OpModify:
			if op.ID == nil {
				return nil, fmt.Errorf("operation %d: modify requires id", i)
			}
			idx, ok := idToIdx[*op.ID]
			if !ok {
				return nil, fmt.Errorf("operation %d: subtask id %d not found", i, *op.ID)
			}
			if strings.TrimSpace(op.Title) == "" && strings.TrimSpace(op.Description) == "" {
				return nil, fmt.Errorf("operation %d: modify requires title or description", i)
			}
			if strings.TrimSpace(op.Title) != "" {
				result[idx].Title = strings.TrimSpace(op.Title)
			}
			if strings.TrimSpace(op.Description) != "" {
				result[idx].Description = strings.TrimSpace(op.Description)
			}
			result[idx].Status = StatusPlanned
			result[idx].Result = ""
			result[idx].Error = ""
			result[idx].CompletedAt = nil
		case OpAdd, OpReorder:
			// handled in the second pass after removals/modifications
		default:
			return nil, fmt.Errorf("operation %d: unsupported op %q", i, op.Op)
		}
	}

	if len(removed) > 0 {
		filtered := make([]store.Subtask, 0, len(result)-len(removed))
		for _, st := range result {
			if st.ID == 0 || !removed[st.ID] {
				filtered = append(filtered, st)
			}
		}
		result = filtered
	}
	idToIdx = buildIDIndex(result)

	for i, op := range patch.Operations {
		switch op.Op {
		case OpAdd:
			if strings.TrimSpace(op.Title) == "" {
				return nil, fmt.Errorf("operation %d: add requires title", i)
			}
			if strings.TrimSpace(op.Description) == "" {
				return nil, fmt.Errorf("operation %d: add requires description", i)
			}
			st := store.Subtask{
				SessionID:     opts.SessionID,
				ParentID:      opts.ParentTaskID,
				AgentName:     AgentName,
				AgentType:     AgentType,
				ChallengeType: opts.ChallengeType,
				Title:         strings.TrimSpace(op.Title),
				Description:   strings.TrimSpace(op.Description),
				Target:        opts.Target,
				Status:        StatusPlanned,
			}
			idx := insertIndex(op.AfterID, idToIdx, len(result))
			result = slices.Insert(result, idx, st)
			idToIdx = buildIDIndex(result)
		case OpReorder:
			if op.ID == nil {
				return nil, fmt.Errorf("operation %d: reorder requires id", i)
			}
			currentIdx, ok := idToIdx[*op.ID]
			if !ok {
				return nil, fmt.Errorf("operation %d: subtask id %d not found", i, *op.ID)
			}
			moving := result[currentIdx]
			result = slices.Delete(result, currentIdx, currentIdx+1)
			idToIdx = buildIDIndex(result)
			idx := insertIndex(op.AfterID, idToIdx, len(result))
			result = slices.Insert(result, idx, moving)
			idToIdx = buildIDIndex(result)
		}
	}

	for i := range result {
		result[i].SortOrder = i + 1
		if result[i].TaskID == "" {
			result[i].TaskID = fmt.Sprintf("%s-plan-patch-%02d", opts.ParentTaskID, i+1)
		}
		if result[i].SessionID == 0 {
			result[i].SessionID = opts.SessionID
		}
		if result[i].ParentID == "" {
			result[i].ParentID = opts.ParentTaskID
		}
		if result[i].AgentName == "" {
			result[i].AgentName = AgentName
		}
		if result[i].AgentType == "" {
			result[i].AgentType = AgentType
		}
		if result[i].ChallengeType == "" {
			result[i].ChallengeType = opts.ChallengeType
		}
		if result[i].Target == "" {
			result[i].Target = opts.Target
		}
	}

	return result, nil
}

func buildIDIndex(plan []store.Subtask) map[int64]int {
	m := make(map[int64]int, len(plan))
	for i, st := range plan {
		if st.ID != 0 {
			m[st.ID] = i
		}
	}
	return m
}

func insertIndex(afterID *int64, idToIdx map[int64]int, length int) int {
	if afterID == nil || *afterID == 0 {
		return 0
	}
	if idx, ok := idToIdx[*afterID]; ok {
		return idx + 1
	}
	return length
}
