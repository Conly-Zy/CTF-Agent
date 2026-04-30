package agent

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Conly-Zy/CTF-Agent/internal/llm"
	"github.com/Conly-Zy/CTF-Agent/internal/tools"
)

type Logger interface {
	Log(level, message string)
	ToolStart(tool string)
	ToolResult(tool, result string)
	Thinking(content string)
	Flag(flag string)
}

type Orchestrator struct {
	llmClient    *llm.Client
	toolRegistry *tools.Registry
	logger       *slog.Logger
	store        Store
	maxIter      int
	timeout      time.Duration
}

type Store interface {
	AddConversationMessage(msg any) error
}

type SolveRequest struct {
	ChallengeType string
	Description   string
	Target        string
	Files         []string
}

type SolveResult struct {
	Success     bool
	Flag        string
	Iterations  int
	Duration    time.Duration
	CompletedAt time.Time
	Error       error
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

func (o *Orchestrator) SetStore(store Store) {
	o.store = store
}

func (o *Orchestrator) Solve(ctx context.Context, req SolveRequest) (*SolveResult, error) {
	return o.SolveWithCallback(req, nil)
}

func (o *Orchestrator) SolveWithCallback(req SolveRequest, log Logger) (*SolveResult, error) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), o.timeout)
	defer cancel()

	if log != nil {
		log.Log("info", "Building system prompt...")
	}

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

	if log != nil {
		log.Log("info", "Starting solving loop...")
	}

	for i := 0; i < o.maxIter; i++ {
		o.logger.Info("iteration", "count", i+1)

		if log != nil {
			log.Log("info", fmt.Sprintf("Iteration %d/%d", i+1, o.maxIter))
		}

		resp, err := o.llmClient.CreateMessage(ctx, llm.MessageParams{
			SystemPrompt: systemPrompt,
			Messages:     messages,
			Tools:        tools,
			MaxTokens:    4096,
		})
		if err != nil {
			return nil, fmt.Errorf("LLM call failed: %w", err)
		}

		// Log thinking/text
		text := resp.GetText()
		if text != "" && log != nil {
			log.Thinking(text)
		}

		// Check for flag
		if flag := o.extractFlag(resp); flag != "" {
			o.logger.Info("flag found", "flag", flag)
			if log != nil {
				log.Flag(flag)
			}
			return &SolveResult{
				Success:     true,
				Flag:        flag,
				Iterations:  i + 1,
				Duration:    time.Since(start),
				CompletedAt: time.Now(),
			}, nil
		}

		// Execute tools
		toolUses := resp.GetToolUse()
		if len(toolUses) == 0 {
			if resp.StopReason == "end_turn" {
				if log != nil {
					log.Log("warning", "Agent ended turn without finding flag")
				}
				continue
			}
			continue
		}

		// Add assistant message
		assistantMsg := llm.Message{Role: "assistant", Content: text}
		messages = append(messages, assistantMsg)

		// Execute tools and collect results
		for _, tu := range toolUses {
			if log != nil {
				log.ToolStart(tu.Name)
			}

			tool, ok := o.toolRegistry.Get(tu.Name)
			if !ok {
				errMsg := fmt.Sprintf("Tool not found: %s", tu.Name)
				if log != nil {
					log.Log("error", errMsg)
				}
				messages = append(messages, llm.Message{
					Role:    "user",
					Content: errMsg,
				})
				continue
			}

			output, err := tool.Execute(ctx, tu.Input)
			if err != nil {
				errMsg := fmt.Sprintf("Tool error: %v", err)
				if log != nil {
					log.Log("error", errMsg)
				}
				output = errMsg
			}

			if log != nil {
				log.ToolResult(tu.Name, output)
			}

			// Add tool result as user message
			messages = append(messages, llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("Tool %s result:\n%s", tu.Name, output),
			})
		}
	}

	return &SolveResult{
		Success:     false,
		Iterations:  o.maxIter,
		Duration:    time.Since(start),
		CompletedAt: time.Now(),
		Error:       fmt.Errorf("max iterations reached without finding flag"),
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

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
