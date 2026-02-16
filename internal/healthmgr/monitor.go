package healthmgr

import (
	"context"
	"sync"
	"time"
)

// HealthChecker defines the interface for checking agent health.
type HealthChecker interface {
	// CheckHealth checks the health of an agent.
	// Returns nil if healthy, error otherwise.
	CheckHealth(ctx context.Context, agentName string) error
}

// Monitor runs periodic health checks for all registered agents.
type Monitor struct {
	mu            sync.RWMutex
	manager       *Manager
	checker       HealthChecker
	interval      time.Duration
	running       bool
	stopCh        chan struct{}
	onRecovery    func(result RecoveryResult)
	checkTimeout  time.Duration
	failThreshold int
}

// MonitorOption is a functional option for Monitor.
type MonitorOption func(*Monitor)

// NewMonitor creates a new health monitor.
func NewMonitor(manager *Manager, checker HealthChecker, opts ...MonitorOption) *Monitor {
	m := &Monitor{
		manager:       manager,
		checker:       checker,
		interval:      30 * time.Second,
		checkTimeout:  10 * time.Second,
		failThreshold: 3,
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// WithMonitorInterval sets the monitoring interval.
func WithMonitorInterval(d time.Duration) MonitorOption {
	return func(m *Monitor) {
		m.interval = d
	}
}

// WithCheckTimeout sets the timeout for each health check.
func WithCheckTimeout(d time.Duration) MonitorOption {
	return func(m *Monitor) {
		m.checkTimeout = d
	}
}

// WithFailThreshold sets the number of consecutive failures before triggering recovery.
func WithFailThreshold(n int) MonitorOption {
	return func(m *Monitor) {
		m.failThreshold = n
	}
}

// WithOnRecovery sets a callback for recovery events.
func WithOnRecovery(fn func(result RecoveryResult)) MonitorOption {
	return func(m *Monitor) {
		m.onRecovery = fn
	}
}

// Start begins the health monitoring loop.
func (m *Monitor) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return nil
	}
	m.running = true
	m.stopCh = make(chan struct{})
	m.mu.Unlock()

	go m.run(ctx)
	return nil
}

// Stop stops the health monitoring loop.
func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	close(m.stopCh)
	m.running = false
}

// IsRunning returns whether the monitor is running.
func (m *Monitor) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

func (m *Monitor) run(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	// Run initial check immediately.
	m.checkAll(ctx)

	for {
		select {
		case <-ctx.Done():
			m.mu.Lock()
			m.running = false
			m.mu.Unlock()
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkAll(ctx)
		}
	}
}

func (m *Monitor) checkAll(ctx context.Context) {
	agents := m.manager.GetAllHealth()

	for agentName := range agents {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		default:
			m.checkAgent(ctx, agentName)
		}
	}
}

func (m *Monitor) checkAgent(ctx context.Context, agentName string) {
	checkCtx, cancel := context.WithTimeout(ctx, m.checkTimeout)
	defer cancel()

	err := m.checker.CheckHealth(checkCtx, agentName)

	if err != nil {
		m.manager.UpdateHealth(agentName, HealthStatusUnresponsive, err)

		// Check if we should trigger recovery.
		health := m.manager.GetHealth(agentName)
		if health != nil && health.FailCount >= m.failThreshold {
			m.triggerRecovery(ctx, agentName)
		}
	} else {
		m.manager.UpdateHealth(agentName, HealthStatusHealthy, nil)
	}
}

func (m *Monitor) triggerRecovery(ctx context.Context, agentName string) {
	result := m.manager.RecoverAgent(ctx, agentName)

	if m.onRecovery != nil {
		m.onRecovery(result)
	}
}

// CheckNow triggers an immediate health check for all agents.
func (m *Monitor) CheckNow(ctx context.Context) {
	m.checkAll(ctx)
}

// CheckAgentNow triggers an immediate health check for a specific agent.
func (m *Monitor) CheckAgentNow(ctx context.Context, agentName string) {
	m.checkAgent(ctx, agentName)
}
