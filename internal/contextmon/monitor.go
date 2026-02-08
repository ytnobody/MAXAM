// Package contextmon provides context size monitoring and warnings
package contextmon

import (
	"fmt"
	"sync"
)

// DefaultConfig returns the default context monitoring configuration
func DefaultConfig() *Config {
	return &Config{
		MaxChars:            100000, // 100KB default (ARG_MAX ~128KB on Linux)
		WarningThreshold:    0.70,   // Warn at 70%
		CriticalThreshold:   0.85,   // Critical at 85%
		TruncationThreshold: 0.95,   // Auto-truncate at 95%
	}
}

// Config holds context monitoring configuration
type Config struct {
	MaxChars            int     `yaml:"max_chars"`            // Maximum context size in characters
	WarningThreshold    float64 `yaml:"warning_threshold"`    // Percentage to trigger warning (0.0-1.0)
	CriticalThreshold   float64 `yaml:"critical_threshold"`   // Percentage to trigger critical warning
	TruncationThreshold float64 `yaml:"truncation_threshold"` // Percentage to auto-truncate
}

// Level represents the severity level of context size
type Level int

const (
	LevelOK Level = iota
	LevelWarning
	LevelCritical
	LevelTruncate
)

// String returns the string representation of the level
func (l Level) String() string {
	switch l {
	case LevelOK:
		return "OK"
	case LevelWarning:
		return "WARNING"
	case LevelCritical:
		return "CRITICAL"
	case LevelTruncate:
		return "TRUNCATE"
	default:
		return "UNKNOWN"
	}
}

// Status represents the current context size status
type Status struct {
	CurrentSize int     // Current size in characters
	MaxSize     int     // Maximum allowed size
	Percentage  float64 // Usage percentage (0.0-1.0)
	Level       Level   // Current warning level
}

// String returns a human-readable status string
func (s Status) String() string {
	return fmt.Sprintf("%d/%d chars (%.0f%%) [%s]",
		s.CurrentSize, s.MaxSize, s.Percentage*100, s.Level)
}

// FormatWarning returns a formatted warning message for display
func (s Status) FormatWarning() string {
	switch s.Level {
	case LevelWarning:
		return fmt.Sprintf("⚠️ Context size: %.0f%% - approaching limit", s.Percentage*100)
	case LevelCritical:
		return fmt.Sprintf("🔴 Context size: %.0f%% - near limit!", s.Percentage*100)
	case LevelTruncate:
		return fmt.Sprintf("🚨 Context size: %.0f%% - auto-reducing history", s.Percentage*100)
	default:
		return ""
	}
}

// Monitor tracks and reports context size
type Monitor struct {
	config *Config
	mu     sync.RWMutex

	// Current status cache
	lastStatus *Status
}

// NewMonitor creates a new context monitor with the given configuration
func NewMonitor(cfg *Config) *Monitor {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Monitor{
		config: cfg,
	}
}

// Check evaluates the given context size and returns the status
func (m *Monitor) Check(size int) Status {
	m.mu.Lock()
	defer m.mu.Unlock()

	percentage := float64(size) / float64(m.config.MaxChars)
	level := m.calculateLevel(percentage)

	status := Status{
		CurrentSize: size,
		MaxSize:     m.config.MaxChars,
		Percentage:  percentage,
		Level:       level,
	}

	m.lastStatus = &status
	return status
}

// calculateLevel determines the warning level based on percentage
func (m *Monitor) calculateLevel(percentage float64) Level {
	switch {
	case percentage >= m.config.TruncationThreshold:
		return LevelTruncate
	case percentage >= m.config.CriticalThreshold:
		return LevelCritical
	case percentage >= m.config.WarningThreshold:
		return LevelWarning
	default:
		return LevelOK
	}
}

// LastStatus returns the last checked status (may be nil)
func (m *Monitor) LastStatus() *Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastStatus
}

// ShouldTruncate returns true if the context should be truncated
func (m *Monitor) ShouldTruncate(size int) bool {
	percentage := float64(size) / float64(m.config.MaxChars)
	return percentage >= m.config.TruncationThreshold
}

// RecommendedHistoryCount returns the recommended number of history messages
// based on current context size and average message length
func (m *Monitor) RecommendedHistoryCount(currentSize, avgMsgLen int) int {
	if avgMsgLen == 0 {
		return 8 // Default
	}

	// Calculate remaining space (target 70% of max)
	targetSize := int(float64(m.config.MaxChars) * m.config.WarningThreshold)
	remaining := targetSize - currentSize

	if remaining <= 0 {
		return 1 // Minimum
	}

	// How many messages can fit?
	recommended := remaining / avgMsgLen
	if recommended < 1 {
		return 1
	}
	if recommended > 10 {
		return 10 // Max
	}
	return recommended
}

// Config returns the current configuration (read-only)
func (m *Monitor) Config() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return *m.config
}

// UpdateConfig updates the monitor configuration
func (m *Monitor) UpdateConfig(cfg *Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg
}
