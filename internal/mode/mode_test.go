package mode

import (
	"testing"

	"github.com/ytnobody/MAXAM/internal/config"
)

func TestNewManager(t *testing.T) {
	tests := []struct {
		name        string
		defaultMode config.Mode
		wantMode    config.Mode
	}{
		{
			name:        "empty default mode defaults to interactive",
			defaultMode: "",
			wantMode:    config.ModeInteractive,
		},
		{
			name:        "explicit interactive mode",
			defaultMode: config.ModeInteractive,
			wantMode:    config.ModeInteractive,
		},
		{
			name:        "explicit plan mode",
			defaultMode: config.ModePlan,
			wantMode:    config.ModePlan,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager(tt.defaultMode)
			if got := m.Current(); got != tt.wantMode {
				t.Errorf("Current() = %v, want %v", got, tt.wantMode)
			}
		})
	}
}

func TestModeTransitions(t *testing.T) {
	t.Run("interactive to plan mode", func(t *testing.T) {
		m := NewManager(config.ModeInteractive)

		if err := m.EnterPlanMode(); err != nil {
			t.Errorf("EnterPlanMode() unexpected error: %v", err)
		}

		if !m.IsPlan() {
			t.Error("expected to be in plan mode")
		}
	})

	t.Run("plan to auto mode with approval", func(t *testing.T) {
		m := NewManager(config.ModeInteractive)
		m.EnterPlanMode()

		ctx := &PlanContext{Approved: true}
		if err := m.EnterAutoMode(ctx); err != nil {
			t.Errorf("EnterAutoMode() unexpected error: %v", err)
		}

		if !m.IsAuto() {
			t.Error("expected to be in auto mode")
		}
	})

	t.Run("plan to auto mode without approval fails", func(t *testing.T) {
		m := NewManager(config.ModeInteractive)
		m.EnterPlanMode()

		ctx := &PlanContext{Approved: false}
		if err := m.EnterAutoMode(ctx); err == nil {
			t.Error("EnterAutoMode() expected error for unapproved plan")
		}
	})

	t.Run("direct auto mode from interactive fails", func(t *testing.T) {
		m := NewManager(config.ModeInteractive)

		ctx := &PlanContext{Approved: true}
		if err := m.EnterAutoMode(ctx); err == nil {
			t.Error("EnterAutoMode() expected error when not in plan mode")
		}
	})

	t.Run("stop returns to interactive", func(t *testing.T) {
		m := NewManager(config.ModeInteractive)
		m.EnterPlanMode()

		m.Stop()

		if !m.IsInteractive() {
			t.Error("expected to be in interactive mode after stop")
		}
	})

	t.Run("stop from auto returns to interactive", func(t *testing.T) {
		m := NewManager(config.ModeInteractive)
		m.EnterPlanMode()
		m.EnterAutoMode(&PlanContext{Approved: true})

		m.Stop()

		if !m.IsInteractive() {
			t.Error("expected to be in interactive mode after stop")
		}
	})
}

func TestPlanContext(t *testing.T) {
	t.Run("plan context is set in plan mode", func(t *testing.T) {
		m := NewManager(config.ModeInteractive)
		m.EnterPlanMode()

		ctx := m.GetPlanContext()
		if ctx == nil {
			t.Error("expected plan context to be set")
		}
	})

	t.Run("set plan approved", func(t *testing.T) {
		m := NewManager(config.ModeInteractive)
		m.EnterPlanMode()

		m.SetPlanApproved(true)

		ctx := m.GetPlanContext()
		if !ctx.Approved {
			t.Error("expected plan to be approved")
		}
	})

	t.Run("add issue to plan", func(t *testing.T) {
		m := NewManager(config.ModeInteractive)
		m.EnterPlanMode()

		m.AddIssue(123)
		m.AddIssue(456)

		ctx := m.GetPlanContext()
		if len(ctx.IssueNumbers) != 2 {
			t.Errorf("expected 2 issues, got %d", len(ctx.IssueNumbers))
		}
		if ctx.IssueNumbers[0] != 123 || ctx.IssueNumbers[1] != 456 {
			t.Errorf("unexpected issue numbers: %v", ctx.IssueNumbers)
		}
	})

	t.Run("plan context cleared on stop", func(t *testing.T) {
		m := NewManager(config.ModeInteractive)
		m.EnterPlanMode()
		m.AddIssue(123)

		m.Stop()

		if ctx := m.GetPlanContext(); ctx != nil {
			t.Error("expected plan context to be nil after stop")
		}
	})
}

func TestStatusString(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*Manager)
		contains string
	}{
		{
			name:     "interactive mode",
			setup:    func(m *Manager) {},
			contains: "Interactive",
		},
		{
			name: "plan mode",
			setup: func(m *Manager) {
				m.EnterPlanMode()
			},
			contains: "Plan Mode",
		},
		{
			name: "plan mode approved",
			setup: func(m *Manager) {
				m.EnterPlanMode()
				m.SetPlanApproved(true)
			},
			contains: "approved",
		},
		{
			name: "auto mode",
			setup: func(m *Manager) {
				m.EnterPlanMode()
				m.EnterAutoMode(&PlanContext{Approved: true})
			},
			contains: "Auto Mode",
		},
		{
			name: "auto mode with issues",
			setup: func(m *Manager) {
				m.EnterPlanMode()
				m.AddIssue(42)
				m.EnterAutoMode(&PlanContext{Approved: true, IssueNumbers: []int{42}})
			},
			contains: "42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager(config.ModeInteractive)
			tt.setup(m)

			status := m.StatusString()
			if !containsString(status, tt.contains) {
				t.Errorf("StatusString() = %q, expected to contain %q", status, tt.contains)
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestShouldEnforceMention(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*Manager)
		expected bool
	}{
		{
			name:     "interactive mode - OFF",
			setup:    func(m *Manager) {},
			expected: false,
		},
		{
			name: "plan mode - ON",
			setup: func(m *Manager) {
				m.EnterPlanMode()
			},
			expected: true,
		},
		{
			name: "auto mode - ON",
			setup: func(m *Manager) {
				m.EnterPlanMode()
				m.EnterAutoMode(&PlanContext{Approved: true})
			},
			expected: true,
		},
		{
			name: "after stop - OFF",
			setup: func(m *Manager) {
				m.EnterPlanMode()
				m.EnterAutoMode(&PlanContext{Approved: true})
				m.Stop()
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager(config.ModeInteractive)
			tt.setup(m)

			got := m.ShouldEnforceMention()
			if got != tt.expected {
				t.Errorf("ShouldEnforceMention() = %v, want %v", got, tt.expected)
			}
		})
	}
}
