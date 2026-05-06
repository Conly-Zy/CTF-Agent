package metrics

import (
	"sync"
	"time"
)

// MetricsCollector 性能指标收集器
type MetricsCollector struct {
	mu sync.RWMutex

	// Agent 指标
	agentMetrics map[string]*AgentMetrics

	// 工具指标
	toolMetrics map[string]*ToolMetrics

	// LLM 指标
	llmMetrics *LLMMetrics

	// 系统指标
	systemMetrics *SystemMetrics
}

// AgentMetrics Agent 指标
type AgentMetrics struct {
	Name          string
	TotalTasks    int64
	SuccessTasks  int64
	FailedTasks   int64
	TotalDuration time.Duration
	AvgDuration   time.Duration
	LastActive    time.Time
}

// ToolMetrics 工具指标
type ToolMetrics struct {
	Name          string
	TotalCalls    int64
	SuccessCalls  int64
	FailedCalls   int64
	TotalDuration time.Duration
	AvgDuration   time.Duration
	LastUsed      time.Time
}

// LLMMetrics LLM 指标
type LLMMetrics struct {
	TotalCalls     int64
	TotalTokens    int64
	InputTokens    int64
	OutputTokens   int64
	TotalDuration  time.Duration
	AvgDuration    time.Duration
	CacheHitRate   float64
	LastCall       time.Time
}

// SystemMetrics 系统指标
type SystemMetrics struct {
	StartTime       time.Time
	Uptime          time.Duration
	TotalSessions   int64
	ActiveSessions  int64
	TotalFlags      int64
	SuccessRate     float64
	AvgSessionDuration time.Duration
}

// NewMetricsCollector 创建新的指标收集器
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		agentMetrics:  make(map[string]*AgentMetrics),
		toolMetrics:   make(map[string]*ToolMetrics),
		llmMetrics:    &LLMMetrics{},
		systemMetrics: &SystemMetrics{StartTime: time.Now()},
	}
}

// RecordAgentTask 记录 Agent 任务
func (c *MetricsCollector) RecordAgentTask(agentName string, success bool, duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	metrics, ok := c.agentMetrics[agentName]
	if !ok {
		metrics = &AgentMetrics{Name: agentName}
		c.agentMetrics[agentName] = metrics
	}

	metrics.TotalTasks++
	if success {
		metrics.SuccessTasks++
	} else {
		metrics.FailedTasks++
	}
	metrics.TotalDuration += duration
	metrics.AvgDuration = metrics.TotalDuration / time.Duration(metrics.TotalTasks)
	metrics.LastActive = time.Now()
}

// RecordToolCall 记录工具调用
func (c *MetricsCollector) RecordToolCall(toolName string, success bool, duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	metrics, ok := c.toolMetrics[toolName]
	if !ok {
		metrics = &ToolMetrics{Name: toolName}
		c.toolMetrics[toolName] = metrics
	}

	metrics.TotalCalls++
	if success {
		metrics.SuccessCalls++
	} else {
		metrics.FailedCalls++
	}
	metrics.TotalDuration += duration
	metrics.AvgDuration = metrics.TotalDuration / time.Duration(metrics.TotalCalls)
	metrics.LastUsed = time.Now()
}

// RecordLLMCall 记录 LLM 调用
func (c *MetricsCollector) RecordLLMCall(inputTokens, outputTokens int, duration time.Duration, cacheHit bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.llmMetrics.TotalCalls++
	c.llmMetrics.InputTokens += int64(inputTokens)
	c.llmMetrics.OutputTokens += int64(outputTokens)
	c.llmMetrics.TotalTokens += int64(inputTokens + outputTokens)
	c.llmMetrics.TotalDuration += duration
	c.llmMetrics.AvgDuration = c.llmMetrics.TotalDuration / time.Duration(c.llmMetrics.TotalCalls)
	c.llmMetrics.LastCall = time.Now()

	// 更新缓存命中率
	if cacheHit {
		hitRate := c.llmMetrics.CacheHitRate * float64(c.llmMetrics.TotalCalls-1)
		c.llmMetrics.CacheHitRate = (hitRate + 1.0) / float64(c.llmMetrics.TotalCalls)
	} else {
		hitRate := c.llmMetrics.CacheHitRate * float64(c.llmMetrics.TotalCalls-1)
		c.llmMetrics.CacheHitRate = hitRate / float64(c.llmMetrics.TotalCalls)
	}
}

