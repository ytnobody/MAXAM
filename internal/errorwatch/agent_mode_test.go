package errorwatch

import (
	"sync"
	"testing"
	"time"
)

func TestAgentModeManager_GetMode(t *testing.T) {
	m := NewAgentModeManager()

	t.Run("returns normal for unknown agent", func(t *testing.T) {
		mode := m.GetMode("unknown")
		if mode != AgentModeNormal {
			t.Errorf("GetMode(unknown) = %v, want %v", mode, AgentModeNormal)
		}
	})

	t.Run("returns correct mode after switch", func(t *testing.T) {
		m.SwitchToAuto("yuki", "test error")
		mode := m.GetMode("yuki")
		if mode != AgentModeAuto {
			t.Errorf("GetMode(yuki) = %v, want %v", mode, AgentModeAuto)
		}
	})
}

func TestAgentModeManager_SwitchToAuto(t *testing.T) {
	t.Run("switches agent to auto mode", func(t *testing.T) {
		m := NewAgentModeManager()
		m.SwitchToAuto("yuki", "context deadline exceeded")

		if !m.IsAuto("yuki") {
			t.Error("agent should be in auto mode")
		}

		state := m.GetState("yuki")
		if state.ErrorMessage != "context deadline exceeded" {
			t.Errorf("ErrorMessage = %q, want %q", state.ErrorMessage, "context deadline exceeded")
		}
	})

	t.Run("updates error message if already in auto mode", func(t *testing.T) {
		m := NewAgentModeManager()
		m.SwitchToAuto("yuki", "first error")
		m.SwitchToAuto("yuki", "second error")

		state := m.GetState("yuki")
		if state.ErrorMessage != "second error" {
			t.Errorf("ErrorMessage = %q, want %q", state.ErrorMessage, "second error")
		}
	})

	t.Run("triggers callback on mode change", func(t *testing.T) {
		m := NewAgentModeManager()
		var called bool
		var fromMode, toMode AgentMode
		var agentName string

		m.SetOnModeChange(func(name string, from, to AgentMode) {
			called = true
			agentName = name
			fromMode = from
			toMode = to
		})

		m.SwitchToAuto("yuki", "test error")

		if !called {
			t.Error("callback should be called")
		}
		if agentName != "yuki" {
			t.Errorf("agentName = %q, want %q", agentName, "yuki")
		}
		if fromMode != AgentModeNormal {
			t.Errorf("fromMode = %v, want %v", fromMode, AgentModeNormal)
		}
		if toMode != AgentModeAuto {
			t.Errorf("toMode = %v, want %v", toMode, AgentModeAuto)
		}
	})

	t.Run("does not trigger callback if already in auto mode", func(t *testing.T) {
		m := NewAgentModeManager()
		m.SwitchToAuto("yuki", "first error")

		var called bool
		m.SetOnModeChange(func(name string, from, to AgentMode) {
			called = true
		})

		m.SwitchToAuto("yuki", "second error")

		if called {
			t.Error("callback should not be called when already in auto mode")
		}
	})
}

func TestAgentModeManager_SwitchToNormal(t *testing.T) {
	t.Run("switches agent back to normal mode", func(t *testing.T) {
		m := NewAgentModeManager()
		m.SwitchToAuto("yuki", "test error")
		m.SwitchToNormal("yuki")

		if m.IsAuto("yuki") {
			t.Error("agent should be in normal mode")
		}
	})

	t.Run("triggers callback on mode change", func(t *testing.T) {
		m := NewAgentModeManager()
		m.SwitchToAuto("yuki", "test error")

		var called bool
		var fromMode, toMode AgentMode

		m.SetOnModeChange(func(name string, from, to AgentMode) {
			called = true
			fromMode = from
			toMode = to
		})

		m.SwitchToNormal("yuki")

		if !called {
			t.Error("callback should be called")
		}
		if fromMode != AgentModeAuto {
			t.Errorf("fromMode = %v, want %v", fromMode, AgentModeAuto)
		}
		if toMode != AgentModeNormal {
			t.Errorf("toMode = %v, want %v", toMode, AgentModeNormal)
		}
	})

	t.Run("no-op if already in normal mode", func(t *testing.T) {
		m := NewAgentModeManager()

		var called bool
		m.SetOnModeChange(func(name string, from, to AgentMode) {
			called = true
		})

		m.SwitchToNormal("yuki")

		if called {
			t.Error("callback should not be called when already in normal mode")
		}
	})
}

