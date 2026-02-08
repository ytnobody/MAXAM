// Package heartbeat provides agent health monitoring.
package heartbeat

import (
	"context"
	"sync"
	"time"
)

// Config holds heartbeat monitoring configuration.
type Config struct {
	Interval   time.Duration // Monitoring interval (default: 10s)
	Timeout    time.Duration // Response timeout (default: 30s)
	MaxRetries int           // Max restart attempts (default: 3)
}

// DefaultConfig returns the default heartbeat configuration.
func DefaultConfig() Config {
	return Config{
		Interval:   10 * time.Second,
		Timeout:    30 * time.Second,
		MaxRetries: 3,
	}
}

// Status represents the health status of a worker.
type Status int

const (
	StatusUnknown Status = iota
	StatusHealthy
	StatusUnresponsive
	StatusDead
)

func (s Status) String() string {
	switch s {
	case StatusHealthy:
		return "healthy"
	case StatusUnresponsive:
		return "unresponsive"
	case StatusDead:
		return "dead"
	default:
		return "unknown"
	}
}

// WorkerHealth tracks the health state of a worker.
type WorkerHealth struct {
	Name         string
	Status       Status
	LastPing     time.Time
	LastResponse time.Time
	FailCount    int
}

// PingFunc is called to send a ping to a worker.
// It should return nil if the worker responds, error otherwise.
type PingFunc func(ctx context.Context, workerName string) error

// UnresponsiveCallback is called when a worker becomes unresponsive.
type UnresponsiveCallback func(workerName string, failCount int)

// Monitor watches worker health using heartbeat pings.
type Monitor struct {
	cfg       Config
	workers   map[string]*WorkerHealth
	mu        sync.RWMutex
	pingFn    PingFunc
	onUnresp  UnresponsiveCallback
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	running   bool
	runningMu sync.Mutex
}

// NewMonitor creates a new heartbeat monitor.
func NewMonitor(cfg Config, pingFn PingFunc) *Monitor {
	if cfg.Interval <= 0 {
		cfg.Interval = DefaultConfig().Interval
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultConfig().Timeout
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = DefaultConfig().MaxRetries
	}

	return &Monitor{
		cfg:     cfg,
		workers: make(map[string]*WorkerHealth),
		pingFn:  pingFn,
	}
}

// SetUnresponsiveCallback sets the callback for unresponsive workers.
func (m *Monitor) SetUnresponsiveCallback(fn UnresponsiveCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onUnresp = fn
}

// RegisterWorker adds a worker to monitoring.
func (m *Monitor) RegisterWorker(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.workers[name] = &WorkerHealth{
		Name:   name,
		Status: StatusUnknown,
	}
}

// UnregisterWorker removes a worker from monitoring.
func (m *Monitor) UnregisterWorker(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.workers, name)
}

// Start begins the monitoring loop.
func (m *Monitor) Start() {
	m.runningMu.Lock()
	if m.running {
		m.runningMu.Unlock()
		return
	}
	m.running = true
	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.runningMu.Unlock()

	m.wg.Add(1)
	go m.monitorLoop()
}

// Stop stops the monitoring loop.
func (m *Monitor) Stop() {
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
func (m *Monitor) monitorLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.cfg.Interval)
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

// checkAll pings all registered workers.
func (m *Monitor) checkAll() {
	m.mu.RLock()
	names := make([]string, 0, len(m.workers))
	for name := range m.workers {
		names = append(names, name)
	}
	m.mu.RUnlock()

	for _, name := range names {
		m.checkWorker(name)
	}
}

// checkWorker pings a single worker and updates its health.
func (m *Monitor) checkWorker(name string) {
	m.mu.RLock()
	health, exists := m.workers[name]
	m.mu.RUnlock()
	if !exists {
		return
	}

	// Use background context if monitor not started
	parentCtx := m.ctx
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, cancel := context.WithTimeout(parentCtx, m.cfg.Timeout)
	defer cancel()

	now := time.Now()

	m.mu.Lock()
	health.LastPing = now
	m.mu.Unlock()

	err := m.pingFn(ctx, name)

	m.mu.Lock()
	defer m.mu.Unlock()

	if err != nil {
		health.FailCount++
		if health.FailCount >= m.cfg.MaxRetries {
			health.Status = StatusDead
		} else {
			health.Status = StatusUnresponsive
		}

		// Trigger callback
		if m.onUnresp != nil {
			go m.onUnresp(name, health.FailCount)
		}
	} else {
		health.Status = StatusHealthy
		health.LastResponse = time.Now()
		health.FailCount = 0
	}
}

// GetHealth returns the current health of a worker.
func (m *Monitor) GetHealth(name string) (WorkerHealth, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	health, exists := m.workers[name]
	if !exists {
		return WorkerHealth{}, false
	}
	return *health, true
}

// GetAllHealth returns health status of all workers.
func (m *Monitor) GetAllHealth() map[string]WorkerHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]WorkerHealth, len(m.workers))
	for name, health := range m.workers {
		result[name] = *health
	}
	return result
}

// ResetFailCount resets the failure count for a worker.
func (m *Monitor) ResetFailCount(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if health, exists := m.workers[name]; exists {
		health.FailCount = 0
		health.Status = StatusUnknown
	}
}

// IsHealthy returns true if the worker is healthy.
func (m *Monitor) IsHealthy(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if health, exists := m.workers[name]; exists {
		return health.Status == StatusHealthy
	}
	return false
}

// GetUnresponsiveWorkers returns names of unresponsive or dead workers.
func (m *Monitor) GetUnresponsiveWorkers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []string
	for name, health := range m.workers {
		if health.Status == StatusUnresponsive || health.Status == StatusDead {
			result = append(result, name)
		}
	}
	return result
}
