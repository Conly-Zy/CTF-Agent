package planner

import (
	"context"
	"testing"

	"github.com/Conly-Zy/CTF-Agent/internal/store"
)

func TestParseStepsJSON(t *testing.T) {
	steps, err := ParseStepsJSON("```json\n{\"steps\":[{\"title\":\"Recon\",\"description\":\"Collect headers\"}]}\n```")
	if err != nil {
		t.Fatalf("parse steps: %v", err)
	}
	if len(steps) != 1 || steps[0].Title != "Recon" || steps[0].Description != "Collect headers" {
		t.Fatalf("unexpected steps: %+v", steps)
	}
}

func TestGeneratorWithLLM(t *testing.T) {
	client := fakeChatClient{text: `{"steps":[{"title":"Recon","description":"Collect headers"},{"title":"Exploit","description":"Validate bug"}]}`}
	gen := NewGenerator(client)
	res := gen.Generate(context.Background(), GenerateRequest{ChallengeType: "web", Description: "test"}, GenerateOptions{UseLLM: true})
	if res.Source != "llm" || len(res.Steps) != 2 || res.Steps[1].Title != "Exploit" {
		t.Fatalf("unexpected generator result: %+v", res)
	}
}

func TestGeneratorFallback(t *testing.T) {
	gen := NewGenerator(fakeChatClient{text: `not json`})
	res := gen.Generate(context.Background(), GenerateRequest{
		ChallengeType: "web",
		Description:   "fallback description",
		Template: &store.FlowTemplate{
			Title:   "Template",
			Content: "1. Recon: collect source",
		},
	}, GenerateOptions{UseLLM: true})
	if res.Source != "deterministic_fallback" || len(res.Steps) != 1 || res.Error == "" {
		t.Fatalf("expected fallback, got %+v", res)
	}
	if res.Steps[0].Title != "Recon" {
		t.Fatalf("unexpected fallback steps: %+v", res.Steps)
	}
}
