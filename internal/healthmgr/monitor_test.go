package healthmgr

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockChecker is a mock implementation of HealthChecker for testing.
type mockChecker struct {
	mu        sync.Mutex
	responses map[string]error
	callCount atomic.Int32
}

func newMockChecker() *mockChecker {
	return &mockChecker{
		responses: make(map[string]error),
	}
}

func (m *mockChecker) SetResponse(agentName string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[agentName] = err
}

func (m *mockChecker) CheckHealth(_ context.Context, agentName string) error {
	m.callCount.Add(1)
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.responses[agentName]
}

func (m *mockChecker) GetCallCount() int {
	return int(m.callCount.Load())
}

func TestMonitor_StartStop(t *testing.T) {
	manager := NewManager()
	manager.RegisterAgent("agent1")

	checker := newMockChecker()
	monitor := NewMonitor(manager, 50*time.Millisecond, checker)

	// Initially not running.
	if monitor.IsRunning() {
		t.Error("monitor should not be running initially")
	}

	// Start monitoring.
	ctx := context.Background()
	monitor.Start(ctx)

	// Give it time to start.
	time.Sleep(10 * time.Millisecond)

	if !monitor.IsRunning() {
		t.Error("monitor should be running after Start()")
	}

	// Stop monitoring.
	monitor.Stop()

	// Give it time to stop.
	time.Sleep(10 * time.Millisecond)

	if monitor.IsRunning() {
		t.Error("monitor should not be running after Stop()")
	}
}

func TestMonitor_DoubleStart(t *testing.T) {
	manager := NewManager()
	checker := newMockChecker()
	monitor := NewMonitor(manager, 50*time.Millisecond, checker)

	ctx := context.Background()
	monitor.Start(ctx)
	defer monitor.Stop()

	// Double start should be no-op.
	monitor.Start(ctx)

	if !monitor.IsRunning() {
		t.Error("monitor should still be running after double Start()")
	}
}

func TestMonitor_ChecksAgents(t *testing.T) {
	manager := NewManager()
	manager.RegisterAgent("agent1")
	manager.RegisterAgent("agent2")

	checker := newMockChecker()
	monitor := NewMonitor(manager, 50*time.Millisecond, checker)

	ctx := context.Background()
	monitor.Start(ctx)
	defer monitor.Stop()

	// Wait for at least one check cycle.
	time.Sleep(80 * time.Millisecond)

	// Should have checked both agents at least once.
	if checker.GetCallCount() < 2 {
		t.Errorf("expected at least 2 health checks, got %d", checker.GetCallCount())
	}
}

func TestMonitor_DetectsUnhealthy(t *testing.T) {
	manager := NewManager()
	manager.RegisterAgent("agent1")

	checker := newMockChecker()
	checker.SetResponse("agent1", errors.New("timeout"))

	monitor := NewMonitor(manager, 50*time.Millisecond, checker)

	ctx := context.Background()
	monitor.Start(ctx)
	defer monitor.Stop()

	// Wait for check.
	time.Sleep(80 * time.Millisecond)

	health := manager.GetHealth("agent1")
	if health == nil {
		t.Fatal("expected health to be set")
	}

	// Should be unresponsive or dead (depending on recovery attempts).
	if health.Status == HealthStatusHealthy {
		t.Error("agent should not be healthy after error")
	}
}

func TestMonitor_ContextCancellation(t *testing.T) {
	manager := NewManager()
	manager.RegisterAgent("agent1")

	checker := newMockChecker()
	monitor := NewMonitor(manager, 50*time.Millisecond, checker)

	ctx, cancel := context.WithCancel(context.Background())
	monitor.Start(ctx)

	// Verify running.
	time.Sleep(10 * time.Millisecond)
	if !monitor.IsRunning() {
		t.Error("monitor should be running")
	}

	// Cancel context.
	cancel()

	// Wait for shutdown.
	time.Sleep(80 * time.Millisecond)

	if monitor.IsRunning() {
		t.Error("monitor should stop after context cancellation")
	}
}

func TestMonitor_NoCheckerNoPanic(t *testing.T) {
	manager := NewManager()
	manager.RegisterAgent("agent1")

	// No checker - should not panic.
	monitor := NewMonitor(manager, 50*time.Millisecond, nil)

	ctx := context.Background()
	monitor.Start(ctx)
	defer monitor.Stop()

	// Wait for check cycle.
	time.Sleep(80 * time.Millisecond)

	// Should complete without panic.
}

func TestMonitor_RecoveryOnError(t *testing.T) {
	// Create manager with mock recovery handler.
	recovered := atomic.Bool{}
	handler := &mockRecoveryHandler{
		startFunc: func(_ context.Context, _ string) error {
			recovered.Store(true)
			return nil
		},
	}

	manager := NewManager(WithRecoveryHandler(handler))
	manager.RegisterAgent("agent1")

	checker := newMockChecker()
	checker.SetResponse("agent1", errors.New("agent down"))

	monitor := NewMonitor(manager, 50*time.Millisecond, checker)

	ctx := context.Background()
	monitor.Start(ctx)
	defer monitor.Stop()

	// Wait for check and recovery.
	time.Sleep(200 * time.Millisecond)

	if !recovered.Load() {
		t.Error("expected recovery handler to be called")
	}
}

type mockRecoveryHandler struct {
	startFunc func(ctx context.Context, agentName string) error
	stopFunc  func(ctx context.Context, agentName string) error
}

func (h *mockRecoveryHandler) StartAgent(ctx context.Context, agentName string) error {
	if h.startFunc != nil {
		return h.startFunc(ctx, agentName)
	}
	return nil
}

func (h *mockRecoveryHandler) StopAgent(ctx context.Context, agentName string) error {
	if h.stopFunc != nil {
		return h.stopFunc(ctx, agentName)
	}
	return nil
}
