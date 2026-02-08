package recovery

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", cfg.MaxRetries)
	}
	if cfg.RetryDelay != 5*time.Second {
		t.Errorf("expected RetryDelay 5s, got %v", cfg.RetryDelay)
	}
	if cfg.NotifyTimeout != 10*time.Second {
		t.Errorf("expected NotifyTimeout 10s, got %v", cfg.NotifyTimeout)
	}
	if cfg.PMAgent != "mei" {
		t.Errorf("expected PMAgent 'mei', got %s", cfg.PMAgent)
	}
	if cfg.OwnerChannel != "owner" {
		t.Errorf("expected OwnerChannel 'owner', got %s", cfg.OwnerChannel)
	}
}

func TestHandler_HandleUnresponsive_RestartSuccess(t *testing.T) {
	restartCalled := false
	restartFn := func(ctx context.Context, name string) error {
		restartCalled = true
		return nil
	}

	cfg := Config{
		MaxRetries:    3,
		RetryDelay:    10 * time.Millisecond,
		NotifyTimeout: 100 * time.Millisecond,
	}
	h := NewHandler(cfg, restartFn, nil)

	ctx := context.Background()
	result := h.HandleUnresponsive(ctx, "worker1", 1)

	if !restartCalled {
		t.Error("restart should be called")
	}
	if !result {
		t.Error("should return true on successful restart")
	}

	state, exists := h.GetState("worker1")
	if !exists {
		t.Error("state should exist")
	}
	if !state.Recovered {
		t.Error("state should be marked as recovered")
	}
}

func TestHandler_HandleUnresponsive_RestartFail(t *testing.T) {
	restartFn := func(ctx context.Context, name string) error {
		return errors.New("restart failed")
	}

	var notifiedTarget string
	var mu sync.Mutex
	notifyFn := func(ctx context.Context, target, message string) error {
		mu.Lock()
		notifiedTarget = target
		mu.Unlock()
		return nil
	}

	cfg := Config{
		MaxRetries:    3,
		RetryDelay:    10 * time.Millisecond,
		NotifyTimeout: 100 * time.Millisecond,
		PMAgent:       "mei",
	}
	h := NewHandler(cfg, restartFn, notifyFn)

	ctx := context.Background()
	result := h.HandleUnresponsive(ctx, "worker1", 3) // At max retries

	if result {
		t.Error("should return false when restart fails")
	}

	// Wait for notification goroutine
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if notifiedTarget != "mei" {
		t.Errorf("expected PM 'mei' to be notified, got %s", notifiedTarget)
	}
	mu.Unlock()
}

func TestHandler_HandleUnresponsive_PMNotifyFail_OwnerFallback(t *testing.T) {
	restartFn := func(ctx context.Context, name string) error {
		return errors.New("restart failed")
	}

	var notifications []string
	var mu sync.Mutex
	notifyFn := func(ctx context.Context, target, message string) error {
		mu.Lock()
		notifications = append(notifications, target)
		mu.Unlock()
		if target == "mei" {
			return errors.New("PM unreachable")
		}
		return nil
	}

	cfg := Config{
		MaxRetries:    3,
		RetryDelay:    10 * time.Millisecond,
		NotifyTimeout: 100 * time.Millisecond,
		PMAgent:       "mei",
		OwnerChannel:  "owner",
	}
	h := NewHandler(cfg, restartFn, notifyFn)

	ctx := context.Background()
	h.HandleUnresponsive(ctx, "worker1", 3)

	// Wait for notification goroutines
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(notifications) != 2 {
		t.Errorf("expected 2 notifications (PM then owner), got %d: %v", len(notifications), notifications)
	}
	if len(notifications) >= 2 {
		if notifications[0] != "mei" {
			t.Errorf("expected first notification to mei, got %s", notifications[0])
		}
		if notifications[1] != "owner" {
			t.Errorf("expected second notification to owner, got %s", notifications[1])
		}
	}
}

func TestHandler_HandleUnresponsive_NoRestartBeyondMaxRetries(t *testing.T) {
	var restartCount atomic.Int32
	restartFn := func(ctx context.Context, name string) error {
		restartCount.Add(1)
		return errors.New("always fail")
	}

	cfg := Config{
		MaxRetries:    2,
		RetryDelay:    10 * time.Millisecond,
		NotifyTimeout: 100 * time.Millisecond,
	}
	h := NewHandler(cfg, restartFn, nil)

	ctx := context.Background()

	// Within retry limit
	h.HandleUnresponsive(ctx, "worker1", 1)
	h.HandleUnresponsive(ctx, "worker1", 2)

	// Beyond retry limit
	h.HandleUnresponsive(ctx, "worker1", 3)
	h.HandleUnresponsive(ctx, "worker1", 4)

	// Should only have tried restart twice (for failCount <= MaxRetries)
	if restartCount.Load() != 2 {
		t.Errorf("expected 2 restart attempts, got %d", restartCount.Load())
	}
}

