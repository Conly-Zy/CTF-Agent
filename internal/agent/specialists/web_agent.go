package specialists

import (
	"context"
	"log/slog"
	"time"

	"github.com/Conly-Zy/CTF-Agent/internal/agent"
	"github.com/Conly-Zy/CTF-Agent/internal/llm"
	"github.com/Conly-Zy/CTF-Agent/internal/tools"
)

// WebAgent 专业 Web 安全 Agent
type WebAgent struct {
	*agent.BaseAgent
}

func NewWebAgent(
	llmClient *llm.Client,
	toolRegistry *tools.Registry,
	logger *slog.Logger,
	maxIter int,
	timeout time.Duration,
) *WebAgent {
	base := agent.NewBaseAgent(
		"WebAgent",
		agent.AgentTypeWeb,
		llmClient,
		toolRegistry,
		logger,
		maxIter,
		timeout,
	)

	return &WebAgent{
		BaseAgent: base,
	}
}

func (a *WebAgent) Run(ctx context.Context, task agent.Task) (*agent.Result, error) {
	a.Logger.Info("WebAgent starting", "task_id", task.ID)

	// 构建系统提示词
	systemPrompt, err := a.BuildSystemPrompt(task)
	if err != nil {
		return nil, err
	}

	// 执行解题循环
	return a.SolveLoop(ctx, task, systemPrompt, nil)
}
