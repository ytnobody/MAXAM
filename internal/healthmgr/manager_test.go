package healthmgr

import (
	"context"
	"errors"
	"testing"
	"time"
)

// MockRecoveryHandler is a test implementation of RecoveryHandler.
type MockRecoveryHandler struct {
	startAgentFn func(ctx context.Context, name string) error
	stopAgentFn  func(ctx context.Context, name string) error
	callCount    int
}

func (m *MockRecoveryHandler) StartAgent(ctx context.Context, agentName string) error {
	m.callCount++
	if m.startAgentFn != nil {
		return m.startAgentFn(ctx, agentName)
	}
	return nil
}

func (m *MockRecoveryHandler) StopAgent(ctx context.Context, agentName string) error {
	if m.stopAgentFn != nil {
		return m.stopAgentFn(ctx, agentName)
	}
	return nil
}

func TestManager_RegisterAgent(t *testing.T) {
	m := NewManager()
	m.RegisterAgent("test-agent")

	health := m.GetHealth("test-agent")
	if health == nil {
		t.Fatal("agent not registered")
	}

	if health.AgentName != "test-agent" {
		t.Errorf("agent name mismatch: got %q, want test-agent", health.AgentName)
	}

	if health.Status != HealthStatusHealthy {
		t.Errorf("initial status should be healthy, got %q", health.Status)
	}
}

func TestManager_UpdateHealth(t *testing.T) {
	m := NewManager()
	m.RegisterAgent("test-agent")

	m.UpdateHealth("test-agent", HealthStatusUnresponsive, errors.New("timeout"))

	health := m.GetHealth("test-agent")
	if health.Status != HealthStatusUnresponsive {
		t.Errorf("status not updated: got %q, want unresponsive", health.Status)
	}

	if health.FailCount != 1 {
		t.Errorf("fail count not incremented: got %d, want 1", health.FailCount)
	}

	if health.LastError != "timeout" {
		t.Errorf("last error not set: got %q, want timeout", health.LastError)
	}
}

func TestManager_UpdateHealth_Recovered(t *testing.T) {
	m := NewManager()
	m.RegisterAgent("test-agent")

	// First, mark as unresponsive.
	m.UpdateHealth("test-agent", HealthStatusUnresponsive, errors.New("timeout"))
	health := m.GetHealth("test-agent")
	if health.FailCount != 1 {
		t.Fatalf("fail count should be 1, got %d", health.FailCount)
	}

	// Then mark as healthy (recovered).
	m.UpdateHealth("test-agent", HealthStatusHealthy, nil)
	health = m.GetHealth("test-agent")

	if health.Status != HealthStatusHealthy {
		t.Errorf("status not recovered: got %q, want healthy", health.Status)
	}

	if health.FailCount != 0 {
		t.Errorf("fail count not reset: got %d, want 0", health.FailCount)
	}
}

func TestManager_RecoverAgent_Success(t *testing.T) {
	handler := &MockRecoveryHandler{
		startAgentFn: func(ctx context.Context, name string) error {
			return nil // Success
		},
	}

	m := NewManager(
		WithRecoveryHandler(handler),
		WithMaxRetries(3),
	)
	m.RegisterAgent("test-agent")
	m.UpdateHealth("test-agent", HealthStatusUnresponsive, errors.New("timeout"))

	result := m.RecoverAgent(context.Background(), "test-agent")

	if !result.Success {
		t.Errorf("recovery should succeed, got %v", result.Success)
	}

	if handler.callCount != 1 {
		t.Errorf("should call StartAgent once, got %d calls", handler.callCount)
	}

	health := m.GetHealth("test-agent")
	if health.Status != HealthStatusHealthy {
		t.Errorf("status not recovered: got %q, want healthy", health.Status)
	}
}

func TestManager_RecoverAgent_MaxRetriesExceeded(t *testing.T) {
	handler := &MockRecoveryHandler{
		startAgentFn: func(ctx context.Context, name string) error {
			return errors.New("restart failed") // Always fail
		},
	}

	m := NewManager(
		WithRecoveryHandler(handler),
		WithMaxRetries(2),
		WithRetryDelay(10 * time.Millisecond), // Short delay for test
	)
	m.RegisterAgent("test-agent")

	result := m.RecoverAgent(context.Background(), "test-agent")

	if result.Success {
		t.Errorf("recovery should fail after max retries, got %v", result.Success)
	}

	if handler.callCount != 2 {
		t.Errorf("should retry 2 times, got %d calls", handler.callCount)
	}

	health := m.GetHealth("test-agent")
	if health.Status != HealthStatusDead {
		t.Errorf("status should be dead, got %q", health.Status)
	}
}

func TestManager_IsTaskHung(t *testing.T) {
	m := NewManager(WithTaskTimeout(100 * time.Millisecond))
	m.RegisterAgent("test-agent")

	// Initially, no task is running.
	if m.IsTaskHung("test-agent") {
		t.Error("should not be hung when no task is running")
	}

	// Start a task.
	m.SetTaskStartTime("test-agent", time.Now().Add(-200*time.Millisecond))

	if !m.IsTaskHung("test-agent") {
		t.Error("should be hung after timeout")
	}

	// Clear task.
	m.ClearTaskStartTime("test-agent")
	if m.IsTaskHung("test-agent") {
		t.Error("should not be hung after clearing task")
	}
}

func TestManager_GetAllHealth(t *testing.T) {
	m := NewManager()
	m.RegisterAgent("agent1")
	m.RegisterAgent("agent2")
	m.RegisterAgent("agent3")

	allHealth := m.GetAllHealth()

	if len(allHealth) != 3 {
		t.Errorf("should have 3 agents, got %d", len(allHealth))
	}

	for _, name := range []string{"agent1", "agent2", "agent3"} {
		if _, exists := allHealth[name]; !exists {
			t.Errorf("agent %q not found in all health", name)
		}
	}
}

func TestManager_IsHealthy(t *testing.T) {
	m := NewManager()
	m.RegisterAgent("test-agent")

	if !m.IsHealthy("test-agent") {
		t.Error("agent should be healthy initially")
	}

	m.UpdateHealth("test-agent", HealthStatusUnresponsive, errors.New("timeout"))
	if m.IsHealthy("test-agent") {
		t.Error("agent should not be healthy after marking unresponsive")
	}
}

func TestManager_RecoverAgent_DisabledAutoRestart(t *testing.T) {
	handler := &MockRecoveryHandler{}

	m := NewManager(
		WithRecoveryHandler(handler),
		WithAutoRestart(false),
	)
	m.RegisterAgent("test-agent")

	result := m.RecoverAgent(context.Background(), "test-agent")

	if result.Success {
		t.Error("recovery should fail when auto-restart is disabled")
	}

	if handler.callCount != 0 {
		t.Errorf("should not call StartAgent when disabled, got %d calls", handler.callCount)
	}
}
