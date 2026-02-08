// Package mode manages MAXAM operating modes
package mode

import (
	"fmt"
	"sync"

	"github.com/ytnobody/MAXAM/internal/config"
)

// Manager manages the current operating mode
type Manager struct {
	mu          sync.RWMutex
	currentMode config.Mode
	// planContext stores context from plan mode for auto mode transition
	planContext *PlanContext
}

// PlanContext stores the context from plan mode
type PlanContext struct {
	DiscussionURL string // URL of approved discussion
	IssueNumbers  []int  // Issue numbers created from the plan
	Approved      bool   // Whether the plan was approved
}

// NewManager creates a new mode manager with the given default mode
func NewManager(defaultMode config.Mode) *Manager {
	if defaultMode == "" {
		defaultMode = config.ModeInteractive
	}
	return &Manager{
		currentMode: defaultMode,
	}
}

// Current returns the current operating mode
func (m *Manager) Current() config.Mode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentMode
}

// IsInteractive returns true if in interactive mode
func (m *Manager) IsInteractive() bool {
	return m.Current() == config.ModeInteractive
}

// IsPlan returns true if in plan mode
func (m *Manager) IsPlan() bool {
	return m.Current() == config.ModePlan
}

// IsAuto returns true if in auto mode
func (m *Manager) IsAuto() bool {
	return m.Current() == config.ModeAuto
}

// EnterPlanMode transitions to plan mode from interactive mode
func (m *Manager) EnterPlanMode() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentMode != config.ModeInteractive {
		return fmt.Errorf("can only enter plan mode from interactive mode (current: %s)", m.currentMode)
	}

	m.currentMode = config.ModePlan
	m.planContext = &PlanContext{}
	return nil
}

// EnterAutoMode transitions to auto mode from plan mode
// Requires plan approval
func (m *Manager) EnterAutoMode(ctx *PlanContext) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentMode != config.ModePlan {
		return fmt.Errorf("can only enter auto mode from plan mode (current: %s)", m.currentMode)
	}

	if ctx == nil || !ctx.Approved {
		return fmt.Errorf("cannot enter auto mode without approved plan")
	}

	m.currentMode = config.ModeAuto
	m.planContext = ctx
	return nil
}

// EnterAutoModeWithoutPlan transitions to auto mode directly from interactive mode
// This bypasses the plan mode and allows immediate auto execution
func (m *Manager) EnterAutoModeWithoutPlan() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentMode != config.ModeInteractive {
		return fmt.Errorf("can only enter auto mode directly from interactive mode (current: %s)", m.currentMode)
	}

	m.currentMode = config.ModeAuto
	m.planContext = nil // No plan context when entering directly
	return nil
}

// Stop returns to interactive mode from any mode
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.currentMode = config.ModeInteractive
	m.planContext = nil
}

// GetPlanContext returns the current plan context (nil if not in plan/auto mode)
func (m *Manager) GetPlanContext() *PlanContext {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.planContext
}

// SetPlanApproved marks the current plan as approved
func (m *Manager) SetPlanApproved(approved bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.planContext != nil {
		m.planContext.Approved = approved
	}
}

// AddIssue adds an issue number to the plan context
func (m *Manager) AddIssue(issueNum int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.planContext != nil {
		m.planContext.IssueNumbers = append(m.planContext.IssueNumbers, issueNum)
	}
}

// ShouldEnforceMention returns whether mention check should be enforced
// Plan mode and Auto mode require mentions, Interactive mode does not
func (m *Manager) ShouldEnforceMention() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	switch m.currentMode {
	case config.ModePlan, config.ModeAuto:
		return true
	default:
		return false
	}
}

// StatusString returns a human-readable status string
func (m *Manager) StatusString() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	switch m.currentMode {
	case config.ModeInteractive:
		return "Interactive Mode - Free conversation with the team"
	case config.ModePlan:
		if m.planContext != nil && m.planContext.Approved {
			return "Plan Mode - Plan approved, ready for auto execution"
		}
		return "Plan Mode - Analyzing project and creating plan"
	case config.ModeAuto:
		if m.planContext != nil && len(m.planContext.IssueNumbers) > 0 {
			return fmt.Sprintf("Auto Mode - Implementing Issues: %v", m.planContext.IssueNumbers)
		}
		return "Auto Mode - Automated implementation in progress"
	default:
		return fmt.Sprintf("Unknown Mode: %s", m.currentMode)
	}
}
