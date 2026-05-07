package store

import "time"

type Session struct {
	ID            int64      `json:"id" db:"id"`
	ChallengeType string     `json:"challenge_type" db:"challenge_type"`
	Description   string     `json:"description" db:"description"`
	Target        string     `json:"target" db:"target"`
	Files         string     `json:"files" db:"files"`
	Status        string     `json:"status" db:"status"`
	Flag          string     `json:"flag" db:"flag"`
	Iterations    int        `json:"iterations" db:"iterations"`
	DurationMs    int64      `json:"duration_ms" db:"duration_ms"`
	Error         string     `json:"error" db:"error"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	CompletedAt   *time.Time `json:"completed_at" db:"completed_at"`
}

type Knowledge struct {
	ID        int64     `json:"id" db:"id"`
	SessionID int64     `json:"session_id" db:"session_id"`
	Title     string    `json:"title" db:"title"`
	Content   string    `json:"content" db:"content"`
	Type      string    `json:"type" db:"type"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type Tag struct {
	ID   int64  `json:"id" db:"id"`
	Name string `json:"name" db:"name"`
}

type KnowledgeTag struct {
	KnowledgeID int64 `json:"knowledge_id" db:"knowledge_id"`
	TagID       int64 `json:"tag_id" db:"tag_id"`
}

type ConversationMessage struct {
	ID        int64     `json:"id" db:"id"`
	SessionID int64     `json:"session_id" db:"session_id"`
	Role      string    `json:"role" db:"role"`
	Content   string    `json:"content" db:"content"`
	ToolName  string    `json:"tool_name" db:"tool_name"`
	ToolInput string    `json:"tool_input" db:"tool_input"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// Subtask is a persisted execution unit inspired by PentAGI's Flow→Task→Subtask
// hierarchy. In CTF-Agent a session acts as the top-level flow.
type Subtask struct {
	ID            int64      `json:"id" db:"id"`
	SessionID     int64      `json:"session_id" db:"session_id"`
	TaskID        string     `json:"task_id" db:"task_id"`
	ParentID      string     `json:"parent_id" db:"parent_id"`
	AgentName     string     `json:"agent_name" db:"agent_name"`
	AgentType     string     `json:"agent_type" db:"agent_type"`
	ChallengeType string     `json:"challenge_type" db:"challenge_type"`
	Title         string     `json:"title" db:"title"`
	Description   string     `json:"description" db:"description"`
	Target        string     `json:"target" db:"target"`
	Status        string     `json:"status" db:"status"`
	Result        string     `json:"result" db:"result"`
	Error         string     `json:"error" db:"error"`
	SortOrder     int        `json:"sort_order" db:"sort_order"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
	CompletedAt   *time.Time `json:"completed_at" db:"completed_at"`
}

// ToolCall stores every executed tool action with timing and status.
type ToolCall struct {
	ID          int64      `json:"id" db:"id"`
	SessionID   int64      `json:"session_id" db:"session_id"`
	SubtaskID   *int64     `json:"subtask_id,omitempty" db:"subtask_id"`
	TaskID      string     `json:"task_id" db:"task_id"`
	AgentName   string     `json:"agent_name" db:"agent_name"`
	AgentType   string     `json:"agent_type" db:"agent_type"`
	ToolUseID   string     `json:"tool_use_id" db:"tool_use_id"`
	ToolName    string     `json:"tool_name" db:"tool_name"`
	Input       string     `json:"input" db:"input"`
	Output      string     `json:"output" db:"output"`
	Status      string     `json:"status" db:"status"`
	Error       string     `json:"error" db:"error"`
	StartedAt   time.Time  `json:"started_at" db:"started_at"`
	CompletedAt *time.Time `json:"completed_at" db:"completed_at"`
	DurationMs  int64      `json:"duration_ms" db:"duration_ms"`
}

// FlowTemplate is a reusable CTF playbook/template. It mirrors PentAGI's
// flow_templates concept but is scoped to challenge categories.
type FlowTemplate struct {
	ID            int64     `json:"id" db:"id"`
	ChallengeType string    `json:"challenge_type" db:"challenge_type"`
	Title         string    `json:"title" db:"title"`
	Description   string    `json:"description" db:"description"`
	Content       string    `json:"content" db:"content"`
	Tags          string    `json:"tags" db:"tags"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

type ToolCallStats struct {
	TotalCalls      int                 `json:"total_calls"`
	SuccessCalls    int                 `json:"success_calls"`
	FailedCalls     int                 `json:"failed_calls"`
	TotalDurationMs int64               `json:"total_duration_ms"`
	AvgDurationMs   float64             `json:"avg_duration_ms"`
	ByTool          []ToolCallGroupStat `json:"by_tool"`
	ByAgent         []ToolCallGroupStat `json:"by_agent"`
}

type ToolCallGroupStat struct {
	Name            string    `json:"name"`
	TotalCalls      int       `json:"total_calls"`
	SuccessCalls    int       `json:"success_calls"`
	FailedCalls     int       `json:"failed_calls"`
	TotalDurationMs int64     `json:"total_duration_ms"`
	AvgDurationMs   float64   `json:"avg_duration_ms"`
	LastUsed        time.Time `json:"last_used"`
}

type SessionStats struct {
	TotalSessions   int            `json:"total_sessions"`
	SuccessSessions int            `json:"success_sessions"`
	FailedSessions  int            `json:"failed_sessions"`
	ByType          map[string]int `json:"by_type"`
	AvgDuration     float64        `json:"avg_duration_ms"`
}
