package errorwatch

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// ResetFunc is called to reset an agent.
type ResetFunc func(ctx context.Context, agentName string) error

// NotifyFunc is called to send notifications.
type NotifyFunc func(ctx context.Context, target, message string) error

// AutoResetConfig holds configuration for automatic reset.
type AutoResetConfig struct {
	// CooldownPeriod is the minimum time between reset attempts for the same agent.
	CooldownPeriod time.Duration
	// MaxConsecutiveFailures is the maximum number of consecutive reset failures
	// before escalating to PM.
	MaxConsecutiveFailures int
	// ResetTimeout is the timeout for each reset attempt.
	ResetTimeout time.Duration
	// PMAgent is the PM agent name for notifications.
	PMAgent string
	// Logger for reset events.
	Logger *log.Logger
}

// DefaultAutoResetConfig returns the default configuration.
func DefaultAutoResetConfig() AutoResetConfig {
	return AutoResetConfig{
		CooldownPeriod:         30 * time.Second,
		MaxConsecutiveFailures: 3,
		ResetTimeout:           10 * time.Second,
		PMAgent:                "mei",
		Logger:                 log.Default(),
	}
}

// agentResetState tracks reset state for an agent.
type agentResetState struct {
	lastResetAttempt    time.Time
	consecutiveFailures int
	lastError           error
}

// AutoResetter automatically resets agents when LLM errors are detected.
type AutoResetter struct {
	mu       sync.Mutex
	cfg      AutoResetConfig
	resetFn  ResetFunc
	notifyFn NotifyFunc
	states   map[string]*agentResetState

	// Callback for reset events
	onResetAttempt func(agentName string, success bool, err error)
	onPMEscalation func(agentName string, failCount int)
}

// NewAutoResetter creates a new auto resetter.
func NewAutoResetter(cfg AutoResetConfig, resetFn ResetFunc) *AutoResetter {
	if cfg.CooldownPeriod <= 0 {
		cfg.CooldownPeriod = DefaultAutoResetConfig().CooldownPeriod
	}
	if cfg.MaxConsecutiveFailures <= 0 {
		cfg.MaxConsecutiveFailures = DefaultAutoResetConfig().MaxConsecutiveFailures
	}
	if cfg.ResetTimeout <= 0 {
		cfg.ResetTimeout = DefaultAutoResetConfig().ResetTimeout
	}
	if cfg.PMAgent == "" {
		cfg.PMAgent = DefaultAutoResetConfig().PMAgent
	}
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}

	return &AutoResetter{
		cfg:     cfg,
		resetFn: resetFn,
		states:  make(map[string]*agentResetState),
	}
}

// SetNotifyFunc sets the notification function for PM escalation.
func (r *AutoResetter) SetNotifyFunc(fn NotifyFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notifyFn = fn
}

// SetOnResetAttempt sets the callback for reset attempts.
func (r *AutoResetter) SetOnResetAttempt(fn func(agentName string, success bool, err error)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onResetAttempt = fn
}

// SetOnPMEscalation sets the callback for PM escalation.
func (r *AutoResetter) SetOnPMEscalation(fn func(agentName string, failCount int)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onPMEscalation = fn
}

