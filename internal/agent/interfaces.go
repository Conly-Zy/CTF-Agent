package agent

import (
	"context"
	"encoding/json"
	"time"
)

// AgentType Agent 类型枚举
type AgentType string

const (
	AgentTypePrimary AgentType = "primary"
	AgentTypeWeb     AgentType = "web"
	AgentTypePwn     AgentType = "pwn"
	AgentTypeCrypto  AgentType = "crypto"
	AgentTypeReverse AgentType = "reverse"
)

// Agent 定义所有 Agent 的统一接口
type Agent interface {
	// Name 返回 Agent 名称
	Name() string

	// Type 返回 Agent 类型 (primary/web/pwn/crypto/reverse)
	Type() AgentType

	// Run 执行 Agent 任务，返回结果
	Run(ctx context.Context, task Task) (*Result, error)

	// SetMessageChannel 设置消息发送通道
	SetMessageChannel(ch chan<- Message)
}

// Task 表示分配给 Agent 的任务
type Task struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`        // 任务类型
	Description string          `json:"description"` // 任务描述
	Target      string          `json:"target"`      // 目标地址
	Files       []string        `json:"files"`       // 相关文件
	Context     json.RawMessage `json:"context"`     // 额外上下文
	ParentID    string          `json:"parent_id"`   // 父任务 ID
}

// Result 表示 Agent 执行结果
type Result struct {
	TaskID     string        `json:"task_id"`
	Success    bool          `json:"success"`
	Flag       string        `json:"flag,omitempty"`
	Output     string        `json:"output"`           // 详细输出
	Summary    string        `json:"summary"`          // 简要摘要
	Error      string        `json:"error,omitempty"`
	Iterations int           `json:"iterations"`
	Duration   time.Duration `json:"duration"`
}

// Message Agent 间通信消息
type Message struct {
	From    AgentType       `json:"from"`
	To      AgentType       `json:"to"`
	Type    MessageType     `json:"type"`
	TaskID  string          `json:"task_id"`
	Payload json.RawMessage `json:"payload"`
}

// MessageType 消息类型
type MessageType string

const (
	MsgTypeTask     MessageType = "task"     // 任务分配
	MsgTypeResult   MessageType = "result"   // 结果返回
	MsgTypeAsk      MessageType = "ask"      // 请求协助
	MsgTypeAnswer   MessageType = "answer"   // 回答协助
	MsgTypeDone     MessageType = "done"     // 任务完成
	MsgTypeProgress MessageType = "progress" // 进度更新
)

// Logger 日志接口，与现有兼容
type Logger interface {
	Log(level, message string)
	ToolStart(tool string)
	ToolResult(tool, result string)
	Thinking(content string)
	Flag(flag string)
	// 新增：Agent 级别日志
	AgentStart(agentName string)
	AgentComplete(agentName string, success bool)
	TaskAssigned(taskID, agentName string)
}

// Store 存储接口，与现有兼容
type Store interface {
	AddConversationMessage(msg any) error
}

// KnowledgeStore 知识库接口，与现有兼容
type KnowledgeStore interface {
	SearchKnowledgeByType(challengeType string, limit int) ([]KnowledgeEntry, error)
}

// KnowledgeEntry 知识库条目
type KnowledgeEntry struct {
	Title   string
	Content string
	Type    string
}
