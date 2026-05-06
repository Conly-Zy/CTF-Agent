package logging

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

// LogLevel 日志级别
type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
)

// LogEntry 日志条目
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     LogLevel               `json:"level"`
	Message   string                 `json:"message"`
	Source    string                 `json:"source,omitempty"`
	SessionID int64                  `json:"session_id,omitempty"`
	AgentName string                 `json:"agent_name,omitempty"`
	ToolName  string                 `json:"tool_name,omitempty"`
	Extra     map[string]interface{} `json:"extra,omitempty"`
}

// LogAggregator 日志聚合器
type LogAggregator struct {
	mu          sync.RWMutex
	entries     []LogEntry
	maxEntries  int
	filePath    string
	logger      *slog.Logger
	callbacks   []func(LogEntry)
	buffer      []LogEntry
	bufferSize  int
	flushInterval time.Duration
	stopCh      chan struct{}
}

// NewLogAggregator 创建日志聚合器
func NewLogAggregator(logger *slog.Logger, maxEntries int, filePath string) *LogAggregator {
	return &LogAggregator{
		entries:       make([]LogEntry, 0, maxEntries),
		maxEntries:    maxEntries,
		filePath:      filePath,
		logger:        logger,
		buffer:        make([]LogEntry, 0, 100),
		bufferSize:    100,
		flushInterval: 5 * time.Second,
		stopCh:        make(chan struct{}),
	}
}

// OnLog 注册日志回调
func (a *LogAggregator) OnLog(callback func(LogEntry)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.callbacks = append(a.callbacks, callback)
}

// Start 启动聚合器
func (a *LogAggregator) Start() {
	a.logger.Info("log aggregator started", "file", a.filePath)

	// 启动刷新协程
	go func() {
		ticker := time.NewTicker(a.flushInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				a.flush()
			case <-a.stopCh:
				a.flush()
				return
			}
		}
	}()
}

// Stop 停止聚合器
func (a *LogAggregator) Stop() {
	close(a.stopCh)
	a.logger.Info("log aggregator stopped")
}

// Log 记录日志
func (a *LogAggregator) Log(level LogLevel, message string, extras ...map[string]interface{}) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
	}

	if len(extras) > 0 {
		entry.Extra = extras[0]
	}

	// 添加到缓冲区
	a.mu.Lock()
	a.buffer = append(a.buffer, entry)
	a.entries = append(a.entries, entry)

	// 限制条目数量
	if len(a.entries) > a.maxEntries {
		a.entries = a.entries[1:]
	}
	a.mu.Unlock()

	// 调用回调
	a.mu.RLock()
	callbacks := a.callbacks
	a.mu.RUnlock()

	for _, callback := range callbacks {
		callback(entry)
	}
}

// LogWithSession 记录会话日志
func (a *LogAggregator) LogWithSession(level LogLevel, message string, sessionID int64) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		SessionID: sessionID,
	}

	a.mu.Lock()
	a.buffer = append(a.buffer, entry)
	a.entries = append(a.entries, entry)

	if len(a.entries) > a.maxEntries {
		a.entries = a.entries[1:]
	}
	a.mu.Unlock()

	a.mu.RLock()
	callbacks := a.callbacks
	a.mu.RUnlock()

	for _, callback := range callbacks {
		callback(entry)
	}
}

// LogWithAgent 记录 Agent 日志
func (a *LogAggregator) LogWithAgent(level LogLevel, message string, agentName string) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		AgentName: agentName,
	}

	a.mu.Lock()
	a.buffer = append(a.buffer, entry)
	a.entries = append(a.entries, entry)

	if len(a.entries) > a.maxEntries {
		a.entries = a.entries[1:]
	}
	a.mu.Unlock()

	a.mu.RLock()
	callbacks := a.callbacks
	a.mu.RUnlock()

	for _, callback := range callbacks {
		callback(entry)
	}
}

