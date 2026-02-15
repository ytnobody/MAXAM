package heartbeat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestDefaultFileMonitorConfig(t *testing.T) {
	cfg := DefaultFileMonitorConfig()

	if cfg.CheckInterval != 60*time.Second {
		t.Errorf("expected CheckInterval 60s, got %v", cfg.CheckInterval)
	}
	if cfg.DegradedTimeout != 60*time.Second {
		t.Errorf("expected DegradedTimeout 60s, got %v", cfg.DegradedTimeout)
	}
	if cfg.UnresponsiveTimeout != 120*time.Second {
		t.Errorf("expected UnresponsiveTimeout 120s, got %v", cfg.UnresponsiveTimeout)
	}
	if cfg.BaseDir == "" {
		t.Error("expected non-empty BaseDir")
	}
}

func TestAgentState_String(t *testing.T) {
	tests := []struct {
		state AgentState
		want  string
	}{
		{StateUnknown, "unknown"},
		{StateHealthy, "healthy"},
		{StateDegraded, "degraded"},
		{StateUnresponsive, "unresponsive"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func writeTestHeartbeat(t *testing.T, dir, agentName string, timestamp time.Time, status string) {
	t.Helper()

	hb := HeartbeatData{
		AgentName: agentName,
		Timestamp: timestamp,
		Status:    status,
		PID:       12345,
	}

	data, err := json.Marshal(hb)
	if err != nil {
		t.Fatalf("failed to marshal heartbeat: %v", err)
	}

	filePath := filepath.Join(dir, agentName+".json")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatalf("failed to write heartbeat file: %v", err)
	}
}

func TestFileMonitor_CheckOnce_Healthy(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := FileMonitorConfig{
		BaseDir:             tmpDir,
		CheckInterval:       100 * time.Millisecond,
		DegradedTimeout:     60 * time.Second,
		UnresponsiveTimeout: 120 * time.Second,
	}
	m := NewFileMonitor(cfg)

	// Write a fresh heartbeat
	now := time.Now()
	writeTestHeartbeat(t, tmpDir, "agent1", now.Add(-10*time.Second), "active")

	m.SetNowFunc(func() time.Time { return now })
	m.CheckOnce()

	health, exists := m.GetAgentHealth("agent1")
	if !exists {
		t.Fatal("agent1 should exist")
	}
	if health.State != StateHealthy {
		t.Errorf("expected healthy, got %v", health.State)
	}
	if health.AgentName != "agent1" {
		t.Errorf("expected agent name 'agent1', got %s", health.AgentName)
	}
}

func TestFileMonitor_CheckOnce_Degraded(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := FileMonitorConfig{
		BaseDir:             tmpDir,
		CheckInterval:       100 * time.Millisecond,
		DegradedTimeout:     60 * time.Second,
		UnresponsiveTimeout: 120 * time.Second,
	}
	m := NewFileMonitor(cfg)

	// Write a degraded heartbeat (90s old)
	now := time.Now()
	writeTestHeartbeat(t, tmpDir, "agent1", now.Add(-90*time.Second), "active")

	m.SetNowFunc(func() time.Time { return now })
	m.CheckOnce()

	health, exists := m.GetAgentHealth("agent1")
	if !exists {
		t.Fatal("agent1 should exist")
	}
	if health.State != StateDegraded {
		t.Errorf("expected degraded, got %v", health.State)
	}
}

func TestFileMonitor_CheckOnce_Unresponsive(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := FileMonitorConfig{
		BaseDir:             tmpDir,
		CheckInterval:       100 * time.Millisecond,
		DegradedTimeout:     60 * time.Second,
		UnresponsiveTimeout: 120 * time.Second,
	}
	m := NewFileMonitor(cfg)

	// Write an unresponsive heartbeat (150s old)
	now := time.Now()
	writeTestHeartbeat(t, tmpDir, "agent1", now.Add(-150*time.Second), "active")

	m.SetNowFunc(func() time.Time { return now })
	m.CheckOnce()

	health, exists := m.GetAgentHealth("agent1")
	if !exists {
		t.Fatal("agent1 should exist")
	}
	if health.State != StateUnresponsive {
		t.Errorf("expected unresponsive, got %v", health.State)
	}
}

func TestFileMonitor_MultipleAgents(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := FileMonitorConfig{
		BaseDir:             tmpDir,
		CheckInterval:       100 * time.Millisecond,
		DegradedTimeout:     60 * time.Second,
		UnresponsiveTimeout: 120 * time.Second,
	}
	m := NewFileMonitor(cfg)

	now := time.Now()
	writeTestHeartbeat(t, tmpDir, "healthy-agent", now.Add(-10*time.Second), "active")
	writeTestHeartbeat(t, tmpDir, "degraded-agent", now.Add(-90*time.Second), "idle")
	writeTestHeartbeat(t, tmpDir, "unresponsive-agent", now.Add(-150*time.Second), "processing")

	m.SetNowFunc(func() time.Time { return now })
	m.CheckOnce()

	all := m.GetAllAgentHealth()
	if len(all) != 3 {
		t.Errorf("expected 3 agents, got %d", len(all))
	}

	if all["healthy-agent"].State != StateHealthy {
		t.Errorf("healthy-agent should be healthy, got %v", all["healthy-agent"].State)
	}
	if all["degraded-agent"].State != StateDegraded {
		t.Errorf("degraded-agent should be degraded, got %v", all["degraded-agent"].State)
	}
	if all["unresponsive-agent"].State != StateUnresponsive {
		t.Errorf("unresponsive-agent should be unresponsive, got %v", all["unresponsive-agent"].State)
	}
}

func TestFileMonitor_GetUnresponsiveAgents(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := FileMonitorConfig{
		BaseDir:             tmpDir,
		DegradedTimeout:     60 * time.Second,
		UnresponsiveTimeout: 120 * time.Second,
	}
	m := NewFileMonitor(cfg)

	now := time.Now()
	writeTestHeartbeat(t, tmpDir, "healthy", now.Add(-10*time.Second), "active")
	writeTestHeartbeat(t, tmpDir, "unresponsive1", now.Add(-150*time.Second), "active")
	writeTestHeartbeat(t, tmpDir, "unresponsive2", now.Add(-200*time.Second), "active")

	m.SetNowFunc(func() time.Time { return now })
	m.CheckOnce()

	unresponsive := m.GetUnresponsiveAgents()
	if len(unresponsive) != 2 {
		t.Errorf("expected 2 unresponsive agents, got %d", len(unresponsive))
	}
}

func TestFileMonitor_GetDegradedAgents(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := FileMonitorConfig{
		BaseDir:             tmpDir,
		DegradedTimeout:     60 * time.Second,
		UnresponsiveTimeout: 120 * time.Second,
	}
	m := NewFileMonitor(cfg)

	now := time.Now()
	writeTestHeartbeat(t, tmpDir, "healthy", now.Add(-10*time.Second), "active")
	writeTestHeartbeat(t, tmpDir, "degraded1", now.Add(-90*time.Second), "active")
	writeTestHeartbeat(t, tmpDir, "degraded2", now.Add(-100*time.Second), "active")

	m.SetNowFunc(func() time.Time { return now })
	m.CheckOnce()

	degraded := m.GetDegradedAgents()
	if len(degraded) != 2 {
		t.Errorf("expected 2 degraded agents, got %d", len(degraded))
	}
}

func TestFileMonitor_AlertCallback(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := FileMonitorConfig{
		BaseDir:             tmpDir,
		DegradedTimeout:     60 * time.Second,
		UnresponsiveTimeout: 120 * time.Second,
	}
	m := NewFileMonitor(cfg)

	var callbackCalled atomic.Int32
	var lastHealth AgentHealth
	var lastPrevState AgentState

	m.SetAlertCallback(func(health AgentHealth, prevState AgentState) {
		callbackCalled.Add(1)
		lastHealth = health
		lastPrevState = prevState
	})

	now := time.Now()
	// First check - agent is degraded
	writeTestHeartbeat(t, tmpDir, "test-agent", now.Add(-90*time.Second), "active")

	m.SetNowFunc(func() time.Time { return now })
	m.CheckOnce()

	// Wait for callback
	time.Sleep(50 * time.Millisecond)

	if callbackCalled.Load() != 1 {
		t.Errorf("expected callback called once, got %d", callbackCalled.Load())
	}
	if lastHealth.AgentName != "test-agent" {
		t.Errorf("expected agent 'test-agent', got %s", lastHealth.AgentName)
	}
	if lastHealth.State != StateDegraded {
		t.Errorf("expected degraded state, got %v", lastHealth.State)
	}
	if lastPrevState != StateUnknown {
		t.Errorf("expected previous state unknown, got %v", lastPrevState)
	}
}

func TestFileMonitor_StartStop(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := FileMonitorConfig{
		BaseDir:       tmpDir,
		CheckInterval: 50 * time.Millisecond,
	}
	m := NewFileMonitor(cfg)

	err := m.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for some checks
	time.Sleep(150 * time.Millisecond)

	m.Stop()
}

func TestFileMonitor_StartStop_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := FileMonitorConfig{
		BaseDir: tmpDir,
	}
	m := NewFileMonitor(cfg)

	// Multiple starts should be safe
	_ = m.Start()
	_ = m.Start()
	_ = m.Start()

	// Multiple stops should be safe
	m.Stop()
	m.Stop()
	m.Stop()
}

func TestFormatAlert(t *testing.T) {
	health := AgentHealth{
		AgentName:     "test-agent",
		State:         StateUnresponsive,
		SinceLastBeat: 2*time.Minute + 30*time.Second,
	}

	alert := FormatAlert(health)
	expected := "[WARN] Agent 'test-agent' is unresponsive (last heartbeat: 2m30s ago)"
	if alert != expected {
		t.Errorf("expected %q, got %q", expected, alert)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{60 * time.Second, "1m"},
		{90 * time.Second, "1m30s"},
		{2 * time.Minute, "2m"},
		{2*time.Minute + 30*time.Second, "2m30s"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := formatDuration(tt.d); got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestFileMonitor_StateTransition(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := FileMonitorConfig{
		BaseDir:             tmpDir,
		DegradedTimeout:     60 * time.Second,
		UnresponsiveTimeout: 120 * time.Second,
	}
	m := NewFileMonitor(cfg)

	var callbackCount atomic.Int32
	m.SetAlertCallback(func(health AgentHealth, prevState AgentState) {
		callbackCount.Add(1)
	})

	// Start healthy
	now := time.Now()
	writeTestHeartbeat(t, tmpDir, "agent", now.Add(-10*time.Second), "active")
	m.SetNowFunc(func() time.Time { return now })
	m.CheckOnce()
	time.Sleep(20 * time.Millisecond)

	if callbackCount.Load() != 0 {
		t.Errorf("expected no callback for healthy, got %d", callbackCount.Load())
	}

	// Become degraded
	writeTestHeartbeat(t, tmpDir, "agent", now.Add(-90*time.Second), "active")
	m.CheckOnce()
	time.Sleep(20 * time.Millisecond)

	if callbackCount.Load() != 1 {
		t.Errorf("expected 1 callback for degraded, got %d", callbackCount.Load())
	}

	// Become unresponsive
	writeTestHeartbeat(t, tmpDir, "agent", now.Add(-150*time.Second), "active")
	m.CheckOnce()
	time.Sleep(20 * time.Millisecond)

	if callbackCount.Load() != 2 {
		t.Errorf("expected 2 callbacks total, got %d", callbackCount.Load())
	}
}
