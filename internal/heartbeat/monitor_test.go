package heartbeat

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

	if cfg.Interval != 10*time.Second {
		t.Errorf("expected Interval 10s, got %v", cfg.Interval)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("expected Timeout 30s, got %v", cfg.Timeout)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", cfg.MaxRetries)
	}
}

func TestStatus_String(t *testing.T) {
	tests := []struct {
		status Status
		want   string
	}{
		{StatusUnknown, "unknown"},
		{StatusHealthy, "healthy"},
		{StatusUnresponsive, "unresponsive"},
		{StatusDead, "dead"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMonitor_RegisterUnregister(t *testing.T) {
	m := NewMonitor(DefaultConfig(), nil)

	m.RegisterWorker("worker1")
	m.RegisterWorker("worker2")

	health, exists := m.GetHealth("worker1")
	if !exists {
		t.Error("worker1 should exist")
	}
	if health.Name != "worker1" {
		t.Errorf("expected name worker1, got %s", health.Name)
	}

	m.UnregisterWorker("worker1")
	_, exists = m.GetHealth("worker1")
	if exists {
		t.Error("worker1 should not exist after unregister")
	}

	_, exists = m.GetHealth("worker2")
	if !exists {
		t.Error("worker2 should still exist")
	}
}

func TestMonitor_GetAllHealth(t *testing.T) {
	m := NewMonitor(DefaultConfig(), nil)
	m.RegisterWorker("a")
	m.RegisterWorker("b")
	m.RegisterWorker("c")

	all := m.GetAllHealth()
	if len(all) != 3 {
		t.Errorf("expected 3 workers, got %d", len(all))
	}

	for _, name := range []string{"a", "b", "c"} {
		if _, ok := all[name]; !ok {
			t.Errorf("worker %s not found", name)
		}
	}
}

func TestMonitor_CheckWorker_Healthy(t *testing.T) {
	pingFn := func(ctx context.Context, name string) error {
		return nil // Always healthy
	}

	cfg := Config{
		Interval:   100 * time.Millisecond,
		Timeout:    1 * time.Second,
		MaxRetries: 3,
	}
	m := NewMonitor(cfg, pingFn)
	m.RegisterWorker("test")

	m.checkWorker("test")

	health, _ := m.GetHealth("test")
	if health.Status != StatusHealthy {
		t.Errorf("expected healthy, got %v", health.Status)
	}
	if health.FailCount != 0 {
		t.Errorf("expected 0 failures, got %d", health.FailCount)
	}
}

func TestMonitor_CheckWorker_Unresponsive(t *testing.T) {
	pingFn := func(ctx context.Context, name string) error {
		return errors.New("timeout")
	}

	cfg := Config{
		Interval:   100 * time.Millisecond,
		Timeout:    1 * time.Second,
		MaxRetries: 3,
	}
	m := NewMonitor(cfg, pingFn)
	m.RegisterWorker("test")

	// First failure
	m.checkWorker("test")
	health, _ := m.GetHealth("test")
	if health.Status != StatusUnresponsive {
		t.Errorf("expected unresponsive, got %v", health.Status)
	}
	if health.FailCount != 1 {
		t.Errorf("expected 1 failure, got %d", health.FailCount)
	}

	// Second failure
	m.checkWorker("test")
	health, _ = m.GetHealth("test")
	if health.FailCount != 2 {
		t.Errorf("expected 2 failures, got %d", health.FailCount)
	}
}

func TestMonitor_CheckWorker_Dead(t *testing.T) {
	pingFn := func(ctx context.Context, name string) error {
		return errors.New("timeout")
	}

	cfg := Config{
		Interval:   100 * time.Millisecond,
		Timeout:    1 * time.Second,
		MaxRetries: 2,
	}
	m := NewMonitor(cfg, pingFn)
	m.RegisterWorker("test")

	// Exceed max retries
	m.checkWorker("test")
	m.checkWorker("test")

	health, _ := m.GetHealth("test")
	if health.Status != StatusDead {
		t.Errorf("expected dead, got %v", health.Status)
	}
}

func TestMonitor_UnresponsiveCallback(t *testing.T) {
	pingFn := func(ctx context.Context, name string) error {
		return errors.New("timeout")
	}

	cfg := Config{
		Interval:   100 * time.Millisecond,
		Timeout:    1 * time.Second,
		MaxRetries: 3,
	}
	m := NewMonitor(cfg, pingFn)

	var callbackCalled atomic.Int32
	var callbackName string
	var callbackFailCount int
	var mu sync.Mutex

	m.SetUnresponsiveCallback(func(name string, failCount int) {
		callbackCalled.Add(1)
		mu.Lock()
		callbackName = name
		callbackFailCount = failCount
		mu.Unlock()
	})

	m.RegisterWorker("test")
	m.checkWorker("test")

	// Wait for callback
	time.Sleep(50 * time.Millisecond)

	if callbackCalled.Load() != 1 {
		t.Errorf("expected callback called once, got %d", callbackCalled.Load())
	}
	mu.Lock()
	if callbackName != "test" {
		t.Errorf("expected name 'test', got %s", callbackName)
	}
	if callbackFailCount != 1 {
		t.Errorf("expected failCount 1, got %d", callbackFailCount)
	}
	mu.Unlock()
}

func TestMonitor_ResetFailCount(t *testing.T) {
	pingFn := func(ctx context.Context, name string) error {
		return errors.New("timeout")
	}

	cfg := Config{
		Interval:   100 * time.Millisecond,
		Timeout:    1 * time.Second,
		MaxRetries: 3,
	}
	m := NewMonitor(cfg, pingFn)
	m.RegisterWorker("test")

	m.checkWorker("test")
	m.checkWorker("test")

	health, _ := m.GetHealth("test")
	if health.FailCount != 2 {
		t.Errorf("expected 2 failures, got %d", health.FailCount)
	}

	m.ResetFailCount("test")

	health, _ = m.GetHealth("test")
	if health.FailCount != 0 {
		t.Errorf("expected 0 failures after reset, got %d", health.FailCount)
	}
	if health.Status != StatusUnknown {
		t.Errorf("expected unknown status after reset, got %v", health.Status)
	}
}

func TestMonitor_IsHealthy(t *testing.T) {
	var healthy bool
	pingFn := func(ctx context.Context, name string) error {
		if healthy {
			return nil
		}
		return errors.New("error")
	}

	m := NewMonitor(DefaultConfig(), pingFn)
	m.RegisterWorker("test")

	if m.IsHealthy("test") {
		t.Error("should not be healthy initially")
	}

	healthy = true
	m.checkWorker("test")

	if !m.IsHealthy("test") {
		t.Error("should be healthy after successful ping")
	}

	if m.IsHealthy("nonexistent") {
		t.Error("nonexistent worker should not be healthy")
	}
}

func TestMonitor_GetUnresponsiveWorkers(t *testing.T) {
	callCount := make(map[string]int)
	var mu sync.Mutex
	pingFn := func(ctx context.Context, name string) error {
		mu.Lock()
		callCount[name]++
		mu.Unlock()
		if name == "healthy" {
			return nil
		}
		return errors.New("error")
	}

	cfg := Config{
		Interval:   100 * time.Millisecond,
		Timeout:    1 * time.Second,
		MaxRetries: 3,
	}
	m := NewMonitor(cfg, pingFn)
	m.RegisterWorker("healthy")
	m.RegisterWorker("sick")

	m.checkWorker("healthy")
	m.checkWorker("sick")

	unresponsive := m.GetUnresponsiveWorkers()
	if len(unresponsive) != 1 {
		t.Errorf("expected 1 unresponsive, got %d", len(unresponsive))
	}
	if len(unresponsive) > 0 && unresponsive[0] != "sick" {
		t.Errorf("expected 'sick', got %s", unresponsive[0])
	}
}

func TestMonitor_StartStop(t *testing.T) {
	var pingCount atomic.Int32
	pingFn := func(ctx context.Context, name string) error {
		pingCount.Add(1)
		return nil
	}

	cfg := Config{
		Interval:   50 * time.Millisecond,
		Timeout:    100 * time.Millisecond,
		MaxRetries: 3,
	}
	m := NewMonitor(cfg, pingFn)
	m.RegisterWorker("test")

	m.Start()

	// Wait for some pings
	time.Sleep(150 * time.Millisecond)

	m.Stop()

	count := pingCount.Load()
	if count < 1 {
		t.Errorf("expected at least 1 ping, got %d", count)
	}

	// Verify stopped
	countAfterStop := pingCount.Load()
	time.Sleep(100 * time.Millisecond)
	if pingCount.Load() != countAfterStop {
		t.Error("monitor should be stopped")
	}
}

func TestMonitor_StartStop_Idempotent(t *testing.T) {
	m := NewMonitor(DefaultConfig(), nil)

	// Multiple starts should be safe
	m.Start()
	m.Start()
	m.Start()

	// Multiple stops should be safe
	m.Stop()
	m.Stop()
	m.Stop()
}

func TestMonitor_ConfigDefaults(t *testing.T) {
	// Zero config should use defaults
	cfg := Config{}
	m := NewMonitor(cfg, nil)

	if m.cfg.Interval != DefaultConfig().Interval {
		t.Errorf("expected default interval, got %v", m.cfg.Interval)
	}
	if m.cfg.Timeout != DefaultConfig().Timeout {
		t.Errorf("expected default timeout, got %v", m.cfg.Timeout)
	}
	if m.cfg.MaxRetries != DefaultConfig().MaxRetries {
		t.Errorf("expected default max retries, got %d", m.cfg.MaxRetries)
	}
}

func TestMonitor_Recovery(t *testing.T) {
	// Simulate worker that recovers
	failCount := 0
	pingFn := func(ctx context.Context, name string) error {
		failCount++
		if failCount <= 2 {
			return errors.New("error")
		}
		return nil // Recovers on 3rd attempt
	}

	cfg := Config{
		Interval:   100 * time.Millisecond,
		Timeout:    1 * time.Second,
		MaxRetries: 5,
	}
	m := NewMonitor(cfg, pingFn)
	m.RegisterWorker("test")

	// Fail twice
	m.checkWorker("test")
	m.checkWorker("test")

	health, _ := m.GetHealth("test")
	if health.Status != StatusUnresponsive {
		t.Errorf("expected unresponsive, got %v", health.Status)
	}

	// Third check - should recover
	m.checkWorker("test")

	health, _ = m.GetHealth("test")
	if health.Status != StatusHealthy {
		t.Errorf("expected healthy after recovery, got %v", health.Status)
	}
	if health.FailCount != 0 {
		t.Errorf("expected fail count reset to 0, got %d", health.FailCount)
	}
}
