package procmgr

import (
	"context"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	t.Run("creates manager with default factory", func(t *testing.T) {
		cfg := DefaultConfig()
		mgr := NewManager(cfg, nil)

		if mgr == nil {
			t.Fatal("expected manager, got nil")
		}
		if mgr.config.GracefulTimeout != 10*time.Second {
			t.Errorf("expected GracefulTimeout 10s, got %v", mgr.config.GracefulTimeout)
		}
	})

	t.Run("creates manager with custom factory", func(t *testing.T) {
		cfg := DefaultConfig()
		factory := func(name string) *exec.Cmd {
			return exec.Command("echo", name)
		}
		mgr := NewManager(cfg, factory)

		if mgr == nil {
			t.Fatal("expected manager, got nil")
		}
		if mgr.factory == nil {
			t.Error("expected factory to be set")
		}
	})
}

func TestManager_StartStop(t *testing.T) {
	t.Run("start and stop manager", func(t *testing.T) {
		cfg := DefaultConfig()
		mgr := NewManager(cfg, nil)

		ctx := context.Background()
		mgr.Start(ctx)

		if !mgr.IsRunning() {
			t.Error("expected manager to be running after Start")
		}

		mgr.Stop()

		if mgr.IsRunning() {
			t.Error("expected manager to be stopped after Stop")
		}
	})

	t.Run("multiple start calls are idempotent", func(t *testing.T) {
		cfg := DefaultConfig()
		mgr := NewManager(cfg, nil)

		ctx := context.Background()
		mgr.Start(ctx)
		mgr.Start(ctx) // Should not panic

		if !mgr.IsRunning() {
			t.Error("expected manager to be running")
		}

		mgr.Stop()
	})
}

func TestManager_StartAgent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping signal tests on Windows")
	}

	t.Run("start agent process", func(t *testing.T) {
		cfg := DefaultConfig()
		factory := func(name string) *exec.Cmd {
			return exec.Command("sleep", "10")
		}
		mgr := NewManager(cfg, factory)

		ctx := context.Background()
		mgr.Start(ctx)
		defer mgr.Stop()

		err := mgr.StartAgent(ctx, "test-agent")
		if err != nil {
			t.Fatalf("failed to start agent: %v", err)
		}

		proc := mgr.GetProcess("test-agent")
		if proc == nil {
			t.Fatal("expected process, got nil")
		}
		if proc.State() != StateRunning {
			t.Errorf("expected state Running, got %v", proc.State())
		}
		if proc.PID() == 0 {
			t.Error("expected non-zero PID")
		}
	})

	t.Run("start agent fails when manager not running", func(t *testing.T) {
		cfg := DefaultConfig()
		mgr := NewManager(cfg, nil)

		err := mgr.StartAgent(context.Background(), "test-agent")
		if err == nil {
			t.Error("expected error when manager not running")
		}
	})

	t.Run("start agent with nil factory result", func(t *testing.T) {
		cfg := DefaultConfig()
		factory := func(name string) *exec.Cmd {
			return nil
		}
		mgr := NewManager(cfg, factory)

		ctx := context.Background()
		mgr.Start(ctx)
		defer mgr.Stop()

		err := mgr.StartAgent(ctx, "test-agent")
		if err == nil {
			t.Error("expected error when factory returns nil")
		}
	})
}

