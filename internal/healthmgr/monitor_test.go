package healthmgr

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockHealthChecker implements HealthChecker for testing.
type mockHealthChecker struct {
	mu       sync.RWMutex
	results  map[string]error
	checkCnt int32
}

func newMockHealthChecker() *mockHealthChecker {
	return &mockHealthChecker{
		results: make(map[string]error),
	}
}

func (m *mockHealthChecker) CheckHealth(_ context.Context, agentName string) error {
	atomic.AddInt32(&m.checkCnt, 1)
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.results[agentName]
}

func (m *mockHealthChecker) SetResult(agentName string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.results[agentName] = err
}

func (m *mockHealthChecker) CheckCount() int {
	return int(atomic.LoadInt32(&m.checkCnt))
}

// mockRecoveryHandler implements RecoveryHandler for testing.
type mockRecoveryHandler struct {
	startErr   error
	startCount int32
}

func (m *mockRecoveryHandler) StartAgent(_ context.Context, _ string) error {
	atomic.AddInt32(&m.startCount, 1)
	return m.startErr
}

func (m *mockRecoveryHandler) StopAgent(_ context.Context, _ string) error {
	return nil
}

func (m *mockRecoveryHandler) StartCount() int {
	return int(atomic.LoadInt32(&m.startCount))
}

func TestMonitor_StartStop(t *testing.T) {
	manager := NewManager()
	checker := newMockHealthChecker()
	monitor := NewMonitor(manager, checker, WithMonitorInterval(10*time.Millisecond))

	ctx := context.Background()

	// Initially not running.
	if monitor.IsRunning() {
		t.Error("expected monitor to not be running initially")
	}

	// Start monitoring.
	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("failed to start monitor: %v", err)
	}

	if !monitor.IsRunning() {
		t.Error("expected monitor to be running after Start")
	}

	// Starting again should be idempotent.
	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("second Start should not fail: %v", err)
	}

	// Stop monitoring.
	monitor.Stop()

	// Give it time to stop.
	time.Sleep(20 * time.Millisecond)

	if monitor.IsRunning() {
		t.Error("expected monitor to not be running after Stop")
	}

	// Stopping again should be idempotent.
	monitor.Stop()
}

func TestMonitor_PeriodicCheck(t *testing.T) {
	manager := NewManager()
	checker := newMockHealthChecker()
	monitor := NewMonitor(manager, checker, WithMonitorInterval(10*time.Millisecond))

	// Register an agent.
	manager.RegisterAgent("test-agent")
	checker.SetResult("test-agent", nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("failed to start monitor: %v", err)
	}
	defer monitor.Stop()

	// Wait for at least 3 check cycles.
	time.Sleep(50 * time.Millisecond)

	if checker.CheckCount() < 2 {
		t.Errorf("expected at least 2 health checks, got %d", checker.CheckCount())
	}
}

func TestMonitor_HealthyAgent(t *testing.T) {
	manager := NewManager()
	checker := newMockHealthChecker()
	monitor := NewMonitor(manager, checker, WithMonitorInterval(10*time.Millisecond))

	manager.RegisterAgent("healthy-agent")
	checker.SetResult("healthy-agent", nil)

	ctx := context.Background()
	monitor.CheckNow(ctx)

	health := manager.GetHealth("healthy-agent")
	if health == nil {
		t.Fatal("expected health to be tracked")
	}

	if health.Status != HealthStatusHealthy {
		t.Errorf("expected status %s, got %s", HealthStatusHealthy, health.Status)
	}

	if health.FailCount != 0 {
		t.Errorf("expected failCount 0, got %d", health.FailCount)
	}
}

