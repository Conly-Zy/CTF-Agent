package agent

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"text/template"
	"time"

	"github.com/Conly-Zy/CTF-Agent/internal/llm"
	"github.com/Conly-Zy/CTF-Agent/internal/tools"
)

//go:embed prompts/*.md
var promptTemplates embed.FS

// BaseAgent 提供 Agent 的基础实现
type BaseAgent struct {
	AgentName      string
	AgentType      AgentType
	LlmClient      *llm.Client
	ToolRegistry   *tools.Registry
	Logger         *slog.Logger
	MaxIter        int
	Timeout        time.Duration
	MessageCh      chan<- Message
	KnowledgeStore KnowledgeStore
	Reflector      *Reflector
	ExecMonitor   *ExecutionMonitor
	Summarizer    *Summarizer
}

func NewBaseAgent(
	name string,
	agentType AgentType,
	llmClient *llm.Client,
	toolRegistry *tools.Registry,
	logger *slog.Logger,
	maxIter int,
	timeout time.Duration,
) *BaseAgent {
	return &BaseAgent{
		AgentName:    name,
		AgentType:    agentType,
		LlmClient:    llmClient,
		ToolRegistry: toolRegistry,
		Logger:       logger,
		MaxIter:      maxIter,
		Timeout:      timeout,
		Reflector:    NewReflector(logger, 3),
		ExecMonitor: NewExecutionMonitor(logger, 5, 30, 10),
		Summarizer:  NewSummarizer(llmClient, logger, 20, 5),
	}
}

func (a *BaseAgent) Name() string {
	return a.AgentName
}

func (a *BaseAgent) Type() AgentType {
	return a.AgentType
}

func (a *BaseAgent) SetMessageChannel(ch chan<- Message) {
	a.MessageCh = ch
}

func (a *BaseAgent) SetKnowledgeStore(ks KnowledgeStore) {
	a.KnowledgeStore = ks
}

// BuildSystemPrompt 从模板构建系统提示词
func (a *BaseAgent) BuildSystemPrompt(task Task) (string, error) {
	templateFile := fmt.Sprintf("prompts/%s.md", a.AgentType)
	tmplContent, err := promptTemplates.ReadFile(templateFile)
	if err != nil {
		return "", fmt.Errorf("read template %s: %w", templateFile, err)
	}

	tmpl, err := template.New(string(a.AgentType)).Parse(string(tmplContent))
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	// 构建知识库内容
	knowledge := ""
	if a.KnowledgeStore != nil {
		entries, err := a.KnowledgeStore.SearchKnowledgeByType(task.Type, 3)
		if err == nil && len(entries) > 0 {
			knowledge = "\n## 历史解题经验\n以下是过去成功解题的经验总结，可以参考：\n"
			for i, e := range entries {
				knowledge += fmt.Sprintf("\n### 经验 %d: %s\n", i+1, e.Title)
				content := e.Content
				if len(content) > 800 {
					content = content[:800] + "..."
				}
				knowledge += content + "\n"
			}
		}
	}

	// 构建工具列表
	toolsList := "\n## 可用工具\n"
	agentTools := a.ToolRegistry.GetForAgent(tools.AgentType(a.AgentType))
	for _, tool := range agentTools {
		toolsList += fmt.Sprintf("- %s: %s\n", tool.Name(), tool.Description())
	}

	var result strings.Builder
	err = tmpl.Execute(&result, map[string]string{
		"Knowledge": knowledge,
		"Tools":     toolsList,
	})
	if err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return result.String(), nil
}

