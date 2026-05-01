package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/Conly-Zy/CTF-Agent/internal/agent"
	"github.com/Conly-Zy/CTF-Agent/internal/config"
	"github.com/Conly-Zy/CTF-Agent/internal/knowledge"
	"github.com/Conly-Zy/CTF-Agent/internal/store"
	"github.com/Conly-Zy/CTF-Agent/internal/tools"
	"github.com/gorilla/websocket"
)

type Server struct {
	store        *store.SQLiteStore
	logger       *slog.Logger
	mux          *http.ServeMux
	addr         string
	orchestrator *agent.Orchestrator
	extractor    *knowledge.Extractor
	registry     *tools.Registry
	cfg          *config.Config
	uploadDir    string

	// Embedded frontend
	frontendFS fs.FS
	spaFS      fs.FS // sub-filesystem for SPA dist

	// Session management
	activeSessions *SessionManager

	// WebSocket clients
	wsClients map[*websocket.Conn]bool
	wsMu      sync.RWMutex
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func NewServer(store *store.SQLiteStore, logger *slog.Logger, addr string) *Server {
	uploadDir := filepath.Join(os.TempDir(), "ctf-agent-uploads")
	os.MkdirAll(uploadDir, 0755)

	s := &Server{
		store:          store,
		logger:         logger,
		mux:            http.NewServeMux(),
		addr:           addr,
		extractor:      knowledge.NewExtractor(store),
		activeSessions: NewSessionManager(),
		wsClients:      make(map[*websocket.Conn]bool),
		uploadDir:      uploadDir,
	}

	s.routes()
	return s
}

func (s *Server) SetOrchestrator(o *agent.Orchestrator) {
	s.orchestrator = o
}

func (s *Server) SetRegistry(r *tools.Registry) {
	s.registry = r
}

func (s *Server) SetConfig(cfg *config.Config) {
	s.cfg = cfg
}

func (s *Server) SetFrontendFS(fsys fs.FS) {
	s.frontendFS = fsys
	s.routes() // re-register routes with frontend
}

func (s *Server) routes() {
	s.mux = http.NewServeMux()

	// API routes
	s.mux.HandleFunc("GET /api/dashboard", s.handleDashboard)
	s.mux.HandleFunc("GET /api/sessions", s.handleListSessions)
	s.mux.HandleFunc("GET /api/sessions/{id}", s.handleGetSession)
	s.mux.HandleFunc("GET /api/sessions/{id}/messages", s.handleGetSessionMessages)
	s.mux.HandleFunc("POST /api/solve", s.handleSolve)
	s.mux.HandleFunc("POST /api/upload", s.handleUpload)
	s.mux.HandleFunc("POST /api/sessions/{id}/stop", s.handleStopSession)
	s.mux.HandleFunc("POST /api/sessions/{id}/pause", s.handlePauseSession)
	s.mux.HandleFunc("POST /api/sessions/{id}/resume", s.handleResumeSession)
	s.mux.HandleFunc("POST /api/sessions/{id}/inject", s.handleInjectMessage)
	s.mux.HandleFunc("GET /api/sessions/{id}/export", s.handleExportSession)
	s.mux.HandleFunc("GET /api/tools", s.handleListTools)
	s.mux.HandleFunc("POST /api/tools/{name}/test", s.handleTestTool)
	s.mux.HandleFunc("GET /api/knowledge", s.handleListKnowledge)
	s.mux.HandleFunc("GET /api/knowledge/{id}", s.handleGetKnowledge)
	s.mux.HandleFunc("GET /api/knowledge/search", s.handleSearchKnowledge)
	s.mux.HandleFunc("GET /api/tags", s.handleListTags)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
	s.mux.HandleFunc("GET /api/config", s.handleGetConfig)
	s.mux.HandleFunc("PUT /api/config", s.handleUpdateConfig)

	// WebSocket
	s.mux.HandleFunc("GET /ws", s.handleWebSocket)

	if s.frontendFS != nil {
		// Serve embedded frontend with SPA fallback
		// The embed root is cmd/ctf-agent/web_dist/. Use fs.Sub to get its contents.
		webFS, err := fs.Sub(s.frontendFS, "web_dist")
		if err != nil {
			s.logger.Error("failed to load frontend fs", "error", err)
		} else {
			s.spaFS = webFS
		}
	} else {
		// Dev mode: serve from filesystem
		s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
		s.mux.HandleFunc("GET /{$}", s.handleIndex)
		s.mux.HandleFunc("GET /index.html", s.handleIndex)
	}
}

func (s *Server) Start() error {
	s.logger.Info("starting HTTP server", "addr", s.addr)
	if s.spaFS != nil {
		return http.ListenAndServe(s.addr, s.spaHandler())
	}
	return http.ListenAndServe(s.addr, s.mux)
}

// spaHandler serves the embedded SPA frontend. API/WS routes go to the mux,
// everything else is served from the embedded filesystem with index.html fallback.
func (s *Server) spaHandler() http.Handler {
	fileServer := http.FileServer(http.FS(s.spaFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// API and WebSocket routes go to the mux
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/ws" {
			s.mux.ServeHTTP(w, r)
			return
		}

		// Try to serve static file from embedded FS
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if f, err := s.spaFS.Open(path); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback: serve index.html
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}

// WebSocket handler
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("websocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	s.wsMu.Lock()
	s.wsClients[conn] = true
	s.wsMu.Unlock()

	defer func() {
		s.wsMu.Lock()
		delete(s.wsClients, conn)
		s.wsMu.Unlock()
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (s *Server) broadcast(msg map[string]any) {
	data, _ := json.Marshal(msg)

	s.wsMu.RLock()
	defer s.wsMu.RUnlock()

	for conn := range s.wsClients {
		conn.WriteMessage(websocket.TextMessage, data)
	}
}

// Handlers
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/index.html")
}

func (s *Server) handleSolve(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChallengeType string   `json:"challenge_type"`
		Description   string   `json:"description"`
		Target        string   `json:"target"`
		Files         []string `json:"files"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}

	// Create session
	sess := &store.Session{
		ChallengeType: req.ChallengeType,
		Description:   req.Description,
		Target:        req.Target,
		Files:         store.ToJSON(req.Files),
		Status:        "solving",
	}

	if err := s.store.CreateSession(sess); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	// Start solving in background
	go s.solveAsync(sess, req)

	s.writeJSON(w, map[string]any{
		"session_id": sess.ID,
		"status":     "solving",
	})
}

func (s *Server) solveAsync(sess *store.Session, req struct {
	ChallengeType string   `json:"challenge_type"`
	Description   string   `json:"description"`
	Target        string   `json:"target"`
	Files         []string `json:"files"`
}) {
	s.broadcast(map[string]any{
		"type":    "log",
		"level":   "info",
		"message": fmt.Sprintf("会话 #%d 开始解题", sess.ID),
	})

	if s.orchestrator == nil {
		sess.Status = "failed"
		sess.Error = "Orchestrator not configured"
		s.store.UpdateSession(sess)
		s.broadcast(map[string]any{"type": "complete", "success": false, "error": "Orchestrator not configured"})
		return
	}

	// Create cancellable context and register with session manager
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	active := s.activeSessions.Register(sess.ID, cancel)
	defer s.activeSessions.Remove(sess.ID)

	controls := &agent.SessionControls{
		PauseCh:  active.PauseCh,
		ResumeCh: active.ResumeCh,
		InjectCh: active.InjectCh,
	}

	// Create a logger that broadcasts to WebSocket
	wsLogger := &wsLogger{server: s}

	result, err := s.orchestrator.SolveWithControls(
		ctx,
		agent.SolveRequest{
			ChallengeType: req.ChallengeType,
			Description:   req.Description,
			Target:        req.Target,
			Files:         req.Files,
		},
		wsLogger,
		controls,
	)

	if err != nil {
		sess.Status = "failed"
		sess.Error = err.Error()
		s.store.UpdateSession(sess)
		s.broadcast(map[string]any{"type": "complete", "success": false, "error": err.Error()})
		return
	}

	sess.Status = "success"
	sess.Flag = result.Flag
	sess.Iterations = result.Iterations
	completedAt := result.CompletedAt
	sess.CompletedAt = &completedAt
	s.store.UpdateSession(sess)

	// Extract knowledge
	messages, _ := s.store.GetConversationMessages(sess.ID)
	s.extractor.ExtractFromSession(sess, messages)

	s.broadcast(map[string]any{
		"type":       "complete",
		"success":    true,
		"flag":       result.Flag,
		"iterations": result.Iterations,
		"duration":   result.Duration.String(),
	})
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.GetStats()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	sessions, err := s.store.ListSessions(10, 0)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.writeJSON(w, map[string]any{
		"stats":    stats,
		"sessions": sessions,
	})
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit <= 0 {
		limit = 20
	}

	sessions, err := s.store.ListSessions(limit, offset)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.writeJSON(w, sessions)
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("invalid id"))
		return
	}

	session, err := s.store.GetSession(id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, fmt.Errorf("session not found"))
		return
	}

	s.writeJSON(w, session)
}

func (s *Server) handleGetSessionMessages(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("invalid id"))
		return
	}

	messages, err := s.store.GetConversationMessages(id)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.writeJSON(w, messages)
}

// Session control handlers

func (s *Server) handleStopSession(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("invalid id"))
		return
	}

	if err := s.activeSessions.Stop(id); err != nil {
		s.writeError(w, http.StatusNotFound, err)
		return
	}

	s.broadcast(map[string]any{"type": "log", "level": "info", "message": fmt.Sprintf("会话 #%d 已停止", id)})
	s.writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handlePauseSession(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("invalid id"))
		return
	}

	if err := s.activeSessions.Pause(id); err != nil {
		s.writeError(w, http.StatusConflict, err)
		return
	}

	s.broadcast(map[string]any{"type": "log", "level": "info", "message": fmt.Sprintf("会话 #%d 已暂停", id)})
	s.writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleResumeSession(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("invalid id"))
		return
	}

	if err := s.activeSessions.Resume(id); err != nil {
		s.writeError(w, http.StatusConflict, err)
		return
	}

	s.broadcast(map[string]any{"type": "log", "level": "info", "message": fmt.Sprintf("会话 #%d 已恢复", id)})
	s.writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleInjectMessage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("invalid id"))
		return
	}

	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Message == "" {
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("message is required"))
		return
	}

	if err := s.activeSessions.InjectMessage(id, req.Message); err != nil {
		s.writeError(w, http.StatusConflict, err)
		return
	}

	s.broadcast(map[string]any{"type": "log", "level": "info", "message": fmt.Sprintf("会话 #%d 收到手动消息", id)})
	s.writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleExportSession(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("invalid id"))
		return
	}

	session, err := s.store.GetSession(id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, fmt.Errorf("session not found"))
		return
	}

	messages, err := s.store.GetConversationMessages(id)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "markdown"
	}

	switch format {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"session-%d.json\"", id))
		json.NewEncoder(w).Encode(map[string]any{
			"session":  session,
			"messages": messages,
		})
	case "markdown":
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"session-%d.md\"", id))
		fmt.Fprintf(w, "# CTF 解题记录 #%d\n\n", id)
		fmt.Fprintf(w, "- **类型**: %s\n", session.ChallengeType)
		fmt.Fprintf(w, "- **状态**: %s\n", session.Status)
		fmt.Fprintf(w, "- **迭代次数**: %d\n", session.Iterations)
		if session.Flag != "" {
			fmt.Fprintf(w, "- **Flag**: `%s`\n", session.Flag)
		}
		fmt.Fprintf(w, "- **创建时间**: %s\n\n", session.CreatedAt.Format("2006-01-02 15:04:05"))
		if session.Description != "" {
			fmt.Fprintf(w, "## 题目描述\n\n%s\n\n", session.Description)
		}
		if session.Target != "" {
			fmt.Fprintf(w, "## 目标\n\n%s\n\n", session.Target)
		}
		fmt.Fprintf(w, "## 对话记录\n\n")
		for _, msg := range messages {
			role := "用户"
			if msg.Role == "assistant" {
				role = "AI"
			}
			fmt.Fprintf(w, "### %s\n\n", role)
			if msg.ToolName != "" {
				fmt.Fprintf(w, "**工具调用**: `%s`\n\n", msg.ToolName)
			}
			content := msg.Content
			if content == "" && msg.ToolInput != "" {
				content = msg.ToolInput
			}
			if content != "" {
				fmt.Fprintf(w, "%s\n\n", content)
			}
			fmt.Fprintf(w, "---\n\n")
		}
	default:
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("unsupported format: %s", format))
	}
}

func (s *Server) handleListKnowledge(w http.ResponseWriter, r *http.Request) {
	knowledgeType := r.URL.Query().Get("type")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit <= 0 {
		limit = 20
	}

	items, err := s.store.ListKnowledge(knowledgeType, limit, offset)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.writeJSON(w, items)
}

func (s *Server) handleGetKnowledge(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("invalid id"))
		return
	}

	knowledge, err := s.store.GetKnowledge(id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, fmt.Errorf("knowledge not found"))
		return
	}

	tags, _ := s.store.GetTagsByKnowledge(id)

	s.writeJSON(w, map[string]any{
		"knowledge": knowledge,
		"tags":      tags,
	})
}

func (s *Server) handleSearchKnowledge(w http.ResponseWriter, r *http.Request) {
	keyword := r.URL.Query().Get("q")
	if keyword == "" {
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("search query required"))
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}

	items, err := s.store.SearchKnowledge(keyword, limit)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.writeJSON(w, items)
}

func (s *Server) handleListTags(w http.ResponseWriter, r *http.Request) {
	tags, err := s.store.ListTags()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.writeJSON(w, tags)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.GetStats()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	s.writeJSON(w, stats)
}

// Config handlers
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	if s.cfg == nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Errorf("config not available"))
		return
	}
	s.cfg.SyncTimeoutSec()
	s.writeJSON(w, s.cfg)
}

func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	if s.cfg == nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Errorf("config not available"))
		return
	}

	var incoming config.Config
	if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}

	// Update config in place
	s.cfg.Anthropic = incoming.Anthropic
	s.cfg.Agent = incoming.Agent
	s.cfg.Sandbox = incoming.Sandbox
	s.cfg.Flag = incoming.Flag
	s.cfg.Submit = incoming.Submit

	if err := s.cfg.Save(); err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Errorf("save config: %w", err))
		return
	}

	s.writeJSON(w, map[string]any{"ok": true})
}

// Tools handlers
func (s *Server) handleListTools(w http.ResponseWriter, r *http.Request) {
	if s.registry == nil {
		s.writeJSON(w, []any{})
		return
	}

	tools := s.registry.ToClaudeTools()
	s.writeJSON(w, tools)
}

func (s *Server) handleTestTool(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if s.registry == nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Errorf("registry not available"))
		return
	}

	tool, ok := s.registry.Get(name)
	if !ok {
		s.writeError(w, http.StatusNotFound, fmt.Errorf("tool not found: %s", name))
		return
	}

	var req struct {
		Input json.RawMessage `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}

	output, err := tool.Execute(r.Context(), req.Input)
	if err != nil {
		s.writeJSON(w, map[string]any{
			"output": "",
			"error":  err.Error(),
		})
		return
	}

	s.writeJSON(w, map[string]any{
		"output": output,
		"error":  "",
	})
}

// File upload handler
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20) // 32 MB max

	file, handler, err := r.FormFile("file")
	if err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("missing file"))
		return
	}
	defer file.Close()

	// Create session-specific directory
	sessionID := r.FormValue("session_id")
	var destDir string
	if sessionID != "" {
		destDir = filepath.Join(s.uploadDir, sessionID)
	} else {
		destDir = filepath.Join(s.uploadDir, "temp")
	}
	os.MkdirAll(destDir, 0755)

	// Save file
	destPath := filepath.Join(destDir, filepath.Base(handler.Filename))
	dst, err := os.Create(destPath)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Errorf("create file: %w", err))
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Errorf("save file: %w", err))
		return
	}

	s.logger.Info("file uploaded", "name", handler.Filename, "size", handler.Size, "path", destPath)
	s.writeJSON(w, map[string]any{
		"path": destPath,
		"name": handler.Filename,
		"size": handler.Size,
	})
}

func (s *Server) writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (s *Server) writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

// WebSocket logger
type wsLogger struct {
	server *Server
}

func (l *wsLogger) Log(level, message string) {
	l.server.broadcast(map[string]any{
		"type":    "log",
		"level":   level,
		"message": message,
	})
}

func (l *wsLogger) ToolStart(tool string) {
	l.server.broadcast(map[string]any{
		"type": "tool_start",
		"tool": tool,
	})
}

func (l *wsLogger) ToolResult(tool, result string) {
	l.server.broadcast(map[string]any{
		"type":   "tool_result",
		"tool":   tool,
		"result": result,
	})
}

func (l *wsLogger) Thinking(content string) {
	l.server.broadcast(map[string]any{
		"type":    "thinking",
		"content": content,
	})
}

func (l *wsLogger) Flag(flag string) {
	l.server.broadcast(map[string]any{
		"type": "flag",
		"flag": flag,
	})
}
