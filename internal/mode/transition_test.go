package mode

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ytnobody/MAXAM/internal/auto"
	"github.com/ytnobody/MAXAM/internal/config"
)

// stubExecutor is a test stub for TaskExecutor
type stubExecutor struct {
	executeFunc func(ctx context.Context, issueNumber int, title string) (int, error)
}

func (s *stubExecutor) Execute(ctx context.Context, issueNumber int, title string) (int, error) {
	if s.executeFunc != nil {
		return s.executeFunc(ctx, issueNumber, title)
	}
	return issueNumber * 10, nil // default: return PR number as issue*10
}

func newTestRunner() *auto.Runner {
	return auto.NewRunner(&stubExecutor{}, auto.DefaultConfig())
}

func TestNewTransitionManager(t *testing.T) {
	mm := NewManager(config.ModeInteractive)
	runner := newTestRunner()

	tm := NewTransitionManager(mm, runner)

	if tm == nil {
		t.Fatal("expected non-nil TransitionManager")
	}
	if tm.modeManager != mm {
		t.Error("modeManager not set correctly")
	}
	if tm.runner != runner {
		t.Error("runner not set correctly")
	}
}

func TestTransitionManager_Transition(t *testing.T) {
	tests := []struct {
		name        string
		initialMode config.Mode
		request     TransitionRequest
		wantSuccess bool
		wantTo      config.Mode
	}{
		{
			name:        "interactive to plan",
			initialMode: config.ModeInteractive,
			request: TransitionRequest{
				To: config.ModePlan,
			},
			wantSuccess: true,
			wantTo:      config.ModePlan,
		},
		{
			name:        "plan to auto with approved context",
			initialMode: config.ModePlan,
			request: TransitionRequest{
				To:          config.ModeAuto,
				PlanContext: &PlanContext{Approved: true},
			},
			wantSuccess: true,
			wantTo:      config.ModeAuto,
		},
		{
			name:        "plan to auto without approval",
			initialMode: config.ModePlan,
			request: TransitionRequest{
				To:          config.ModeAuto,
				PlanContext: &PlanContext{Approved: false},
			},
			wantSuccess: false,
			wantTo:      config.ModeAuto,
		},
		{
			name:        "plan to auto without context",
			initialMode: config.ModePlan,
			request: TransitionRequest{
				To: config.ModeAuto,
			},
			wantSuccess: false,
			wantTo:      config.ModeAuto,
		},
		{
			name:        "auto to interactive",
			initialMode: config.ModeAuto,
			request: TransitionRequest{
				To: config.ModeInteractive,
			},
			wantSuccess: true,
			wantTo:      config.ModeInteractive,
		},
		{
			name:        "wrong from mode",
			initialMode: config.ModeInteractive,
			request: TransitionRequest{
				From: config.ModePlan, // wrong
				To:   config.ModeAuto,
			},
			wantSuccess: false,
			wantTo:      config.ModeAuto,
		},
		{
			name:        "plan to interactive (cancel)",
			initialMode: config.ModePlan,
			request: TransitionRequest{
				To: config.ModeInteractive,
			},
			wantSuccess: true,
			wantTo:      config.ModeInteractive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mm := NewManager(config.ModeInteractive)
			runner := newTestRunner()
			tm := NewTransitionManager(mm, runner)

			// Set up initial mode
			switch tt.initialMode {
			case config.ModePlan:
				mm.EnterPlanMode()
			case config.ModeAuto:
				mm.EnterPlanMode()
				mm.SetPlanApproved(true)
				mm.EnterAutoMode(mm.GetPlanContext())
			}

			result := tm.Transition(tt.request)

			if result.Success != tt.wantSuccess {
				t.Errorf("Transition() success = %v, want %v (error: %v)", result.Success, tt.wantSuccess, result.Error)
			}
			if result.To != tt.wantTo {
				t.Errorf("Transition() to = %v, want %v", result.To, tt.wantTo)
			}

			if tt.wantSuccess {
				if mm.Current() != tt.wantTo {
					t.Errorf("current mode = %v, want %v", mm.Current(), tt.wantTo)
				}
			}
		})
	}
}

func TestTransitionManager_Callbacks(t *testing.T) {
	mm := NewManager(config.ModeInteractive)
	runner := newTestRunner()
	tm := NewTransitionManager(mm, runner)

	var transitionCalled bool
	var transitionFrom, transitionTo config.Mode
	var mu sync.Mutex

	tm.SetOnTransition(func(from, to config.Mode) {
		mu.Lock()
		defer mu.Unlock()
		transitionCalled = true
		transitionFrom = from
		transitionTo = to
	})

	result := tm.Transition(TransitionRequest{To: config.ModePlan})
	if !result.Success {
		t.Fatalf("transition failed: %v", result.Error)
	}

	mu.Lock()
	if !transitionCalled {
		t.Error("onTransition callback not called")
	}
	if transitionFrom != config.ModeInteractive {
		t.Errorf("transitionFrom = %v, want %v", transitionFrom, config.ModeInteractive)
	}
	if transitionTo != config.ModePlan {
		t.Errorf("transitionTo = %v, want %v", transitionTo, config.ModePlan)
	}
	mu.Unlock()
}