// LogWithTool 记录工具日志
func (a *LogAggregator) LogWithTool(level LogLevel, message string, toolName string) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		ToolName:  toolName,
	}

	a.mu.Lock()
	a.buffer = append(a.buffer, entry)
	a.entries = append(a.entries, entry)

	if len(a.entries) > a.maxEntries {
		a.entries = a.entries[1:]
	}
	a.mu.Unlock()

	a.mu.RLock()
	callbacks := a.callbacks
	a.mu.RUnlock()

	for _, callback := range callbacks {
		callback(entry)
	}
}

// GetEntries 获取所有日志条目
func (a *LogAggregator) GetEntries(limit int) []LogEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if limit <= 0 || limit > len(a.entries) {
		limit = len(a.entries)
	}

	start := len(a.entries) - limit
	result := make([]LogEntry, limit)
	copy(result, a.entries[start:])
	return result
}

// GetEntriesByLevel 按级别获取日志条目
func (a *LogAggregator) GetEntriesByLevel(level LogLevel, limit int) []LogEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var result []LogEntry
	for i := len(a.entries) - 1; i >= 0; i-- {
		if a.entries[i].Level == level {
			result = append(result, a.entries[i])
			if len(result) >= limit {
				break
			}
		}
	}
	return result
}

// GetEntriesBySession 按会话获取日志条目
func (a *LogAggregator) GetEntriesBySession(sessionID int64, limit int) []LogEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var result []LogEntry
	for i := len(a.entries) - 1; i >= 0; i-- {
		if a.entries[i].SessionID == sessionID {
			result = append(result, a.entries[i])
			if len(result) >= limit {
				break
			}
		}
	}
	return result
}

// GetEntriesByAgent 按 Agent 获取日志条目
func (a *LogAggregator) GetEntriesByAgent(agentName string, limit int) []LogEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var result []LogEntry
	for i := len(a.entries) - 1; i >= 0; i-- {
		if a.entries[i].AgentName == agentName {
			result = append(result, a.entries[i])
			if len(result) >= limit {
				break
			}
		}
	}
	return result
}

// LogFilter 日志过滤器
type LogFilter struct {
	Level     string
	SessionID string
	Agent     string
	Tool      string
	Limit     int
}

// Query 按条件查询日志
func (a *LogAggregator) Query(filter LogFilter) []LogEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()

	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}

	var result []LogEntry
	for i := len(a.entries) - 1; i >= 0; i-- {
		entry := a.entries[i]
		if filter.Level != "" && string(entry.Level) != filter.Level {
			continue
		}
		if filter.Agent != "" && entry.AgentName != filter.Agent {
			continue
		}
		if filter.Tool != "" && entry.ToolName != filter.Tool {
			continue
		}
		result = append(result, entry)
		if len(result) >= limit {
			break
		}
	}
	return result
}

// Search 搜索日志
func (a *LogAggregator) Search(keyword string, limit int) []LogEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var result []LogEntry
	for i := len(a.entries) - 1; i >= 0; i-- {
		if contains(a.entries[i].Message, keyword) {
			result = append(result, a.entries[i])
			if len(result) >= limit {
				break
			}
		}
	}
	return result
}

// GetStats 获取日志统计
func (a *LogAggregator) GetStats() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	stats := map[string]interface{}{
		"total":  len(a.entries),
		"levels": make(map[LogLevel]int),
	}

	levelCounts := stats["levels"].(map[LogLevel]int)
	for _, entry := range a.entries {
		levelCounts[entry.Level]++
	}

	return stats
}

func (a *LogAggregator) flush() {
	a.mu.Lock()
	buffer := a.buffer
	a.buffer = a.buffer[:0]
	a.mu.Unlock()

	if len(buffer) == 0 {
		return
	}

	// 写入文件
	if a.filePath != "" {
		a.writeToFile(buffer)
	}
}

func (a *LogAggregator) writeToFile(entries []LogEntry) {
	f, err := os.OpenFile(a.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		a.logger.Error("failed to open log file", "error", err)
		return
	}
	defer f.Close()

	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		fmt.Fprintf(f, "%s\n", data)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