func TestAgentModeManager_AllAutoAgents(t *testing.T) {
	m := NewAgentModeManager()
	m.SwitchToAuto("yuki", "error 1")
	m.SwitchToAuto("rin", "error 2")
	m.SwitchToAuto("mei", "error 3")
	m.SwitchToNormal("rin")

	agents := m.AllAutoAgents()

	if len(agents) != 2 {
		t.Errorf("len(agents) = %d, want 2", len(agents))
	}

	agentSet := make(map[string]bool)
	for _, a := range agents {
		agentSet[a] = true
	}

	if !agentSet["yuki"] {
		t.Error("yuki should be in auto agents")
	}
	if agentSet["rin"] {
		t.Error("rin should not be in auto agents")
	}
	if !agentSet["mei"] {
		t.Error("mei should be in auto agents")
	}
}

func TestAgentModeManager_Reset(t *testing.T) {
	m := NewAgentModeManager()
	m.SwitchToAuto("yuki", "test error")
	m.Reset("yuki")

	mode := m.GetMode("yuki")
	if mode != AgentModeNormal {
		t.Errorf("GetMode(yuki) = %v, want %v after reset", mode, AgentModeNormal)
	}
}

func TestAgentModeManager_ResetAll(t *testing.T) {
	m := NewAgentModeManager()
	m.SwitchToAuto("yuki", "error 1")
	m.SwitchToAuto("rin", "error 2")
	m.ResetAll()

	if m.IsAuto("yuki") {
		t.Error("yuki should not be in auto mode after reset all")
	}
	if m.IsAuto("rin") {
		t.Error("rin should not be in auto mode after reset all")
	}
}

func TestAgentModeManager_GetState(t *testing.T) {
	m := NewAgentModeManager()
	before := time.Now()
	m.SwitchToAuto("yuki", "test error message")
	after := time.Now()

	state := m.GetState("yuki")

	if state.Mode != AgentModeAuto {
		t.Errorf("Mode = %v, want %v", state.Mode, AgentModeAuto)
	}
	if state.ErrorMessage != "test error message" {
		t.Errorf("ErrorMessage = %q, want %q", state.ErrorMessage, "test error message")
	}
	if state.SwitchedAt.Before(before) || state.SwitchedAt.After(after) {
		t.Errorf("SwitchedAt = %v, should be between %v and %v", state.SwitchedAt, before, after)
	}
}

func TestAgentModeManager_Concurrency(t *testing.T) {
	m := NewAgentModeManager()
	var wg sync.WaitGroup

	// Concurrent switches
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			agentName := "agent"
			if n%2 == 0 {
				m.SwitchToAuto(agentName, "error")
			} else {
				m.SwitchToNormal(agentName)
			}
		}(i)
	}

	wg.Wait()

	// Should not panic, state should be consistent
	mode := m.GetMode("agent")
	if mode != AgentModeNormal && mode != AgentModeAuto {
		t.Errorf("unexpected mode: %v", mode)
	}
}

func TestDetector_HandleError(t *testing.T) {
	t.Run("switches to auto mode for recoverable error", func(t *testing.T) {
		d := DefaultDetector()

		result := d.HandleError("yuki", "context deadline exceeded")

		if !result {
			t.Error("HandleError should return true for recoverable error")
		}
		if !d.IsAgentInAutoMode("yuki") {
			t.Error("agent should be in auto mode")
		}
	})

	t.Run("does not switch for non-recoverable error", func(t *testing.T) {
		d := DefaultDetector()

		result := d.HandleError("yuki", "syntax error")

		if result {
			t.Error("HandleError should return false for non-recoverable error")
		}
		if d.IsAgentInAutoMode("yuki") {
			t.Error("agent should not be in auto mode")
		}
	})

	t.Run("handles exit status 1", func(t *testing.T) {
		d := DefaultDetector()

		result := d.HandleError("yuki", "run claude: exit status 1")

		if !result {
			t.Error("HandleError should return true for exit status 1")
		}
		if !d.IsAgentInAutoMode("yuki") {
			t.Error("agent should be in auto mode")
		}
	})
}

func TestDetector_ResetAgentMode(t *testing.T) {
	d := DefaultDetector()
	d.HandleError("yuki", "context deadline exceeded")

	d.ResetAgentMode("yuki")

	if d.IsAgentInAutoMode("yuki") {
		t.Error("agent should not be in auto mode after reset")
	}
}

func TestCheckAndGenerateFollowUp_SwitchesToAutoMode(t *testing.T) {
	d := DefaultDetector()
	result := d.CheckAndGenerateFollowUp("yuki", "context deadline exceeded")

	if !result.SwitchedToAuto {
		t.Error("SwitchedToAuto should be true")
	}
	if !d.IsAgentInAutoMode("yuki") {
		t.Error("agent should be in auto mode")
	}
}
