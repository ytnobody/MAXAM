// Package procmgr provides process management for agent lifecycles.
package procmgr

import (
	"os/exec"
	"sync"
	"time"
)

// Config holds configuration for the process manager.
type Config struct {
	// GracefulTimeout is the duration to wait for graceful shutdown before SIGKILL.
	GracefulTimeout time.Duration

	// StartTimeout is the maximum duration to wait for process startup.
	StartTimeout time.Duration
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		GracefulTimeout: 10 * time.Second,
		StartTimeout:    30 * time.Second,
	}
}

// ProcessState represents the state of a managed process.
type ProcessState int

const (
	// StateIdle indicates the process has not been started.
	StateIdle ProcessState = iota
	// StateRunning indicates the process is currently running.
	StateRunning
	// StateStopping indicates the process is being stopped.
	StateStopping
	// StateStopped indicates the process has been stopped.
	StateStopped
	// StateFailed indicates the process failed to start or crashed.
	StateFailed
)

func (s ProcessState) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateRunning:
		return "running"
	case StateStopping:
		return "stopping"
	case StateStopped:
		return "stopped"
	case StateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// Process represents a managed process.
type Process struct {
	mu        sync.RWMutex
	name      string
	cmd       *exec.Cmd
	state     ProcessState
	startedAt time.Time
	stoppedAt time.Time
	lastError error
}

// Name returns the process name.
func (p *Process) Name() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.name
}

// State returns the current process state.
func (p *Process) State() ProcessState {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

// PID returns the process ID, or 0 if not running.
func (p *Process) PID() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.cmd != nil && p.cmd.Process != nil {
		return p.cmd.Process.Pid
	}
	return 0
}

// LastError returns the last error encountered.
func (p *Process) LastError() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastError
}

// StartedAt returns when the process was started.
func (p *Process) StartedAt() time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.startedAt
}

// StoppedAt returns when the process was stopped.
func (p *Process) StoppedAt() time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stoppedAt
}

// ProcessFactory creates exec.Cmd for a given agent name.
// This allows customization of how processes are created.
type ProcessFactory func(agentName string) *exec.Cmd
