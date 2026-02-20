// Package session provides session management with timeout and auto-restart.
// This manages the overall execution time of an agent session, separate from
// individual task timeouts handled by Runner/Worker.
package session

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	// DefaultTimeout is the default session timeout (3 minutes)
	DefaultTimeout = 3 * time.Minute

	// DefaultMaxRestarts is the default maximum number of automatic restarts
	// 0 means unlimited restarts
	DefaultMaxRestarts = 0
)

// StatusReporter is called when session times out to report current status
type StatusReporter func(ctx context.Context) string

// OnTimeoutFunc is called when session times out before restart
type OnTimeoutFunc func(ctx context.Context, status string)

// Config holds session configuration
type Config struct {
	// Timeout is the maximum continuous execution time before forced restart
	Timeout time.Duration

	// MaxRestarts is the maximum number of automatic restarts (0 = unlimited)
	MaxRestarts int

	// StatusReporter returns the current work status for handover
	StatusReporter StatusReporter

	// OnTimeout is called when session times out (for logging/notification)
	OnTimeout OnTimeoutFunc
}

// DefaultConfig returns default session configuration
func DefaultConfig() Config {
	return Config{
		Timeout:     DefaultTimeout,
		MaxRestarts: DefaultMaxRestarts,
	}
}

// Session manages an agent's continuous execution with timeout
type Session struct {
	config Config

	// Current session state
	mu           sync.RWMutex
	restartCount int
	running      bool
	startTime    time.Time

	// Control channels
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new session with the given configuration
func New(cfg Config) *Session {
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Session{
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Run executes the provided function with session timeout management.
// The function is automatically restarted when it times out.
// Returns when the function completes without timeout, max restarts reached,
// or the session is stopped.
func (s *Session) Run(fn func(ctx context.Context) error) error {
	s.mu.Lock()
	s.running = true
	s.startTime = time.Now()
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	for {
		// Check if we've exceeded max restarts
		if s.config.MaxRestarts > 0 && s.restartCount >= s.config.MaxRestarts {
			return fmt.Errorf("max restarts (%d) exceeded", s.config.MaxRestarts)
		}

		// Create timeout context for this iteration
		iterCtx, iterCancel := context.WithTimeout(s.ctx, s.config.Timeout)

		// Run the function
		errCh := make(chan error, 1)
		go func() {
			errCh <- fn(iterCtx)
		}()

		select {
		case err := <-errCh:
			iterCancel()
			if err != nil {
				return err
			}
			// Function completed successfully
			return nil

		case <-iterCtx.Done():
			iterCancel()
			// Check if it's our parent context that was cancelled
			if s.ctx.Err() != nil {
				return s.ctx.Err()
			}

			// Timeout occurred - handle restart
			s.mu.Lock()
			s.restartCount++
			s.mu.Unlock()

			// Get current status if reporter is configured
			status := ""
			if s.config.StatusReporter != nil {
				status = s.config.StatusReporter(context.Background())
			}

			// Call timeout callback if configured
			if s.config.OnTimeout != nil {
				s.config.OnTimeout(context.Background(), status)
			}

			// Continue to next iteration (automatic restart)
		}
	}
}

// Stop gracefully stops the session
func (s *Session) Stop() {
	s.cancel()
}

// RestartCount returns the number of times the session has been restarted
func (s *Session) RestartCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.restartCount
}

// IsRunning returns whether the session is currently running
func (s *Session) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Elapsed returns the time since the session started
func (s *Session) Elapsed() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.startTime.IsZero() {
		return 0
	}
	return time.Since(s.startTime)
}

// Manager coordinates multiple sessions
type Manager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewManager creates a new session manager
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
	}
}

// Create creates and registers a new session
func (m *Manager) Create(name string, cfg Config) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess := New(cfg)
	m.sessions[name] = sess
	return sess
}

// Get returns a session by name
func (m *Manager) Get(name string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sess, ok := m.sessions[name]
	return sess, ok
}

// Stop stops a specific session
func (m *Manager) Stop(name string) bool {
	m.mu.RLock()
	sess, ok := m.sessions[name]
	m.mu.RUnlock()
	if ok {
		sess.Stop()
	}
	return ok
}

// StopAll stops all sessions
func (m *Manager) StopAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, sess := range m.sessions {
		sess.Stop()
	}
}

// Status returns status of all sessions
func (m *Manager) Status() map[string]SessionStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]SessionStatus)
	for name, sess := range m.sessions {
		status[name] = SessionStatus{
			Running:      sess.IsRunning(),
			RestartCount: sess.RestartCount(),
			Elapsed:      sess.Elapsed(),
		}
	}
	return status
}

// SessionStatus represents the status of a session
type SessionStatus struct {
	Running      bool
	RestartCount int
	Elapsed      time.Duration
}
