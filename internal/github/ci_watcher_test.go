package github

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultCIWatcherConfig(t *testing.T) {
	config := DefaultCIWatcherConfig()

	if config.PollInterval != 30*time.Second {
		t.Errorf("expected PollInterval to be 30s, got %v", config.PollInterval)
	}

	if config.Timeout != 24*time.Hour {
		t.Errorf("expected Timeout to be 24h, got %v", config.Timeout)
	}
}

func TestMapStatusState(t *testing.T) {
	tests := []struct {
		input    string
		expected CIStatus
	}{
		{"success", CIStatusSuccess},
		{"failure", CIStatusFailure},
		{"error", CIStatusFailure},
		{"pending", CIStatusPending},
		{"unknown", CIStatusPending},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mapStatusState(tt.input)
			if result != tt.expected {
				t.Errorf("mapStatusState(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMapCheckRunStatus(t *testing.T) {
	tests := []struct {
		status     string
		conclusion string
		expected   CIStatus
	}{
		{"in_progress", "", CIStatusRunning},
		{"queued", "", CIStatusPending},
		{"completed", "success", CIStatusSuccess},
		{"completed", "failure", CIStatusFailure},
		{"completed", "cancelled", CIStatusCancelled},
		{"completed", "skipped", CIStatusCancelled},
		{"completed", "neutral", CIStatusSuccess},
		{"completed", "timed_out", CIStatusFailure},
	}

	for _, tt := range tests {
		t.Run(tt.status+"_"+tt.conclusion, func(t *testing.T) {
			result := mapCheckRunStatus(tt.status, tt.conclusion)
			if result != tt.expected {
				t.Errorf("mapCheckRunStatus(%q, %q) = %v, want %v", tt.status, tt.conclusion, result, tt.expected)
			}
		})
	}
}

func TestFormatCIResult(t *testing.T) {
	tests := []struct {
		name     string
		result   CIResult
		contains string
	}{
		{
			name: "success",
			result: CIResult{
				PRNumber:    123,
				PRTitle:     "Test PR",
				PRURL:       "https://github.com/test/repo/pull/123",
				Status:      CIStatusSuccess,
				TotalChecks: 5,
				Passed:      5,
			},
			contains: "✅",
		},
		{
			name: "failure",
			result: CIResult{
				PRNumber:    456,
				PRTitle:     "Failing PR",
				PRURL:       "https://github.com/test/repo/pull/456",
				Status:      CIStatusFailure,
				TotalChecks: 3,
				Passed:      2,
				Failed:      1,
			},
			contains: "❌",
		},
		{
			name: "running",
			result: CIResult{
				PRNumber:    789,
				PRTitle:     "In Progress PR",
				PRURL:       "https://github.com/test/repo/pull/789",
				Status:      CIStatusRunning,
				TotalChecks: 2,
				Passed:      1,
				Pending:     1,
			},
			contains: "🔄",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatCIResult(tt.result)
			if len(result) == 0 {
				t.Error("expected non-empty result")
			}
			// Check that PR number is included
			if !strings.Contains(result, "123") && !strings.Contains(result, "456") && !strings.Contains(result, "789") {
				t.Errorf("expected result to contain PR number, got %s", result)
			}
		})
	}
}

func TestCIWatcherWatchingPRs(t *testing.T) {
	// Create a watcher without client (nil client is ok for this test)
	config := CIWatcherConfig{
		PollInterval: 1 * time.Second,
		Timeout:      1 * time.Minute,
	}
	watcher := NewCIWatcher(nil, config)

	// Initially no PRs being watched
	prs := watcher.WatchingPRs()
	if len(prs) != 0 {
		t.Errorf("expected 0 watching PRs, got %d", len(prs))
	}

	// IsWatching should return false
	if watcher.IsWatching(123) {
		t.Error("expected IsWatching(123) to be false")
	}
}

func TestCIStatusConstants(t *testing.T) {
	// Verify status constants are distinct
	statuses := []CIStatus{
		CIStatusPending,
		CIStatusRunning,
		CIStatusSuccess,
		CIStatusFailure,
		CIStatusCancelled,
	}

	seen := make(map[CIStatus]bool)
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("duplicate status: %v", s)
		}
		seen[s] = true
	}
}
