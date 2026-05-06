package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Conly-Zy/CTF-Agent/internal/llm"
	"github.com/Conly-Zy/CTF-Agent/internal/tools"
)

// PrimaryAgent 主控 Agent，负责任务分析和调度
type PrimaryAgent struct {
	llmClient    *llm.Client
	toolRegistry *tools.Registry
	logger       *slog.Logger
	agents       map[AgentType]Agent // 专业 Agent 注册表
	maxIter      int
	timeout      time.Duration
	messageCh    chan<- Message
}

func NewPrimaryAgent(
	llmClient *llm.Client,
	toolRegistry *tools.Registry,
	logger *slog.Logger,
	maxIter int,
	timeout time.Duration,
) *PrimaryAgent {
	return &PrimaryAgent{
		llmClient:    llmClient,
		toolRegistry: toolRegistry,
		logger:       logger,
		agents:       make(map[AgentType]Agent),
		maxIter:      maxIter,
		timeout:      timeout,
	}
}

func (a *PrimaryAgent) Name() string {
	return "PrimaryAgent"
}

func (a *PrimaryAgent) Type() AgentType {
	return AgentTypePrimary
}

func (a *PrimaryAgent) SetMessageChannel(ch chan<- Message) {
	a.messageCh = ch
}

// RegisterAgent 注册专业 Agent
func (a *PrimaryAgent) RegisterAgent(agent Agent) {
	a.agents[agent.Type()] = agent
	agent.SetMessageChannel(a.messageCh)
}

// Run 实现 Agent 接口
func (a *PrimaryAgent) Run(ctx context.Context, task Task) (*Result, error) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	a.logger.Info("PrimaryAgent starting", "task_id", task.ID, "type", task.Type)

	// 1. 分析任务类型
	agentType := a.analyzeTaskType(task)

	// 2. 如果是明确的专业任务，委托给专业 Agent
	if agentType != AgentTypePrimary {
		return a.delegateToSpecialist(ctx, agentType, task)
	}

	// 3. 复杂任务：分解并协调多个 Agent
	return a.coordinateMultipleAgents(ctx, task, start)
}

// analyzeTaskType 分析任务应该由哪个 Agent 处理
func (a *PrimaryAgent) analyzeTaskType(task Task) AgentType {
	// 首先检查明确的类型
	switch task.Type {
	case "web":
		return AgentTypeWeb
	case "pwn":
		return AgentTypePwn
	case "crypto":
		return AgentTypeCrypto
	case "reverse":
		return AgentTypeReverse
	}

	// 使用 LLM 进行智能分析
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	analysisPrompt := fmt.Sprintf(`分析以下 CTF 题目，判断最适合的专业 Agent 类型。

题目描述: %s
目标: %s
文件: %v

可选的 Agent 类型:
- web: Web 安全（SQL注入、XSS、SSRF、文件上传等）
- pwn: 二进制漏洞利用（栈溢出、堆利用、ROP等）
- crypto: 密码学（RSA、AES、哈希、编码等）
- reverse: 逆向工程（反汇编、动态分析等）

请只返回一个单词：web, pwn, crypto, 或 reverse`, task.Description, task.Target, task.Files)

	resp, err := a.llmClient.CreateMessage(ctx, llm.MessageParams{
		SystemPrompt: "你是一个 CTF 题目分类专家。根据题目描述判断最合适的解题方向。",
		Messages:     []llm.Message{llm.NewTextMessage("user", analysisPrompt)},
		MaxTokens:    10,
	})
	if err != nil {
		a.logger.Warn("failed to analyze task type", "error", err)
		return AgentTypePrimary
	}

	analysis := strings.TrimSpace(strings.ToLower(resp.GetText()))
	switch analysis {
	case "web":
		return AgentTypeWeb
	case "pwn":
		return AgentTypePwn
	case "crypto":
		return AgentTypeCrypto
	case "reverse":
		return AgentTypeReverse
	default:
		return AgentTypePrimary
	}
}

// delegateToSpecialist 委托任务给专业 Agent
func (a *PrimaryAgent) delegateToSpecialist(ctx context.Context, agentType AgentType, task Task) (*Result, error) {
	agent, ok := a.agents[agentType]
	if !ok {
		return nil, fmt.Errorf("agent type %s not registered", agentType)
	}

	a.logger.Info("delegating task",
		"task_id", task.ID,
		"agent", agent.Name(),
		"type", agentType)

	return agent.Run(ctx, task)
}