// RecordSession 记录会话
func (c *MetricsCollector) RecordSession(success bool, duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.systemMetrics.TotalSessions++
	if success {
		c.systemMetrics.TotalFlags++
	}
	c.systemMetrics.Uptime = time.Since(c.systemMetrics.StartTime)
	c.systemMetrics.SuccessRate = float64(c.systemMetrics.TotalFlags) / float64(c.systemMetrics.TotalSessions)
	c.systemMetrics.AvgSessionDuration = c.systemMetrics.Uptime / time.Duration(c.systemMetrics.TotalSessions)
}

// SetActiveSessions 设置活跃会话数
func (c *MetricsCollector) SetActiveSessions(count int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.systemMetrics.ActiveSessions = count
}

// GetAgentMetrics 获取 Agent 指标
func (c *MetricsCollector) GetAgentMetrics(agentName string) *AgentMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.agentMetrics[agentName]
}

// GetAllAgentMetrics 获取所有 Agent 指标
func (c *MetricsCollector) GetAllAgentMetrics() map[string]*AgentMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*AgentMetrics)
	for k, v := range c.agentMetrics {
		result[k] = v
	}
	return result
}

// GetToolMetrics 获取工具指标
func (c *MetricsCollector) GetToolMetrics(toolName string) *ToolMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.toolMetrics[toolName]
}

// GetAllToolMetrics 获取所有工具指标
func (c *MetricsCollector) GetAllToolMetrics() map[string]*ToolMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*ToolMetrics)
	for k, v := range c.toolMetrics {
		result[k] = v
	}
	return result
}

// GetLLMMetrics 获取 LLM 指标
func (c *MetricsCollector) GetLLMMetrics() *LLMMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.llmMetrics
}

// GetSystemMetrics 获取系统指标
func (c *MetricsCollector) GetSystemMetrics() *SystemMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	c.systemMetrics.Uptime = time.Since(c.systemMetrics.StartTime)
	return c.systemMetrics
}

// GetSummary 获取指标摘要
func (c *MetricsCollector) GetSummary() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	agentSummaries := make(map[string]interface{})
	for name, metrics := range c.agentMetrics {
		agentSummaries[name] = map[string]interface{}{
			"total_tasks":   metrics.TotalTasks,
			"success_tasks": metrics.SuccessTasks,
			"failed_tasks":  metrics.FailedTasks,
			"success_rate":  float64(metrics.SuccessTasks) / float64(metrics.TotalTasks),
			"avg_duration":  metrics.AvgDuration.String(),
		}
	}

	toolSummaries := make(map[string]interface{})
	for name, metrics := range c.toolMetrics {
		toolSummaries[name] = map[string]interface{}{
			"total_calls":   metrics.TotalCalls,
			"success_calls": metrics.SuccessCalls,
			"failed_calls":  metrics.FailedCalls,
			"success_rate":  float64(metrics.SuccessCalls) / float64(metrics.TotalCalls),
			"avg_duration":  metrics.AvgDuration.String(),
		}
	}

	return map[string]interface{}{
		"agents":  agentSummaries,
		"tools":   toolSummaries,
		"llm": map[string]interface{}{
			"total_calls":    c.llmMetrics.TotalCalls,
			"total_tokens":   c.llmMetrics.TotalTokens,
			"input_tokens":   c.llmMetrics.InputTokens,
			"output_tokens":  c.llmMetrics.OutputTokens,
			"avg_duration":   c.llmMetrics.AvgDuration.String(),
			"cache_hit_rate": c.llmMetrics.CacheHitRate,
		},
		"system": map[string]interface{}{
			"uptime":           c.systemMetrics.Uptime.String(),
			"total_sessions":   c.systemMetrics.TotalSessions,
			"active_sessions":  c.systemMetrics.ActiveSessions,
			"total_flags":      c.systemMetrics.TotalFlags,
			"success_rate":     c.systemMetrics.SuccessRate,
			"avg_session_duration": c.systemMetrics.AvgSessionDuration.String(),
		},
	}
}

// Reset 重置所有指标
func (c *MetricsCollector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.agentMetrics = make(map[string]*AgentMetrics)
	c.toolMetrics = make(map[string]*ToolMetrics)
	c.llmMetrics = &LLMMetrics{}
	c.systemMetrics = &SystemMetrics{StartTime: time.Now()}
}
