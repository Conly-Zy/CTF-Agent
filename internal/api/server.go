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
	"time"

	"github.com/Conly-Zy/CTF-Agent/internal/agent"
	"github.com/Conly-Zy/CTF-Agent/internal/alerting"
	"github.com/Conly-Zy/CTF-Agent/internal/auth"
	"github.com/Conly-Zy/CTF-Agent/internal/cache"
	"github.com/Conly-Zy/CTF-Agent/internal/config"
	"github.com/Conly-Zy/CTF-Agent/internal/health"
	"github.com/Conly-Zy/CTF-Agent/internal/knowledge"
	"github.com/Conly-Zy/CTF-Agent/internal/logging"
	"github.com/Conly-Zy/CTF-Agent/internal/metrics"
	"github.com/Conly-Zy/CTF-Agent/internal/plugin"
	"github.com/Conly-Zy/CTF-Agent/internal/report"
	"github.com/Conly-Zy/CTF-Agent/internal/replay"
	"github.com/Conly-Zy/CTF-Agent/internal/store"
	"github.com/Conly-Zy/CTF-Agent/internal/taskqueue"
	"github.com/Conly-Zy/CTF-Agent/internal/tools"
	"github.com/gorilla/websocket"
)

type Server struct {
	store        *store.SQLiteStore
	logger       *slog.Logger
	mux          *http.ServeMux
	addr         string
	orchestrator *agent.Orchestrator
	primaryAgent *agent.PrimaryAgent
	extractor    *knowledge.Extractor
	registry     *tools.Registry
	cfg          *config.Config
	uploadDir    string

	// Embedded frontend
	frontendFS fs.FS
	spaFS      fs.FS // sub-filesystem for SPA dist

	// Session management
	activeSessions *SessionManager

	// Metrics
	metricsCollector *metrics.MetricsCollector

	// Infrastructure
	authenticator  *auth.Authenticator
	rateLimiter    *auth.RateLimiter
	cache          *cache.MemoryCache
	healthChecker  *health.HealthChecker
	alertManager   *alerting.AlertManager
	logAggregator  *logging.LogAggregator

	// New features
	taskQueue      *taskqueue.TaskQueue
	pluginLoader   *plugin.PluginLoader

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
		store:            store,
		logger:           logger,
		mux:              http.NewServeMux(),
		addr:             addr,
		extractor:        knowledge.NewExtractor(store),
		activeSessions:   NewSessionManager(),
		metricsCollector: metrics.NewMetricsCollector(),
		wsClients:        make(map[*websocket.Conn]bool),
		uploadDir:        uploadDir,
	}

	s.initInfrastructure()
	s.routes()
	return s
}

func (s *Server) initInfrastructure() {
	s.authenticator = auth.NewAuthenticator("", false)
	s.rateLimiter = auth.NewRateLimiter(10, 20)
	s.cache = cache.NewMemoryCache(1000, 30*time.Minute)
	s.healthChecker = health.NewHealthChecker(s.logger, 5*time.Second)
	s.alertManager = alerting.NewAlertManager(s.logger)
	s.logAggregator = logging.NewLogAggregator(s.logger, 10000, "/tmp/ctf-agent-logs.json")
	s.taskQueue = taskqueue.NewTaskQueue("/tmp/ctf-agent-tasks.json", s.logger)
	s.pluginLoader = plugin.NewPluginLoader(nil, "/tmp/ctf-agent-plugins", s.logger)

	// Register default health checks
	for name, check := range health.DefaultChecks() {
		s.healthChecker.Register(name, check)
	}

	// Start alert manager
	s.alertManager.Start()
	s.logAggregator.Start()
}

func (s *Server) SetOrchestrator(o *agent.Orchestrator) {
	s.orchestrator = o
}

func (s *Server) SetPrimaryAgent(pa *agent.PrimaryAgent) {
	s.primaryAgent = pa
}

