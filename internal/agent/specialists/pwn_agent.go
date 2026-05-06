package specialists

import (
	"context"
	"log/slog"
	"time"

	"github.com/Conly-Zy/CTF-Agent/internal/agent"
	"github.com/Conly-Zy/CTF-Agent/internal/llm"
	"github.com/Conly-Zy/CTF-Agent/internal/tools"
)

// PwnAgent 专业二进制漏洞利用 Agent
type PwnAgent struct {
	*agent.BaseAgent
}

func NewPwnAgent(
	llmClient *llm.Client,
	toolRegistry *tools.Registry,
	logger *slog.Logger,
	maxIter int,
	timeout time.Duration,
) *PwnAgent {
	base := agent.NewBaseAgent(
		"PwnAgent",
		agent.AgentTypePwn,
		llmClient,
		toolRegistry,
		logger,
		maxIter,
		timeout,
	)

	return &PwnAgent{
		BaseAgent: base,
	}
}

func (a *PwnAgent) Run(ctx context.Context, task agent.Task) (*agent.Result, error) {
	a.Logger.Info("PwnAgent starting", "task_id", task.ID)

	// 构建系统提示词
	systemPrompt, err := a.BuildSystemPrompt(task)
	if err != nil {
		return nil, err
	}

	// 执行解题循环
	return a.SolveLoop(ctx, task, systemPrompt, nil)
}
