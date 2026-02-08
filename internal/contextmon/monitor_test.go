package contextmon

import (
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxChars != 100000 {
		t.Errorf("MaxChars = %d, want 100000", cfg.MaxChars)
	}
	if cfg.WarningThreshold != 0.70 {
		t.Errorf("WarningThreshold = %f, want 0.70", cfg.WarningThreshold)
	}
	if cfg.CriticalThreshold != 0.85 {
		t.Errorf("CriticalThreshold = %f, want 0.85", cfg.CriticalThreshold)
	}
	if cfg.TruncationThreshold != 0.95 {
		t.Errorf("TruncationThreshold = %f, want 0.95", cfg.TruncationThreshold)
	}
}

func TestMonitor_Check_LevelOK(t *testing.T) {
	m := NewMonitor(DefaultConfig())

	// 50% usage should be OK
	status := m.Check(50000)

	if status.Level != LevelOK {
		t.Errorf("Level = %v, want LevelOK", status.Level)
	}
	if status.Percentage != 0.5 {
		t.Errorf("Percentage = %f, want 0.5", status.Percentage)
	}
	if status.CurrentSize != 50000 {
		t.Errorf("CurrentSize = %d, want 50000", status.CurrentSize)
	}
}

func TestMonitor_Check_LevelWarning(t *testing.T) {
	m := NewMonitor(DefaultConfig())

	// 75% usage should trigger warning
	status := m.Check(75000)

	if status.Level != LevelWarning {
		t.Errorf("Level = %v, want LevelWarning", status.Level)
	}
}

func TestMonitor_Check_LevelCritical(t *testing.T) {
	m := NewMonitor(DefaultConfig())

	// 90% usage should trigger critical
	status := m.Check(90000)

	if status.Level != LevelCritical {
		t.Errorf("Level = %v, want LevelCritical", status.Level)
	}
}

func TestMonitor_Check_LevelTruncate(t *testing.T) {
	m := NewMonitor(DefaultConfig())

	// 96% usage should trigger truncation
	status := m.Check(96000)

	if status.Level != LevelTruncate {
		t.Errorf("Level = %v, want LevelTruncate", status.Level)
	}
}

func TestMonitor_ShouldTruncate(t *testing.T) {
	m := NewMonitor(DefaultConfig())

	tests := []struct {
		name     string
		size     int
		expected bool
	}{
		{"below threshold", 90000, false},
		{"at threshold", 95000, true},
		{"above threshold", 98000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.ShouldTruncate(tt.size)
			if result != tt.expected {
				t.Errorf("ShouldTruncate(%d) = %v, want %v", tt.size, result, tt.expected)
			}
		})
	}
}

func TestMonitor_RecommendedHistoryCount(t *testing.T) {
	m := NewMonitor(DefaultConfig())

	tests := []struct {
		name        string
		currentSize int
		avgMsgLen   int
		minExpected int
		maxExpected int
	}{
		{"plenty of room", 10000, 1000, 5, 10},
		{"half full", 50000, 1000, 1, 10},
		{"near limit", 68000, 1000, 1, 5},
		{"zero avg length", 50000, 0, 8, 8}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.RecommendedHistoryCount(tt.currentSize, tt.avgMsgLen)
			if result < tt.minExpected || result > tt.maxExpected {
				t.Errorf("RecommendedHistoryCount(%d, %d) = %d, want between %d and %d",
					tt.currentSize, tt.avgMsgLen, result, tt.minExpected, tt.maxExpected)
			}
		})
	}
}

func TestStatus_FormatWarning(t *testing.T) {
	tests := []struct {
		level    Level
		hasText  bool
		contains string
	}{
		{LevelOK, false, ""},
		{LevelWarning, true, "⚠️"},
		{LevelCritical, true, "🔴"},
		{LevelTruncate, true, "🚨"},
	}

	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			status := Status{
				CurrentSize: 75000,
				MaxSize:     100000,
				Percentage:  0.75,
				Level:       tt.level,
			}

			result := status.FormatWarning()

			if tt.hasText && result == "" {
				t.Error("Expected warning text, got empty")
			}
			if !tt.hasText && result != "" {
				t.Errorf("Expected empty, got %q", result)
			}
			if tt.contains != "" && !strings.Contains(result, tt.contains) {
				t.Errorf("Expected %q to contain %q", result, tt.contains)
			}
		})
	}
}

