package cinotify

import (
	"context"
	"strings"
	"testing"
	"time"

	ghlib "github.com/ytnobody/MAXAM/internal/github"
)

func TestDefaultServiceConfig(t *testing.T) {
	config := DefaultServiceConfig()

	if config.PollInterval != 30*time.Second {
		t.Errorf("expected PollInterval to be 30s, got %v", config.PollInterval)
	}

	if config.Timeout != 24*time.Hour {
		t.Errorf("expected Timeout to be 24h, got %v", config.Timeout)
	}
}

func TestFormatNotification(t *testing.T) {
	tests := []struct {
		name     string
		result   ghlib.CIResult
		contains []string
	}{
		{
			name: "success notification",
			result: ghlib.CIResult{
				PRNumber:    123,
				PRTitle:     "Add feature X",
				PRURL:       "https://github.com/test/repo/pull/123",
				Status:      ghlib.CIStatusSuccess,
				TotalChecks: 5,
				Passed:      5,
			},
			contains: []string{"✅", "CI passed", "#123", "Add feature X", "5/5"},
		},
		{
			name: "failure notification",
			result: ghlib.CIResult{
				PRNumber:    456,
				PRTitle:     "Fix bug Y",
				PRURL:       "https://github.com/test/repo/pull/456",
				Status:      ghlib.CIStatusFailure,
				TotalChecks: 3,
				Passed:      2,
				Failed:      1,
				Details: []ghlib.CheckDetail{
					{Name: "lint", Status: ghlib.CIStatusSuccess},
					{Name: "test", Status: ghlib.CIStatusSuccess},
					{Name: "build", Status: ghlib.CIStatusFailure},
				},
			},
			contains: []string{"❌", "CI failed", "#456", "2/3", "Failed checks", "build"},
		},
		{
			name: "cancelled notification",
			result: ghlib.CIResult{
				PRNumber:    789,
				PRTitle:     "Update docs",
				PRURL:       "https://github.com/test/repo/pull/789",
				Status:      ghlib.CIStatusCancelled,
				TotalChecks: 0,
			},
			contains: []string{"⏹️", "cancelled", "#789"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := formatNotification(tt.result)

			for _, substr := range tt.contains {
				if !strings.Contains(msg, substr) {
					t.Errorf("expected message to contain %q, got: %s", substr, msg)
				}
			}
		})
	}
}

type mockNotifier struct {
	messages []string
}

func (m *mockNotifier) NotifyCI(ctx context.Context, message string) error {
	m.messages = append(m.messages, message)
	return nil
}

func TestServiceStartStop(t *testing.T) {
	notifier := &mockNotifier{}
	config := ServiceConfig{
		PollInterval: 100 * time.Millisecond,
		Timeout:      1 * time.Second,
	}

	// Create service with nil client (won't actually poll)
	service := NewService(nil, notifier, config)

	// Initially not running
	if service.IsRunning() {
		t.Error("expected service to not be running initially")
	}

	// Start service
	ctx := context.Background()
	err := service.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start service: %v", err)
	}

	// Should be running
	if !service.IsRunning() {
		t.Error("expected service to be running after Start")
	}

	// Start again should fail
	err = service.Start(ctx)
	if err == nil {
		t.Error("expected error when starting already running service")
	}

	// Stop service
	service.Stop()

	// Should not be running
	if service.IsRunning() {
		t.Error("expected service to not be running after Stop")
	}
}

func TestServiceWatchingPRs(t *testing.T) {
	notifier := &mockNotifier{}
	config := DefaultServiceConfig()

	service := NewService(nil, notifier, config)

	// Initially no PRs being watched
	prs := service.WatchingPRs()
	if len(prs) != 0 {
		t.Errorf("expected 0 watching PRs, got %d", len(prs))
	}
}
