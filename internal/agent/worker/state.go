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
	// StateStopped means the agent is manually stopped and won't accept new tasks
	StateStopped
)

func (s State) String() string {
	switch s {
	case StateAvailable:
		return "available"
	case StateWorking:
		return "working"
	case StateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// AgentState holds the current state of an agent
type AgentState struct {
	mu            sync.RWMutex
	state         State
	currentTask   string    // Description of current task
	startedAt     time.Time // When current task started
	stopRequested bool      // Flag to stop after current task completes
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

// IsStopped returns true if the agent is stopped
func (s *AgentState) IsStopped() bool {
	return s.Get() == StateStopped
}

// IsStopRequested returns true if stop has been requested
func (s *AgentState) IsStopRequested() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stopRequested
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

// CompleteTask transitions to available state or stopped state if stop was requested
func (s *AgentState) CompleteTask() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopRequested {
		s.state = StateStopped
		s.stopRequested = false
	} else {
		s.state = StateAvailable
	}
	s.currentTask = ""
	s.startedAt = time.Time{}
}

// RequestStop sets the stop flag - agent will stop after current task completes
func (s *AgentState) RequestStop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == StateWorking {
		s.stopRequested = true
	} else {
		s.state = StateStopped
	}
}

// Resume transitions from stopped to available state
func (s *AgentState) Resume() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == StateStopped {
		s.state = StateAvailable
		s.stopRequested = false
	}
}

// ForceStop immediately sets state to stopped regardless of current state
func (s *AgentState) ForceStop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = StateStopped
	s.stopRequested = false
	s.currentTask = ""
	s.startedAt = time.Time{}
}

// GetStatus returns a human-readable status message
func (s *AgentState) GetStatus() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	switch s.state {
	case StateAvailable:
		return "待機中"
	case StateStopped:
		return "停止中"
	case StateWorking:
		duration := time.Since(s.startedAt).Round(time.Second)
		status := "作業中"
		if s.currentTask != "" {
			status = s.currentTask
		}
		if s.stopRequested {
			status += " (停止予定)"
		}
		return status + " (" + duration.String() + ")"
	default:
		return "不明"
	}
}
