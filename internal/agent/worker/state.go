package worker

import (
	"sync"
	"time"
)

// State represents the current state of an agent
type State int

const (
	// StateAvailable means the agent can accept new tasks
	StateAvailable State = iota
	// StateWorking means the agent is currently executing a task
	StateWorking
)

func (s State) String() string {
	switch s {
	case StateAvailable:
		return "available"
	case StateWorking:
		return "working"
	default:
		return "unknown"
	}
}

// AgentState holds the current state of an agent
type AgentState struct {
	mu          sync.RWMutex
	state       State
	currentTask string    // Description of current task
	startedAt   time.Time // When current task started
}

// NewAgentState creates a new agent state
func NewAgentState() *AgentState {
	return &AgentState{
		state: StateAvailable,
	}
}

// Get returns the current state
func (s *AgentState) Get() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// IsWorking returns true if the agent is currently working
func (s *AgentState) IsWorking() bool {
	return s.Get() == StateWorking
}

// IsAvailable returns true if the agent is available
func (s *AgentState) IsAvailable() bool {
	return s.Get() == StateAvailable
}

// GetCurrentTask returns the current task description
func (s *AgentState) GetCurrentTask() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentTask
}

// GetTaskDuration returns how long the current task has been running
func (s *AgentState) GetTaskDuration() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.state != StateWorking {
		return 0
	}
	return time.Since(s.startedAt)
}

// StartTask transitions to working state
func (s *AgentState) StartTask(description string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = StateWorking
	s.currentTask = description
	s.startedAt = time.Now()
}

// CompleteTask transitions to available state
func (s *AgentState) CompleteTask() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = StateAvailable
	s.currentTask = ""
	s.startedAt = time.Time{}
}

// GetStatus returns a human-readable status message
func (s *AgentState) GetStatus() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.state == StateAvailable {
		return "待機中"
	}

	duration := time.Since(s.startedAt).Round(time.Second)
	if s.currentTask != "" {
		return s.currentTask + " (" + duration.String() + ")"
	}
	return "作業中 (" + duration.String() + ")"
}
