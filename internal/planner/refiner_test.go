package planner

import (
	"context"
	"testing"

	"github.com/Conly-Zy/CTF-Agent/internal/llm"
	"github.com/Conly-Zy/CTF-Agent/internal/store"
)

type fakeChatClient struct {
	text string
	err  error
}

func (f fakeChatClient) CreateMessage(ctx context.Context, params llm.MessageParams) (*llm.MessageResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &llm.MessageResponse{Content: []llm.ContentBlock{{Type: "text", Text: f.text}}}, nil
}

func TestSuggestDeterministicPatchFailed(t *testing.T) {
	ev := Evidence{
		Session: &store.Session{Status: "failed", Error: "max iterations"},
		Plan:    []store.Subtask{{ID: 1, Title: "Recon"}},
		ToolCalls: []store.ToolCall{{
			AgentName: "WebAgent",
			ToolName:  "http_request",
			Status:    "failed",
			Error:     "timeout",
		}},
	}
	patch := SuggestDeterministicPatch(ev)
	if len(patch.Operations) != 2 {
		t.Fatalf("expected 2 recovery ops, got %+v", patch)
	}
	if patch.Operations[0].Op != OpAdd || patch.Operations[0].Title != "Review failure" {
		t.Fatalf("unexpected first op: %+v", patch.Operations[0])
	}
}

func TestSuggestPatchWithLLM(t *testing.T) {
	client := fakeChatClient{text: `{"message":"ok","operations":[{"op":"add","title":"New","description":"Do new thing"}]}`}
	refiner := NewRefiner(client)
	res := refiner.SuggestPatch(context.Background(), Evidence{Session: &store.Session{Status: "failed"}}, SuggestOptions{UseLLM: true})
	if res.Source != "llm" || len(res.Patch.Operations) != 1 || res.Patch.Operations[0].Title != "New" {
		t.Fatalf("unexpected llm result: %+v", res)
	}
}

func TestParsePatchJSONFenced(t *testing.T) {
	patch, err := ParsePatchJSON("```json\n{\"operations\":[{\"op\":\"remove\",\"id\":1}]}\n```")
	if err != nil {
		t.Fatalf("parse patch: %v", err)
	}
	if len(patch.Operations) != 1 || patch.Operations[0].Op != OpRemove {
		t.Fatalf("unexpected patch: %+v", patch)
	}
}
