package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Conly-Zy/CTF-Agent/internal/store"
)

type Server struct {
	store  *store.SQLiteStore
	logger *slog.Logger
	mux    *http.ServeMux
	addr   string
}

func NewServer(store *store.SQLiteStore, logger *slog.Logger, addr string) *Server {
	s := &Server{
		store:  store,
		logger: logger,
		mux:    http.NewServeMux(),
		addr:   addr,
	}

	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/dashboard", s.handleDashboard)
	s.mux.HandleFunc("GET /api/sessions", s.handleListSessions)
	s.mux.HandleFunc("GET /api/sessions/{id}", s.handleGetSession)
	s.mux.HandleFunc("GET /api/sessions/{id}/messages", s.handleGetSessionMessages)
	s.mux.HandleFunc("GET /api/knowledge", s.handleListKnowledge)
	s.mux.HandleFunc("GET /api/knowledge/{id}", s.handleGetKnowledge)
	s.mux.HandleFunc("GET /api/knowledge/search", s.handleSearchKnowledge)
	s.mux.HandleFunc("GET /api/tags", s.handleListTags)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)

	s.mux.HandleFunc("GET /", s.handleIndex)
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
}

func (s *Server) Start() error {
	s.logger.Info("starting HTTP server", "addr", s.addr)
	return http.ListenAndServe(s.addr, s.mux)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/index.html")
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
