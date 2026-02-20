// Package timeout provides agent timeout detection and reporting.
package timeout

import (
	"context"
	"sync"
	"time"
)

// DefaultCheckInterval is the default interval for checking timeouts.
const DefaultCheckInterval = 30 * time.Second

// RecoveryHandler handles agent recovery after timeout.
type RecoveryHandler interface {
	// RestartAgent restarts the specified agent.
	RestartAgent(ctx context.Context, agentName string) error
}

// ReportOutput handles timeout report output.
type ReportOutput interface {
	// SendReport sends the timeout report to the chat.
	SendReport(ctx context.Context, report Report) error
}

// Config holds monitor configuration.
type Config struct {
	Timeout       time.Duration // Timeout duration (default: 3 minutes)
	CheckInterval time.Duration // Check interval (default: 30 seconds)
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Timeout:       DefaultTimeout,
		CheckInterval: DefaultCheckInterval,
	}
}

// Monitor watches agent activity and triggers recovery on timeout.
type Monitor struct {
	tracker *Tracker
	handler RecoveryHandler
	output  ReportOutput
	cfg     Config
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool
	runMu   sync.Mutex
}

// NewMonitor creates a new timeout monitor.
func NewMonitor(cfg Config, handler RecoveryHandler, output ReportOutput) *Monitor {
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultTimeout
	}
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = DefaultCheckInterval
	}

	m := &Monitor{
		handler: handler,
		output:  output,
		cfg:     cfg,
	}

	// Create tracker with callback to handle timeouts
	m.tracker = NewTracker(cfg.Timeout, m.handleTimeout)

	return m
}

// Start begins the timeout monitoring loop.
func (m *Monitor) Start() {
	m.runMu.Lock()
	if m.running {
		m.runMu.Unlock()
		return
	}
	m.running = true
	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.runMu.Unlock()

	m.wg.Add(1)
	go m.monitorLoop()
}

// Stop stops the monitoring loop.
func (m *Monitor) Stop() {
	m.runMu.Lock()
	if !m.running {
		m.runMu.Unlock()
		return
	}
	m.running = false
	m.cancel()
	m.runMu.Unlock()

	m.wg.Wait()
}

// monitorLoop periodically checks for timeouts.
func (m *Monitor) monitorLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.cfg.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.tracker.CheckTimeouts()
		}
	}
}

// handleTimeout is called when an agent times out.
func (m *Monitor) handleTimeout(report Report) {
	ctx := m.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	// 1. Output the timeout report to chat
	if m.output != nil {
		_ = m.output.SendReport(ctx, report)
	}

	// 2. Restart the agent
	if m.handler != nil {
		_ = m.handler.RestartAgent(ctx, report.AgentName)
	}

	// 3. Reset the activity timer after restart
	m.tracker.RecordHeartbeat(report.AgentName)
}

// RecordActivity records an agent's activity.
func (m *Monitor) RecordActivity(agentName string, ctx AgentContext) {
	m.tracker.RecordActivity(agentName, ctx)
}

// RecordHeartbeat records a heartbeat without changing context.
func (m *Monitor) RecordHeartbeat(agentName string) {
	m.tracker.RecordHeartbeat(agentName)
}

// UpdateProgress updates the progress field.
func (m *Monitor) UpdateProgress(agentName string, progress string) {
	m.tracker.UpdateProgress(agentName, progress)
}

// GetActivity returns the current activity for an agent.
func (m *Monitor) GetActivity(agentName string) (Activity, bool) {
	return m.tracker.GetActivity(agentName)
}

// GetTimedOutAgents returns agents that have exceeded the timeout.
func (m *Monitor) GetTimedOutAgents() []string {
	return m.tracker.GetTimedOutAgents()
}

// RemoveAgent removes an agent from tracking.
func (m *Monitor) RemoveAgent(agentName string) {
	m.tracker.RemoveAgent(agentName)
}

// IsRunning returns true if the monitor is running.
func (m *Monitor) IsRunning() bool {
	m.runMu.Lock()
	defer m.runMu.Unlock()
	return m.running
}
