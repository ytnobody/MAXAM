package healthmgr

import (
	"context"
	"sync"
	"time"
)

// Monitor runs periodic health checks for all registered agents.
type Monitor struct {
	manager  *Manager
	interval time.Duration
	checker  HealthChecker

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
}

// HealthChecker defines the interface for checking agent health.
type HealthChecker interface {
	// CheckHealth checks if an agent is healthy.
	// Returns an error if the agent is unresponsive or unhealthy.
	CheckHealth(ctx context.Context, agentName string) error
}

// NewMonitor creates a new health monitor.
func NewMonitor(manager *Manager, interval time.Duration, checker HealthChecker) *Monitor {
	return &Monitor{
		manager:  manager,
		interval: interval,
		checker:  checker,
	}
}

// Start begins the monitoring loop.
// Returns immediately; monitoring runs in a goroutine.
func (m *Monitor) Start(ctx context.Context) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true

	monitorCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.mu.Unlock()

	go m.run(monitorCtx)
}

// Stop stops the monitoring loop.
func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	if m.cancel != nil {
		m.cancel()
	}
	m.running = false
}

// IsRunning returns whether the monitor is currently running.
func (m *Monitor) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// run is the main monitoring loop.
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
		case <-ticker.C:
			m.checkAll(ctx)
		}
	}
}

// checkAll checks the health of all registered agents.
func (m *Monitor) checkAll(ctx context.Context) {
	agents := m.manager.GetAllHealth()

	for agentName := range agents {
		select {
		case <-ctx.Done():
			return
		default:
			m.checkAgent(ctx, agentName)
		}
	}
}

// checkAgent checks the health of a single agent and triggers recovery if needed.
func (m *Monitor) checkAgent(ctx context.Context, agentName string) {
	// Skip if no checker configured.
	if m.checker == nil {
		return
	}

	err := m.checker.CheckHealth(ctx, agentName)
	if err != nil {
		// Update health status to unresponsive.
		m.manager.UpdateHealth(agentName, HealthStatusUnresponsive, err)

		// Attempt recovery.
		m.manager.RecoverAgent(ctx, agentName)
	} else {
		// Agent is healthy.
		m.manager.UpdateHealth(agentName, HealthStatusHealthy, nil)
	}
}
