package errorwatch

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestAutoResetter_HandleError(t *testing.T) {
	t.Run("triggers reset for recoverable error", func(t *testing.T) {
		var resetCalled atomic.Bool
		resetFn := func(ctx context.Context, agentName string) error {
			resetCalled.Store(true)
			return nil
		}

		cfg := DefaultAutoResetConfig()
		cfg.CooldownPeriod = 10 * time.Millisecond
		r := NewAutoResetter(cfg, resetFn)

		attempted, err := r.HandleError(context.Background(), "yuki", "context deadline exceeded")
		if !attempted {
			t.Error("expected reset to be attempted")
		}
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !resetCalled.Load() {
			t.Error("expected reset function to be called")
		}
	})

	t.Run("does not trigger reset for non-recoverable error", func(t *testing.T) {
		var resetCalled atomic.Bool
		resetFn := func(ctx context.Context, agentName string) error {
			resetCalled.Store(true)
			return nil
		}

		r := NewAutoResetter(DefaultAutoResetConfig(), resetFn)

		attempted, _ := r.HandleError(context.Background(), "yuki", "syntax error")
		if attempted {
			t.Error("expected reset not to be attempted for non-recoverable error")
		}
		if resetCalled.Load() {
			t.Error("expected reset function not to be called")
		}
	})

	t.Run("respects cooldown period", func(t *testing.T) {
		resetCount := 0
		resetFn := func(ctx context.Context, agentName string) error {
			resetCount++
			return nil
		}

		cfg := DefaultAutoResetConfig()
		cfg.CooldownPeriod = 100 * time.Millisecond
		r := NewAutoResetter(cfg, resetFn)

		// First reset
		attempted1, _ := r.HandleError(context.Background(), "yuki", "context deadline exceeded")
		if !attempted1 {
			t.Error("first reset should be attempted")
		}

		// Second reset immediately (should be blocked by cooldown)
		attempted2, _ := r.HandleError(context.Background(), "yuki", "context deadline exceeded")
		if attempted2 {
			t.Error("second reset should be blocked by cooldown")
		}

		if resetCount != 1 {
			t.Errorf("expected 1 reset, got %d", resetCount)
		}
	})

	t.Run("allows reset after cooldown expires", func(t *testing.T) {
		resetCount := 0
		resetFn := func(ctx context.Context, agentName string) error {
			resetCount++
			return nil
		}

		cfg := DefaultAutoResetConfig()
		cfg.CooldownPeriod = 50 * time.Millisecond
		r := NewAutoResetter(cfg, resetFn)

		// First reset
		r.HandleError(context.Background(), "yuki", "context deadline exceeded")

		// Wait for cooldown
		time.Sleep(60 * time.Millisecond)

		// Second reset after cooldown
		attempted, _ := r.HandleError(context.Background(), "yuki", "context deadline exceeded")
		if !attempted {
			t.Error("reset should be attempted after cooldown")
		}

		if resetCount != 2 {
			t.Errorf("expected 2 resets, got %d", resetCount)
		}
	})

	t.Run("tracks consecutive failures", func(t *testing.T) {
		resetErr := errors.New("reset failed")
		resetFn := func(ctx context.Context, agentName string) error {
			return resetErr
		}

		cfg := DefaultAutoResetConfig()
		cfg.CooldownPeriod = 1 * time.Millisecond
		cfg.MaxConsecutiveFailures = 3
		r := NewAutoResetter(cfg, resetFn)

		// Fail 3 times
		for i := 0; i < 3; i++ {
			time.Sleep(2 * time.Millisecond) // Wait for cooldown
			r.HandleError(context.Background(), "yuki", "context deadline exceeded")
		}

		_, failures, lastErr := r.GetState("yuki")
		if failures != 3 {
			t.Errorf("expected 3 failures, got %d", failures)
		}
		if lastErr != resetErr {
			t.Errorf("expected last error %v, got %v", resetErr, lastErr)
		}
	})

	t.Run("resets failure count on success", func(t *testing.T) {
		callCount := 0
		resetFn := func(ctx context.Context, agentName string) error {
			callCount++
			if callCount < 3 {
				return errors.New("failed")
			}
			return nil // Success on third call
		}

		cfg := DefaultAutoResetConfig()
		cfg.CooldownPeriod = 1 * time.Millisecond
		r := NewAutoResetter(cfg, resetFn)

		// Fail twice
		time.Sleep(2 * time.Millisecond)
		r.HandleError(context.Background(), "yuki", "context deadline exceeded")
		time.Sleep(2 * time.Millisecond)
		r.HandleError(context.Background(), "yuki", "context deadline exceeded")

		_, failures, _ := r.GetState("yuki")
		if failures != 2 {
			t.Errorf("expected 2 failures, got %d", failures)
		}

		// Succeed
		time.Sleep(2 * time.Millisecond)
		r.HandleError(context.Background(), "yuki", "context deadline exceeded")

		_, failures, lastErr := r.GetState("yuki")
		if failures != 0 {
			t.Errorf("expected 0 failures after success, got %d", failures)
		}
		if lastErr != nil {
			t.Errorf("expected nil error after success, got %v", lastErr)
		}
	})
}

