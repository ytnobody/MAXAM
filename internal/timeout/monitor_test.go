package timeout

import (
	"context"
	"sync"
	"testing"
	"time"
)

// mockRecoveryHandler is a mock implementation of RecoveryHandler.
type mockRecoveryHandler struct {
	mu            sync.Mutex
	restartCalled bool
	restartAgent  string
	restartErr    error
}

func (m *mockRecoveryHandler) RestartAgent(ctx context.Context, agentName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.restartCalled = true
	m.restartAgent = agentName
	return m.restartErr
}

// mockReportOutput is a mock implementation of ReportOutput.
type mockReportOutput struct {
	mu         sync.Mutex
	sendCalled bool
	lastReport Report
	sendErr    error
}

func (m *mockReportOutput) SendReport(ctx context.Context, report Report) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendCalled = true
	m.lastReport = report
	return m.sendErr
}

func TestMonitorStartStop(t *testing.T) {
	cfg := Config{
		Timeout:       time.Minute,
		CheckInterval: 10 * time.Millisecond,
	}
	monitor := NewMonitor(cfg, nil, nil)

	if monitor.IsRunning() {
		t.Error("Monitor should not be running before Start()")
	}

	monitor.Start()
	if !monitor.IsRunning() {
		t.Error("Monitor should be running after Start()")
	}

	// Double start should be safe
	monitor.Start()
	if !monitor.IsRunning() {
		t.Error("Monitor should still be running after double Start()")
	}

	monitor.Stop()
	if monitor.IsRunning() {
		t.Error("Monitor should not be running after Stop()")
	}

	// Double stop should be safe
	monitor.Stop()
}

func TestMonitorRecordActivity(t *testing.T) {
	cfg := Config{
		Timeout:       time.Minute,
		CheckInterval: time.Hour, // Long interval to prevent auto-check
	}
	monitor := NewMonitor(cfg, nil, nil)

	ctx := AgentContext{
		IssueNumber: 367,
		TaskSummary: "テスト機能",
		Progress:    "実装中",
		NextAction:  "テスト作成",
	}

	monitor.RecordActivity("yuki-1", ctx)

	activity, exists := monitor.GetActivity("yuki-1")
	if !exists {
		t.Fatal("Activity should exist after RecordActivity()")
	}
	if activity.Context.IssueNumber != 367 {
		t.Errorf("IssueNumber = %d, want 367", activity.Context.IssueNumber)
	}
}

func TestMonitorTimeoutTrigger(t *testing.T) {
	handler := &mockRecoveryHandler{}
	output := &mockReportOutput{}

	cfg := Config{
		Timeout:       50 * time.Millisecond,
		CheckInterval: 20 * time.Millisecond,
	}
	monitor := NewMonitor(cfg, handler, output)

	ctx := AgentContext{
		IssueNumber: 100,
		TaskSummary: "タイムアウトテスト",
		Progress:    "待機中",
	}
	monitor.RecordActivity("timeout-agent", ctx)

	monitor.Start()
	defer monitor.Stop()

	// Wait for timeout and check
	time.Sleep(200 * time.Millisecond)

	handler.mu.Lock()
	restartCalled := handler.restartCalled
	restartAgent := handler.restartAgent
	handler.mu.Unlock()

	if !restartCalled {
		t.Error("RestartAgent should have been called")
	}
	if restartAgent != "timeout-agent" {
		t.Errorf("RestartAgent called with %q, want %q", restartAgent, "timeout-agent")
	}

	output.mu.Lock()
	sendCalled := output.sendCalled
	lastReport := output.lastReport
	output.mu.Unlock()

	if !sendCalled {
		t.Error("SendReport should have been called")
	}
	if lastReport.AgentName != "timeout-agent" {
		t.Errorf("Report.AgentName = %q, want %q", lastReport.AgentName, "timeout-agent")
	}
	if lastReport.IssueNumber != 100 {
		t.Errorf("Report.IssueNumber = %d, want 100", lastReport.IssueNumber)
	}
}

func TestMonitorNoTimeoutForActiveAgent(t *testing.T) {
	handler := &mockRecoveryHandler{}
	output := &mockReportOutput{}

	cfg := Config{
		Timeout:       100 * time.Millisecond,
		CheckInterval: 20 * time.Millisecond,
	}
	monitor := NewMonitor(cfg, handler, output)

	monitor.RecordActivity("active-agent", AgentContext{})

	monitor.Start()
	defer monitor.Stop()

	// Keep agent active by sending heartbeats
	for i := 0; i < 5; i++ {
		time.Sleep(30 * time.Millisecond)
		monitor.RecordHeartbeat("active-agent")
	}

	handler.mu.Lock()
	restartCalled := handler.restartCalled
	handler.mu.Unlock()

	if restartCalled {
		t.Error("RestartAgent should not be called for active agent")
	}
}

func TestMonitorUpdateProgress(t *testing.T) {
	cfg := Config{
		Timeout:       time.Minute,
		CheckInterval: time.Hour,
	}
	monitor := NewMonitor(cfg, nil, nil)

	monitor.RecordActivity("agent-1", AgentContext{Progress: "10%"})
	monitor.UpdateProgress("agent-1", "50%")

	activity, _ := monitor.GetActivity("agent-1")
	if activity.Context.Progress != "50%" {
		t.Errorf("Progress = %q, want %q", activity.Context.Progress, "50%")
	}
}

func TestMonitorRemoveAgent(t *testing.T) {
	cfg := Config{
		Timeout:       time.Minute,
		CheckInterval: time.Hour,
	}
	monitor := NewMonitor(cfg, nil, nil)

	monitor.RecordActivity("agent-1", AgentContext{})
	monitor.RemoveAgent("agent-1")

	_, exists := monitor.GetActivity("agent-1")
	if exists {
		t.Error("Agent should be removed")
	}
}

func TestMonitorDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Timeout != DefaultTimeout {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, DefaultTimeout)
	}
	if cfg.CheckInterval != DefaultCheckInterval {
		t.Errorf("CheckInterval = %v, want %v", cfg.CheckInterval, DefaultCheckInterval)
	}
}

func TestMonitorNilHandlers(t *testing.T) {
	cfg := Config{
		Timeout:       50 * time.Millisecond,
		CheckInterval: 20 * time.Millisecond,
	}
	// Both handler and output are nil
	monitor := NewMonitor(cfg, nil, nil)

	monitor.RecordActivity("agent", AgentContext{})
	monitor.Start()
	defer monitor.Stop()

	// Wait for timeout - should not panic
	time.Sleep(100 * time.Millisecond)
}

func TestMonitorGetTimedOutAgents(t *testing.T) {
	cfg := Config{
		Timeout:       50 * time.Millisecond,
		CheckInterval: time.Hour,
	}
	monitor := NewMonitor(cfg, nil, nil)

	monitor.RecordActivity("old-agent", AgentContext{})
	time.Sleep(100 * time.Millisecond)
	monitor.RecordActivity("new-agent", AgentContext{})

	timedOut := monitor.GetTimedOutAgents()

	if len(timedOut) != 1 {
		t.Fatalf("GetTimedOutAgents() returned %d, want 1", len(timedOut))
	}
	if timedOut[0] != "old-agent" {
		t.Errorf("Timed out agent = %q, want %q", timedOut[0], "old-agent")
	}
}
