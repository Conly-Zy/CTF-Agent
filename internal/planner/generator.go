package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Conly-Zy/CTF-Agent/internal/llm"
	"github.com/Conly-Zy/CTF-Agent/internal/store"
)

type Generator struct {
	client ChatClient
}

type GenerateRequest struct {
	ChallengeType string              `json:"challenge_type"`
	Description   string              `json:"description"`
	Target        string              `json:"target,omitempty"`
	Files         []string            `json:"files,omitempty"`
	Template      *store.FlowTemplate `json:"template,omitempty"`
}

type GenerateOptions struct {
	UseLLM bool `json:"use_llm"`
}

type GenerateResult struct {
	Steps  []Step `json:"steps"`
	Source string `json:"source"`
	Error  string `json:"error,omitempty"`
}

func NewGenerator(client ChatClient) *Generator {
	return &Generator{client: client}
}

func (g *Generator) SetClient(client ChatClient) {
	g.client = client
}

// Generate creates an initial solve plan. LLM mode is optional and always falls
// back to the deterministic template parser to keep CTF-Agent usable offline.
func (g *Generator) Generate(ctx context.Context, req GenerateRequest, opts GenerateOptions) GenerateResult {
	if opts.UseLLM && g != nil && g.client != nil {
		steps, err := g.generateWithLLM(ctx, req)
		if err == nil && len(steps) > 0 {
			return GenerateResult{Steps: steps, Source: "llm"}
		}
		fallback := GenerateFromTemplate(req.ChallengeType, req.Template, req.Description)
		errMsg := "empty LLM generator response"
		if err != nil {
			errMsg = err.Error()
		}
		for i := range fallback {
			fallback[i].Source = sourceWithFallback(fallback[i].Source, errMsg)
		}
		return GenerateResult{Steps: fallback, Source: "deterministic_fallback", Error: errMsg}
	}
	return GenerateResult{Steps: GenerateFromTemplate(req.ChallengeType, req.Template, req.Description), Source: "deterministic"}
}

func (g *Generator) generateWithLLM(ctx context.Context, req GenerateRequest) ([]Step, error) {
	payload, err := json.MarshalIndent(compactGenerateRequest(req), "", "  ")
	if err != nil {
		return nil, err
	}
	resp, err := g.client.CreateMessage(ctx, llm.MessageParams{
		SystemPrompt: `You are a CTF solve-plan generator. Return only JSON with schema {"steps":[{"title":"short action","description":"concrete verification step"}]}. Produce 3 to 8 ordered steps. Keep steps practical, evidence-driven, and CTF-scoped. Do not invent flags or target facts.`,
		Messages:     []llm.Message{llm.NewTextMessage("user", fmt.Sprintf("Create an initial solve plan for this CTF challenge.\n%s", string(payload)))},
		MaxTokens:    1024,
	})
	if err != nil {
		return nil, err
	}
	return ParseStepsJSON(resp.GetText())
}

func ParseStepsJSON(text string) ([]Step, error) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	}

	startObj := strings.Index(text, "{")
	endObj := strings.LastIndex(text, "}")
	if startObj >= 0 && endObj > startObj {
		var obj struct {
			Steps []Step `json:"steps"`
		}
		if err := json.Unmarshal([]byte(text[startObj:endObj+1]), &obj); err != nil {
			return nil, fmt.Errorf("parse generator steps object: %w", err)
		}
		return normalizeSteps(obj.Steps)
	}

	startArr := strings.Index(text, "[")
	endArr := strings.LastIndex(text, "]")
	if startArr >= 0 && endArr > startArr {
		var steps []Step
		if err := json.Unmarshal([]byte(text[startArr:endArr+1]), &steps); err != nil {
			return nil, fmt.Errorf("parse generator steps array: %w", err)
		}
		return normalizeSteps(steps)
	}

	return nil, fmt.Errorf("no JSON steps found in generator response")
}

func normalizeSteps(steps []Step) ([]Step, error) {
	out := make([]Step, 0, len(steps))
	for _, step := range steps {
		step.Title = strings.TrimSpace(step.Title)
		step.Description = strings.TrimSpace(step.Description)
		if step.Title == "" && step.Description == "" {
			continue
		}
		if step.Title == "" {
			step.Title = splitLongTitle(step.Description)
		}
		if step.Description == "" {
			step.Description = step.Title
		}
		out = append(out, step)
		if len(out) >= 15 {
			break
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("generator returned no usable steps")
	}
	return out, nil
}

func splitLongTitle(text string) string {
	text = strings.TrimSpace(text)
	if len(text) <= 80 {
		return text
	}
	return strings.TrimSpace(text[:80]) + "..."
}

type compactGenerateRequestPayload struct {
	ChallengeType string   `json:"challenge_type"`
	Description   string   `json:"description"`
	Target        string   `json:"target,omitempty"`
	Files         []string `json:"files,omitempty"`
	TemplateTitle string   `json:"template_title,omitempty"`
	TemplateText  string   `json:"template_text,omitempty"`
}

func compactGenerateRequest(req GenerateRequest) compactGenerateRequestPayload {
	payload := compactGenerateRequestPayload{
		ChallengeType: req.ChallengeType,
		Description:   req.Description,
		Target:        req.Target,
		Files:         req.Files,
	}
	if req.Template != nil {
		payload.TemplateTitle = req.Template.Title
		payload.TemplateText = req.Template.Content
	}
	return payload
}

func sourceWithFallback(source, errMsg string) string {
	if source == "" {
		return "LLM generator fallback: " + errMsg
	}
	return source + " (LLM fallback: " + errMsg + ")"
}