func TestTransitionManager_ErrorCallback(t *testing.T) {
	mm := NewManager(config.ModeInteractive)
	runner := newTestRunner()
	tm := NewTransitionManager(mm, runner)

	var errorCalled bool
	var errorReceived error

	tm.SetOnError(func(err error) {
		errorCalled = true
		errorReceived = err
	})

	// Try invalid transition (interactive -> auto without plan)
	result := tm.Transition(TransitionRequest{To: config.ModeAuto})

	if result.Success {
		t.Error("expected transition to fail")
	}
	if !errorCalled {
		t.Error("onError callback not called")
	}
	if errorReceived == nil {
		t.Error("expected error to be set")
	}
}

func TestTransitionManager_CanTransition(t *testing.T) {
	tests := []struct {
		from config.Mode
		to   config.Mode
		want bool
	}{
		{config.ModeInteractive, config.ModePlan, true},
		{config.ModeInteractive, config.ModeAuto, false},
		{config.ModeInteractive, config.ModeInteractive, false},
		{config.ModePlan, config.ModeAuto, true},
		{config.ModePlan, config.ModeInteractive, true},
		{config.ModePlan, config.ModePlan, false},
		{config.ModeAuto, config.ModeInteractive, true},
		{config.ModeAuto, config.ModePlan, false},
		{config.ModeAuto, config.ModeAuto, false},
	}

	mm := NewManager(config.ModeInteractive)
	runner := newTestRunner()
	tm := NewTransitionManager(mm, runner)

	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			got := tm.CanTransition(tt.from, tt.to)
			if got != tt.want {
				t.Errorf("CanTransition(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestTransitionManager_ValidTransitions(t *testing.T) {
	tests := []struct {
		currentMode config.Mode
		want        []config.Mode
	}{
		{config.ModeInteractive, []config.Mode{config.ModePlan}},
		{config.ModePlan, []config.Mode{config.ModeAuto, config.ModeInteractive}},
		{config.ModeAuto, []config.Mode{config.ModeInteractive}},
	}

	for _, tt := range tests {
		t.Run(string(tt.currentMode), func(t *testing.T) {
			mm := NewManager(tt.currentMode)
			runner := newTestRunner()
			tm := NewTransitionManager(mm, runner)

			got := tm.ValidTransitions()

			if len(got) != len(tt.want) {
				t.Errorf("ValidTransitions() = %v, want %v", got, tt.want)
				return
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("ValidTransitions()[%d] = %v, want %v", i, v, tt.want[i])
				}
			}
		})
	}
}

func TestTransitionManager_TransitionToAutoAndRun(t *testing.T) {
	mm := NewManager(config.ModeInteractive)

	executed := make(chan bool, 1)
	executor := &stubExecutor{
		executeFunc: func(ctx context.Context, issueNumber int, title string) (int, error) {
			executed <- true
			return issueNumber * 10, nil
		},
	}
	runner := auto.NewRunner(executor, auto.DefaultConfig())

	tm := NewTransitionManager(mm, runner)

	// First enter plan mode
	mm.EnterPlanMode()
	mm.SetPlanApproved(true)
	planCtx := mm.GetPlanContext()

	issues := []auto.IssueInfo{
		{Number: 123, Title: "Test Issue"},
	}

	err := tm.TransitionToAutoAndRun(context.Background(), planCtx, issues)
	if err != nil {
		t.Fatalf("TransitionToAutoAndRun() error = %v", err)
	}

	// Check mode changed
	if mm.Current() != config.ModeAuto {
		t.Errorf("current mode = %v, want %v", mm.Current(), config.ModeAuto)
	}

	// Wait for runner to execute
	select {
	case <-executed:
		// OK
	case <-time.After(time.Second):
		t.Error("runner did not execute within timeout")
	}
}

func TestTransitionManager_CurrentMode(t *testing.T) {
	mm := NewManager(config.ModeInteractive)
	runner := newTestRunner()
	tm := NewTransitionManager(mm, runner)

	if tm.CurrentMode() != config.ModeInteractive {
		t.Errorf("CurrentMode() = %v, want %v", tm.CurrentMode(), config.ModeInteractive)
	}

	tm.Transition(TransitionRequest{To: config.ModePlan})

	if tm.CurrentMode() != config.ModePlan {
		t.Errorf("CurrentMode() = %v, want %v", tm.CurrentMode(), config.ModePlan)
	}
}

func TestTransitionManager_Runner(t *testing.T) {
	mm := NewManager(config.ModeInteractive)
	runner := newTestRunner()
	tm := NewTransitionManager(mm, runner)

	if tm.Runner() != runner {
		t.Error("Runner() did not return the same runner instance")
	}
}
