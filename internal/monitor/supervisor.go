// Package monitor provides unified monitoring for agents and issues.
package monitor

import (
	"context"
	"sync"
	"time"

	"github.com/ytnobody/MAXAM/internal/healthmgr"
	"github.com/ytnobody/MAXAM/internal/issuemgr"
)

// AgentStatus represents the current status of an agent.
type AgentStatus struct {
	Name      string
	Status    healthmgr.HealthStatus
	LastCheck time.Time
	FailCount int
}

// Status represents the overall monitoring status.
type Status struct {
	Agents      []AgentStatus
	OpenIssues  int
	LastUpdated time.Time
}

// StatusCallback is called when status changes.
type StatusCallback func(status Status)

// Supervisor coordinates health and issue monitoring.
type Supervisor struct {
	mu sync.RWMutex

	healthManager *healthmgr.Manager
	healthMonitor *healthmgr.Monitor
	issueWatcher  *issuemgr.Watcher

	status   Status
	onChange StatusCallback
	running  bool
	stopCh   chan struct{}
}

// SupervisorOption is a functional option for Supervisor.
type SupervisorOption func(*Supervisor)

// NewSupervisor creates a new monitoring supervisor.
func NewSupervisor(opts ...SupervisorOption) *Supervisor {
	s := &Supervisor{
		status: Status{
			Agents:      make([]AgentStatus, 0),
			LastUpdated: time.Now(),
		},
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// WithHealthManager sets the health manager for agent monitoring.
func WithHealthManager(mgr *healthmgr.Manager) SupervisorOption {
	return func(s *Supervisor) {
		s.healthManager = mgr
	}
}

// WithHealthMonitor sets the health monitor.
func WithHealthMonitor(mon *healthmgr.Monitor) SupervisorOption {
	return func(s *Supervisor) {
		s.healthMonitor = mon
	}
}

// WithIssueWatcher sets the issue watcher.
func WithIssueWatcher(w *issuemgr.Watcher) SupervisorOption {
	return func(s *Supervisor) {
		s.issueWatcher = w
	}
}

// WithOnChange sets the callback for status changes.
func WithOnChange(fn StatusCallback) SupervisorOption {
	return func(s *Supervisor) {
		s.onChange = fn
	}
}

// Start begins the monitoring supervision.
func (s *Supervisor) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	// Start sub-monitors if configured.
	if s.healthMonitor != nil {
		if err := s.healthMonitor.Start(ctx); err != nil {
			return err
		}
	}

	if s.issueWatcher != nil {
		if err := s.issueWatcher.Start(ctx); err != nil {
			return err
		}
	}

	// Start status aggregation loop.
	go s.run(ctx)

	return nil
}

// Stop stops all monitoring.
func (s *Supervisor) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	close(s.stopCh)
	s.running = false

	if s.healthMonitor != nil {
		s.healthMonitor.Stop()
	}

	if s.issueWatcher != nil {
		s.issueWatcher.Stop()
	}
}

// IsRunning returns whether the supervisor is running.
func (s *Supervisor) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetStatus returns the current monitoring status.
func (s *Supervisor) GetStatus() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

func (s *Supervisor) run(ctx context.Context) {
	// Aggregate status every 5 seconds.
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Initial aggregation.
	s.aggregate()

	for {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.aggregate()
		}
	}
}

func (s *Supervisor) aggregate() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Aggregate agent health status.
	if s.healthManager != nil {
		healthMap := s.healthManager.GetAllHealth()
		agents := make([]AgentStatus, 0, len(healthMap))

		for name, health := range healthMap {
			agents = append(agents, AgentStatus{
				Name:      name,
				Status:    health.Status,
				LastCheck: health.LastCheckedAt,
				FailCount: health.FailCount,
			})
		}

		s.status.Agents = agents
	}

	s.status.LastUpdated = time.Now()

	// Notify callback if set.
	if s.onChange != nil {
		// Make a copy to avoid holding lock during callback.
		statusCopy := s.status
		s.mu.Unlock()
		s.onChange(statusCopy)
		s.mu.Lock()
	}
}

// UpdateOpenIssues updates the open issue count.
// This is called by the issue watcher callback.
func (s *Supervisor) UpdateOpenIssues(count int) {
	s.mu.Lock()
	s.status.OpenIssues = count
	s.status.LastUpdated = time.Now()
	s.mu.Unlock()

	if s.onChange != nil {
		s.onChange(s.GetStatus())
	}
}
