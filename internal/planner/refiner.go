package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Conly-Zy/CTF-Agent/internal/llm"
	"github.com/Conly-Zy/CTF-Agent/internal/store"
)

// ChatClient is the minimal LLM surface needed by the planner. *llm.Client
// satisfies it, while tests can provide a fake offline implementation.
type ChatClient interface {
	CreateMessage(ctx context.Context, params llm.MessageParams) (*llm.MessageResponse, error)
}

type Refiner struct {
	client ChatClient
}

type Evidence struct {
	Session   *store.Session   `json:"session"`
	Plan      []store.Subtask  `json:"plan"`
	ToolCalls []store.ToolCall `json:"tool_calls"`
}

type SuggestOptions struct {
	UseLLM bool `json:"use_llm"`
}

type SuggestResult struct {
	Patch  Patch  `json:"patch"`
	Source string `json:"source"`
	Error  string `json:"error,omitempty"`
}

func NewRefiner(client ChatClient) *Refiner {
	return &Refiner{client: client}
}

func (r *Refiner) SetClient(client ChatClient) {
	r.client = client
}

// SuggestPatch returns a PentAGI-style patch for the current plan. If UseLLM is
// true and an LLM client is configured, the LLM gets the first chance to produce
// a patch. Any LLM error falls back to the deterministic refiner.
func (r *Refiner) SuggestPatch(ctx context.Context, ev Evidence, opts SuggestOptions) SuggestResult {
	if opts.UseLLM && r != nil && r.client != nil {
		patch, err := r.suggestWithLLM(ctx, ev)
		if err == nil {
			return SuggestResult{Patch: patch, Source: "llm"}
		}
		fallback := SuggestDeterministicPatch(ev)
		fallback.Message = strings.TrimSpace(fallback.Message + "\nLLM refiner fallback: " + err.Error())
		return SuggestResult{Patch: fallback, Source: "deterministic_fallback", Error: err.Error()}
	}
	return SuggestResult{Patch: SuggestDeterministicPatch(ev), Source: "deterministic"}
}

// SuggestDeterministicPatch is intentionally conservative. It never deletes or
// reorders user-visible plan items; it only adds missing recovery/reporting
// steps for failed solves and tightens flag extraction wording for successful
// solves.
func SuggestDeterministicPatch(ev Evidence) Patch {
	patch := Patch{Message: "Deterministic refiner suggestion"}
	if ev.Session == nil {
		patch.Message = "No session available; no patch suggested"
		return patch
	}

	if ev.Session.Status == "failed" {
		if !planContains(ev.Plan, "review failure") {
			patch.Operations = append(patch.Operations, Operation{
				Op:          OpAdd,
				AfterID:     lastPlanID(ev.Plan),
				Title:       "Review failure",
				Description: failureDescription(ev.Session, ev.ToolCalls),
			})
		}
		if !planContains(ev.Plan, "alternate strategy") {
			patch.Operations = append(patch.Operations, Operation{
				Op:          OpAdd,
				AfterID:     lastPlanID(ev.Plan),
				Title:       "Alternate strategy",
				Description: "Use the failure evidence to choose a different primitive, tool, or challenge hypothesis before retrying.",
			})
		}
		return patch
	}

	if ev.Session.Status == "success" {
		flagIdx := findFlagPlanIndex(ev.Plan)
		if flagIdx >= 0 && !strings.Contains(strings.ToLower(ev.Plan[flagIdx].Description), "replay") {
			id := ev.Plan[flagIdx].ID
			patch.Operations = append(patch.Operations, Operation{
				Op:          OpModify,
				ID:          &id,
				Description: strings.TrimSpace(ev.Plan[flagIdx].Description + " Record exact replay steps and evidence for the final flag."),
			})
		} else if flagIdx < 0 && !planContains(ev.Plan, "document flag") {
			patch.Operations = append(patch.Operations, Operation{
				Op:          OpAdd,
				AfterID:     lastPlanID(ev.Plan),
				Title:       "Document flag path",
				Description: "Record the exact commands, requests, offsets, or scripts that reproduce the recovered flag.",
			})
		}
	}

	return patch
}

