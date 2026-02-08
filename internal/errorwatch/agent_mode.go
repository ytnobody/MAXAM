// Package errorwatch provides error detection and automatic follow-up for agent errors.
package errorwatch

import (
	"sync"
	"time"
)

// AgentMode represents the mode of an individual agent
type AgentMode string

const (
	// AgentModeNormal is the default mode where the agent operates normally
	AgentModeNormal AgentMode = "normal"
	// AgentModeAuto is the mode where the agent operates automatically after error recovery
	AgentModeAuto AgentMode = "auto"
)

// AgentModeState tracks the mode state of an agent
type AgentModeState struct {
	Mode         AgentMode
	SwitchedAt   time.Time
	ErrorMessage string // The error that triggered auto mode
}

// AgentModeManager manages per-agent mode states
type AgentModeManager struct {
	mu     sync.RWMutex
	states map[string]*AgentModeState

	// Callback for mode changes
	onModeChange func(agentName string, from, to AgentMode)
}

// NewAgentModeManager creates a new agent mode manager
func NewAgentModeManager() *AgentModeManager {
	return &AgentModeManager{
		states: make(map[string]*AgentModeState),
	}
}

// SetOnModeChange sets the callback for mode changes
func (m *AgentModeManager) SetOnModeChange(fn func(agentName string, from, to AgentMode)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onModeChange = fn
}

// GetMode returns the current mode of an agent
func (m *AgentModeManager) GetMode(agentName string) AgentMode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.states[agentName]
	if !exists {
		return AgentModeNormal
	}
	return state.Mode
}

// GetState returns the full state of an agent
func (m *AgentModeManager) GetState(agentName string) *AgentModeState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.states[agentName]
	if !exists {
		return &AgentModeState{Mode: AgentModeNormal}
	}
	// Return a copy to avoid race conditions
	return &AgentModeState{
		Mode:         state.Mode,
		SwitchedAt:   state.SwitchedAt,
		ErrorMessage: state.ErrorMessage,
	}
}

// IsAuto returns true if the agent is in auto mode
func (m *AgentModeManager) IsAuto(agentName string) bool {
	return m.GetMode(agentName) == AgentModeAuto
}

// SwitchToAuto switches an agent to auto mode
func (m *AgentModeManager) SwitchToAuto(agentName string, errorMessage string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldMode := AgentModeNormal
	if state, exists := m.states[agentName]; exists {
		oldMode = state.Mode
	}

	// Already in auto mode, update error message
	if oldMode == AgentModeAuto {
		m.states[agentName].ErrorMessage = errorMessage
		return
	}

	m.states[agentName] = &AgentModeState{
		Mode:         AgentModeAuto,
		SwitchedAt:   time.Now(),
		ErrorMessage: errorMessage,
	}

	if m.onModeChange != nil {
		m.onModeChange(agentName, oldMode, AgentModeAuto)
	}
}

// SwitchToNormal switches an agent back to normal mode
func (m *AgentModeManager) SwitchToNormal(agentName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.states[agentName]
	if !exists || state.Mode == AgentModeNormal {
		return
	}

	oldMode := state.Mode
	m.states[agentName] = &AgentModeState{
		Mode:       AgentModeNormal,
		SwitchedAt: time.Now(),
	}

	if m.onModeChange != nil {
		m.onModeChange(agentName, oldMode, AgentModeNormal)
	}
}

// Reset clears the mode state for an agent
func (m *AgentModeManager) Reset(agentName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.states, agentName)
}

// ResetAll clears all mode states
func (m *AgentModeManager) ResetAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states = make(map[string]*AgentModeState)
}

// AllAutoAgents returns a list of agents currently in auto mode
func (m *AgentModeManager) AllAutoAgents() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var agents []string
	for name, state := range m.states {
		if state.Mode == AgentModeAuto {
			agents = append(agents, name)
		}
	}
	return agents
}

// AllStates returns all current states
func (m *AgentModeManager) AllStates() map[string]*AgentModeState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*AgentModeState)
	for name, state := range m.states {
		result[name] = &AgentModeState{
			Mode:         state.Mode,
			SwitchedAt:   state.SwitchedAt,
			ErrorMessage: state.ErrorMessage,
		}
	}
	return result
}
