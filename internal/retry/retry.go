package retry

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"
)

// Config defines retry behavior
type Config struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Second,
		MaxDelay:    30 * time.Second,
	}
}

// IsRetryable checks if an error is worth retrying
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()

	// Timeout errors
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if strings.Contains(msg, "timeout") {
		return true
	}

	// Temporary network errors
	if strings.Contains(msg, "connection refused") {
		return true
	}
	if strings.Contains(msg, "connection reset") {
		return true
	}
	if strings.Contains(msg, "no such host") {
		return true
	}

	// Rate limiting
	if strings.Contains(msg, "rate limit") {
		return true
	}
	if strings.Contains(msg, "429") {
		return true
	}

	// Server errors (5xx)
	if strings.Contains(msg, "500") ||
		strings.Contains(msg, "502") ||
		strings.Contains(msg, "503") ||
		strings.Contains(msg, "504") {
		return true
	}

	return false
}

// Do executes fn with retry logic
func Do(ctx context.Context, cfg Config, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		if !IsRetryable(err) {
			return err
		}

		// Last attempt, don't wait
		if attempt == cfg.MaxAttempts-1 {
			break
		}

		// Exponential backoff: baseDelay * 2^attempt
		delay := time.Duration(float64(cfg.BaseDelay) * math.Pow(2, float64(attempt)))
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return lastErr
}

// DoWithResult executes fn and returns result with retry logic
func DoWithResult[T any](ctx context.Context, cfg Config, fn func() (T, error)) (T, error) {
	var result T
	var lastErr error

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		res, err := fn()
		if err == nil {
			return res, nil
		}

		lastErr = err

		if !IsRetryable(err) {
			return result, err
		}

		if attempt == cfg.MaxAttempts-1 {
			break
		}

		delay := time.Duration(float64(cfg.BaseDelay) * math.Pow(2, float64(attempt)))
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}

		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(delay):
		}
	}

	return result, lastErr
}
