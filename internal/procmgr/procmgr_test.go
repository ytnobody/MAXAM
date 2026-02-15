package procmgr

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestProcessState_String(t *testing.T) {
	tests := []struct {
		state    ProcessState
		expected string
	}{
		{StateNotStarted, "not_started"},
		{StateRunning, "running"},
		{StateStopped, "stopped"},
		{StateRestarting, "restarting"},
		{ProcessState(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.state.String(); got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.processes == nil {
		t.Fatal("processes map is nil")
	}
	m.Close()
}

func TestManager_Start_SimpleProcess(t *testing.T) {
	// Skip if sleep command not available
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep command not available")
	}

	m := NewManager()
	defer m.Close()

	cfg := ProcessConfig{
		Name:    "test-sleep",
		Command: "sleep",
		Args:    []string{"0.1"},
		WorkDir: t.TempDir(),
	}

	err := m.Start(cfg)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Check process is running
	if !m.IsRunning("test-sleep") {
		t.Error("process should be running")
	}

	// Wait for process to finish
	time.Sleep(200 * time.Millisecond)

	// Process should be stopped now
	if m.IsRunning("test-sleep") {
		t.Error("process should be stopped after sleep completed")
	}
}

func TestManager_Start_DuplicateName(t *testing.T) {
	m := NewManager()
	defer m.Close()

	cfg := ProcessConfig{
		Name:    "dup-test",
		Command: "sleep",
		Args:    []string{"10"},
		WorkDir: t.TempDir(),
	}

	if err := m.Start(cfg); err != nil {
		t.Fatalf("first Start failed: %v", err)
	}

	// Second start should fail
	err := m.Start(cfg)
	if err == nil {
		t.Error("expected error for duplicate process name")
	}
}

func TestManager_Stop(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep command not available")
	}

	m := NewManager()
	defer m.Close()

	cfg := ProcessConfig{
		Name:    "stop-test",
		Command: "sleep",
		Args:    []string{"60"},
		WorkDir: t.TempDir(),
	}

	if err := m.Start(cfg); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Stop the process
	if err := m.Stop("stop-test"); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Give it a moment
	time.Sleep(100 * time.Millisecond)

	if m.IsRunning("stop-test") {
		t.Error("process should be stopped")
	}
}

func TestManager_Stop_NotFound(t *testing.T) {
	m := NewManager()
	defer m.Close()

	err := m.Stop("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent process")
	}
}

func TestManager_Kill(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep command not available")
	}

	m := NewManager()
	defer m.Close()

	cfg := ProcessConfig{
		Name:    "kill-test",
		Command: "sleep",
		Args:    []string{"60"},
		WorkDir: t.TempDir(),
	}

	if err := m.Start(cfg); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Kill the process
	if err := m.Kill("kill-test"); err != nil {
		t.Fatalf("Kill failed: %v", err)
	}

	if m.IsRunning("kill-test") {
		t.Error("process should be killed")
	}
}

func TestManager_GetInfo(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep command not available")
	}

	m := NewManager()
	defer m.Close()

	workDir := t.TempDir()
	cfg := ProcessConfig{
		Name:    "info-test",
		Command: "sleep",
		Args:    []string{"60"},
		WorkDir: workDir,
	}

	if err := m.Start(cfg); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	info, exists := m.GetInfo("info-test")
	if !exists {
		t.Fatal("GetInfo should return true for existing process")
	}

	if info.Name != "info-test" {
		t.Errorf("expected name 'info-test', got %s", info.Name)
	}
	if info.State != StateRunning {
		t.Errorf("expected state Running, got %v", info.State)
	}
	if info.PID <= 0 {
		t.Error("PID should be positive")
	}
	if info.WorkDir != workDir {
		t.Errorf("expected workdir %s, got %s", workDir, info.WorkDir)
	}
}

