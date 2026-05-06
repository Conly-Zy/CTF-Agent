package agent

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Reflector 负责纠正 Agent 的错误行为
type Reflector struct {
	logger    *slog.Logger
	maxRetries int
}

func NewReflector(logger *slog.Logger, maxRetries int) *Reflector {
	return &Reflector{
		logger:     logger,
		maxRetries: maxRetries,
	}
}

// ReflectResult 反思结果
type ReflectResult struct {
	ShouldRetry bool
	Message     string
	Suggestion  string
}

// ReflectOnNoToolCall 当 Agent 没有返回 tool_call 时进行反思
func (r *Reflector) ReflectOnNoToolCall(ctx context.Context, agentName string, iteration int, lastResponse string) *ReflectResult {
	r.logger.Warn("reflector: agent returned no tool call",
		"agent", agentName,
		"iteration", iteration)

	if iteration >= r.maxRetries {
		return &ReflectResult{
			ShouldRetry: false,
			Message:     fmt.Sprintf("[%s] 达到最大重试次数，停止反思", agentName),
		}
	}

	return &ReflectResult{
		ShouldRetry: true,
		Message:     fmt.Sprintf("[%s] 未返回工具调用，需要纠正", agentName),
		Suggestion:  "你必须使用工具来完成任务。请使用可用的工具之一，而不是直接返回文本。如果任务完成，请使用 barrier_done 工具报告结果。",
	}
}

// ReflectOnError 当工具执行出错时进行反思
func (r *Reflector) ReflectOnError(ctx context.Context, agentName string, toolName string, err error) *ReflectResult {
	r.logger.Warn("reflector: tool execution error",
		"agent", agentName,
		"tool", toolName,
		"error", err)

	return &ReflectResult{
		ShouldRetry: true,
		Message:     fmt.Sprintf("[%s] 工具 %s 执行出错: %v", agentName, toolName, err),
		Suggestion:  fmt.Sprintf("工具 %s 执行失败。请检查参数是否正确，或尝试使用其他工具。", toolName),
	}
}

// ReflectOnLoop 当 Agent 陷入循环时进行反思
func (r *Reflector) ReflectOnLoop(ctx context.Context, agentName string, toolCalls []string) *ReflectResult {
	r.logger.Warn("reflector: agent appears to be in a loop",
		"agent", agentName,
		"recent_calls", toolCalls)

	return &ReflectResult{
		ShouldRetry: true,
		Message:     fmt.Sprintf("[%s] 检测到循环模式", agentName),
		Suggestion:  "你似乎在重复相同的工具调用。请尝试不同的方法或工具来解决问题。",
	}
}

// ExecutionMonitor 监控 Agent 的执行情况
type ExecutionMonitor struct {
	logger         *slog.Logger
	toolCallCounts map[string]int
	recentCalls    []string
	maxSameTool    int
	maxTotalCalls  int
	windowSize     int
}

func NewExecutionMonitor(logger *slog.Logger, maxSameTool, maxTotalCalls, windowSize int) *ExecutionMonitor {
	return &ExecutionMonitor{
		logger:         logger,
		toolCallCounts: make(map[string]int),
		recentCalls:    make([]string, 0, windowSize),
		maxSameTool:    maxSameTool,
		maxTotalCalls:  maxTotalCalls,
		windowSize:     windowSize,
	}
}

// RecordToolCall 记录一次工具调用
func (m *ExecutionMonitor) RecordToolCall(toolName string) {
	m.toolCallCounts[toolName]++
	m.recentCalls = append(m.recentCalls, toolName)

	// 保持窗口大小
	if len(m.recentCalls) > m.windowSize {
		m.recentCalls = m.recentCalls[1:]
	}
}

// CheckForLoop 检查是否陷入循环
func (m *ExecutionMonitor) CheckForLoop() bool {
	// 检查同一工具调用次数
	for tool, count := range m.toolCallCounts {
		if count > m.maxSameTool {
			m.logger.Warn("execution monitor: same tool called too many times",
				"tool", tool,
				"count", count)
			return true
		}
	}

	// 检查总调用次数
	totalCalls := 0
	for _, count := range m.toolCallCounts {
		totalCalls += count
	}
	if totalCalls > m.maxTotalCalls {
		m.logger.Warn("execution monitor: total tool calls exceeded limit",
			"total", totalCalls)
		return true
	}

	// 检查最近调用是否重复
	if len(m.recentCalls) >= 3 {
		last := m.recentCalls[len(m.recentCalls)-1]
		same := 0
		for i := len(m.recentCalls) - 1; i >= 0; i-- {
			if m.recentCalls[i] == last {
				same++
			} else {
				break
			}
		}
		if same >= 3 {
			m.logger.Warn("execution monitor: same tool called 3+ times in a row",
				"tool", last)
			return true
		}
	}

	return false
}

// Reset 重置监控器
func (m *ExecutionMonitor) Reset() {
	m.toolCallCounts = make(map[string]int)
	m.recentCalls = m.recentCalls[:0]
}

// GetMentorAnalysis 获取导师分析（用于注入到工具响应中）
func (m *ExecutionMonitor) GetMentorAnalysis(agentName string) string {
	totalCalls := 0
	for _, count := range m.toolCallCounts {
		totalCalls += count
	}

	analysis := fmt.Sprintf("\n\n[Mentor Analysis for %s]\n", agentName)
	analysis += fmt.Sprintf("Total tool calls: %d\n", totalCalls)
	analysis += "Recent tool calls: "
	for i, call := range m.recentCalls {
		if i > 0 {
			analysis += " -> "
		}
		analysis += call
	}
	analysis += "\n\n建议：如果你已经多次尝试相同的方法但未成功，请尝试不同的策略。"
	analysis += "如果任务已经完成，请使用 barrier_done 工具报告结果。"

	return analysis
}

// WaitForBackoff 等待一段时间（用于错误重试）
func WaitForBackoff(attempt int) {
	backoff := time.Duration(attempt*100) * time.Millisecond
	if backoff > 2*time.Second {
		backoff = 2 * time.Second
	}
	time.Sleep(backoff)
}
