// Package errorwatch provides error detection and automatic follow-up for agent errors.
package errorwatch

import (
	"strings"
	"sync"
	"time"
)

// ErrorPatterns defines the patterns to detect recoverable agent errors.
var ErrorPatterns = []string{
	"context deadline exceeded",
	"context canceled",
	"timeout",
	"connection refused",
	"connection reset",
}

// Detector detects error patterns and manages follow-up cooldowns.
type Detector struct {
	mu              sync.Mutex
	lastFollowUp    map[string]time.Time
	cooldownPeriod  time.Duration
}

// NewDetector creates a new error detector with the specified cooldown period.
func NewDetector(cooldownPeriod time.Duration) *Detector {
	return &Detector{
		lastFollowUp:   make(map[string]time.Time),
		cooldownPeriod: cooldownPeriod,
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