// fastTestConfig returns a config with short timeouts for testing
func fastTestConfig() Config {
	return Config{
		MaxRetries:    3,
		RetryDelay:    10 * time.Millisecond,
		NotifyTimeout: 100 * time.Millisecond,
		PMAgent:       "mei",
		OwnerChannel:  "owner",
	}
}

func TestHandler_GetState(t *testing.T) {
	h := NewHandler(fastTestConfig(), nil, nil)

	_, exists := h.GetState("unknown")
	if exists {
		t.Error("unknown worker should not exist")
	}

	ctx := context.Background()
	h.HandleUnresponsive(ctx, "worker1", 2)

	state, exists := h.GetState("worker1")
	if !exists {
		t.Error("worker1 should exist after handling")
	}
	if state.WorkerName != "worker1" {
		t.Errorf("expected name worker1, got %s", state.WorkerName)
	}
	if state.Attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", state.Attempts)
	}
}

func TestHandler_ResetState(t *testing.T) {
	restartFn := func(ctx context.Context, name string) error {
		return nil
	}

	h := NewHandler(fastTestConfig(), restartFn, nil)

	ctx := context.Background()
	h.HandleUnresponsive(ctx, "worker1", 1)

	_, exists := h.GetState("worker1")
	if !exists {
		t.Error("worker1 should exist")
	}

	h.ResetState("worker1")

	_, exists = h.GetState("worker1")
	if exists {
		t.Error("worker1 should not exist after reset")
	}
}

func TestHandler_ResetAll(t *testing.T) {
	restartFn := func(ctx context.Context, name string) error {
		return nil
	}

	h := NewHandler(fastTestConfig(), restartFn, nil)

	ctx := context.Background()
	h.HandleUnresponsive(ctx, "worker1", 1)
	h.HandleUnresponsive(ctx, "worker2", 1)
	h.HandleUnresponsive(ctx, "worker3", 1)

	states := h.GetAllStates()
	if len(states) != 3 {
		t.Errorf("expected 3 states, got %d", len(states))
	}

	h.ResetAll()

	states = h.GetAllStates()
	if len(states) != 0 {
		t.Errorf("expected 0 states after reset, got %d", len(states))
	}
}

func TestHandler_GetAllStates(t *testing.T) {
	restartFn := func(ctx context.Context, name string) error {
		return nil
	}

	h := NewHandler(fastTestConfig(), restartFn, nil)

	ctx := context.Background()
	h.HandleUnresponsive(ctx, "a", 1)
	h.HandleUnresponsive(ctx, "b", 2)

	states := h.GetAllStates()

	if len(states) != 2 {
		t.Errorf("expected 2 states, got %d", len(states))
	}
	if _, ok := states["a"]; !ok {
		t.Error("state 'a' should exist")
	}
	if _, ok := states["b"]; !ok {
		t.Error("state 'b' should exist")
	}
}

func TestHandler_ConfigDefaults(t *testing.T) {
	cfg := Config{}
	h := NewHandler(cfg, nil, nil)

	if h.cfg.MaxRetries != DefaultConfig().MaxRetries {
		t.Errorf("expected default max retries, got %d", h.cfg.MaxRetries)
	}
	if h.cfg.RetryDelay != DefaultConfig().RetryDelay {
		t.Errorf("expected default retry delay, got %v", h.cfg.RetryDelay)
	}
	if h.cfg.NotifyTimeout != DefaultConfig().NotifyTimeout {
		t.Errorf("expected default notify timeout, got %v", h.cfg.NotifyTimeout)
	}
	if h.cfg.PMAgent != DefaultConfig().PMAgent {
		t.Errorf("expected default PM agent, got %s", h.cfg.PMAgent)
	}
	if h.cfg.OwnerChannel != DefaultConfig().OwnerChannel {
		t.Errorf("expected default owner channel, got %s", h.cfg.OwnerChannel)
	}
}

func TestHandler_NilRestartFn(t *testing.T) {
	h := NewHandler(DefaultConfig(), nil, nil)

	ctx := context.Background()
	result := h.HandleUnresponsive(ctx, "worker1", 1)

	if result {
		t.Error("should return false when restart fn is nil")
	}
}

func TestHandler_ContextCancellation(t *testing.T) {
	restartFn := func(ctx context.Context, name string) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			return nil
		}
	}

	cfg := Config{
		MaxRetries:    3,
		RetryDelay:    100 * time.Millisecond,
		NotifyTimeout: 100 * time.Millisecond,
	}
	h := NewHandler(cfg, restartFn, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result := h.HandleUnresponsive(ctx, "worker1", 1)

	if result {
		t.Error("should return false when context is cancelled")
	}
}
