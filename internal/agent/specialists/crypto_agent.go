package specialists

import (
	"context"
	"log/slog"
	"time"

	"github.com/Conly-Zy/CTF-Agent/internal/agent"
	"github.com/Conly-Zy/CTF-Agent/internal/llm"
	"github.com/Conly-Zy/CTF-Agent/internal/tools"
)

// CryptoAgent 专业密码学 Agent
type CryptoAgent struct {
	*agent.BaseAgent
}

func NewCryptoAgent(
	llmClient *llm.Client,
	toolRegistry *tools.Registry,
	logger *slog.Logger,
	maxIter int,
	timeout time.Duration,
) *CryptoAgent {
	base := agent.NewBaseAgent(
		"CryptoAgent",
		agent.AgentTypeCrypto,
		llmClient,
		toolRegistry,
		logger,
		maxIter,
		timeout,
	)

	return &CryptoAgent{
		BaseAgent: base,
	}
}

func (a *CryptoAgent) Run(ctx context.Context, task agent.Task) (*agent.Result, error) {
	a.Logger.Info("CryptoAgent starting", "task_id", task.ID)

	// 构建系统提示词
	systemPrompt, err := a.BuildSystemPrompt(task)
	if err != nil {
		return nil, err
	}

	// 执行解题循环
	return a.SolveLoop(ctx, task, systemPrompt, nil)
}
