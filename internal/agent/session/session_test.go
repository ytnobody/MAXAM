package session

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Timeout != DefaultTimeout {
		t.Errorf("expected timeout %v, got %v", DefaultTimeout, cfg.Timeout)
	}
	if cfg.MaxRestarts != DefaultMaxRestarts {
		t.Errorf("expected max restarts %d, got %d", DefaultMaxRestarts, cfg.MaxRestarts)
	}
}

func TestSession_RunCompletes(t *testing.T) {
	cfg := Config{
		Timeout: 1 * time.Second,
	}
	sess := New(cfg)

	called := false
	err := sess.Run(func(ctx context.Context) error {
		called = true
		return nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !called {
		t.Error("function was not called")
	}
	if sess.RestartCount() != 0 {
		t.Errorf("expected 0 restarts, got %d", sess.RestartCount())
	}
}

func TestSession_RunReturnsError(t *testing.T) {
	cfg := Config{
		Timeout: 1 * time.Second,
	}
	sess := New(cfg)

	expectedErr := errors.New("test error")
	err := sess.Run(func(ctx context.Context) error {
		return expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestSession_TimeoutAndRestart(t *testing.T) {
	var callCount int32
	var timeoutStatuses []string

	cfg := Config{
		Timeout:     50 * time.Millisecond,
		MaxRestarts: 3,
		StatusReporter: func(ctx context.Context) string {
			return "working on task"
		},
		OnTimeout: func(ctx context.Context, status string) {
			timeoutStatuses = append(timeoutStatuses, status)
		},
	}
	sess := New(cfg)

	err := sess.Run(func(ctx context.Context) error {
		atomic.AddInt32(&callCount, 1)
		// Simulate long-running work that will timeout
		<-ctx.Done()
		return nil // Return nil on timeout to allow restart
	})

	if err == nil || err.Error() != "max restarts (3) exceeded" {
		t.Errorf("expected max restarts error, got: %v", err)
	}

	// Should have been called 3 times: initial + 2 restarts, then max restarts check fails
	// restartCount: 0 -> run -> timeout -> 1 -> run -> timeout -> 2 -> run -> timeout -> 3 -> check fails
	if atomic.LoadInt32(&callCount) != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}

	// Should have 3 timeout callbacks
	if len(timeoutStatuses) != 3 {
		t.Errorf("expected 3 timeout statuses, got %d", len(timeoutStatuses))
	}

	for i, status := range timeoutStatuses {
		if status != "working on task" {
			t.Errorf("timeout %d: expected status 'working on task', got '%s'", i, status)
		}
	}
}

func TestSession_Stop(t *testing.T) {
	cfg := Config{
		Timeout: 10 * time.Second,
	}
	sess := New(cfg)

	done := make(chan struct{})
	go func() {
		sess.Run(func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		})
		close(done)
	}()

	// Give it time to start
	time.Sleep(10 * time.Millisecond)

	sess.Stop()

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Error("session did not stop in time")
	}
}

func TestSession_UnlimitedRestarts(t *testing.T) {
	var callCount int32

	cfg := Config{
		Timeout:     10 * time.Millisecond,
		MaxRestarts: 0, // Unlimited
	}
	sess := New(cfg)

	go func() {
		// Stop after a few iterations
		time.Sleep(100 * time.Millisecond)
		sess.Stop()
	}()

	err := sess.Run(func(ctx context.Context) error {
		atomic.AddInt32(&callCount, 1)
		<-ctx.Done()
		return nil
	})

	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got: %v", err)
	}

	// Should have been called multiple times
	count := atomic.LoadInt32(&callCount)
	if count < 3 {
		t.Errorf("expected at least 3 calls, got %d", count)
	}
}

func TestSession_IsRunning(t *testing.T) {
	cfg := Config{
		Timeout: 1 * time.Second,
	}
	sess := New(cfg)

	if sess.IsRunning() {
		t.Error("session should not be running before Run")
	}

	started := make(chan struct{})
	done := make(chan struct{})

	go func() {
		sess.Run(func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		})
		close(done)
	}()

	<-started
	if !sess.IsRunning() {
		t.Error("session should be running after Run starts")
	}

	sess.Stop()
	<-done

	if sess.IsRunning() {
		t.Error("session should not be running after Stop")
	}
}

func TestSession_Elapsed(t *testing.T) {
	cfg := Config{
		Timeout: 1 * time.Second,
	}
	sess := New(cfg)

	if sess.Elapsed() != 0 {
		t.Error("elapsed should be 0 before Run")
	}

	started := make(chan struct{})
	go func() {
		sess.Run(func(ctx context.Context) error {
			close(started)
			time.Sleep(50 * time.Millisecond)
			return nil
		})
	}()

	<-started
	time.Sleep(20 * time.Millisecond)

	elapsed := sess.Elapsed()
	if elapsed < 10*time.Millisecond {
		t.Errorf("elapsed should be at least 10ms, got %v", elapsed)
	}
}

func TestManager_Basic(t *testing.T) {
	mgr := NewManager()

	cfg := Config{
		Timeout: 100 * time.Millisecond,
	}

	sess1 := mgr.Create("agent1", cfg)
	sess2 := mgr.Create("agent2", cfg)

	if sess1 == nil || sess2 == nil {
		t.Fatal("sessions should not be nil")
	}

	// Get existing session
	got, ok := mgr.Get("agent1")
	if !ok || got != sess1 {
		t.Error("Get should return the created session")
	}

	// Get non-existing session
	_, ok = mgr.Get("nonexistent")
	if ok {
		t.Error("Get should return false for non-existing session")
	}

	// Stop specific session
	go func() {
		sess1.Run(func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		})
	}()
	time.Sleep(10 * time.Millisecond)

	if !mgr.Stop("agent1") {
		t.Error("Stop should return true for existing session")
	}
	if mgr.Stop("nonexistent") {
		t.Error("Stop should return false for non-existing session")
	}
}

func TestManager_Status(t *testing.T) {
	mgr := NewManager()

	cfg := Config{
		Timeout: 1 * time.Second,
	}

	sess := mgr.Create("test", cfg)

	started := make(chan struct{})
	go func() {
		sess.Run(func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		})
	}()

	<-started
	time.Sleep(10 * time.Millisecond)

	status := mgr.Status()
	if len(status) != 1 {
		t.Errorf("expected 1 status, got %d", len(status))
	}

	testStatus, ok := status["test"]
	if !ok {
		t.Fatal("expected 'test' in status")
	}

	if !testStatus.Running {
		t.Error("session should be running")
	}
	if testStatus.RestartCount != 0 {
		t.Errorf("expected 0 restarts, got %d", testStatus.RestartCount)
	}

	mgr.StopAll()
}

func TestManager_StopAll(t *testing.T) {
	mgr := NewManager()

	cfg := Config{
		Timeout: 1 * time.Second,
	}

	sess1 := mgr.Create("agent1", cfg)
	sess2 := mgr.Create("agent2", cfg)

	done1 := make(chan struct{})
	done2 := make(chan struct{})

	go func() {
		sess1.Run(func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		})
		close(done1)
	}()

	go func() {
		sess2.Run(func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		})
		close(done2)
	}()

	time.Sleep(10 * time.Millisecond)

	mgr.StopAll()

	select {
	case <-done1:
	case <-time.After(1 * time.Second):
		t.Error("agent1 did not stop in time")
	}

	select {
	case <-done2:
	case <-time.After(1 * time.Second):
		t.Error("agent2 did not stop in time")
	}
}
