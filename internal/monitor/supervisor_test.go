package monitor

import (
	"context"
	"testing"
	"time"

	"github.com/ytnobody/MAXAM/internal/healthmgr"
	"github.com/ytnobody/MAXAM/internal/issuemgr"
)

// mockIssueChecker implements issuemgr.IssueChecker for testing.
type mockIssueChecker struct {
	count int
	err   error
}

func (m *mockIssueChecker) CountOpenIssues(ctx context.Context) (int, error) {
	return m.count, m.err
}

// mockHealthChecker implements healthmgr.HealthChecker for testing.
type mockHealthChecker struct {
	healthy map[string]bool
}

func (m *mockHealthChecker) CheckHealth(ctx context.Context, agentName string) error {
	if m.healthy[agentName] {
		return nil
	}
	return context.DeadlineExceeded
}

func TestNewSupervisor(t *testing.T) {
	s := NewSupervisor()

	if s == nil {
		t.Fatal("NewSupervisor returned nil")
	}

	if s.running {
		t.Error("Supervisor should not be running initially")
	}
}

func TestSupervisor_StartStop(t *testing.T) {
	s := NewSupervisor()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !s.IsRunning() {
		t.Error("Supervisor should be running after Start")
	}

	// Double start should be no-op.
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Double Start failed: %v", err)
	}

	s.Stop()

	if s.IsRunning() {
		t.Error("Supervisor should not be running after Stop")
	}

	// Double stop should be no-op.
	s.Stop()
}

func TestSupervisor_WithHealthManager(t *testing.T) {
	mgr := healthmgr.NewManager()
	mgr.RegisterAgent("test-agent")
	mgr.UpdateHealth("test-agent", healthmgr.HealthStatusHealthy, nil)

	s := NewSupervisor(
		WithHealthManager(mgr),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer s.Stop()

	// Wait for initial aggregation.
	time.Sleep(100 * time.Millisecond)

	status := s.GetStatus()
	if len(status.Agents) != 1 {
		t.Errorf("Expected 1 agent, got %d", len(status.Agents))
	}

	if status.Agents[0].Name != "test-agent" {
		t.Errorf("Expected agent name 'test-agent', got '%s'", status.Agents[0].Name)
	}
}

func TestSupervisor_WithIssueWatcher(t *testing.T) {
	checker := &mockIssueChecker{count: 5}
	watcher := issuemgr.NewWatcher(checker,
		issuemgr.WithInterval(10*time.Millisecond),
	)

	var receivedCount int
	s := NewSupervisor(
		WithIssueWatcher(watcher),
	)

	// Set up issue notification callback on watcher.
	watcher = issuemgr.NewWatcher(checker,
		issuemgr.WithInterval(10*time.Millisecond),
		issuemgr.WithNotifyFunc(func(ctx context.Context, count int) {
			receivedCount = count
			s.UpdateOpenIssues(count)
		}),
	)
	s = NewSupervisor(
		WithIssueWatcher(watcher),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer s.Stop()

	// Wait for watcher to run.
	time.Sleep(50 * time.Millisecond)

	if receivedCount != 5 {
		t.Errorf("Expected receivedCount 5, got %d", receivedCount)
	}

	status := s.GetStatus()
	if status.OpenIssues != 5 {
		t.Errorf("Expected OpenIssues 5, got %d", status.OpenIssues)
	}
}

func TestSupervisor_OnChangeCallback(t *testing.T) {
	mgr := healthmgr.NewManager()
	mgr.RegisterAgent("test-agent")

	callCount := 0
	s := NewSupervisor(
		WithHealthManager(mgr),
		WithOnChange(func(status Status) {
			callCount++
		}),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer s.Stop()

	// Wait for callbacks.
	time.Sleep(100 * time.Millisecond)

	if callCount == 0 {
		t.Error("OnChange callback was not called")
	}
}

func TestSupervisor_UpdateOpenIssues(t *testing.T) {
	callbackCalled := false
	s := NewSupervisor(
		WithOnChange(func(status Status) {
			callbackCalled = true
		}),
	)

	s.UpdateOpenIssues(10)

	status := s.GetStatus()
	if status.OpenIssues != 10 {
		t.Errorf("Expected OpenIssues 10, got %d", status.OpenIssues)
	}

	if !callbackCalled {
		t.Error("OnChange callback was not called on UpdateOpenIssues")
	}
}

func TestSupervisor_ContextCancellation(t *testing.T) {
	s := NewSupervisor()

	ctx, cancel := context.WithCancel(context.Background())

	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !s.IsRunning() {
		t.Error("Supervisor should be running")
	}

	// Cancel context.
	cancel()

	// Wait for goroutine to exit.
	time.Sleep(50 * time.Millisecond)

	if s.IsRunning() {
		t.Error("Supervisor should stop when context is cancelled")
	}
}