func (s *Server) SetRegistry(r *tools.Registry) {
	s.registry = r
	if s.pluginLoader != nil {
		s.pluginLoader.SetRegistry(r)
	}
}

func (s *Server) SetConfig(cfg *config.Config) {
	s.cfg = cfg
	if cfg != nil && cfg.Auth.Enabled {
		s.authenticator = auth.NewAuthenticator(cfg.Auth.APIKey, true)
		if cfg.Auth.RateLimit > 0 {
			s.rateLimiter = auth.NewRateLimiter(cfg.Auth.RateLimit, cfg.Auth.RateBurst)
		}
	}
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

	// Metrics and Health
	s.mux.HandleFunc("GET /api/metrics", s.handleMetrics)
	s.mux.HandleFunc("GET /api/metrics/agents", s.handleAgentMetrics)
	s.mux.HandleFunc("GET /api/metrics/tools", s.handleToolMetrics)
	s.mux.HandleFunc("GET /api/metrics/llm", s.handleLLMMetrics)
	s.mux.HandleFunc("GET /api/metrics/system", s.handleSystemMetrics)
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/health/ready", s.handleReadiness)
	s.mux.HandleFunc("GET /api/health/live", s.handleLiveness)

	// Auth management
	s.mux.HandleFunc("POST /api/auth/keys", s.handleCreateAPIKey)
	s.mux.HandleFunc("GET /api/auth/keys", s.handleListAPIKeys)
	s.mux.HandleFunc("DELETE /api/auth/keys/{key}", s.handleDeleteAPIKey)

	// Cache management
	s.mux.HandleFunc("GET /api/cache/stats", s.handleCacheStats)
	s.mux.HandleFunc("DELETE /api/cache", s.handleCacheClear)

	// Alerting
	s.mux.HandleFunc("GET /api/alerts", s.handleListAlerts)
	s.mux.HandleFunc("GET /api/alerts/active", s.handleActiveAlerts)
	s.mux.HandleFunc("POST /api/alerts/{id}/resolve", s.handleResolveAlert)
	s.mux.HandleFunc("GET /api/alerts/stats", s.handleAlertStats)

	// Logs
	s.mux.HandleFunc("GET /api/logs", s.handleListLogs)
	s.mux.HandleFunc("GET /api/logs/search", s.handleSearchLogs)

	// Task Queue
	s.mux.HandleFunc("GET /api/tasks", s.handleListTasks)
	s.mux.HandleFunc("POST /api/tasks", s.handleEnqueueTask)
	s.mux.HandleFunc("POST /api/tasks/{id}/cancel", s.handleCancelTask)
	s.mux.HandleFunc("GET /api/tasks/stats", s.handleTaskStats)

	// Session Replay
	s.mux.HandleFunc("GET /api/sessions/{id}/replay", s.handleGetReplay)

	// Report
	s.mux.HandleFunc("GET /api/sessions/{id}/report", s.handleGetReport)

	// Plugins
	s.mux.HandleFunc("GET /api/plugins", s.handleListPlugins)
	s.mux.HandleFunc("POST /api/plugins/reload", s.handleReloadPlugins)

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

	var handler http.Handler
	if s.spaFS != nil {
		handler = s.spaHandler()
	} else {
		handler = s.mux
	}

	// Apply middleware chain: CORS -> RateLimit -> Auth -> Handler
	corsMiddleware := auth.CORSMiddleware([]string{"*"})
	handler = corsMiddleware(handler)
	handler = s.rateLimiter.RateLimitMiddleware(handler)
	handler = s.authenticator.AuthMiddleware(handler)

	return http.ListenAndServe(s.addr, handler)
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

	// 优先使用 PrimaryAgent，回退到 Orchestrator
	if s.primaryAgent != nil {
		s.solveWithPrimaryAgent(sess, req)
	} else if s.orchestrator != nil {
		s.solveWithOrchestrator(sess, req)
	} else {
		sess.Status = "failed"
		sess.Error = "No agent configured"
		s.store.UpdateSession(sess)
		s.broadcast(map[string]any{"type": "complete", "success": false, "error": "No agent configured"})
	}
}

