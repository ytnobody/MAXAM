package github

import (
	"testing"
	"time"
)

func TestNewEventWatcher(t *testing.T) {
	client := &Client{
		owner: "test",
		repo:  "repo",
	}

	watcher := NewEventWatcher(client)

	if watcher.client != client {
		t.Error("NewEventWatcher() client not set correctly")
	}

	if watcher.seenIDs == nil {
		t.Error("NewEventWatcher() seenIDs should be initialized")
	}

	// lastCheck should be approximately now
	if time.Since(watcher.lastCheck) > time.Second {
		t.Errorf("NewEventWatcher() lastCheck too old: %v", watcher.lastCheck)
	}
}

func TestNewEventWatcherWithSince(t *testing.T) {
	client := &Client{
		owner: "test",
		repo:  "repo",
	}

	customTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	watcher := NewEventWatcherWithSince(client, customTime)

	if !watcher.lastCheck.Equal(customTime) {
		t.Errorf("NewEventWatcherWithSince() lastCheck = %v, want %v",
			watcher.lastCheck, customTime)
	}
}

func TestEventType(t *testing.T) {
	if EventTypeIssue != "issue" {
		t.Errorf("EventTypeIssue = %q, want %q", EventTypeIssue, "issue")
	}

	if EventTypePR != "pr" {
		t.Errorf("EventTypePR = %q, want %q", EventTypePR, "pr")
	}
}

func TestFormatEventNotification(t *testing.T) {
	tests := []struct {
		name     string
		event    Event
		contains []string
	}{
		{
			name: "issue notification",
			event: Event{
				Type:   EventTypeIssue,
				Number: 42,
				Title:  "Bug report",
				Author: "testuser",
				URL:    "https://github.com/test/repo/issues/42",
			},
			contains: []string{"📝", "Issue #42", "Bug report", "@testuser"},
		},
		{
			name: "PR notification",
			event: Event{
				Type:   EventTypePR,
				Number: 123,
				Title:  "New feature",
				Author: "developer",
				URL:    "https://github.com/test/repo/pull/123",
			},
			contains: []string{"🔀", "PR #123", "New feature", "@developer"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatEventNotification(tt.event)
			for _, s := range tt.contains {
				if !containsString(result, s) {
					t.Errorf("FormatEventNotification() = %q, want to contain %q", result, s)
				}
			}
		})
	}
}

func TestMarkAsSeen(t *testing.T) {
	client := &Client{
		owner: "test",
		repo:  "repo",
	}

	watcher := NewEventWatcher(client)

	// Mark as seen
	watcher.MarkAsSeen(EventTypeIssue, 42)

	// Check internal state
	watcher.mu.RLock()
	seen := watcher.seenIDs["issue-42"]
	watcher.mu.RUnlock()

	if !seen {
		t.Error("MarkAsSeen() should mark event as seen")
	}
}

func TestReset(t *testing.T) {
	client := &Client{
		owner: "test",
		repo:  "repo",
	}

	customTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	watcher := NewEventWatcherWithSince(client, customTime)

	// Add some seen IDs
	watcher.MarkAsSeen(EventTypeIssue, 1)
	watcher.MarkAsSeen(EventTypePR, 2)

	// Reset
	watcher.Reset()

	// Check seenIDs is cleared
	watcher.mu.RLock()
	count := len(watcher.seenIDs)
	watcher.mu.RUnlock()

	if count != 0 {
		t.Errorf("Reset() seenIDs count = %d, want 0", count)
	}

	// Check lastCheck is updated
	if time.Since(watcher.lastCheck) > time.Second {
		t.Errorf("Reset() lastCheck should be updated to now")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