func TestLevel_String(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelOK, "OK"},
		{LevelWarning, "WARNING"},
		{LevelCritical, "CRITICAL"},
		{LevelTruncate, "TRUNCATE"},
		{Level(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.level.String()
			if result != tt.expected {
				t.Errorf("Level.String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestMonitor_LastStatus(t *testing.T) {
	m := NewMonitor(DefaultConfig())

	// Initially nil
	if m.LastStatus() != nil {
		t.Error("Expected nil before first check")
	}

	// After check, should be set
	m.Check(50000)
	last := m.LastStatus()
	if last == nil {
		t.Error("Expected non-nil after check")
	}
	if last.CurrentSize != 50000 {
		t.Errorf("LastStatus().CurrentSize = %d, want 50000", last.CurrentSize)
	}
}

func TestMonitor_CustomConfig(t *testing.T) {
	cfg := &Config{
		MaxChars:            50000,
		WarningThreshold:    0.5,
		CriticalThreshold:   0.7,
		TruncationThreshold: 0.9,
	}
	m := NewMonitor(cfg)

	// 60% should be warning with custom config
	status := m.Check(30000)
	if status.Level != LevelWarning {
		t.Errorf("Level = %v, want LevelWarning", status.Level)
	}
}

func TestMonitor_UpdateConfig(t *testing.T) {
	m := NewMonitor(DefaultConfig())

	newCfg := &Config{
		MaxChars:            50000,
		WarningThreshold:    0.5,
		CriticalThreshold:   0.7,
		TruncationThreshold: 0.9,
	}
	m.UpdateConfig(newCfg)

	cfg := m.Config()
	if cfg.MaxChars != 50000 {
		t.Errorf("MaxChars = %d, want 50000", cfg.MaxChars)
	}
}

func TestStatus_String(t *testing.T) {
	status := Status{
		CurrentSize: 75000,
		MaxSize:     100000,
		Percentage:  0.75,
		Level:       LevelWarning,
	}

	result := status.String()

	if !strings.Contains(result, "75000") {
		t.Errorf("Expected status string to contain size, got %q", result)
	}
	if !strings.Contains(result, "75%") {
		t.Errorf("Expected status string to contain percentage, got %q", result)
	}
	if !strings.Contains(result, "WARNING") {
		t.Errorf("Expected status string to contain level, got %q", result)
	}
}

// === Additional tests for edge cases (Shiori's review) ===

func TestMonitor_Check_BoundaryValues(t *testing.T) {
	m := NewMonitor(DefaultConfig())

	tests := []struct {
		name     string
		size     int
		expected Level
	}{
		// OK -> Warning boundary (70%)
		{"just below warning 69.99%", 69990, LevelOK},
		{"exactly at warning 70%", 70000, LevelWarning},
		{"just above warning 70.01%", 70010, LevelWarning},

		// Warning -> Critical boundary (85%)
		{"just below critical 84.99%", 84990, LevelWarning},
		{"exactly at critical 85%", 85000, LevelCritical},
		{"just above critical 85.01%", 85010, LevelCritical},

		// Critical -> Truncate boundary (95%)
		{"just below truncate 94.99%", 94990, LevelCritical},
		{"exactly at truncate 95%", 95000, LevelTruncate},
		{"just above truncate 95.01%", 95010, LevelTruncate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := m.Check(tt.size)
			if status.Level != tt.expected {
				t.Errorf("Check(%d) Level = %v, want %v (percentage: %.4f%%)",
					tt.size, status.Level, tt.expected, status.Percentage*100)
			}
		})
	}
}

func TestMonitor_Check_ZeroAndNegativeValues(t *testing.T) {
	m := NewMonitor(DefaultConfig())

	tests := []struct {
		name     string
		size     int
		expected Level
	}{
		{"zero size", 0, LevelOK},
		{"negative size", -1, LevelOK},
		{"large negative", -100000, LevelOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := m.Check(tt.size)
			if status.Level != tt.expected {
				t.Errorf("Check(%d) Level = %v, want %v", tt.size, status.Level, tt.expected)
			}
			// Verify percentage calculation doesn't panic
			if tt.size <= 0 && status.Percentage > 0 {
				t.Errorf("Check(%d) Percentage = %v, expected <= 0", tt.size, status.Percentage)
			}
		})
	}
}

func TestMonitor_Check_ExceedMaxChars(t *testing.T) {
	m := NewMonitor(DefaultConfig())

	tests := []struct {
		name       string
		size       int
		expected   Level
		minPercent float64
	}{
		{"exactly 100%", 100000, LevelTruncate, 1.0},
		{"110% over max", 110000, LevelTruncate, 1.1},
		{"200% double max", 200000, LevelTruncate, 2.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := m.Check(tt.size)
			if status.Level != tt.expected {
				t.Errorf("Check(%d) Level = %v, want %v", tt.size, status.Level, tt.expected)
			}
			if status.Percentage < tt.minPercent {
				t.Errorf("Check(%d) Percentage = %v, want >= %v", tt.size, status.Percentage, tt.minPercent)
			}
		})
	}
}

func TestMonitor_ShouldTruncate_EdgeCases(t *testing.T) {
	m := NewMonitor(DefaultConfig())

	tests := []struct {
		name     string
		size     int
		expected bool
	}{
		{"zero", 0, false},
		{"negative", -1, false},
		{"just below 95%", 94999, false},
		{"exactly 95%", 95000, true},
		{"over 100%", 100001, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.ShouldTruncate(tt.size)
			if result != tt.expected {
				t.Errorf("ShouldTruncate(%d) = %v, want %v", tt.size, result, tt.expected)
			}
		})
	}
}
