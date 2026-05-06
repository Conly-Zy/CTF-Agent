package replay

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// ReplayEvent 回放事件
type ReplayEvent struct {
	Timestamp time.Time   `json:"timestamp"`
	Type      string      `json:"type"` // message, tool_start, tool_result, thinking, agent_start, agent_complete, flag
	Data      interface{} `json:"data"`
}

// MessageData 消息数据
type MessageData struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ToolStartData 工具开始数据
type ToolStartData struct {
	Tool  string `json:"tool"`
	Input string `json:"input,omitempty"`
}

// ToolResultData 工具结果数据
type ToolResultData struct {
	Tool   string `json:"tool"`
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// AgentEventData Agent 事件数据
type AgentEventData struct {
	AgentName string `json:"agent_name"`
	Success   bool   `json:"success,omitempty"`
	TaskID    string `json:"task_id,omitempty"`
}

// FlagData Flag 数据
type FlagData struct {
	Flag string `json:"flag"`
}

// SessionReplay 会话回放
type SessionReplay struct {
	SessionID int64         `json:"session_id"`
	Events    []ReplayEvent `json:"events"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
}

// ReplayRecorder 回放录制器
type ReplayRecorder struct {
	events    []ReplayEvent
	startTime time.Time
}

// NewReplayRecorder 创建录制器
func NewReplayRecorder() *ReplayRecorder {
	return &ReplayRecorder{
		startTime: time.Now(),
	}
}

// RecordMessage 录制消息
func (r *ReplayRecorder) RecordMessage(role, content string) {
	r.events = append(r.events, ReplayEvent{
		Timestamp: time.Now(),
		Type:      "message",
		Data: MessageData{
			Role:    role,
			Content: content,
		},
	})
}

// RecordToolStart 录制工具开始
func (r *ReplayRecorder) RecordToolStart(tool, input string) {
	r.events = append(r.events, ReplayEvent{
		Timestamp: time.Now(),
		Type:      "tool_start",
		Data: ToolStartData{
			Tool:  tool,
			Input: input,
		},
	})
}

// RecordToolResult 录制工具结果
func (r *ReplayRecorder) RecordToolResult(tool, output string, err string) {
	r.events = append(r.events, ReplayEvent{
		Timestamp: time.Now(),
		Type:      "tool_result",
		Data: ToolResultData{
			Tool:   tool,
			Output: output,
			Error:  err,
		},
	})
}

// RecordAgentStart 录制 Agent 启动
func (r *ReplayRecorder) RecordAgentStart(agentName string) {
	r.events = append(r.events, ReplayEvent{
		Timestamp: time.Now(),
		Type:      "agent_start",
		Data:      AgentEventData{AgentName: agentName},
	})
}

// RecordAgentComplete 录制 Agent 完成
func (r *ReplayRecorder) RecordAgentComplete(agentName string, success bool) {
	r.events = append(r.events, ReplayEvent{
		Timestamp: time.Now(),
		Type:      "agent_complete",
		Data:      AgentEventData{AgentName: agentName, Success: success},
	})
}

// RecordFlag 录制 Flag
func (r *ReplayRecorder) RecordFlag(flag string) {
	r.events = append(r.events, ReplayEvent{
		Timestamp: time.Now(),
		Type:      "flag",
		Data:      FlagData{Flag: flag},
	})
}

// RecordThinking 录制思考过程
func (r *ReplayRecorder) RecordThinking(content string) {
	r.events = append(r.events, ReplayEvent{
		Timestamp: time.Now(),
		Type:      "thinking",
		Data:      map[string]string{"content": content},
	})
}

// Build 构建回放数据
func (r *ReplayRecorder) Build(sessionID int64) *SessionReplay {
	return &SessionReplay{
		SessionID: sessionID,
		Events:    r.events,
		StartTime: r.startTime,
		EndTime:   time.Now(),
	}
}

// Save 保存回放数据到文件
func (r *ReplayRecorder) Save(path string, sessionID int64) error {
	replay := r.Build(sessionID)
	data, err := json.MarshalIndent(replay, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal replay: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// LoadReplay 从文件加载回放数据
func LoadReplay(path string) (*SessionReplay, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read replay: %w", err)
	}

	var replay SessionReplay
	if err := json.Unmarshal(data, &replay); err != nil {
		return nil, fmt.Errorf("parse replay: %w", err)
	}

	return &replay, nil
}

// Duration 回放总时长
func (r *SessionReplay) Duration() time.Duration {
	return r.EndTime.Sub(r.StartTime)
}

// EventCount 事件数量
func (r *SessionReplay) EventCount() int {
	return len(r.Events)
}

// EventsByType 按类型获取事件
func (r *SessionReplay) EventsByType(eventType string) []ReplayEvent {
	var result []ReplayEvent
	for _, e := range r.Events {
		if e.Type == eventType {
			result = append(result, e)
		}
	}
	return result
}

// ToolCalls 获取所有工具调用
func (r *SessionReplay) ToolCalls() []ToolStartData {
	var result []ToolStartData
	for _, e := range r.Events {
		if e.Type == "tool_start" {
			if data, ok := e.Data.(map[string]interface{}); ok {
				result = append(result, ToolStartData{
					Tool:  data["tool"].(string),
					Input: data["input"].(string),
				})
			}
		}
	}
	return result
}

// GetFlag 获取找到的 Flag
func (r *SessionReplay) GetFlag() string {
	for _, e := range r.Events {
		if e.Type == "flag" {
			if data, ok := e.Data.(map[string]interface{}); ok {
				return data["flag"].(string)
			}
		}
	}
	return ""
}
