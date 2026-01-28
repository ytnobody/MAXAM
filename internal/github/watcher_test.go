package github

import (
	"testing"
	"time"
)

func TestFormatEvent(t *testing.T) {
	tests := []struct {
		name     string
		event    PREvent
		expected string
	}{
		{
			name: "merged PR",
			event: PREvent{
				Number:   42,
				Title:    "Add new feature",
				Action:   PRActionMerged,
				MergedBy: "ytnobody",
			},
			expected: "🟢 PR #42 merged: Add new feature (by ytnobody)",
		},
		{
			name: "merged PR without merger info",
			event: PREvent{
				Number:   42,
				Title:    "Add new feature",
				Action:   PRActionMerged,
				MergedBy: "",
			},
			expected: "🟢 PR #42 merged: Add new feature (by unknown)",
		},
		{
			name: "closed PR",
			event: PREvent{
				Number: 43,
				Title:  "Rejected feature",
				Action: PRActionClosed,
			},
			expected: "🔴 PR #43 closed without merge: Rejected feature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatEvent(tt.event)
			if result != tt.expected {
				t.Errorf("FormatEvent() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestNewWatcher(t *testing.T) {
	// Create a minimal client for testing
	client := &Client{
		owner: "test",
		repo:  "repo",
	}

	watcher := NewWatcher(client)

	if watcher.client != client {
		t.Error("NewWatcher() client not set correctly")
	}

	// lastCheck should be approximately 24 hours ago
	expectedStart := time.Now().Add(-25 * time.Hour)
	expectedEnd := time.Now().Add(-23 * time.Hour)

	if watcher.lastCheck.Before(expectedStart) || watcher.lastCheck.After(expectedEnd) {
		t.Errorf("NewWatcher() lastCheck = %v, want between %v and %v",
			watcher.lastCheck, expectedStart, expectedEnd)
	}
}

func TestNewWatcherWithSince(t *testing.T) {
	client := &Client{
		owner: "test",
		repo:  "repo",
	}

	customTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	watcher := NewWatcherWithSince(client, customTime)

	if !watcher.lastCheck.Equal(customTime) {
		t.Errorf("NewWatcherWithSince() lastCheck = %v, want %v",
			watcher.lastCheck, customTime)
	}
}

func TestPRAction(t *testing.T) {
	if PRActionMerged != "merged" {
		t.Errorf("PRActionMerged = %q, want %q", PRActionMerged, "merged")
	}

	if PRActionClosed != "closed" {
		t.Errorf("PRActionClosed = %q, want %q", PRActionClosed, "closed")
	}
}
