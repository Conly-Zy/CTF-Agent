package api

import (
	"context"
	"fmt"
	"sync"
)

// ActiveSession tracks a running solve session with control capabilities.
type ActiveSession struct {
	ID        int64
	Cancel    context.CancelFunc
	Paused    bool
	InjectCh  chan string // channel to inject user messages into the agent
	PauseCh   chan struct{}
	ResumeCh  chan struct{}
	mu        sync.Mutex
}

// SessionManager manages active solve sessions.
type SessionManager struct {
	sessions map[int64]*ActiveSession
	mu       sync.RWMutex
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[int64]*ActiveSession),
	}
}

func (m *SessionManager) Register(id int64, cancel context.CancelFunc) *ActiveSession {
	m.mu.Lock()
	defer m.mu.Unlock()

	s := &ActiveSession{
		ID:       id,
		Cancel:   cancel,
		InjectCh: make(chan string, 10),
		PauseCh:  make(chan struct{}, 1),
		ResumeCh: make(chan struct{}, 1),
	}
	m.sessions[id] = s
	return s
}

func (m *SessionManager) Get(id int64) (*ActiveSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return s, ok
}

func (m *SessionManager) Remove(id int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
}

func (m *SessionManager) Stop(id int64) error {
	s, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("session %d not active", id)
	}
	s.Cancel()
	return nil
}

func (m *SessionManager) Pause(id int64) error {
	s, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("session %d not active", id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Paused {
		return fmt.Errorf("session %d already paused", id)
	}
	s.Paused = true
	s.PauseCh <- struct{}{}
	return nil
}

func (m *SessionManager) Resume(id int64) error {
	s, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("session %d not active", id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.Paused {
		return fmt.Errorf("session %d not paused", id)
	}
	s.Paused = false
	s.ResumeCh <- struct{}{}
	return nil
}

func (m *SessionManager) InjectMessage(id int64, msg string) error {
	s, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("session %d not active", id)
	}
	select {
	case s.InjectCh <- msg:
		return nil
	default:
		return fmt.Errorf("session %d message buffer full", id)
	}
}

func (m *SessionManager) IsActive(id int64) bool {
	_, ok := m.Get(id)
	return ok
}