// coordinateMultipleAgents 协调多个 Agent 处理复杂任务
func (a *PrimaryAgent) coordinateMultipleAgents(ctx context.Context, task Task, start time.Time) (*Result, error) {
	// 1. 使用 LLM 分解任务
	subTasks, err := a.decomposeTask(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("decompose task failed: %w", err)
	}

	if len(subTasks) == 0 {
		// 没有子任务，尝试自己处理
		return a.handleTaskDirectly(ctx, task, start)
	}

	// 2. 并行分发子任务
	resultCh := make(chan *Result, len(subTasks))
	for _, subTask := range subTasks {
		go func(st Task) {
			agentType := a.analyzeTaskType(st)
			result, err := a.delegateToSpecialist(ctx, agentType, st)
			if err != nil {
				result = &Result{
					TaskID: st.ID,
					Success: false,
					Error:  err.Error(),
				}
			}
			resultCh <- result
		}(subTask)
	}

	// 3. 收集结果
	var results []*Result
	for i := 0; i < len(subTasks); i++ {
		select {
		case result := <-resultCh:
			results = append(results, result)
			a.logger.Info("subtask completed",
				"task_id", result.TaskID,
				"success", result.Success,
				"flag", result.Flag)
			// 检查是否找到 flag
			if result.Flag != "" {
				return result, nil
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// 4. 汇总结果
	return a.aggregateResults(ctx, task, results, start)
}

// decomposeTask 使用 LLM 分解任务
func (a *PrimaryAgent) decomposeTask(ctx context.Context, task Task) ([]Task, error) {
	prompt := fmt.Sprintf(`分析以下 CTF 题目，将其分解为可独立执行的子任务。

题目类型: %s
题目描述: %s
目标: %s

请返回一个 JSON 数组，每个元素包含：
- type: 子任务类型 (web/pwn/crypto/reverse)
- description: 子任务描述
- target: 子任务目标（可选）

示例：
[
  {"type": "web", "description": "扫描目标网站目录", "target": "http://example.com"},
  {"type": "reverse", "description": "分析下载的二进制文件", "target": "binary.exe"}
]

如果题目简单，可以直接返回空数组 []。`, task.Type, task.Description, task.Target)

	resp, err := a.llmClient.CreateMessage(ctx, llm.MessageParams{
		SystemPrompt: "你是一名 CTF 题目分析专家，擅长将复杂题目分解为可独立执行的子任务。",
		Messages:     []llm.Message{llm.NewTextMessage("user", prompt)},
		MaxTokens:    1024,
	})
	if err != nil {
		return nil, err
	}

	text := resp.GetText()
	if text == "" {
		return nil, nil
	}

	// 解析 JSON 数组
	var subTasks []struct {
		Type        string `json:"type"`
		Description string `json:"description"`
		Target      string `json:"target"`
	}

	// 尝试提取 JSON
	jsonStart := indexOf(text, "[")
	jsonEnd := indexOf(text, "]")
	if jsonStart == -1 || jsonEnd == -1 {
		return nil, nil
	}

	err = json.Unmarshal([]byte(text[jsonStart:jsonEnd+1]), &subTasks)
	if err != nil {
		a.logger.Warn("failed to parse subtasks", "error", err)
		return nil, nil
	}

	// 转换为 Task
	var tasks []Task
	for i, st := range subTasks {
		tasks = append(tasks, Task{
			ID:          fmt.Sprintf("%s-sub-%d", task.ID, i),
			Type:        st.Type,
			Description: st.Description,
			Target:      st.Target,
			ParentID:    task.ID,
		})
	}

	return tasks, nil
}

// handleTaskDirectly 直接处理任务（当无法分解时）
func (a *PrimaryAgent) handleTaskDirectly(ctx context.Context, task Task, start time.Time) (*Result, error) {
	a.logger.Info("handling task directly", "task_id", task.ID)

	// 使用默认的 web agent 处理
	agentType := AgentTypeWeb
	if agent, ok := a.agents[agentType]; ok {
		return agent.Run(ctx, task)
	}

	// 如果没有 web agent，返回错误
	return &Result{
		TaskID:  task.ID,
		Success: false,
		Error:   "no suitable agent found for task",
	}, nil
}

// aggregateResults 汇总多个子任务的结果
func (a *PrimaryAgent) aggregateResults(ctx context.Context, task Task, results []*Result, start time.Time) (*Result, error) {
	// 检查是否有任何成功的结果
	for _, result := range results {
		if result.Success && result.Flag != "" {
			return result, nil
		}
	}

	// 构建汇总输出
	output := "子任务执行结果汇总：\n\n"
	for i, result := range results {
		output += fmt.Sprintf("子任务 %d (%s):\n", i+1, result.TaskID)
		output += fmt.Sprintf("  状态: %v\n", result.Success)
		if result.Flag != "" {
			output += fmt.Sprintf("  Flag: %s\n", result.Flag)
		}
		if result.Error != "" {
			output += fmt.Sprintf("  错误: %s\n", result.Error)
		}
		output += "\n"
	}

	return &Result{
		TaskID:     task.ID,
		Success:    false,
		Output:     output,
		Summary:    "所有子任务均未找到 flag",
		Iterations: len(results),
		Duration:   time.Since(start),
	}, nil
}

// SolveRequest 转换为 Task
func SolveRequestToTask(req SolveRequest) Task {
	return Task{
		ID:          fmt.Sprintf("task-%d", time.Now().UnixNano()),
		Type:        req.ChallengeType,
		Description: req.Description,
		Target:      req.Target,
		Files:       req.Files,
	}
}