// SolveLoop 执行解题循环
func (a *BaseAgent) SolveLoop(ctx context.Context, task Task, systemPrompt string, log Logger) (*Result, error) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, a.Timeout)
	defer cancel()

	// 获取 Agent 专用工具
	agentTools := a.ToolRegistry.GetForAgent(tools.AgentType(a.AgentType))
	claudeTools := make([]llm.ToolDefinition, len(agentTools))
	for i, t := range agentTools {
		claudeTools[i] = llm.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.Schema(),
		}
	}

	// 创建 Barrier 通道
	barrierCh := make(chan tools.BarrierPayload, 1)
	answersCh := make(chan string, 1)

	// 创建专用 Barrier 工具
	doneTool := tools.NewDoneBarrierTool(barrierCh)
	askTool := tools.NewAskBarrierTool(barrierCh, answersCh)

	// 替换工具列表中的 barrier 工具
	for i, t := range agentTools {
		if t.Name() == "barrier_done" {
			agentTools[i] = doneTool
			claudeTools[i] = llm.ToolDefinition{
				Name:        doneTool.Name(),
				Description: doneTool.Description(),
				InputSchema: doneTool.Schema(),
			}
		} else if t.Name() == "barrier_ask" {
			agentTools[i] = askTool
			claudeTools[i] = llm.ToolDefinition{
				Name:        askTool.Name(),
				Description: askTool.Description(),
				InputSchema: askTool.Schema(),
			}
		}
	}

	// 构建用户消息
	userMsg := llm.NewTextMessage("user", fmt.Sprintf("题目类型: %s\n题目描述: %s\n目标: %s\n\n请分析这道题目并找到 flag。",
		task.Type, task.Description, task.Target))
	messages := []llm.Message{userMsg}

	if log != nil {
		log.Log("info", fmt.Sprintf("[%s] 开始解题循环", a.AgentName))
	}

	// 启动 Barrier 监听
	go func() {
		select {
		case barrier := <-barrierCh:
			if barrier.Type == tools.BarrierDone {
				a.Logger.Info("barrier_done received", "flag", barrier.Flag)
				// 结果会在主循环中处理
			} else if barrier.Type == tools.BarrierAsk {
				a.Logger.Info("barrier_ask received", "question", barrier.Question)
				// 发送协助请求
				if a.MessageCh != nil {
					payload, _ := json.Marshal(barrier)
					a.MessageCh <- Message{
						From:    a.AgentType,
						To:      AgentTypePrimary,
						Type:    MsgTypeAsk,
						TaskID:  task.ID,
						Payload: payload,
					}
				}
			}
		case <-ctx.Done():
			return
		}
	}()

	for i := 0; i < a.MaxIter; i++ {
		select {
		case <-ctx.Done():
			return &Result{
				TaskID:     task.ID,
				Success:    false,
				Error:      "context cancelled",
				Iterations: i,
				Duration:   time.Since(start),
			}, nil
		default:
		}

		if log != nil {
			log.Log("info", fmt.Sprintf("[%s] 迭代 %d/%d", a.AgentName, i+1, a.MaxIter))
		}

		// 检查是否需要压缩消息
		if a.Summarizer.ShouldSummarize(messages) {
			summarized, err := a.Summarizer.Summarize(ctx, messages)
			if err != nil {
				a.Logger.Warn("summarization failed", "error", err)
			} else {
				messages = summarized
				if log != nil {
					log.Log("info", fmt.Sprintf("[%s] 消息已压缩", a.AgentName))
				}
			}
		}

		// 调用 LLM
		resp, err := a.LlmClient.CreateMessage(ctx, llm.MessageParams{
			SystemPrompt: systemPrompt,
			Messages:     messages,
			Tools:        claudeTools,
			MaxTokens:    4096,
		})
		if err != nil {
			return nil, fmt.Errorf("LLM call failed: %w", err)
		}

		// 记录思考过程
		text := resp.GetText()
		if text != "" && log != nil {
			log.Thinking(text)
		}

		// 获取工具调用
		toolUses := resp.GetToolUse()

		// 没有工具调用 - 使用 Reflector 纠正
		if len(toolUses) == 0 {
			reflectResult := a.Reflector.ReflectOnNoToolCall(ctx, a.AgentName, i, text)
			if reflectResult.ShouldRetry {
				// 注入纠正消息
				messages = append(messages, llm.NewTextMessage("user", reflectResult.Suggestion))
				if log != nil {
					log.Log("warning", reflectResult.Message)
				}
				continue
			}
			// 无法纠正，结束
			if log != nil {
				log.Log("warning", reflectResult.Message)
			}
			continue
		}

		// 检查是否陷入循环
		for _, tu := range toolUses {
			a.ExecMonitor.RecordToolCall(tu.Name)
		}
		if a.ExecMonitor.CheckForLoop() {
			analysis := a.ExecMonitor.GetMentorAnalysis(a.AgentName)
			messages = append(messages, llm.NewTextMessage("user", analysis))
			if log != nil {
				log.Log("warning", fmt.Sprintf("[%s] 检测到循环，注入导师分析", a.AgentName))
			}
			a.ExecMonitor.Reset()
			continue
		}

		// 构建助手消息
		messages = append(messages, llm.NewToolUseMessage(text, toolUses))

		// 执行工具调用
		var toolResults []llm.ContentBlock
		for _, tu := range toolUses {
			if log != nil {
				log.ToolStart(tu.Name)
			}

			// 查找工具
			var tool tools.Tool
			for _, t := range agentTools {
				if t.Name() == tu.Name {
					tool = t
					break
				}
			}

			if tool == nil {
				errMsg := fmt.Sprintf("工具未找到: %s", tu.Name)
				if log != nil {
					log.Log("error", errMsg)
				}
				toolResults = append(toolResults, llm.ContentBlock{
					Type:      "tool_result",
					ToolUseID: tu.ID,
					Content:   errMsg,
					IsError:   true,
				})
				continue
			}

			// 执行工具
			output, err := tool.Execute(ctx, tu.Input)
			if err != nil {
				// 使用 Reflector 分析错误
				reflectResult := a.Reflector.ReflectOnError(ctx, a.AgentName, tu.Name, err)
				if log != nil {
					log.Log("error", reflectResult.Message)
				}
				output = reflectResult.Suggestion
			}

			if log != nil {
				log.ToolResult(tu.Name, output)
			}

			// 检查是否是 Barrier 工具
			if tu.Name == "barrier_done" {
				// 从输出中提取 flag
				flag := ExtractFlagFromOutput(output)
				if flag != "" {
					return &Result{
						TaskID:     task.ID,
						Success:    true,
						Flag:       flag,
						Iterations: i + 1,
						Duration:   time.Since(start),
					}, nil
				}
			}

			toolResults = append(toolResults, llm.ContentBlock{
				Type:      "tool_result",
				ToolUseID: tu.ID,
				Content:   output,
			})
		}

		// 添加工具结果
		messages = append(messages, llm.NewToolResultMessage(toolResults))
	}

	return &Result{
		TaskID:     task.ID,
		Success:    false,
		Error:      "max iterations reached",
		Iterations: a.MaxIter,
		Duration:   time.Since(start),
	}, nil
}

// ExtractFlagFromOutput 从工具输出中提取 flag
func ExtractFlagFromOutput(output string) string {
	// 尝试从 "Flag: xxx" 格式中提取
	if idx := indexOf(output, "Flag: "); idx != -1 {
		flag := output[idx+6:]
		if endIdx := indexOf(flag, "\n"); endIdx != -1 {
			return flag[:endIdx]
		}
		return flag
	}

	// 尝试常见的 flag 格式
	fallbacks := []string{"flag{", "CTF{", "picoCTF{", "FLAG{"}
	for _, prefix := range fallbacks {
		start := indexOf(output, prefix)
		if start == -1 {
			continue
		}
		end := indexOf(output[start:], "}")
		if end == -1 {
			continue
		}
		return output[start : start+end+1]
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
