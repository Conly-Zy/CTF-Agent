package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return store, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			challenge_type TEXT NOT NULL,
			description TEXT,
			target TEXT,
			files TEXT,
			status TEXT DEFAULT 'pending',
			flag TEXT,
			iterations INTEGER DEFAULT 0,
			duration_ms INTEGER DEFAULT 0,
			error TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS knowledge (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			type TEXT DEFAULT 'technique',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (session_id) REFERENCES sessions(id)
		)`,
		`CREATE TABLE IF NOT EXISTS tags (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE
		)`,
		`CREATE TABLE IF NOT EXISTS knowledge_tags (
			knowledge_id INTEGER NOT NULL,
			tag_id INTEGER NOT NULL,
			PRIMARY KEY (knowledge_id, tag_id),
			FOREIGN KEY (knowledge_id) REFERENCES knowledge(id),
			FOREIGN KEY (tag_id) REFERENCES tags(id)
		)`,
		`CREATE TABLE IF NOT EXISTS conversation_messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id INTEGER NOT NULL,
			role TEXT NOT NULL,
			content TEXT,
			tool_name TEXT,
			tool_input TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (session_id) REFERENCES sessions(id)
		)`,
		`CREATE TABLE IF NOT EXISTS subtasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id INTEGER NOT NULL,
			task_id TEXT NOT NULL,
			parent_id TEXT,
			agent_name TEXT,
			agent_type TEXT,
			challenge_type TEXT,
			title TEXT,
			description TEXT,
			target TEXT,
			status TEXT DEFAULT 'created',
			result TEXT,
			error TEXT,
			sort_order INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME,
			FOREIGN KEY (session_id) REFERENCES sessions(id)
		)`,
		`CREATE TABLE IF NOT EXISTS tool_calls (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id INTEGER NOT NULL,
			subtask_id INTEGER,
			task_id TEXT,
			agent_name TEXT,
			agent_type TEXT,
			tool_use_id TEXT,
			tool_name TEXT NOT NULL,
			input TEXT,
			output TEXT,
			status TEXT DEFAULT 'received',
			error TEXT,
			started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME,
			duration_ms INTEGER DEFAULT 0,
			FOREIGN KEY (session_id) REFERENCES sessions(id),
			FOREIGN KEY (subtask_id) REFERENCES subtasks(id)
		)`,
		`CREATE TABLE IF NOT EXISTS flow_templates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			challenge_type TEXT NOT NULL,
			title TEXT NOT NULL,
			description TEXT,
			content TEXT NOT NULL,
			tags TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(challenge_type, title)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_type ON sessions(challenge_type)`,
		`CREATE INDEX IF NOT EXISTS idx_knowledge_session ON knowledge(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_knowledge_type ON knowledge(type)`,
		`CREATE INDEX IF NOT EXISTS idx_conversation_session ON conversation_messages(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_subtasks_session ON subtasks(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_subtasks_status ON subtasks(status)`,
		`CREATE INDEX IF NOT EXISTS idx_subtasks_task ON subtasks(task_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tool_calls_session ON tool_calls(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tool_calls_subtask ON tool_calls(subtask_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tool_calls_name ON tool_calls(tool_name)`,
		`CREATE INDEX IF NOT EXISTS idx_flow_templates_type ON flow_templates(challenge_type)`,
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("execute migration: %w", err)
		}
	}

	if err := s.ensureColumn("subtasks", "sort_order", "INTEGER DEFAULT 0"); err != nil {
		return err
	}

	return s.seedDefaultFlowTemplates()
}

func (s *SQLiteStore) ensureColumn(table, column, definition string) error {
	rows, err := s.db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return fmt.Errorf("inspect table %s: %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("scan table info %s: %w", table, err)
		}
		if name == column {
			return nil
		}
	}

	if _, err := s.db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, column, definition)); err != nil {
		return fmt.Errorf("add column %s.%s: %w", table, column, err)
	}
	return nil
}

// Session operations

func (s *SQLiteStore) CreateSession(sess *Session) error {
	result, err := s.db.Exec(
		`INSERT INTO sessions (challenge_type, description, target, files, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		sess.ChallengeType, sess.Description, sess.Target, sess.Files,
		sess.Status, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}
	sess.ID = id

	return nil
}

func (s *SQLiteStore) UpdateSession(sess *Session) error {
	_, err := s.db.Exec(
		`UPDATE sessions SET status = ?, flag = ?, iterations = ?, duration_ms = ?, error = ?, completed_at = ?
		 WHERE id = ?`,
		sess.Status, sess.Flag, sess.Iterations, sess.DurationMs, sess.Error, sess.CompletedAt, sess.ID,
	)
	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	return nil
}

func (s *SQLiteStore) GetSession(id int64) (*Session, error) {
	sess := &Session{}
	var flag, error sql.NullString
	var completedAt sql.NullTime

	err := s.db.QueryRow(
		`SELECT id, challenge_type, description, target, files, status, flag, iterations, duration_ms, error, created_at, completed_at
		 FROM sessions WHERE id = ?`, id,
	).Scan(&sess.ID, &sess.ChallengeType, &sess.Description, &sess.Target, &sess.Files,
		&sess.Status, &flag, &sess.Iterations, &sess.DurationMs, &error,
		&sess.CreatedAt, &completedAt)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	if flag.Valid {
		sess.Flag = flag.String
	}
	if error.Valid {
		sess.Error = error.String
	}
	if completedAt.Valid {
		sess.CompletedAt = &completedAt.Time
	}

	return sess, nil
}

func (s *SQLiteStore) ListSessions(limit, offset int) ([]Session, error) {
	rows, err := s.db.Query(
		`SELECT id, challenge_type, description, target, files, status, flag, iterations, duration_ms, error, created_at, completed_at
		 FROM sessions ORDER BY created_at DESC LIMIT ? OFFSET ?`, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	sessions := make([]Session, 0)
	for rows.Next() {
		var sess Session
		var flag, errMsg sql.NullString
		var completedAt sql.NullTime

		if err := rows.Scan(&sess.ID, &sess.ChallengeType, &sess.Description, &sess.Target, &sess.Files,
			&sess.Status, &flag, &sess.Iterations, &sess.DurationMs, &errMsg,
			&sess.CreatedAt, &completedAt); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}

		if flag.Valid {
			sess.Flag = flag.String
		}
		if errMsg.Valid {
			sess.Error = errMsg.String
		}
		if completedAt.Valid {
			sess.CompletedAt = &completedAt.Time
		}

		sessions = append(sessions, sess)
	}

	return sessions, nil
}

// Knowledge operations

func (s *SQLiteStore) CreateKnowledge(k *Knowledge) error {
	result, err := s.db.Exec(
		`INSERT INTO knowledge (session_id, title, content, type, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		k.SessionID, k.Title, k.Content, k.Type, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("insert knowledge: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}
	k.ID = id

	return nil
}

func (s *SQLiteStore) GetKnowledge(id int64) (*Knowledge, error) {
	k := &Knowledge{}
	err := s.db.QueryRow(
		`SELECT id, session_id, title, content, type, created_at
		 FROM knowledge WHERE id = ?`, id,
	).Scan(&k.ID, &k.SessionID, &k.Title, &k.Content, &k.Type, &k.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get knowledge: %w", err)
	}

	return k, nil
}

func (s *SQLiteStore) ListKnowledge(knowledgeType string, limit, offset int) ([]Knowledge, error) {
	query := `SELECT id, session_id, title, content, type, created_at FROM knowledge`
	args := []any{}

	if knowledgeType != "" {
		query += " WHERE type = ?"
		args = append(args, knowledgeType)
	}

	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list knowledge: %w", err)
	}
	defer rows.Close()

	items := make([]Knowledge, 0)
	for rows.Next() {
		var k Knowledge
		if err := rows.Scan(&k.ID, &k.SessionID, &k.Title, &k.Content, &k.Type, &k.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan knowledge: %w", err)
		}
		items = append(items, k)
	}

	return items, nil
}

func (s *SQLiteStore) SearchKnowledge(keyword string, limit int) ([]Knowledge, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, title, content, type, created_at
		 FROM knowledge WHERE title LIKE ? OR content LIKE ?
		 ORDER BY created_at DESC LIMIT ?`,
		"%"+keyword+"%", "%"+keyword+"%", limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search knowledge: %w", err)
	}
	defer rows.Close()

	items := make([]Knowledge, 0)
	for rows.Next() {
		var k Knowledge
		if err := rows.Scan(&k.ID, &k.SessionID, &k.Title, &k.Content, &k.Type, &k.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan knowledge: %w", err)
		}
		items = append(items, k)
	}

	return items, nil
}

// Tag operations

func (s *SQLiteStore) CreateTag(name string) (*Tag, error) {
	result, err := s.db.Exec(`INSERT OR IGNORE INTO tags (name) VALUES (?)`, name)
	if err != nil {
		return nil, fmt.Errorf("insert tag: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get last insert id: %w", err)
	}

	return &Tag{ID: id, Name: name}, nil
}

func (s *SQLiteStore) GetTagsByKnowledge(knowledgeID int64) ([]Tag, error) {
	rows, err := s.db.Query(
		`SELECT t.id, t.name FROM tags t
		 JOIN knowledge_tags kt ON t.id = kt.tag_id
		 WHERE kt.knowledge_id = ?`, knowledgeID,
	)
	if err != nil {
		return nil, fmt.Errorf("get tags: %w", err)
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, t)
	}

	return tags, nil
}

func (s *SQLiteStore) AddTagToKnowledge(knowledgeID, tagID int64) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO knowledge_tags (knowledge_id, tag_id) VALUES (?, ?)`,
		knowledgeID, tagID,
	)
	if err != nil {
		return fmt.Errorf("add tag to knowledge: %w", err)
	}

	return nil
}

func (s *SQLiteStore) ListTags() ([]Tag, error) {
	rows, err := s.db.Query(`SELECT id, name FROM tags ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()

	tags := make([]Tag, 0)
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, t)
	}

	return tags, nil
}

// Conversation operations

func (s *SQLiteStore) AddConversationMessage(msg *ConversationMessage) error {
	result, err := s.db.Exec(
		`INSERT INTO conversation_messages (session_id, role, content, tool_name, tool_input, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		msg.SessionID, msg.Role, msg.Content, msg.ToolName, msg.ToolInput, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("insert message: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}
	msg.ID = id

	return nil
}

func (s *SQLiteStore) GetConversationMessages(sessionID int64) ([]ConversationMessage, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, role, content, tool_name, tool_input, created_at
		 FROM conversation_messages WHERE session_id = ? ORDER BY created_at`, sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}
	defer rows.Close()

	var messages []ConversationMessage
	for rows.Next() {
		var msg ConversationMessage
		if err := rows.Scan(&msg.ID, &msg.SessionID, &msg.Role, &msg.Content, &msg.ToolName, &msg.ToolInput, &msg.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// Subtask operations

func (s *SQLiteStore) CreateSubtask(st *Subtask) error {
	now := time.Now()
	if st.Status == "" {
		st.Status = "created"
	}
	if st.Title == "" {
		st.Title = st.Description
	}
	result, err := s.db.Exec(
		`INSERT INTO subtasks (
			session_id, task_id, parent_id, agent_name, agent_type, challenge_type,
			title, description, target, status, result, error, sort_order, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		st.SessionID, st.TaskID, st.ParentID, st.AgentName, st.AgentType, st.ChallengeType,
		st.Title, st.Description, st.Target, st.Status, st.Result, st.Error, st.SortOrder, now, now,
	)
	if err != nil {
		return fmt.Errorf("insert subtask: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get subtask id: %w", err)
	}
	st.ID = id
	st.CreatedAt = now
	st.UpdatedAt = now
	return nil
}

func (s *SQLiteStore) UpdateSubtask(st *Subtask) error {
	now := time.Now()
	st.UpdatedAt = now
	_, err := s.db.Exec(
		`UPDATE subtasks SET
			status = ?, result = ?, error = ?, updated_at = ?, completed_at = ?
		 WHERE id = ?`,
		st.Status, st.Result, st.Error, now, st.CompletedAt, st.ID,
	)
	if err != nil {
		return fmt.Errorf("update subtask: %w", err)
	}
	return nil
}

func (s *SQLiteStore) UpdateSubtaskPlan(st *Subtask) error {
	now := time.Now()
	st.UpdatedAt = now
	_, err := s.db.Exec(
		`UPDATE subtasks SET
			task_id = ?, parent_id = ?, agent_name = ?, agent_type = ?, challenge_type = ?,
			title = ?, description = ?, target = ?, status = ?, result = ?, error = ?,
			sort_order = ?, updated_at = ?, completed_at = ?
		 WHERE id = ?`,
		st.TaskID, st.ParentID, st.AgentName, st.AgentType, st.ChallengeType,
		st.Title, st.Description, st.Target, st.Status, st.Result, st.Error,
		st.SortOrder, now, st.CompletedAt, st.ID,
	)
	if err != nil {
		return fmt.Errorf("update subtask plan: %w", err)
	}
	return nil
}

func (s *SQLiteStore) DeleteSubtask(id int64) error {
	res, err := s.db.Exec(`DELETE FROM subtasks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete subtask: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("subtask %d not found", id)
	}
	return nil
}

func (s *SQLiteStore) ListSubtasks(sessionID int64, status string) ([]Subtask, error) {
	query := `SELECT id, session_id, task_id, parent_id, agent_name, agent_type, challenge_type,
		title, description, target, status, result, error, sort_order, created_at, updated_at, completed_at
		FROM subtasks WHERE session_id = ?`
	args := []any{sessionID}
	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY sort_order ASC, id ASC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list subtasks: %w", err)
	}
	defer rows.Close()

	var subtasks []Subtask
	for rows.Next() {
		st, err := scanSubtask(rows)
		if err != nil {
			return nil, err
		}
		subtasks = append(subtasks, st)
	}
	return subtasks, nil
}

func (s *SQLiteStore) GetSubtask(id int64) (*Subtask, error) {
	row := s.db.QueryRow(
		`SELECT id, session_id, task_id, parent_id, agent_name, agent_type, challenge_type,
		title, description, target, status, result, error, sort_order, created_at, updated_at, completed_at
		FROM subtasks WHERE id = ?`, id,
	)
	st, err := scanSubtask(row)
	if err != nil {
		return nil, fmt.Errorf("get subtask: %w", err)
	}
	return &st, nil
}

type subtaskScanner interface {
	Scan(dest ...any) error
}

func scanSubtask(scanner subtaskScanner) (Subtask, error) {
	var st Subtask
	var parentID, agentName, agentType, challengeType, title, description, target sql.NullString
	var result, errMsg sql.NullString
	var completedAt sql.NullTime
	if err := scanner.Scan(
		&st.ID, &st.SessionID, &st.TaskID, &parentID, &agentName, &agentType, &challengeType,
		&title, &description, &target, &st.Status, &result, &errMsg, &st.SortOrder,
		&st.CreatedAt, &st.UpdatedAt, &completedAt,
	); err != nil {
		return st, fmt.Errorf("scan subtask: %w", err)
	}
	st.ParentID = parentID.String
	st.AgentName = agentName.String
	st.AgentType = agentType.String
	st.ChallengeType = challengeType.String
	st.Title = title.String
	st.Description = description.String
	st.Target = target.String
	st.Result = result.String
	st.Error = errMsg.String
	if completedAt.Valid {
		st.CompletedAt = &completedAt.Time
	}
	return st, nil
}

// Tool call operations

func (s *SQLiteStore) CreateToolCall(tc *ToolCall) error {
	if tc.Status == "" {
		tc.Status = "received"
	}
	if tc.StartedAt.IsZero() {
		tc.StartedAt = time.Now()
	}
	result, err := s.db.Exec(
		`INSERT INTO tool_calls (
			session_id, subtask_id, task_id, agent_name, agent_type, tool_use_id,
			tool_name, input, output, status, error, started_at, completed_at, duration_ms
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tc.SessionID, tc.SubtaskID, tc.TaskID, tc.AgentName, tc.AgentType, tc.ToolUseID,
		tc.ToolName, tc.Input, tc.Output, tc.Status, tc.Error, tc.StartedAt, tc.CompletedAt, tc.DurationMs,
	)
	if err != nil {
		return fmt.Errorf("insert tool call: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get tool call id: %w", err)
	}
	tc.ID = id
	return nil
}

func (s *SQLiteStore) CompleteToolCall(tc *ToolCall) error {
	if tc.CompletedAt == nil {
		now := time.Now()
		tc.CompletedAt = &now
	}
	if tc.DurationMs == 0 && !tc.StartedAt.IsZero() {
		tc.DurationMs = tc.CompletedAt.Sub(tc.StartedAt).Milliseconds()
	}
	_, err := s.db.Exec(
		`UPDATE tool_calls SET output = ?, status = ?, error = ?, completed_at = ?, duration_ms = ?
		 WHERE id = ?`,
		tc.Output, tc.Status, tc.Error, tc.CompletedAt, tc.DurationMs, tc.ID,
	)
	if err != nil {
		return fmt.Errorf("complete tool call: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListToolCalls(sessionID int64, limit int) ([]ToolCall, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(
		`SELECT id, session_id, subtask_id, task_id, agent_name, agent_type, tool_use_id,
			tool_name, input, output, status, error, started_at, completed_at, duration_ms
		 FROM tool_calls WHERE session_id = ? ORDER BY id ASC LIMIT ?`,
		sessionID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list tool calls: %w", err)
	}
	defer rows.Close()

	var calls []ToolCall
	for rows.Next() {
		tc, err := scanToolCall(rows)
		if err != nil {
			return nil, err
		}
		calls = append(calls, tc)
	}
	return calls, nil
}

func (s *SQLiteStore) GetToolCallStats(sessionID int64) (*ToolCallStats, error) {
	stats := &ToolCallStats{}

	where := ""
	args := []any{}
	if sessionID > 0 {
		where = " WHERE session_id = ?"
		args = append(args, sessionID)
	}

	err := s.db.QueryRow(
		`SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN status = 'finished' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(duration_ms), 0),
			COALESCE(AVG(duration_ms), 0)
		 FROM tool_calls`+where,
		args...,
	).Scan(&stats.TotalCalls, &stats.SuccessCalls, &stats.FailedCalls, &stats.TotalDurationMs, &stats.AvgDurationMs)
	if err != nil {
		return nil, fmt.Errorf("get tool call totals: %w", err)
	}

	byTool, err := s.getToolCallGroupStats("tool_name", where, args...)
	if err != nil {
		return nil, err
	}
	stats.ByTool = byTool

	byAgent, err := s.getToolCallGroupStats("agent_name", where, args...)
	if err != nil {
		return nil, err
	}
	stats.ByAgent = byAgent

	return stats, nil
}

func (s *SQLiteStore) getToolCallGroupStats(column, where string, args ...any) ([]ToolCallGroupStat, error) {
	rows, err := s.db.Query(
		fmt.Sprintf(`SELECT
			COALESCE(NULLIF(%[1]s, ''), 'unknown') AS name,
			COUNT(*) AS total_calls,
			COALESCE(SUM(CASE WHEN status = 'finished' THEN 1 ELSE 0 END), 0) AS success_calls,
			COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0) AS failed_calls,
			COALESCE(SUM(duration_ms), 0) AS total_duration_ms,
			COALESCE(AVG(duration_ms), 0) AS avg_duration_ms,
			MAX(started_at) AS last_used
		 FROM tool_calls%[2]s
		 GROUP BY name
		 ORDER BY total_calls DESC, name ASC`, column, where),
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("get tool call group stats: %w", err)
	}
	defer rows.Close()

	var groups []ToolCallGroupStat
	for rows.Next() {
		var group ToolCallGroupStat
		var lastUsed sql.NullString
		if err := rows.Scan(
			&group.Name,
			&group.TotalCalls,
			&group.SuccessCalls,
			&group.FailedCalls,
			&group.TotalDurationMs,
			&group.AvgDurationMs,
			&lastUsed,
		); err != nil {
			return nil, fmt.Errorf("scan tool call group stats: %w", err)
		}
		if lastUsed.Valid {
			group.LastUsed = parseSQLiteTime(lastUsed.String)
		}
		groups = append(groups, group)
	}
	return groups, nil
}

func parseSQLiteTime(value string) time.Time {
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05",
	} {
		if ts, err := time.Parse(layout, value); err == nil {
			return ts
		}
	}
	return time.Time{}
}

type toolCallScanner interface {
	Scan(dest ...any) error
}

func scanToolCall(scanner toolCallScanner) (ToolCall, error) {
	var tc ToolCall
	var subtaskID sql.NullInt64
	var taskID, agentName, agentType, toolUseID, input, output, status, errMsg sql.NullString
	var completedAt sql.NullTime
	if err := scanner.Scan(
		&tc.ID, &tc.SessionID, &subtaskID, &taskID, &agentName, &agentType, &toolUseID,
		&tc.ToolName, &input, &output, &status, &errMsg, &tc.StartedAt, &completedAt, &tc.DurationMs,
	); err != nil {
		return tc, fmt.Errorf("scan tool call: %w", err)
	}
	if subtaskID.Valid {
		v := subtaskID.Int64
		tc.SubtaskID = &v
	}
	tc.TaskID = taskID.String
	tc.AgentName = agentName.String
	tc.AgentType = agentType.String
	tc.ToolUseID = toolUseID.String
	tc.Input = input.String
	tc.Output = output.String
	tc.Status = status.String
	tc.Error = errMsg.String
	if completedAt.Valid {
		tc.CompletedAt = &completedAt.Time
	}
	return tc, nil
}

// Flow template operations

func (s *SQLiteStore) CreateFlowTemplate(tpl *FlowTemplate) error {
	now := time.Now()
	result, err := s.db.Exec(
		`INSERT INTO flow_templates (challenge_type, title, description, content, tags, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		tpl.ChallengeType, tpl.Title, tpl.Description, tpl.Content, tpl.Tags, now, now,
	)
	if err != nil {
		return fmt.Errorf("insert flow template: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get flow template id: %w", err)
	}
	tpl.ID = id
	tpl.CreatedAt = now
	tpl.UpdatedAt = now
	return nil
}

func (s *SQLiteStore) UpdateFlowTemplate(tpl *FlowTemplate) error {
	now := time.Now()
	_, err := s.db.Exec(
		`UPDATE flow_templates SET challenge_type = ?, title = ?, description = ?, content = ?, tags = ?, updated_at = ?
		 WHERE id = ?`,
		tpl.ChallengeType, tpl.Title, tpl.Description, tpl.Content, tpl.Tags, now, tpl.ID,
	)
	if err != nil {
		return fmt.Errorf("update flow template: %w", err)
	}
	tpl.UpdatedAt = now
	return nil
}

func (s *SQLiteStore) DeleteFlowTemplate(id int64) error {
	res, err := s.db.Exec(`DELETE FROM flow_templates WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete flow template: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("flow template %d not found", id)
	}
	return nil
}

func (s *SQLiteStore) GetFlowTemplate(id int64) (*FlowTemplate, error) {
	row := s.db.QueryRow(
		`SELECT id, challenge_type, title, description, content, tags, created_at, updated_at
		 FROM flow_templates WHERE id = ?`, id,
	)
	tpl, err := scanFlowTemplate(row)
	if err != nil {
		return nil, fmt.Errorf("get flow template: %w", err)
	}
	return &tpl, nil
}

func (s *SQLiteStore) ListFlowTemplates(challengeType string) ([]FlowTemplate, error) {
	query := `SELECT id, challenge_type, title, description, content, tags, created_at, updated_at FROM flow_templates`
	args := []any{}
	if challengeType != "" {
		query += ` WHERE challenge_type = ?`
		args = append(args, challengeType)
	}
	query += ` ORDER BY challenge_type, title`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list flow templates: %w", err)
	}
	defer rows.Close()

	var templates []FlowTemplate
	for rows.Next() {
		tpl, err := scanFlowTemplate(rows)
		if err != nil {
			return nil, err
		}
		templates = append(templates, tpl)
	}
	return templates, nil
}

type flowTemplateScanner interface {
	Scan(dest ...any) error
}

func scanFlowTemplate(scanner flowTemplateScanner) (FlowTemplate, error) {
	var tpl FlowTemplate
	var description, tags sql.NullString
	if err := scanner.Scan(
		&tpl.ID, &tpl.ChallengeType, &tpl.Title, &description, &tpl.Content, &tags,
		&tpl.CreatedAt, &tpl.UpdatedAt,
	); err != nil {
		return tpl, fmt.Errorf("scan flow template: %w", err)
	}
	tpl.Description = description.String
	tpl.Tags = tags.String
	return tpl, nil
}

// Stats

func (s *SQLiteStore) GetStats() (*SessionStats, error) {
	stats := &SessionStats{
		ByType: make(map[string]int),
	}

	err := s.db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&stats.TotalSessions)
	if err != nil {
		return nil, fmt.Errorf("count total: %w", err)
	}

	err = s.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE status = 'success'`).Scan(&stats.SuccessSessions)
	if err != nil {
		return nil, fmt.Errorf("count success: %w", err)
	}

	stats.FailedSessions = stats.TotalSessions - stats.SuccessSessions

	rows, err := s.db.Query(`SELECT challenge_type, COUNT(*) FROM sessions GROUP BY challenge_type`)
	if err != nil {
		return nil, fmt.Errorf("count by type: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var t string
		var c int
		if err := rows.Scan(&t, &c); err != nil {
			return nil, fmt.Errorf("scan type count: %w", err)
		}
		stats.ByType[t] = c
	}

	var avgDuration sql.NullFloat64
	err = s.db.QueryRow(`SELECT AVG(duration_ms) FROM sessions WHERE status = 'success'`).Scan(&avgDuration)
	if err != nil {
		return nil, fmt.Errorf("avg duration: %w", err)
	}
	if avgDuration.Valid {
		stats.AvgDuration = avgDuration.Float64
	}

	return stats, nil
}

// KnowledgeStore implements agent.KnowledgeStore interface.
func (s *SQLiteStore) SearchKnowledgeByType(challengeType string, limit int) ([]Knowledge, error) {
	typeMap := map[string]string{
		"web":     "vulnerability",
		"pwn":     "exploit",
		"crypto":  "technique",
		"reverse": "analysis",
	}
	kType, ok := typeMap[challengeType]
	if !ok {
		return nil, nil
	}

	rows, err := s.db.Query(
		`SELECT id, session_id, title, content, type, created_at
		 FROM knowledge WHERE type = ?
		 ORDER BY created_at DESC LIMIT ?`,
		kType, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search knowledge by type: %w", err)
	}
	defer rows.Close()

	var items []Knowledge
	for rows.Next() {
		var k Knowledge
		if err := rows.Scan(&k.ID, &k.SessionID, &k.Title, &k.Content, &k.Type, &k.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan knowledge: %w", err)
		}
		items = append(items, k)
	}

	return items, nil
}

func (s *SQLiteStore) seedDefaultFlowTemplates() error {
	for _, tpl := range DefaultFlowTemplates() {
		_, err := s.db.Exec(
			`INSERT OR IGNORE INTO flow_templates (challenge_type, title, description, content, tags, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			tpl.ChallengeType, tpl.Title, tpl.Description, tpl.Content, tpl.Tags, time.Now(), time.Now(),
		)
		if err != nil {
			return fmt.Errorf("seed flow template %q: %w", tpl.Title, err)
		}
	}
	return nil
}

func DefaultFlowTemplates() []FlowTemplate {
	return []FlowTemplate{
		{
			ChallengeType: "web",
			Title:         "Web CTF baseline",
			Description:   "PentAGI-inspired ordered checklist for web challenges.",
			Tags:          "web,recon,exploit,report",
			Content: `1. Recon: capture headers, source, robots.txt, sitemap.xml, JavaScript, cookies, and visible routes.
2. Discovery: enumerate paths, parameters, upload points, auth flows, and API schemas.
3. Hypotheses: rank likely bugs such as SQLi, SSRF, path traversal, SSTI, deserialization, command injection, and auth bypass.
4. Exploit: validate one hypothesis at a time with minimal payloads and preserve exact requests.
5. Extraction: read or trigger the flag, then summarize the decisive primitive and replay steps.`,
		},
		{
			ChallengeType: "pwn",
			Title:         "Pwn CTF baseline",
			Description:   "Structured binary exploitation workflow.",
			Tags:          "pwn,binary,exploit,report",
			Content: `1. Triage: run file/checksec, identify architecture, libc, mitigations, and run behavior.
2. Reverse: locate input paths, dangerous calls, heap objects, format strings, and validation branches.
3. Primitive: prove the controllable crash/leak/write and measure exact offsets.
4. Exploit: build the shortest reliable payload, then add bypasses for canary/NX/PIE/RELRO as needed.
5. Extraction: get shell or direct file read and record environment assumptions.`,
		},
		{
			ChallengeType: "crypto",
			Title:         "Crypto CTF baseline",
			Description:   "Transform-chain-first workflow for crypto challenges.",
			Tags:          "crypto,math,decode,report",
			Content: `1. Inventory: collect ciphertexts, keys, nonces, public parameters, source, and service transcripts.
2. Identify: determine encoding layers, primitive family, parameter sizes, randomness reuse, and oracle behavior.
3. Attack: map observed weakness to a concrete attack such as CRT, small exponent, nonce reuse, padding oracle, or lattice.
4. Implement: write a reproducible script with explicit parameters and intermediate assertions.
5. Extraction: decode final plaintext and document the full transform chain.`,
		},
		{
			ChallengeType: "reverse",
			Title:         "Reverse CTF baseline",
			Description:   "Static-to-dynamic reverse engineering workflow.",
			Tags:          "reverse,static,dynamic,report",
			Content: `1. Triage: record file type, strings, imports, packers, sections, entropy, and basic metadata.
2. Locate: find input parsing, comparison, crypto/checksum routines, and success/failure branches.
3. Model: translate the decisive transform or constraints into pseudocode.
4. Validate: run dynamic checks, patch branches, or script the inverse transform.
5. Extraction: recover the flag and save offsets/functions used for replay.`,
		},
	}
}

// Helper

func ToJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}