func (s *Server) solveWithPrimaryAgent(sess *store.Session, req struct {
	ChallengeType string   `json:"challenge_type"`
	Description   string   `json:"description"`
	Target        string   `json:"target"`
	Files         []string `json:"files"`
}) {
	// Create cancellable context and register with session manager
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.activeSessions.Register(sess.ID, cancel)
	defer s.activeSessions.Remove(sess.ID)

	// 创建任务
	task := agent.Task{
		ID:          fmt.Sprintf("session-%d", sess.ID),
		Type:        req.ChallengeType,
		Description: req.Description,
		Target:      req.Target,
		Files:       req.Files,
	}

	// 执行任务
	result, err := s.primaryAgent.Run(ctx, task)

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
	completedAt := time.Now()
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

func (s *Server) solveWithOrchestrator(sess *store.Session, req struct {
	ChallengeType string   `json:"challenge_type"`
	Description   string   `json:"description"`
	Target        string   `json:"target"`
	Files         []string `json:"files"`
}) {
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

func (l *wsLogger) AgentStart(agentName string) {
	l.server.broadcast(map[string]any{
		"type":       "agent_start",
		"agent_name": agentName,
	})
}

func (l *wsLogger) AgentComplete(agentName string, success bool) {
	l.server.broadcast(map[string]any{
		"type":       "agent_complete",
		"agent_name": agentName,
		"success":    success,
	})
}

func (l *wsLogger) TaskAssigned(taskID, agentName string) {
	l.server.broadcast(map[string]any{
		"type":       "task_assigned",
		"task_id":    taskID,
		"agent_name": agentName,
	})
}

// Metrics handlers
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	summary := s.metricsCollector.GetSummary()
	s.writeJSON(w, summary)
}

func (s *Server) handleAgentMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := s.metricsCollector.GetAllAgentMetrics()
	s.writeJSON(w, metrics)
}

func (s *Server) handleToolMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := s.metricsCollector.GetAllToolMetrics()
	s.writeJSON(w, metrics)
}

func (s *Server) handleLLMMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := s.metricsCollector.GetLLMMetrics()
	s.writeJSON(w, metrics)
}

func (s *Server) handleSystemMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := s.metricsCollector.GetSystemMetrics()
	s.writeJSON(w, metrics)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	report := s.healthChecker.Check(r.Context())
	s.writeJSON(w, map[string]any{
		"status":    string(report.Status),
		"checks":    report.Checks,
		"timestamp": report.Timestamp.Unix(),
		"duration":  report.Duration.String(),
		"version":   "1.0.0",
	})
}

func (s *Server) handleReadiness(w http.ResponseWriter, r *http.Request) {
	// Readiness is based on store availability
	ready := s.store != nil
	if !ready {
		w.WriteHeader(http.StatusServiceUnavailable)
		s.writeJSON(w, map[string]any{"ready": false})
		return
	}
	s.writeJSON(w, map[string]any{"ready": true})
}

func (s *Server) handleLiveness(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, map[string]any{"alive": true, "timestamp": time.Now().Unix()})
}

// Auth handlers
func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string `json:"name"`
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Name == "" {
		req.Name = "default"
	}
	if req.UserID == "" {
		req.UserID = auth.GetUserID(r.Context())
	}

	key, err := s.authenticator.GenerateAPIKey(req.Name, req.UserID, 0)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, key)
}

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	keys := s.authenticator.ListAPIKeys(userID)
	s.writeJSON(w, keys)
}

func (s *Server) handleDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if err := s.authenticator.DeleteAPIKey(key); err != nil {
		s.writeError(w, http.StatusNotFound, err)
		return
	}
	s.writeJSON(w, map[string]any{"ok": true})
}

// Cache handlers
func (s *Server) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, s.cache.Stats())
}

