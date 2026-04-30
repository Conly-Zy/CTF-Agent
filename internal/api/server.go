package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"

	"github.com/Conly-Zy/CTF-Agent/internal/agent"
	"github.com/Conly-Zy/CTF-Agent/internal/knowledge"
	"github.com/Conly-Zy/CTF-Agent/internal/store"
	"github.com/Conly-Zy/CTF-Agent/internal/tools"
	"github.com/gorilla/websocket"
)

type Server struct {
	store       *store.SQLiteStore
	logger      *slog.Logger
	mux         *http.ServeMux
	addr        string
	orchestrator *agent.Orchestrator
	extractor   *knowledge.Extractor
	registry    *tools.Registry

	// WebSocket clients
	wsClients map[*websocket.Conn]bool
	wsMu      sync.RWMutex
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func NewServer(store *store.SQLiteStore, logger *slog.Logger, addr string) *Server {
	s := &Server{
		store:     store,
		logger:    logger,
		mux:       http.NewServeMux(),
		addr:      addr,
		extractor: knowledge.NewExtractor(store),
		wsClients: make(map[*websocket.Conn]bool),
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

func (s *Server) routes() {
	// API routes
	s.mux.HandleFunc("GET /api/dashboard", s.handleDashboard)
	s.mux.HandleFunc("GET /api/sessions", s.handleListSessions)
	s.mux.HandleFunc("GET /api/sessions/{id}", s.handleGetSession)
	s.mux.HandleFunc("GET /api/sessions/{id}/messages", s.handleGetSessionMessages)
	s.mux.HandleFunc("POST /api/solve", s.handleSolve)
	s.mux.HandleFunc("GET /api/knowledge", s.handleListKnowledge)
	s.mux.HandleFunc("GET /api/knowledge/{id}", s.handleGetKnowledge)
	s.mux.HandleFunc("GET /api/knowledge/search", s.handleSearchKnowledge)
	s.mux.HandleFunc("GET /api/tags", s.handleListTags)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)

	// WebSocket
	s.mux.HandleFunc("/ws", s.handleWebSocket)

	// Static files
	s.mux.HandleFunc("GET /", s.handleIndex)
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
}

func (s *Server) Start() error {
	s.logger.Info("starting HTTP server", "addr", s.addr)
	return http.ListenAndServe(s.addr, s.mux)
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
		"message": fmt.Sprintf("Starting session #%d", sess.ID),
	})

	if s.orchestrator == nil {
		sess.Status = "failed"
		sess.Error = "Orchestrator not configured"
		s.store.UpdateSession(sess)
		s.broadcast(map[string]any{"type": "complete", "success": false, "error": "Orchestrator not configured"})
		return
	}

	// Create a logger that broadcasts to WebSocket
	wsLogger := &wsLogger{server: s}

	result, err := s.orchestrator.SolveWithCallback(
		agent.SolveRequest{
			ChallengeType: req.ChallengeType,
			Description:   req.Description,
			Target:        req.Target,
			Files:         req.Files,
		},
		wsLogger,
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
