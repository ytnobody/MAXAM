// Package heartbeat provides agent health monitoring.
package heartbeat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AgentState represents the health state of an agent.
type AgentState int

const (
	StateUnknown      AgentState = iota
	StateHealthy                 // Last update < 60s
	StateDegraded                // 60s <= last update < 120s
	StateUnresponsive            // Last update >= 120s
)

func (s AgentState) String() string {
	switch s {
	case StateHealthy:
		return "healthy"
	case StateDegraded:
		return "degraded"
	case StateUnresponsive:
		return "unresponsive"
	default:
		return "unknown"
	}
}

// FileMonitorConfig holds file monitor configuration.
type FileMonitorConfig struct {
	BaseDir             string        // Directory to watch (default: ~/.maxam/heartbeat)
	CheckInterval       time.Duration // How often to check (default: 60s)
	DegradedTimeout     time.Duration // Time before degraded state (default: 60s)
	UnresponsiveTimeout time.Duration // Time before unresponsive state (default: 120s)
}

// DefaultFileMonitorConfig returns the default file monitor configuration.
func DefaultFileMonitorConfig() FileMonitorConfig {
	homeDir, _ := os.UserHomeDir()
	return FileMonitorConfig{
		BaseDir:             filepath.Join(homeDir, ".maxam", "heartbeat"),
		CheckInterval:       60 * time.Second,
		DegradedTimeout:     60 * time.Second,
		UnresponsiveTimeout: 120 * time.Second,
	}
}

// AgentHealth represents the current health status of an agent.
type AgentHealth struct {
	AgentName     string
	State         AgentState
	LastHeartbeat time.Time
	LastStatus    string
	PID           int
	SinceLastBeat time.Duration
}

// AlertCallback is called when an agent's state changes.
type AlertCallback func(health AgentHealth, prevState AgentState)

// FileMonitor monitors heartbeat files for all agents.
type FileMonitor struct {
	cfg       FileMonitorConfig
	agents    map[string]*AgentHealth
	mu        sync.RWMutex
	onAlert   AlertCallback
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	running   bool
	runningMu sync.Mutex
	nowFunc   func() time.Time // For testing
}

// NewFileMonitor creates a new file monitor.
func NewFileMonitor(cfg FileMonitorConfig) *FileMonitor {
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = DefaultFileMonitorConfig().CheckInterval
	}
	if cfg.DegradedTimeout <= 0 {
		cfg.DegradedTimeout = DefaultFileMonitorConfig().DegradedTimeout
	}
	if cfg.UnresponsiveTimeout <= 0 {
		cfg.UnresponsiveTimeout = DefaultFileMonitorConfig().UnresponsiveTimeout
	}
	if cfg.BaseDir == "" {
		cfg.BaseDir = DefaultFileMonitorConfig().BaseDir
	}

	return &FileMonitor{
		cfg:     cfg,
		agents:  make(map[string]*AgentHealth),
		nowFunc: time.Now,
	}
}

// SetAlertCallback sets the callback for state changes.
func (m *FileMonitor) SetAlertCallback(fn AlertCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onAlert = fn
}

// SetNowFunc sets the time function (for testing).
func (m *FileMonitor) SetNowFunc(fn func() time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nowFunc = fn
}

// Start begins the monitoring loop.
func (m *FileMonitor) Start() error {
	m.runningMu.Lock()
	if m.running {
		m.runningMu.Unlock()
		return nil
	}
	m.running = true
	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.runningMu.Unlock()

	// Ensure directory exists
	if err := os.MkdirAll(m.cfg.BaseDir, 0755); err != nil {
		return err
	}

	// Initial check
	m.checkAll()

	m.wg.Add(1)
	go m.monitorLoop()

	return nil
}

// Stop stops the monitoring loop.
func (m *FileMonitor) Stop() {
	m.runningMu.Lock()
	if !m.running {
		m.runningMu.Unlock()
		return
	}
	m.running = false
	m.cancel()
	m.runningMu.Unlock()
	m.wg.Wait()
}

// monitorLoop runs periodic health checks.
func (m *FileMonitor) monitorLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.cfg.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkAll()
		}
	}
}

// checkAll scans all heartbeat files and updates agent states.
func (m *FileMonitor) checkAll() {
	files, err := filepath.Glob(filepath.Join(m.cfg.BaseDir, "*.json"))
	if err != nil {
		return
	}

	now := m.nowFunc()

	for _, file := range files {
		m.checkFile(file, now)
	}
}

// checkFile reads a single heartbeat file and updates state.
func (m *FileMonitor) checkFile(filePath string, now time.Time) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return
	}

	var hb HeartbeatData
	if err := json.Unmarshal(data, &hb); err != nil {
		return
	}

	sinceLastBeat := now.Sub(hb.Timestamp)
	newState := m.determineState(sinceLastBeat)

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.agents[hb.AgentName]
	var prevState AgentState
	if exists {
		prevState = existing.State
	} else {
		prevState = StateUnknown
	}

	health := &AgentHealth{
		AgentName:     hb.AgentName,
		State:         newState,
		LastHeartbeat: hb.Timestamp,
		LastStatus:    hb.Status,
		PID:           hb.PID,
		SinceLastBeat: sinceLastBeat,
	}
	m.agents[hb.AgentName] = health

	// Trigger alert if state changed (and got worse)
	if newState != prevState && newState > StateHealthy && m.onAlert != nil {
		go m.onAlert(*health, prevState)
	}
}

// determineState determines the agent state based on time since last heartbeat.
func (m *FileMonitor) determineState(sinceLastBeat time.Duration) AgentState {
	if sinceLastBeat >= m.cfg.UnresponsiveTimeout {
		return StateUnresponsive
	}
	if sinceLastBeat >= m.cfg.DegradedTimeout {
		return StateDegraded
	}
	return StateHealthy
}

// GetAgentHealth returns the health of a specific agent.
func (m *FileMonitor) GetAgentHealth(agentName string) (AgentHealth, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	health, exists := m.agents[agentName]
	if !exists {
		return AgentHealth{}, false
	}
	return *health, true
}

// GetAllAgentHealth returns health status of all known agents.
func (m *FileMonitor) GetAllAgentHealth() map[string]AgentHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]AgentHealth, len(m.agents))
	for name, health := range m.agents {
		result[name] = *health
	}
	return result
}

// GetUnresponsiveAgents returns names of unresponsive agents.
func (m *FileMonitor) GetUnresponsiveAgents() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []string
	for name, health := range m.agents {
		if health.State == StateUnresponsive {
			result = append(result, name)
		}
	}
	return result
}

// GetDegradedAgents returns names of degraded agents.
func (m *FileMonitor) GetDegradedAgents() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []string
	for name, health := range m.agents {
		if health.State == StateDegraded {
			result = append(result, name)
		}
	}
	return result
}

// FormatAlert formats an alert message for an agent.
func FormatAlert(health AgentHealth) string {
	return fmt.Sprintf("[WARN] Agent '%s' is %s (last heartbeat: %s ago)",
		health.AgentName,
		health.State.String(),
		formatDuration(health.SinceLastBeat),
	)
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	if seconds == 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dm%ds", minutes, seconds)
}

// CheckOnce performs a single check (for external use).
func (m *FileMonitor) CheckOnce() {
	m.checkAll()
}