func (s *Server) handleCacheClear(w http.ResponseWriter, r *http.Request) {
	s.cache.Clear()
	s.writeJSON(w, map[string]any{"ok": true})
}

// Alert handlers
func (s *Server) handleListAlerts(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	alerts := s.alertManager.GetAllAlerts(limit)
	s.writeJSON(w, alerts)
}

func (s *Server) handleActiveAlerts(w http.ResponseWriter, r *http.Request) {
	alerts := s.alertManager.GetActiveAlerts()
	s.writeJSON(w, alerts)
}

func (s *Server) handleResolveAlert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.alertManager.Resolve(id); err != nil {
		s.writeError(w, http.StatusNotFound, err)
		return
	}
	s.writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleAlertStats(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, s.alertManager.GetStats())
}

// Log handlers
func (s *Server) handleListLogs(w http.ResponseWriter, r *http.Request) {
	level := r.URL.Query().Get("level")
	sessionID := r.URL.Query().Get("session_id")
	agentName := r.URL.Query().Get("agent")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 100
	}

	filter := logging.LogFilter{
		Level:     level,
		SessionID: sessionID,
		Agent:     agentName,
		Limit:     limit,
	}
	logs := s.logAggregator.Query(filter)
	s.writeJSON(w, logs)
}

func (s *Server) handleSearchLogs(w http.ResponseWriter, r *http.Request) {
	keyword := r.URL.Query().Get("q")
	if keyword == "" {
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("search query required"))
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}

	logs := s.logAggregator.Search(keyword, limit)
	s.writeJSON(w, logs)
}

// Task Queue handlers
func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	status := taskqueue.TaskStatus(r.URL.Query().Get("status"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	tasks := s.taskQueue.List(status, limit)
	s.writeJSON(w, tasks)
}

func (s *Server) handleEnqueueTask(w http.ResponseWriter, r *http.Request) {
	var task taskqueue.QueuedTask
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if task.MaxRetries == 0 {
		task.MaxRetries = 3
	}
	if err := s.taskQueue.Enqueue(&task); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, task)
}

func (s *Server) handleCancelTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.taskQueue.Cancel(id); err != nil {
		s.writeError(w, http.StatusNotFound, err)
		return
	}
	s.writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleTaskStats(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, s.taskQueue.Stats())
}

// Session Replay handler
func (s *Server) handleGetReplay(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("invalid id"))
		return
	}

	replayPath := filepath.Join("/tmp/ctf-agent-replays", fmt.Sprintf("session-%d.json", id))
	rp, err := replay.LoadReplay(replayPath)
	if err != nil {
		s.writeError(w, http.StatusNotFound, fmt.Errorf("replay not found for session %d", id))
		return
	}

	s.writeJSON(w, rp)
}

// Report handler
func (s *Server) handleGetReport(w http.ResponseWriter, r *http.Request) {
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

	gen := report.NewReportGenerator()

	// Try to load replay for richer report
	replayPath := filepath.Join("/tmp/ctf-agent-replays", fmt.Sprintf("session-%d.json", id))
	rp, err := replay.LoadReplay(replayPath)

	var rpt *report.Report
	if err == nil {
		rpt = gen.GenerateFromReplay(session, rp)
	} else {
		rpt = gen.GenerateFromSession(session)
	}

	format := r.URL.Query().Get("format")
	if format == "markdown" {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Write([]byte(gen.RenderMarkdown(rpt)))
		return
	}

	s.writeJSON(w, rpt)
}

// Plugin handlers
func (s *Server) handleListPlugins(w http.ResponseWriter, r *http.Request) {
	plugins := s.pluginLoader.ListPlugins()
	s.writeJSON(w, plugins)
}

func (s *Server) handleReloadPlugins(w http.ResponseWriter, r *http.Request) {
	if err := s.pluginLoader.Reload(); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, map[string]any{"ok": true, "count": len(s.pluginLoader.ListPlugins())})
}