func (r *Refiner) suggestWithLLM(ctx context.Context, ev Evidence) (Patch, error) {
	payload, err := json.MarshalIndent(compactEvidence(ev), "", "  ")
	if err != nil {
		return Patch{}, err
	}

	resp, err := r.client.CreateMessage(ctx, llm.MessageParams{
		SystemPrompt: `You are a CTF planning refiner. Return only a JSON object with this schema: {"message":"...","operations":[{"op":"add|remove|modify|reorder","id":12,"after_id":10,"title":"...","description":"..."}]}. Use only conservative, useful plan changes. Do not invent flags.`,
		Messages:     []llm.Message{llm.NewTextMessage("user", fmt.Sprintf("Refine this CTF solve plan from the evidence.\n%s", string(payload)))},
		MaxTokens:    1024,
	})
	if err != nil {
		return Patch{}, err
	}
	text := strings.TrimSpace(resp.GetText())
	if text == "" {
		return Patch{}, fmt.Errorf("empty LLM refiner response")
	}
	patch, err := ParsePatchJSON(text)
	if err != nil {
		return Patch{}, err
	}
	return patch, nil
}

func ParsePatchJSON(text string) (Patch, error) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return Patch{}, fmt.Errorf("no JSON object found in refiner response")
	}
	var patch Patch
	if err := json.Unmarshal([]byte(text[start:end+1]), &patch); err != nil {
		return Patch{}, fmt.Errorf("parse refiner patch: %w", err)
	}
	return patch, nil
}

type compactEvidencePayload struct {
	Session   compactSession    `json:"session"`
	Plan      []compactPlanItem `json:"plan"`
	ToolCalls []compactToolCall `json:"tool_calls"`
}

type compactSession struct {
	ID            int64  `json:"id"`
	ChallengeType string `json:"challenge_type"`
	Status        string `json:"status"`
	Flag          string `json:"flag,omitempty"`
	Error         string `json:"error,omitempty"`
	Iterations    int    `json:"iterations"`
}

type compactPlanItem struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	SortOrder   int    `json:"sort_order"`
	Error       string `json:"error,omitempty"`
}

type compactToolCall struct {
	ToolName  string `json:"tool_name"`
	AgentName string `json:"agent_name"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

func compactEvidence(ev Evidence) compactEvidencePayload {
	payload := compactEvidencePayload{}
	if ev.Session != nil {
		payload.Session = compactSession{
			ID:            ev.Session.ID,
			ChallengeType: ev.Session.ChallengeType,
			Status:        ev.Session.Status,
			Flag:          ev.Session.Flag,
			Error:         ev.Session.Error,
			Iterations:    ev.Session.Iterations,
		}
	}
	for _, st := range ev.Plan {
		payload.Plan = append(payload.Plan, compactPlanItem{
			ID:          st.ID,
			Title:       st.Title,
			Description: st.Description,
			Status:      st.Status,
			SortOrder:   st.SortOrder,
			Error:       st.Error,
		})
	}
	limit := len(ev.ToolCalls)
	if limit > 20 {
		limit = 20
	}
	for _, call := range ev.ToolCalls[:limit] {
		payload.ToolCalls = append(payload.ToolCalls, compactToolCall{
			ToolName:  call.ToolName,
			AgentName: call.AgentName,
			Status:    call.Status,
			Error:     call.Error,
		})
	}
	return payload
}

func planContains(plan []store.Subtask, needle string) bool {
	needle = strings.ToLower(needle)
	for _, st := range plan {
		if strings.Contains(strings.ToLower(st.Title+" "+st.Description), needle) {
			return true
		}
	}
	return false
}

func lastPlanID(plan []store.Subtask) *int64 {
	if len(plan) == 0 {
		return nil
	}
	id := plan[len(plan)-1].ID
	return &id
}

func findFlagPlanIndex(plan []store.Subtask) int {
	for i, st := range plan {
		text := strings.ToLower(st.Title + " " + st.Description)
		if strings.Contains(text, "flag") || strings.Contains(text, "extract") {
			return i
		}
	}
	return -1
}

func failureDescription(session *store.Session, calls []store.ToolCall) string {
	parts := []string{"Summarize why the previous solve attempt failed and identify the earliest uncertain step."}
	if session != nil && session.Error != "" {
		parts = append(parts, "Session error: "+session.Error)
	}
	if len(calls) > 0 {
		last := calls[len(calls)-1]
		parts = append(parts, fmt.Sprintf("Last tool evidence: %s/%s", last.AgentName, last.ToolName))
		if last.Error != "" {
			parts = append(parts, "Tool error: "+last.Error)
		}
	}
	return strings.Join(parts, " ")
}
