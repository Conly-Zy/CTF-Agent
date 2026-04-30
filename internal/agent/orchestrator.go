package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Conly-Zy/CTF-Agent/internal/llm"
	"github.com/Conly-Zy/CTF-Agent/internal/tools"
)

type Orchestrator struct {
	llmClient    *llm.Client
	toolRegistry *tools.Registry
	logger       *slog.Logger
	maxIter      int
	timeout      time.Duration
}

type SolveRequest struct {
	ChallengeType string
	Description   string
	Target        string
	Files         []string
}

type SolveResult struct {
	Success   bool
	Flag      string
	Iterations int
	Duration  time.Duration
	Error     error
}

func NewOrchestrator(llmClient *llm.Client, toolRegistry *tools.Registry, logger *slog.Logger, maxIter int, timeout time.Duration) *Orchestrator {
	return &Orchestrator{
		llmClient:    llmClient,
		toolRegistry: toolRegistry,
		logger:       logger,
		maxIter:      maxIter,
		timeout:      timeout,
	}
}

func (o *Orchestrator) Solve(ctx context.Context, req SolveRequest) (*SolveResult, error) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, o.timeout)
	defer cancel()

	systemPrompt := o.buildSystemPrompt(req)
	messages := []llm.Message{
		{Role: "user", Content: o.buildUserMessage(req)},
	}

	claudeTools := o.toolRegistry.ToClaudeTools()
	tools := make([]llm.ToolDefinition, len(claudeTools))
	for i, t := range claudeTools {
		tools[i] = llm.ToolDefinition{
			Name:        t["name"].(string),
			Description: t["description"].(string),
			InputSchema: t["input_schema"].(map[string]any),
		}
	}

	for i := 0; i < o.maxIter; i++ {
		o.logger.Info("iteration", "count", i+1)

		resp, err := o.llmClient.CreateMessage(ctx, llm.MessageParams{
			SystemPrompt: systemPrompt,
			Messages:     messages,
			Tools:        tools,
			MaxTokens:    4096,
		})
		if err != nil {
			return nil, fmt.Errorf("LLM call failed: %w", err)
		}

		o.logResponse(resp)

		if flag := o.extractFlag(resp); flag != "" {
			o.logger.Info("flag found", "flag", flag)
			return &SolveResult{
				Success:    true,
				Flag:       flag,
				Iterations: i + 1,
				Duration:   time.Since(start),
			}, nil
		}

		if resp.StopReason == "end_turn" {
			o.logger.Info("LLM ended turn without finding flag")
			continue
		}

		toolUses := resp.GetToolUse()
		if len(toolUses) == 0 {
			continue
		}

		assistantMsg := llm.Message{Role: "assistant", Content: resp.GetText()}
		messages = append(messages, assistantMsg)

		toolResults := o.executeTools(ctx, toolUses)
		messages = append(messages, llm.Message{Role: "user", Content: toolResults})
	}

	return &SolveResult{
		Success:    false,
		Iterations: o.maxIter,
		Duration:   time.Since(start),
		Error:      fmt.Errorf("max iterations reached without finding flag"),
	}, nil
}

func (o *Orchestrator) buildSystemPrompt(req SolveRequest) string {
	return fmt.Sprintf(`You are an expert CTF player specializing in solving %s challenges.

Your goal is to analyze the challenge, identify vulnerabilities, and extract the flag.

Available tools:
- file_read: Read file contents
- file_write: Write content to files
- shell_exec: Execute shell commands

When you find the flag, output it in the format: flag{...} or CTF{...}

Be systematic in your approach:
1. First analyze the challenge
2. Identify potential vulnerabilities
3. Develop an exploit strategy
4. Execute your exploit
5. Extract the flag`, req.ChallengeType)
}

func (o *Orchestrator) buildUserMessage(req SolveRequest) string {
	msg := fmt.Sprintf("Challenge Type: %s\n", req.ChallengeType)
	msg += fmt.Sprintf("Description: %s\n", req.Description)
	if req.Target != "" {
		msg += fmt.Sprintf("Target: %s\n", req.Target)
	}
	if len(req.Files) > 0 {
		msg += "Files:\n"
		for _, f := range req.Files {
			msg += fmt.Sprintf("- %s\n", f)
		}
	}
	msg += "\nPlease analyze this challenge and find the flag."
	return msg
}

func (o *Orchestrator) executeTools(ctx context.Context, toolUses []llm.ContentBlock) string {
	var results []map[string]any

	for _, tu := range toolUses {
		o.logger.Info("executing tool", "tool", tu.Name)

		tool, ok := o.toolRegistry.Get(tu.Name)
		if !ok {
			results = append(results, map[string]any{
				"tool_use_id": tu.ID,
				"content":     fmt.Sprintf("Tool not found: %s", tu.Name),
			})
			continue
		}

		output, err := tool.Execute(ctx, tu.Input)
		if err != nil {
			o.logger.Error("tool execution failed", "tool", tu.Name, "error", err)
			results = append(results, map[string]any{
				"tool_use_id": tu.ID,
				"content":     fmt.Sprintf("Error: %v", err),
			})
			continue
		}

		o.logger.Info("tool execution completed", "tool", tu.Name, "output_len", len(output))
		results = append(results, map[string]any{
			"tool_use_id": tu.ID,
			"content":     output,
		})
	}

	data, _ := json.Marshal(results)
	return string(data)
}

func (o *Orchestrator) extractFlag(resp *llm.MessageResponse) string {
	text := resp.GetText()
	flagPatterns := []string{"flag{", "CTF{", "picoCTF{", "FLAG{"}

	for _, pattern := range flagPatterns {
		start := indexOf(text, pattern)
		if start == -1 {
			continue
		}
		end := indexOf(text[start:], "}")
		if end == -1 {
			continue
		}
		return text[start : start+end+1]
	}

	return ""
}

func (o *Orchestrator) logResponse(resp *llm.MessageResponse) {
	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			o.logger.Debug("LLM text", "content", block.Text)
		case "tool_use":
			o.logger.Debug("LLM tool use", "tool", block.Name, "input", string(block.Input))
		}
	}
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
