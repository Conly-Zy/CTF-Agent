package specialists

import (
	"context"
	"log/slog"
	"time"

	"github.com/Conly-Zy/CTF-Agent/internal/agent"
	"github.com/Conly-Zy/CTF-Agent/internal/llm"
	"github.com/Conly-Zy/CTF-Agent/internal/tools"
)

// ReverseAgent 专业逆向工程 Agent
type ReverseAgent struct {
	*agent.BaseAgent
}

func NewReverseAgent(
	llmClient *llm.Client,
	toolRegistry *tools.Registry,
	logger *slog.Logger,
	maxIter int,
	timeout time.Duration,
) *ReverseAgent {
	base := agent.NewBaseAgent(
		"ReverseAgent",
		agent.AgentTypeReverse,
		llmClient,
		toolRegistry,
		logger,
		maxIter,
		timeout,
	)

	return &ReverseAgent{
		BaseAgent: base,
	}
}

func (a *ReverseAgent) Run(ctx context.Context, task agent.Task) (*agent.Result, error) {
	a.Logger.Info("ReverseAgent starting", "task_id", task.ID)

	// 构建系统提示词
	systemPrompt, err := a.BuildSystemPrompt(task)
	if err != nil {
		return nil, err
	}

	// 执行解题循环
	return a.SolveLoop(ctx, task, systemPrompt, nil)
}
