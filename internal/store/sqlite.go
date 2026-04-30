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
		`CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_type ON sessions(challenge_type)`,
		`CREATE INDEX IF NOT EXISTS idx_knowledge_session ON knowledge(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_knowledge_type ON knowledge(type)`,
		`CREATE INDEX IF NOT EXISTS idx_conversation_session ON conversation_messages(session_id)`,
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("execute migration: %w", err)
		}
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

	var sessions []Session
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

// Helper

func ToJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}
