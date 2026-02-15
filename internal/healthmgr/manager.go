// Package healthmgr provides centralized health monitoring and recovery coordination for agents.
package healthmgr

import (
	"context"
	"sync"
	"time"
)

// Manager coordinates health monitoring and recovery for agents.
type Manager struct {
	mu              sync.RWMutex
	health          map[string]*Health
	recoveryLock    map[string]*sync.Mutex
	maxRetries      int
	retryDelay      time.Duration
	taskTimeout     time.Duration
	autoRestart     bool
	recoveryHandler RecoveryHandler
}

// RecoveryHandler defines the interface for handling agent recovery.
type RecoveryHandler interface {
	// StartAgent starts or restarts an agent.
	StartAgent(ctx context.Context, agentName string) error
	// StopAgent stops an agent.
	StopAgent(ctx context.Context, agentName string) error
}

// NewManager creates a new health manager.
func NewManager(opts ...Option) *Manager {
	m := &Manager{
		health:       make(map[string]*Health),
		recoveryLock: make(map[string]*sync.Mutex),
		maxRetries:   3,
		retryDelay:   5 * time.Second,
		taskTimeout:  5 * time.Minute,
		autoRestart:  true,
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// Option is a functional option for Manager.
type Option func(*Manager)

// WithMaxRetries sets the maximum retry attempts.
func WithMaxRetries(n int) Option {
	return func(m *Manager) {
		m.maxRetries = n
	}
}

// WithRetryDelay sets the delay between retry attempts.
func WithRetryDelay(d time.Duration) Option {
	return func(m *Manager) {
		m.retryDelay = d
	}
}

// WithTaskTimeout sets the timeout for hanging tasks.
func WithTaskTimeout(d time.Duration) Option {
	return func(m *Manager) {
		m.taskTimeout = d
	}
}

// WithAutoRestart enables/disables automatic restart.
func WithAutoRestart(enabled bool) Option {
	return func(m *Manager) {
		m.autoRestart = enabled
	}
}

// WithRecoveryHandler sets the recovery handler.
func WithRecoveryHandler(h RecoveryHandler) Option {
	return func(m *Manager) {
		m.recoveryHandler = h
	}
}

// RegisterAgent registers an agent for health monitoring.
func (m *Manager) RegisterAgent(agentName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.health[agentName]; !exists {
		m.health[agentName] = &Health{
			AgentName:      agentName,
			Status:         HealthStatusHealthy,
			LastCheckedAt:  time.Now(),
			LastResponseAt: time.Now(),
		}
		m.recoveryLock[agentName] = &sync.Mutex{}
	}
}

// UpdateHealth updates the health status of an agent.
func (m *Manager) UpdateHealth(agentName string, status HealthStatus, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	h, exists := m.health[agentName]
	if !exists {
		h = &Health{AgentName: agentName}
		m.health[agentName] = h
		m.recoveryLock[agentName] = &sync.Mutex{}
	}

	h.Status = status
	h.LastCheckedAt = time.Now()

	if status == HealthStatusHealthy {
		h.LastResponseAt = time.Now()
		h.FailCount = 0
	} else {
		h.FailCount++
		if err != nil {
			h.LastError = err.Error()
		}
	}
}

// RecoverAgent attempts to recover an unresponsive agent.
func (m *Manager) RecoverAgent(ctx context.Context, agentName string) RecoveryResult {
	result := RecoveryResult{
		AgentName: agentName,
		Success:   false,
	}

	m.mu.RLock()
	_, exists := m.health[agentName]
	lock := m.recoveryLock[agentName]
	m.mu.RUnlock()

	if !exists {
		result.LastError = "agent not registered"
		return result
	}

	// Acquire recovery lock to prevent concurrent recovery attempts.
	lock.Lock()
	defer lock.Unlock()

	// Check if recovery handler is configured.
	if m.recoveryHandler == nil {
		result.LastError = "recovery handler not configured"
		return result
	}

	// Attempt restart if auto-restart is enabled.
	if m.autoRestart {
		for attempt := 1; attempt <= m.maxRetries; attempt++ {
			result.Attempt = attempt

			if err := m.recoveryHandler.StartAgent(ctx, agentName); err == nil {
				m.UpdateHealth(agentName, HealthStatusHealthy, nil)
				result.Success = true
				return result
			}

			// Wait before retry (exponential backoff).
			if attempt < m.maxRetries {
				select {
				case <-time.After(m.retryDelay):
				case <-ctx.Done():
					result.LastError = ctx.Err().Error()
					return result
				}
			}
		}
	}

	// Mark as dead if recovery failed.
	m.UpdateHealth(agentName, HealthStatusDead, nil)
	result.LastError = "max retries exceeded"
	return result
}

// GetHealth returns the health status of an agent.
func (m *Manager) GetHealth(agentName string) *Health {
	m.mu.RLock()
	defer m.mu.RUnlock()

	health, exists := m.health[agentName]
	if !exists {
		return nil
	}

	// Return a copy to prevent external mutations.
	h := *health
	return &h
}

// GetAllHealth returns the health status of all agents.
func (m *Manager) GetAllHealth() map[string]*Health {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*Health, len(m.health))
	for name, h := range m.health {
		h2 := *h
		result[name] = &h2
	}

	return result
}

// IsHealthy checks if an agent is healthy.
func (m *Manager) IsHealthy(agentName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	h, exists := m.health[agentName]
	return exists && h.Status == HealthStatusHealthy
}

// IsTaskHung checks if an agent's task has been running too long.
func (m *Manager) IsTaskHung(agentName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	h, exists := m.health[agentName]
	if !exists {
		return false
	}

	// If task duration is not set, assume not hung.
	if h.TaskDurationSince.IsZero() {
		return false
	}

	elapsed := time.Since(h.TaskDurationSince)
	return elapsed > m.taskTimeout
}

// SetTaskStartTime sets when a task started for an agent.
func (m *Manager) SetTaskStartTime(agentName string, startTime time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if h, exists := m.health[agentName]; exists {
		h.TaskDurationSince = startTime
	}
}

// ClearTaskStartTime clears the task start time for an agent.
func (m *Manager) ClearTaskStartTime(agentName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if h, exists := m.health[agentName]; exists {
		h.TaskDurationSince = time.Time{}
	}
}
