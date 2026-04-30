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

type SessionStats struct {
	TotalSessions   int            `json:"total_sessions"`
	SuccessSessions int            `json:"success_sessions"`
	FailedSessions  int            `json:"failed_sessions"`
	ByType          map[string]int `json:"by_type"`
	AvgDuration     float64        `json:"avg_duration_ms"`
}