func TestAutoResetter_PMEscalation(t *testing.T) {
	t.Run("escalates to PM after max failures", func(t *testing.T) {
		resetErr := errors.New("reset failed")
		resetFn := func(ctx context.Context, agentName string) error {
			return resetErr
		}

		var escalationCalled atomic.Bool
		var escalatedAgent string
		var escalatedCount int

		cfg := DefaultAutoResetConfig()
		cfg.CooldownPeriod = 1 * time.Millisecond
		cfg.MaxConsecutiveFailures = 2
		r := NewAutoResetter(cfg, resetFn)

		r.SetOnPMEscalation(func(agentName string, failCount int) {
			escalationCalled.Store(true)
			escalatedAgent = agentName
			escalatedCount = failCount
		})

		// Fail twice (should trigger escalation)
		time.Sleep(2 * time.Millisecond)
		r.HandleError(context.Background(), "yuki", "context deadline exceeded")
		time.Sleep(2 * time.Millisecond)
		r.HandleError(context.Background(), "yuki", "context deadline exceeded")

		if !escalationCalled.Load() {
			t.Error("expected PM escalation to be called")
		}
		if escalatedAgent != "yuki" {
			t.Errorf("expected escalated agent 'yuki', got '%s'", escalatedAgent)
		}
		if escalatedCount != 2 {
			t.Errorf("expected escalated count 2, got %d", escalatedCount)
		}
	})

	t.Run("sends notification on escalation", func(t *testing.T) {
		resetFn := func(ctx context.Context, agentName string) error {
			return errors.New("reset failed")
		}

		var notifyCalled atomic.Bool
		var notifyTarget string
		notifyFn := func(ctx context.Context, target, message string) error {
			notifyCalled.Store(true)
			notifyTarget = target
			return nil
		}

		cfg := DefaultAutoResetConfig()
		cfg.CooldownPeriod = 1 * time.Millisecond
		cfg.MaxConsecutiveFailures = 1
		cfg.PMAgent = "mei"
		r := NewAutoResetter(cfg, resetFn)
		r.SetNotifyFunc(notifyFn)

		// Fail once (should trigger escalation with max=1)
		time.Sleep(2 * time.Millisecond)
		r.HandleError(context.Background(), "yuki", "context deadline exceeded")

		// Wait for async notification
		time.Sleep(50 * time.Millisecond)

		if !notifyCalled.Load() {
			t.Error("expected notification to be sent")
		}
		if notifyTarget != "mei" {
			t.Errorf("expected notification target 'mei', got '%s'", notifyTarget)
		}
	})
}

func TestAutoResetter_Callbacks(t *testing.T) {
	t.Run("calls onResetAttempt callback", func(t *testing.T) {
		resetFn := func(ctx context.Context, agentName string) error {
			return nil
		}

		var callbackCalled atomic.Bool
		var callbackAgent string
		var callbackSuccess bool

		cfg := DefaultAutoResetConfig()
		r := NewAutoResetter(cfg, resetFn)
		r.SetOnResetAttempt(func(agentName string, success bool, err error) {
			callbackCalled.Store(true)
			callbackAgent = agentName
			callbackSuccess = success
		})

		r.HandleError(context.Background(), "yuki", "context deadline exceeded")

		if !callbackCalled.Load() {
			t.Error("expected onResetAttempt callback to be called")
		}
		if callbackAgent != "yuki" {
			t.Errorf("expected agent 'yuki', got '%s'", callbackAgent)
		}
		if !callbackSuccess {
			t.Error("expected success to be true")
		}
	})
}

