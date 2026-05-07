package planner

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Conly-Zy/CTF-Agent/internal/store"
)

const (
	AgentName = "Planner"
	AgentType = "planner"

	StatusPlanned     = "planned"
	StatusCovered     = "covered"
	StatusNeedsReview = "needs_review"
	StatusSkipped     = "skipped"
)

var listPrefixRe = regexp.MustCompile(`^\s*(?:[-*•]|\d+[.)])\s*`)

// Step is a template-generated plan item. It is intentionally deterministic so
// planning works in tests and offline CTF environments without another LLM call.
type Step struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Source      string `json:"source,omitempty"`
}

// GenerateFromTemplate converts a reusable flow template into ordered plan
// steps. Numbered lines and bullets become steps; "Title: detail" is split into
// title/description.
func GenerateFromTemplate(challengeType string, tpl *store.FlowTemplate, fallbackDescription string) []Step {
	var source, content string
	if tpl != nil {
		source = tpl.Title
		content = tpl.Content
	}

	steps := parseTemplateLines(source, content)
	if len(steps) == 0 && strings.TrimSpace(fallbackDescription) != "" {
		steps = []Step{{
			Title:       fmt.Sprintf("Analyze %s challenge", challengeType),
			Description: strings.TrimSpace(fallbackDescription),
			Source:      source,
		}}
	}
	if len(steps) == 0 {
		steps = []Step{{
			Title:       fmt.Sprintf("Analyze %s challenge", challengeType),
			Description: "Inspect the challenge artifacts, form hypotheses, validate them one at a time, and extract the flag.",
			Source:      source,
		}}
	}
	return steps
}

func parseTemplateLines(source, content string) []Step {
	lines := strings.Split(content, "\n")
	steps := make([]Step, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !listPrefixRe.MatchString(line) {
			continue
		}
		line = strings.TrimSpace(listPrefixRe.ReplaceAllString(line, ""))
		if line == "" {
			continue
		}
		title, desc := splitTitleDescription(line)
		steps = append(steps, Step{Title: title, Description: desc, Source: source})
		if len(steps) >= 15 {
			break
		}
	}
	return steps
}

func splitTitleDescription(line string) (string, string) {
	for _, sep := range []string{":", "：", " - ", " — "} {
		if idx := strings.Index(line, sep); idx > 0 {
			title := strings.TrimSpace(line[:idx])
			desc := strings.TrimSpace(line[idx+len(sep):])
			if title != "" && desc != "" {
				return title, desc
			}
		}
	}

	if len(line) <= 80 {
		return line, line
	}
	return strings.TrimSpace(line[:80]) + "...", line
}

// RefineStatus assigns a lightweight post-run status to a planned step.
// It does not claim exact completion; it records whether the final solve result
// and available tool evidence covered the checklist item.
func RefineStatus(session *store.Session, step store.Subtask, toolCalls []store.ToolCall) (status, result, errMsg string) {
	if session == nil {
		return StatusNeedsReview, "No session result available for plan refinement.", "missing session"
	}

	if session.Status == "success" {
		if strings.Contains(strings.ToLower(step.Title+" "+step.Description), "flag") || strings.Contains(strings.ToLower(step.Title), "extract") {
			return "success", "Flag was found; extraction step is confirmed by session result.", ""
		}
		if len(toolCalls) > 0 {
			return StatusCovered, fmt.Sprintf("Session solved with %d recorded tool call(s); checklist item considered covered by execution evidence.", len(toolCalls)), ""
		}
		return StatusCovered, "Session solved; checklist item considered covered by final result.", ""
	}

	if session.Status == "failed" {
		if session.Error != "" {
			return StatusNeedsReview, "Solve attempt failed before this planned item could be confirmed.", session.Error
		}
		return StatusNeedsReview, "Solve attempt failed before this planned item could be confirmed.", "needs review"
	}

	return StatusSkipped, fmt.Sprintf("Session ended with status %q; planned item was not refined as covered.", session.Status), ""
}
