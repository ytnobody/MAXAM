// Package mode manages MAXAM operating modes
package mode

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/ytnobody/MAXAM/internal/auto"
	"github.com/ytnobody/MAXAM/internal/config"
)

// TransitionManager manages mode transitions
type TransitionManager struct {
	mu sync.RWMutex

	modeManager *Manager
	runner      *auto.Runner

	// Callbacks
	onTransition func(from, to config.Mode)
	onError      func(err error)
}

// NewTransitionManager creates a new transition manager
func NewTransitionManager(modeManager *Manager, runner *auto.Runner) *TransitionManager {
	return &TransitionManager{
		modeManager: modeManager,
		runner:      runner,
	}
}

// SetOnTransition sets the callback for mode transitions
func (t *TransitionManager) SetOnTransition(fn func(from, to config.Mode)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onTransition = fn
}

// SetOnError sets the callback for transition errors
func (t *TransitionManager) SetOnError(fn func(err error)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onError = fn
}

// TransitionRequest represents a request to transition modes
type TransitionRequest struct {
	// From is the expected current mode (optional, for validation)
	From config.Mode
	// To is the target mode
	To config.Mode
	// PlanContext is required when transitioning to auto mode
	PlanContext *PlanContext
	// Issues is the list of issues to execute (for auto mode)
	Issues []auto.IssueInfo
}

// TransitionResult represents the result of a transition
type TransitionResult struct {
	Success bool
	From    config.Mode
	To      config.Mode
	Error   error
}

// Transition performs a mode transition
func (t *TransitionManager) Transition(req TransitionRequest) TransitionResult {
	t.mu.Lock()
	defer t.mu.Unlock()

	currentMode := t.modeManager.Current()

	// Validate from mode if specified
	if req.From != "" && req.From != currentMode {
		return TransitionResult{
			Success: false,
			From:    currentMode,
			To:      req.To,
			Error:   fmt.Errorf("expected mode %s but current is %s", req.From, currentMode),
		}
	}

	var err error
	switch req.To {
	case config.ModeInteractive:
		t.modeManager.Stop()
	case config.ModePlan:
		err = t.modeManager.EnterPlanMode()
	case config.ModeAuto:
		err = t.transitionToAuto(req.PlanContext)
	default:
		err = fmt.Errorf("unknown target mode: %s", req.To)
	}

	if err != nil {
		if t.onError != nil {
			t.onError(err)
		}
		return TransitionResult{
			Success: false,
			From:    currentMode,
			To:      req.To,
			Error:   err,
		}
	}

	// Notify transition
	if t.onTransition != nil {
		t.onTransition(currentMode, req.To)
	}

	return TransitionResult{
		Success: true,
		From:    currentMode,
		To:      req.To,
	}
}

// transitionToAuto handles the transition to auto mode
func (t *TransitionManager) transitionToAuto(planCtx *PlanContext) error {
	if planCtx == nil {
		return fmt.Errorf("plan context is required for auto mode")
	}
	if !planCtx.Approved {
		return fmt.Errorf("plan must be approved before transitioning to auto mode")
	}

	return t.modeManager.EnterAutoMode(planCtx)
}

// TransitionToAutoAndRun transitions to auto mode and starts execution
// This is a convenience method that combines transition and execution
func (t *TransitionManager) TransitionToAutoAndRun(ctx context.Context, planCtx *PlanContext, issues []auto.IssueInfo) error {
	result := t.Transition(TransitionRequest{
		From:        config.ModePlan,
		To:          config.ModeAuto,
		PlanContext: planCtx,
		Issues:      issues,
	})

	if !result.Success {
		return result.Error
	}

	// Add tasks to runner
	t.runner.AddTasks(issues)

	// Start runner in background
	go func() {
		if err := t.runner.Run(ctx); err != nil {
			log.Printf("Auto runner failed: %v", err)
			if t.onError != nil {
				t.onError(fmt.Errorf("auto runner failed: %w", err))
			}
		}
	}()

	return nil
}

// CanTransition checks if a transition is valid without performing it
func (t *TransitionManager) CanTransition(from, to config.Mode) bool {
	switch from {
	case config.ModeInteractive:
		return to == config.ModePlan
	case config.ModePlan:
		return to == config.ModeAuto || to == config.ModeInteractive
	case config.ModeAuto:
		return to == config.ModeInteractive
	default:
		return false
	}
}

// ValidTransitions returns all valid transitions from the current mode
func (t *TransitionManager) ValidTransitions() []config.Mode {
	currentMode := t.modeManager.Current()
	var valid []config.Mode

	switch currentMode {
	case config.ModeInteractive:
		valid = append(valid, config.ModePlan)
	case config.ModePlan:
		valid = append(valid, config.ModeAuto, config.ModeInteractive)
	case config.ModeAuto:
		valid = append(valid, config.ModeInteractive)
	}

	return valid
}

// CurrentMode returns the current mode
func (t *TransitionManager) CurrentMode() config.Mode {
	return t.modeManager.Current()
}

// Runner returns the auto runner
func (t *TransitionManager) Runner() *auto.Runner {
	return t.runner
}