func TestMonitor_UnhealthyAgent(t *testing.T) {
	manager := NewManager()
	checker := newMockHealthChecker()
	monitor := NewMonitor(manager, checker,
		WithMonitorInterval(10*time.Millisecond),
		WithFailThreshold(3),
	)

	manager.RegisterAgent("unhealthy-agent")
	checker.SetResult("unhealthy-agent", errors.New("connection refused"))

	ctx := context.Background()

	// First check - should mark as unresponsive.
	monitor.CheckAgentNow(ctx, "unhealthy-agent")

	health := manager.GetHealth("unhealthy-agent")
	if health.Status != HealthStatusUnresponsive {
		t.Errorf("expected status %s, got %s", HealthStatusUnresponsive, health.Status)
	}

	if health.FailCount != 1 {
		t.Errorf("expected failCount 1, got %d", health.FailCount)
	}

	if health.LastError != "connection refused" {
		t.Errorf("expected error message 'connection refused', got %s", health.LastError)
	}
}

func TestMonitor_RecoveryTrigger(t *testing.T) {
	handler := &mockRecoveryHandler{}
	manager := NewManager(
		WithRecoveryHandler(handler),
		WithMaxRetries(1),
		WithRetryDelay(1*time.Millisecond),
	)
	checker := newMockHealthChecker()

	var recoveryResults []RecoveryResult
	var mu sync.Mutex

	monitor := NewMonitor(manager, checker,
		WithMonitorInterval(10*time.Millisecond),
		WithFailThreshold(2),
		WithOnRecovery(func(result RecoveryResult) {
			mu.Lock()
			recoveryResults = append(recoveryResults, result)
			mu.Unlock()
		}),
	)

	manager.RegisterAgent("failing-agent")
	checker.SetResult("failing-agent", errors.New("timeout"))

	ctx := context.Background()

	// First failure - should not trigger recovery.
	monitor.CheckAgentNow(ctx, "failing-agent")
	if handler.StartCount() != 0 {
		t.Error("recovery should not trigger before threshold")
	}

	// Second failure - should trigger recovery.
	monitor.CheckAgentNow(ctx, "failing-agent")

	// Give recovery time to complete.
	time.Sleep(20 * time.Millisecond)

	if handler.StartCount() != 1 {
		t.Errorf("expected 1 recovery attempt, got %d", handler.StartCount())
	}

	mu.Lock()
	if len(recoveryResults) != 1 {
		t.Errorf("expected 1 recovery result, got %d", len(recoveryResults))
	}
	if len(recoveryResults) > 0 && recoveryResults[0].AgentName != "failing-agent" {
		t.Errorf("expected recovery for 'failing-agent', got %s", recoveryResults[0].AgentName)
	}
	mu.Unlock()
}

func TestMonitor_ContextCancellation(t *testing.T) {
	manager := NewManager()
	checker := newMockHealthChecker()
	monitor := NewMonitor(manager, checker, WithMonitorInterval(100*time.Millisecond))

	manager.RegisterAgent("test-agent")

	ctx, cancel := context.WithCancel(context.Background())

	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("failed to start monitor: %v", err)
	}

	// Cancel context.
	cancel()

	// Give it time to stop.
	time.Sleep(20 * time.Millisecond)

	if monitor.IsRunning() {
		t.Error("expected monitor to stop on context cancellation")
	}
}

func TestMonitor_CheckTimeout(t *testing.T) {
	manager := NewManager()

	// Create a checker that takes longer than timeout.
	slowChecker := &slowHealthChecker{delay: 100 * time.Millisecond}
	monitor := NewMonitor(manager, slowChecker,
		WithCheckTimeout(10*time.Millisecond),
	)

	manager.RegisterAgent("slow-agent")

	ctx := context.Background()
	start := time.Now()
	monitor.CheckAgentNow(ctx, "slow-agent")
	elapsed := time.Since(start)

	// Should timeout faster than checker delay.
	if elapsed >= 100*time.Millisecond {
		t.Errorf("expected check to timeout quickly, took %v", elapsed)
	}
}

type slowHealthChecker struct {
	delay time.Duration
}

func (s *slowHealthChecker) CheckHealth(ctx context.Context, _ string) error {
	select {
	case <-time.After(s.delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