func TestManager_StopAgent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping signal tests on Windows")
	}

	t.Run("stop running agent gracefully", func(t *testing.T) {
		cfg := Config{
			GracefulTimeout: 5 * time.Second,
			StartTimeout:    30 * time.Second,
		}
		factory := func(name string) *exec.Cmd {
			return exec.Command("sleep", "30")
		}
		mgr := NewManager(cfg, factory)

		ctx := context.Background()
		mgr.Start(ctx)
		defer mgr.Stop()

		err := mgr.StartAgent(ctx, "test-agent")
		if err != nil {
			t.Fatalf("failed to start agent: %v", err)
		}

		// Give the process time to fully start
		time.Sleep(100 * time.Millisecond)

		err = mgr.StopAgent(ctx, "test-agent")
		if err != nil {
			t.Errorf("failed to stop agent: %v", err)
		}

		proc := mgr.GetProcess("test-agent")
		// State should be stopped or failed (terminated by signal)
		state := proc.State()
		if state != StateStopped && state != StateFailed {
			t.Errorf("expected state Stopped or Failed, got %v", state)
		}
	})

	t.Run("stop non-existent agent", func(t *testing.T) {
		cfg := DefaultConfig()
		mgr := NewManager(cfg, nil)

		ctx := context.Background()
		mgr.Start(ctx)
		defer mgr.Stop()

		err := mgr.StopAgent(ctx, "non-existent")
		if err == nil {
			t.Error("expected error for non-existent agent")
		}
	})

	t.Run("stop already stopped agent", func(t *testing.T) {
		cfg := DefaultConfig()
		factory := func(name string) *exec.Cmd {
			return exec.Command("echo", "quick")
		}
		mgr := NewManager(cfg, factory)

		ctx := context.Background()
		mgr.Start(ctx)
		defer mgr.Stop()

		err := mgr.StartAgent(ctx, "test-agent")
		if err != nil {
			t.Fatalf("failed to start agent: %v", err)
		}

		// Wait for echo to complete
		time.Sleep(100 * time.Millisecond)

		// Should not error
		err = mgr.StopAgent(ctx, "test-agent")
		if err != nil {
			t.Errorf("unexpected error stopping already-stopped agent: %v", err)
		}
	})
}

func TestManager_GetAllProcesses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping signal tests on Windows")
	}

	t.Run("returns all processes", func(t *testing.T) {
		cfg := DefaultConfig()
		factory := func(name string) *exec.Cmd {
			return exec.Command("sleep", "10")
		}
		mgr := NewManager(cfg, factory)

		ctx := context.Background()
		mgr.Start(ctx)
		defer mgr.Stop()

		_ = mgr.StartAgent(ctx, "agent1")
		_ = mgr.StartAgent(ctx, "agent2")

		procs := mgr.GetAllProcesses()
		if len(procs) != 2 {
			t.Errorf("expected 2 processes, got %d", len(procs))
		}
		if _, ok := procs["agent1"]; !ok {
			t.Error("expected agent1 in processes")
		}
		if _, ok := procs["agent2"]; !ok {
			t.Error("expected agent2 in processes")
		}
	})
}

func TestProcessState_String(t *testing.T) {
	tests := []struct {
		state    ProcessState
		expected string
	}{
		{StateIdle, "idle"},
		{StateRunning, "running"},
		{StateStopping, "stopping"},
		{StateStopped, "stopped"},
		{StateFailed, "failed"},
		{ProcessState(99), "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			if got := tc.state.String(); got != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.GracefulTimeout != 10*time.Second {
		t.Errorf("expected GracefulTimeout 10s, got %v", cfg.GracefulTimeout)
	}
	if cfg.StartTimeout != 30*time.Second {
		t.Errorf("expected StartTimeout 30s, got %v", cfg.StartTimeout)
	}
}

func TestProcess_Getters(t *testing.T) {
	t.Run("process getters return correct values", func(t *testing.T) {
		proc := &Process{
			name:      "test",
			state:     StateRunning,
			startedAt: time.Now(),
		}

		if proc.Name() != "test" {
			t.Errorf("expected name 'test', got %q", proc.Name())
		}
		if proc.State() != StateRunning {
			t.Errorf("expected state Running, got %v", proc.State())
		}
		if proc.StartedAt().IsZero() {
			t.Error("expected non-zero startedAt")
		}
		if !proc.StoppedAt().IsZero() {
			t.Error("expected zero stoppedAt")
		}
		if proc.LastError() != nil {
			t.Error("expected nil lastError")
		}
	})
}