func TestManager_GetInfo_NotFound(t *testing.T) {
	m := NewManager()
	defer m.Close()

	_, exists := m.GetInfo("nonexistent")
	if exists {
		t.Error("GetInfo should return false for nonexistent process")
	}
}

func TestManager_GetAllInfo(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep command not available")
	}

	m := NewManager()
	defer m.Close()

	// Start multiple processes
	for _, name := range []string{"proc1", "proc2", "proc3"} {
		cfg := ProcessConfig{
			Name:    name,
			Command: "sleep",
			Args:    []string{"60"},
			WorkDir: t.TempDir(),
		}
		if err := m.Start(cfg); err != nil {
			t.Fatalf("Start %s failed: %v", name, err)
		}
	}

	all := m.GetAllInfo()
	if len(all) != 3 {
		t.Errorf("expected 3 processes, got %d", len(all))
	}

	for _, name := range []string{"proc1", "proc2", "proc3"} {
		if _, ok := all[name]; !ok {
			t.Errorf("process %s not found", name)
		}
	}
}

func TestManager_Restart(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep command not available")
	}

	m := NewManager()
	defer m.Close()

	cfg := ProcessConfig{
		Name:    "restart-test",
		Command: "sleep",
		Args:    []string{"60"},
		WorkDir: t.TempDir(),
	}

	if err := m.Start(cfg); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Get original PID
	info1, _ := m.GetInfo("restart-test")
	originalPID := info1.PID

	// Restart
	if err := m.Restart("restart-test"); err != nil {
		t.Fatalf("Restart failed: %v", err)
	}

	// Give it a moment
	time.Sleep(100 * time.Millisecond)

	// Check it's running with new PID
	info2, exists := m.GetInfo("restart-test")
	if !exists {
		t.Fatal("process should exist after restart")
	}
	if info2.State != StateRunning {
		t.Errorf("expected Running, got %v", info2.State)
	}
	if info2.PID == originalPID {
		t.Error("PID should be different after restart")
	}
	if info2.RestartCount != 1 {
		t.Errorf("expected restart count 1, got %d", info2.RestartCount)
	}
}

func TestManager_Remove(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep command not available")
	}

	m := NewManager()
	defer m.Close()

	cfg := ProcessConfig{
		Name:    "remove-test",
		Command: "sleep",
		Args:    []string{"0.1"},
		WorkDir: t.TempDir(),
	}

	if err := m.Start(cfg); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Can't remove running process
	if err := m.Remove("remove-test"); err == nil {
		t.Error("should not be able to remove running process")
	}

	// Wait for process to stop
	time.Sleep(200 * time.Millisecond)

	// Now we can remove
	if err := m.Remove("remove-test"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	_, exists := m.GetInfo("remove-test")
	if exists {
		t.Error("process should not exist after remove")
	}
}

func TestManager_StopAll(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep command not available")
	}

	m := NewManager()
	defer m.Close()

	// Start multiple processes
	for _, name := range []string{"all1", "all2", "all3"} {
		cfg := ProcessConfig{
			Name:    name,
			Command: "sleep",
			Args:    []string{"60"},
			WorkDir: t.TempDir(),
		}
		if err := m.Start(cfg); err != nil {
			t.Fatalf("Start %s failed: %v", name, err)
		}
	}

	// Verify all running
	for _, name := range []string{"all1", "all2", "all3"} {
		if !m.IsRunning(name) {
			t.Errorf("process %s should be running", name)
		}
	}

	m.StopAll()

	// Give processes time to stop
	time.Sleep(200 * time.Millisecond)

	// Verify all stopped
	for _, name := range []string{"all1", "all2", "all3"} {
		if m.IsRunning(name) {
			t.Errorf("process %s should be stopped", name)
		}
	}
}

