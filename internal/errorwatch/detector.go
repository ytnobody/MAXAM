// Package errorwatch provides error detection and automatic follow-up for agent errors.
package errorwatch

import (
	"strings"
	"sync"
	"time"
)

// ErrorPatterns defines the patterns to detect recoverable agent errors.
var ErrorPatterns = []string{
	// Context and timeout errors
	"context deadline exceeded",
	"context canceled",
	"timeout",
	// Connection errors
	"connection refused",
	"connection reset",
	// LLM API errors
	"rate limit",
	"rate_limit",
	"too many requests",
	"429",
	"503",
	"service unavailable",
	"overloaded",
	"capacity",
	// General errors that may be recoverable
	"exit status 1",
}

// Detector detects error patterns and manages follow-up cooldowns.
type Detector struct {
	mu             sync.Mutex
	lastFollowUp   map[string]time.Time
	cooldownPeriod time.Duration
	modeManager    *AgentModeManager
}

// NewDetector creates a new error detector with the specified cooldown period.
func NewDetector(cooldownPeriod time.Duration) *Detector {
	return &Detector{
		lastFollowUp:   make(map[string]time.Time),
		cooldownPeriod: cooldownPeriod,
		modeManager:    NewAgentModeManager(),
	}
}

// DefaultDetector creates a detector with 30-second cooldown.
func DefaultDetector() *Detector {
	return NewDetector(30 * time.Second)
}

// IsRecoverableError checks if the error message matches known recoverable patterns.
func IsRecoverableError(errMsg string) bool {
	lower := strings.ToLower(errMsg)
	for _, pattern := range ErrorPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// ShouldFollowUp checks if a follow-up should be sent to the agent.
// Returns true if the error is recoverable and cooldown has passed.
func (d *Detector) ShouldFollowUp(agentName string, errMsg string) bool {
	if !IsRecoverableError(errMsg) {
		return false
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	lastTime, exists := d.lastFollowUp[agentName]
	if exists && time.Since(lastTime) < d.cooldownPeriod {
		return false
	}

	return true
}

// RecordFollowUp records that a follow-up was sent to the agent.
func (d *Detector) RecordFollowUp(agentName string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.lastFollowUp[agentName] = time.Now()
}

// Reset clears the follow-up history for an agent.
func (d *Detector) Reset(agentName string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.lastFollowUp, agentName)
}

// ResetAll clears all follow-up history.
func (d *Detector) ResetAll() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.lastFollowUp = make(map[string]time.Time)
}

// HandleError processes an error and switches to auto mode if applicable.
// Returns true if the agent was switched to auto mode.
func (d *Detector) HandleError(agentName string, errMsg string) bool {
	if !IsRecoverableError(errMsg) {
		return false
	}

	d.modeManager.SwitchToAuto(agentName, errMsg)
	return true
}

// ModeManager returns the agent mode manager.
func (d *Detector) ModeManager() *AgentModeManager {
	return d.modeManager
}

// SetModeManager sets a custom agent mode manager.
func (d *Detector) SetModeManager(m *AgentModeManager) {
	d.modeManager = m
}

// IsAgentInAutoMode checks if an agent is in auto mode.
func (d *Detector) IsAgentInAutoMode(agentName string) bool {
	return d.modeManager.IsAuto(agentName)
}

// ResetAgentMode resets an agent to normal mode.
func (d *Detector) ResetAgentMode(agentName string) {
	d.modeManager.SwitchToNormal(agentName)
}