// HandleError handles an LLM error and triggers auto reset if applicable.
// Returns true if reset was attempted, and any error from the reset.
func (r *AutoResetter) HandleError(ctx context.Context, agentName string, errMsg string) (attempted bool, resetErr error) {
	// Check if this is a recoverable error
	if !IsRecoverableError(errMsg) {
		return false, nil
	}

	r.mu.Lock()
	state, exists := r.states[agentName]
	if !exists {
		state = &agentResetState{}
		r.states[agentName] = state
	}

	// Check cooldown
	if time.Since(state.lastResetAttempt) < r.cfg.CooldownPeriod {
		r.mu.Unlock()
		r.cfg.Logger.Printf("[auto-reset] Skipping reset for %s: cooldown active (last attempt: %v ago)",
			agentName, time.Since(state.lastResetAttempt).Round(time.Second))
		return false, nil
	}

	state.lastResetAttempt = time.Now()
	resetFn := r.resetFn
	onResetAttempt := r.onResetAttempt
	r.mu.Unlock()

	// Attempt reset
	r.cfg.Logger.Printf("[auto-reset] Attempting reset for agent %s (error: %s)", agentName, errMsg)

	if resetFn == nil {
		err := fmt.Errorf("reset function not set")
		r.recordFailure(agentName, err)
		return true, err
	}

	resetCtx, cancel := context.WithTimeout(ctx, r.cfg.ResetTimeout)
	defer cancel()

	resetErr = resetFn(resetCtx, agentName)

	if resetErr != nil {
		r.cfg.Logger.Printf("[auto-reset] Reset failed for agent %s: %v", agentName, resetErr)
		r.recordFailure(agentName, resetErr)
	} else {
		r.cfg.Logger.Printf("[auto-reset] Reset successful for agent %s", agentName)
		r.recordSuccess(agentName)
	}

	if onResetAttempt != nil {
		onResetAttempt(agentName, resetErr == nil, resetErr)
	}

	return true, resetErr
}

// recordFailure records a failed reset attempt.
func (r *AutoResetter) recordFailure(agentName string, err error) {
	r.mu.Lock()
	state := r.states[agentName]
	state.consecutiveFailures++
	state.lastError = err
	failCount := state.consecutiveFailures
	maxFailures := r.cfg.MaxConsecutiveFailures
	notifyFn := r.notifyFn
	onPMEscalation := r.onPMEscalation
	pmAgent := r.cfg.PMAgent
	r.mu.Unlock()

	// Check if we need to escalate to PM
	if failCount >= maxFailures {
		r.cfg.Logger.Printf("[auto-reset] Escalating to PM: agent %s failed %d consecutive resets", agentName, failCount)

		if onPMEscalation != nil {
			onPMEscalation(agentName, failCount)
		}

		if notifyFn != nil {
			message := fmt.Sprintf("エージェント %s の自動リセットが %d 回連続で失敗しました。手動確認が必要です。最後のエラー: %v",
				agentName, failCount, err)
			// Best effort notification
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = notifyFn(ctx, pmAgent, message)
		}
	}
}

// recordSuccess records a successful reset.
func (r *AutoResetter) recordSuccess(agentName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state := r.states[agentName]
	state.consecutiveFailures = 0
	state.lastError = nil
}

// GetState returns the reset state for an agent.
func (r *AutoResetter) GetState(agentName string) (lastAttempt time.Time, failures int, lastErr error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, exists := r.states[agentName]
	if !exists {
		return time.Time{}, 0, nil
	}
	return state.lastResetAttempt, state.consecutiveFailures, state.lastError
}

// ResetState clears the reset state for an agent.
func (r *AutoResetter) ResetState(agentName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.states, agentName)
}

// ResetAllStates clears all reset states.
func (r *AutoResetter) ResetAllStates() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.states = make(map[string]*agentResetState)
}

// AutoResetResult holds the result of an auto reset check.
type AutoResetResult struct {
	// ErrorDetected is true if a recoverable error was detected.
	ErrorDetected bool
	// ResetAttempted is true if a reset was attempted.
	ResetAttempted bool
	// ResetSucceeded is true if the reset succeeded.
	ResetSucceeded bool
	// Error is the error from the reset attempt, if any.
	Error error
	// CooldownActive is true if reset was skipped due to cooldown.
	CooldownActive bool
}

// CheckAndReset checks for LLM errors and triggers auto reset.
// This is a convenience method that combines error detection and reset.
func (r *AutoResetter) CheckAndReset(ctx context.Context, agentName string, errMsg string) AutoResetResult {
	result := AutoResetResult{}

	if !IsRecoverableError(errMsg) {
		return result
	}
	result.ErrorDetected = true

	attempted, err := r.HandleError(ctx, agentName, errMsg)
	result.ResetAttempted = attempted
	result.Error = err
	result.ResetSucceeded = attempted && err == nil
	result.CooldownActive = !attempted && result.ErrorDetected

	return result
}
