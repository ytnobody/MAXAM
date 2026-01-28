package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"timeout", errors.New("timeout"), true},
		{"context deadline", context.DeadlineExceeded, true},
		{"connection refused", errors.New("connection refused"), true},
		{"connection reset", errors.New("connection reset"), true},
		{"no such host", errors.New("no such host"), true},
		{"rate limit", errors.New("rate limit exceeded"), true},
		{"429", errors.New("HTTP 429"), true},
		{"500", errors.New("HTTP 500"), true},
		{"502", errors.New("HTTP 502"), true},
		{"503", errors.New("HTTP 503"), true},
		{"504", errors.New("HTTP 504"), true},
		{"not retryable", errors.New("invalid input"), false},
		{"file not found", errors.New("file not found"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.expected {
				t.Errorf("IsRetryable(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestDo_SuccessFirstAttempt(t *testing.T) {
	cfg := Config{MaxAttempts: 3, BaseDelay: 10 * time.Millisecond, MaxDelay: 100 * time.Millisecond}
	attempts := 0

	err := Do(context.Background(), cfg, func() error {
		attempts++
		return nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestDo_RetryOnRetryableError(t *testing.T) {
	cfg := Config{MaxAttempts: 3, BaseDelay: 10 * time.Millisecond, MaxDelay: 100 * time.Millisecond}
	attempts := 0

	err := Do(context.Background(), cfg, func() error {
		attempts++
		if attempts < 3 {
			return errors.New("timeout")
		}
		return nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDo_NoRetryOnNonRetryableError(t *testing.T) {
	cfg := Config{MaxAttempts: 3, BaseDelay: 10 * time.Millisecond, MaxDelay: 100 * time.Millisecond}
	attempts := 0

	err := Do(context.Background(), cfg, func() error {
		attempts++
		return errors.New("invalid input")
	})

	if err == nil {
		t.Error("expected error, got nil")
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestDo_MaxAttemptsExhausted(t *testing.T) {
	cfg := Config{MaxAttempts: 3, BaseDelay: 10 * time.Millisecond, MaxDelay: 100 * time.Millisecond}
	attempts := 0

	err := Do(context.Background(), cfg, func() error {
		attempts++
		return errors.New("timeout")
	})

	if err == nil {
		t.Error("expected error, got nil")
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDo_ContextCancellation(t *testing.T) {
	cfg := Config{MaxAttempts: 10, BaseDelay: 100 * time.Millisecond, MaxDelay: 1 * time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := Do(ctx, cfg, func() error {
		attempts++
		return errors.New("timeout")
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestDoWithResult_Success(t *testing.T) {
	cfg := Config{MaxAttempts: 3, BaseDelay: 10 * time.Millisecond, MaxDelay: 100 * time.Millisecond}

	result, err := DoWithResult(context.Background(), cfg, func() (string, error) {
		return "success", nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != "success" {
		t.Errorf("expected 'success', got '%s'", result)
	}
}

func TestDoWithResult_RetryAndSucceed(t *testing.T) {
	cfg := Config{MaxAttempts: 3, BaseDelay: 10 * time.Millisecond, MaxDelay: 100 * time.Millisecond}
	attempts := 0

	result, err := DoWithResult(context.Background(), cfg, func() (int, error) {
		attempts++
		if attempts < 2 {
			return 0, errors.New("timeout")
		}
		return 42, nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts=3, got %d", cfg.MaxAttempts)
	}
	if cfg.BaseDelay != 1*time.Second {
		t.Errorf("expected BaseDelay=1s, got %v", cfg.BaseDelay)
	}
	if cfg.MaxDelay != 30*time.Second {
		t.Errorf("expected MaxDelay=30s, got %v", cfg.MaxDelay)
	}
}