func TestManager_SetOnExit(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep command not available")
	}

	m := NewManager()
	defer m.Close()

	cfg := ProcessConfig{
		Name:    "exit-callback-test",
		Command: "sleep",
		Args:    []string{"0.1"},
		WorkDir: t.TempDir(),
	}

	if err := m.Start(cfg); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	exitCalled := make(chan bool, 1)
	m.SetOnExit("exit-callback-test", func(name string, err error) {
		if name == "exit-callback-test" {
			exitCalled <- true
		}
	})

	// Wait for process to exit
	select {
	case <-exitCalled:
		// Good
	case <-time.After(2 * time.Second):
		t.Error("exit callback was not called")
	}
}

func TestRecoveryAdapter_StartAgent(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep command not available")
	}

	m := NewManager()
	defer m.Close()

	workDir := t.TempDir()
	configBuilder := func(agentName string) ProcessConfig {
		return ProcessConfig{
			Name:    agentName,
			Command: "sleep",
			Args:    []string{"60"},
			WorkDir: workDir,
		}
	}

	adapter := NewRecoveryAdapter(m, configBuilder)

	// Start agent
	ctx := context.Background()
	if err := adapter.StartAgent(ctx, "test-agent"); err != nil {
		t.Fatalf("StartAgent failed: %v", err)
	}

	if !m.IsRunning("test-agent") {
		t.Error("agent should be running after StartAgent")
	}
}

func TestRecoveryAdapter_StartAgent_Restart(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep command not available")
	}

	m := NewManager()
	defer m.Close()

	workDir := t.TempDir()
	configBuilder := func(agentName string) ProcessConfig {
		return ProcessConfig{
			Name:    agentName,
			Command: "sleep",
			Args:    []string{"60"},
			WorkDir: workDir,
		}
	}

	adapter := NewRecoveryAdapter(m, configBuilder)
	ctx := context.Background()

	// Start agent first time
	if err := adapter.StartAgent(ctx, "restart-agent"); err != nil {
		t.Fatalf("first StartAgent failed: %v", err)
	}

	info1, _ := m.GetInfo("restart-agent")
	originalPID := info1.PID

	// Start again (should restart)
	if err := adapter.StartAgent(ctx, "restart-agent"); err != nil {
		t.Fatalf("second StartAgent failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	info2, _ := m.GetInfo("restart-agent")
	if info2.PID == originalPID {
		t.Error("PID should be different after restart")
	}
	if info2.RestartCount != 1 {
		t.Errorf("expected restart count 1, got %d", info2.RestartCount)
	}
}

func TestRecoveryAdapter_StopAgent(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep command not available")
	}

	m := NewManager()
	defer m.Close()

	workDir := t.TempDir()
	configBuilder := func(agentName string) ProcessConfig {
		return ProcessConfig{
			Name:    agentName,
			Command: "sleep",
			Args:    []string{"60"},
			WorkDir: workDir,
		}
	}

	adapter := NewRecoveryAdapter(m, configBuilder)
	ctx := context.Background()

	// Start then stop
	if err := adapter.StartAgent(ctx, "stop-agent"); err != nil {
		t.Fatalf("StartAgent failed: %v", err)
	}

	if err := adapter.StopAgent(ctx, "stop-agent"); err != nil {
		t.Fatalf("StopAgent failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if m.IsRunning("stop-agent") {
		t.Error("agent should be stopped after StopAgent")
	}
}

func TestRecoveryAdapter_StopAgent_NotRunning(t *testing.T) {
	m := NewManager()
	defer m.Close()

	configBuilder := func(agentName string) ProcessConfig {
		return ProcessConfig{
			Name:    agentName,
			Command: "sleep",
			Args:    []string{"60"},
			WorkDir: t.TempDir(),
		}
	}

	adapter := NewRecoveryAdapter(m, configBuilder)
	ctx := context.Background()

	// Stop non-running agent should succeed (no-op)
	if err := adapter.StopAgent(ctx, "nonexistent"); err != nil {
		t.Fatalf("StopAgent should succeed for non-running agent: %v", err)
	}
}