func TestAutoResetter_CheckAndReset(t *testing.T) {
	t.Run("returns full result for recoverable error", func(t *testing.T) {
		resetFn := func(ctx context.Context, agentName string) error {
			return nil
		}

		r := NewAutoResetter(DefaultAutoResetConfig(), resetFn)
		result := r.CheckAndReset(context.Background(), "yuki", "context deadline exceeded")

		if !result.ErrorDetected {
			t.Error("expected ErrorDetected to be true")
		}
		if !result.ResetAttempted {
			t.Error("expected ResetAttempted to be true")
		}
		if !result.ResetSucceeded {
			t.Error("expected ResetSucceeded to be true")
		}
		if result.Error != nil {
			t.Errorf("expected nil error, got %v", result.Error)
		}
		if result.CooldownActive {
			t.Error("expected CooldownActive to be false")
		}
	})

	t.Run("returns cooldown active when blocked", func(t *testing.T) {
		resetFn := func(ctx context.Context, agentName string) error {
			return nil
		}

		cfg := DefaultAutoResetConfig()
		cfg.CooldownPeriod = 1 * time.Hour
		r := NewAutoResetter(cfg, resetFn)

		// First reset
		r.CheckAndReset(context.Background(), "yuki", "context deadline exceeded")

		// Second attempt (blocked by cooldown)
		result := r.CheckAndReset(context.Background(), "yuki", "context deadline exceeded")

		if !result.ErrorDetected {
			t.Error("expected ErrorDetected to be true")
		}
		if result.ResetAttempted {
			t.Error("expected ResetAttempted to be false (cooldown)")
		}
		if !result.CooldownActive {
			t.Error("expected CooldownActive to be true")
		}
	})

	t.Run("returns not detected for non-recoverable error", func(t *testing.T) {
		resetFn := func(ctx context.Context, agentName string) error {
			return nil
		}

		r := NewAutoResetter(DefaultAutoResetConfig(), resetFn)
		result := r.CheckAndReset(context.Background(), "yuki", "syntax error")

		if result.ErrorDetected {
			t.Error("expected ErrorDetected to be false")
		}
		if result.ResetAttempted {
			t.Error("expected ResetAttempted to be false")
		}
	})
}

func TestAutoResetter_StateManagement(t *testing.T) {
	t.Run("ResetState clears agent state", func(t *testing.T) {
		resetFn := func(ctx context.Context, agentName string) error {
			return errors.New("failed")
		}

		cfg := DefaultAutoResetConfig()
		cfg.CooldownPeriod = 1 * time.Millisecond
		r := NewAutoResetter(cfg, resetFn)

		time.Sleep(2 * time.Millisecond)
		r.HandleError(context.Background(), "yuki", "context deadline exceeded")

		_, failures, _ := r.GetState("yuki")
		if failures != 1 {
			t.Errorf("expected 1 failure, got %d", failures)
		}

		r.ResetState("yuki")

		_, failures, _ = r.GetState("yuki")
		if failures != 0 {
			t.Errorf("expected 0 failures after reset, got %d", failures)
		}
	})

	t.Run("ResetAllStates clears all states", func(t *testing.T) {
		resetFn := func(ctx context.Context, agentName string) error {
			return errors.New("failed")
		}

		cfg := DefaultAutoResetConfig()
		cfg.CooldownPeriod = 1 * time.Millisecond
		r := NewAutoResetter(cfg, resetFn)

		time.Sleep(2 * time.Millisecond)
		r.HandleError(context.Background(), "yuki", "context deadline exceeded")
		time.Sleep(2 * time.Millisecond)
		r.HandleError(context.Background(), "rin", "context deadline exceeded")

		r.ResetAllStates()

		_, failures1, _ := r.GetState("yuki")
		_, failures2, _ := r.GetState("rin")
		if failures1 != 0 || failures2 != 0 {
			t.Errorf("expected 0 failures after reset all, got yuki=%d, rin=%d", failures1, failures2)
		}
	})
}

func TestAutoResetter_NilResetFunc(t *testing.T) {
	t.Run("returns error when reset function is nil", func(t *testing.T) {
		r := NewAutoResetter(DefaultAutoResetConfig(), nil)

		attempted, err := r.HandleError(context.Background(), "yuki", "context deadline exceeded")
		if !attempted {
			t.Error("expected reset to be attempted")
		}
		if err == nil {
			t.Error("expected error for nil reset function")
		}
	})
}
