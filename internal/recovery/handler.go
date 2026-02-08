// Package recovery provides agent recovery and fallback mechanisms.
package recovery

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrMaxRetriesExceeded is returned when max restart attempts are exhausted.
var ErrMaxRetriesExceeded = errors.New("max retries exceeded")

// ErrPMNotificationFailed is returned when PM notification fails.
var ErrPMNotificationFailed = errors.New("PM notification failed")

// RestartFunc is called to restart a worker.
type RestartFunc func(ctx context.Context, workerName string) error

// NotifyFunc is called to send notifications.
type NotifyFunc func(ctx context.Context, target, message string) error

// Config holds recovery configuration.
type Config struct {
	MaxRetries    int           // Max restart attempts per worker
	RetryDelay    time.Duration // Delay between retries
	NotifyTimeout time.Duration // Timeout for notifications
	PMAgent       string        // PM agent name for notifications
	OwnerChannel  string        // Owner notification channel (fallback)
}

// DefaultConfig returns the default recovery configuration.
func DefaultConfig() Config {
	return Config{
		MaxRetries:    3,
		RetryDelay:    5 * time.Second,
		NotifyTimeout: 10 * time.Second,
		PMAgent:       "mei",
		OwnerChannel:  "owner",
	}
}

// RecoveryState tracks recovery attempts for a worker.
type RecoveryState struct {
	WorkerName  string
	Attempts    int
	LastAttempt time.Time
	LastError   error
	Recovered   bool
}

// Handler manages worker recovery.
type Handler struct {
	cfg       Config
	restartFn RestartFunc
	notifyFn  NotifyFunc
	states    map[string]*RecoveryState
	mu        sync.RWMutex
}

// NewHandler creates a new recovery handler.
func NewHandler(cfg Config, restartFn RestartFunc, notifyFn NotifyFunc) *Handler {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = DefaultConfig().MaxRetries
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = DefaultConfig().RetryDelay
	}
	if cfg.NotifyTimeout <= 0 {
		cfg.NotifyTimeout = DefaultConfig().NotifyTimeout
	}
	if cfg.PMAgent == "" {
		cfg.PMAgent = DefaultConfig().PMAgent
	}
	if cfg.OwnerChannel == "" {
		cfg.OwnerChannel = DefaultConfig().OwnerChannel
	}

	return &Handler{
		cfg:       cfg,
		restartFn: restartFn,
		notifyFn:  notifyFn,
		states:    make(map[string]*RecoveryState),
	}
}

// HandleUnresponsive handles an unresponsive worker.
// Returns true if recovery was successful.
func (h *Handler) HandleUnresponsive(ctx context.Context, workerName string, failCount int) bool {
	h.mu.Lock()
	state, exists := h.states[workerName]
	if !exists {
		state = &RecoveryState{WorkerName: workerName}
		h.states[workerName] = state
	}
	state.Attempts = failCount
	state.LastAttempt = time.Now()
	h.mu.Unlock()

	// Try restart
	if failCount <= h.cfg.MaxRetries {
		if err := h.attemptRestart(ctx, workerName); err == nil {
			h.mu.Lock()
			state.Recovered = true
			state.LastError = nil
			h.mu.Unlock()
			return true
		} else {
			h.mu.Lock()
			state.LastError = err
			h.mu.Unlock()
		}
	}

	// Restart failed or max retries exceeded - notify PM
	if failCount >= h.cfg.MaxRetries {
		h.notifyPM(ctx, workerName, failCount)
	}

	return false
}

// attemptRestart tries to restart a worker.
func (h *Handler) attemptRestart(ctx context.Context, workerName string) error {
	if h.restartFn == nil {
		return errors.New("restart function not set")
	}

	// Wait before retry
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(h.cfg.RetryDelay):
	}

	return h.restartFn(ctx, workerName)
}

// notifyPM sends a notification to PM, with owner fallback.
func (h *Handler) notifyPM(ctx context.Context, workerName string, failCount int) {
	if h.notifyFn == nil {
		return
	}

	message := fmt.Sprintf("エージェント %s が応答不能 (失敗回数: %d)。自動復帰に失敗しました。", workerName, failCount)

	notifyCtx, cancel := context.WithTimeout(ctx, h.cfg.NotifyTimeout)
	defer cancel()

	// Try PM notification
	err := h.notifyFn(notifyCtx, h.cfg.PMAgent, message)
	if err != nil {
		// PM notification failed - fallback to owner
		h.notifyOwner(ctx, workerName, failCount, err)
	}
}

// notifyOwner sends notification directly to owner (fallback).
func (h *Handler) notifyOwner(ctx context.Context, workerName string, failCount int, pmError error) {
	if h.notifyFn == nil {
		return
	}

	message := fmt.Sprintf(
		"[FALLBACK] エージェント %s が応答不能 (失敗回数: %d)。PM通知失敗: %v",
		workerName, failCount, pmError,
	)

	notifyCtx, cancel := context.WithTimeout(ctx, h.cfg.NotifyTimeout)
	defer cancel()

	// Best effort - ignore error
	_ = h.notifyFn(notifyCtx, h.cfg.OwnerChannel, message)
}

// GetState returns the recovery state for a worker.
func (h *Handler) GetState(workerName string) (RecoveryState, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	state, exists := h.states[workerName]
	if !exists {
		return RecoveryState{}, false
	}
	return *state, true
}

// ResetState resets the recovery state for a worker.
func (h *Handler) ResetState(workerName string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.states, workerName)
}

// ResetAll resets all recovery states.
func (h *Handler) ResetAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.states = make(map[string]*RecoveryState)
}

// GetAllStates returns all recovery states.
func (h *Handler) GetAllStates() map[string]RecoveryState {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make(map[string]RecoveryState, len(h.states))
	for name, state := range h.states {
		result[name] = *state
	}
	return result
}
