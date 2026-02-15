package procmgr

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// Manager manages agent processes.
// It implements healthmgr.RecoveryHandler interface.
type Manager struct {
	mu              sync.RWMutex
	config          Config
	processes       map[string]*Process
	factory         ProcessFactory
	running         bool
	ctx             context.Context
	cancel          context.CancelFunc
}

// NewManager creates a new process manager.
func NewManager(config Config, factory ProcessFactory) *Manager {
	if factory == nil {
		factory = defaultProcessFactory
	}
	return &Manager{
		config:    config,
		processes: make(map[string]*Process),
		factory:   factory,
	}
}

// defaultProcessFactory creates a simple echo command for testing.
func defaultProcessFactory(agentName string) *exec.Cmd {
	return exec.Command("echo", agentName)
}

// Start initializes the manager and makes it ready to manage processes.
func (m *Manager) Start(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return
	}

	m.ctx, m.cancel = context.WithCancel(ctx)
	m.running = true
}

// Stop stops all managed processes and shuts down the manager.
func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	if m.cancel != nil {
		m.cancel()
	}
	m.mu.Unlock()

	// Stop all processes
	m.mu.RLock()
	names := make([]string, 0, len(m.processes))
	for name := range m.processes {
		names = append(names, name)
	}
	m.mu.RUnlock()

	for _, name := range names {
		_ = m.StopAgent(context.Background(), name)
	}
}

// StartAgent starts or restarts an agent process.
// Implements healthmgr.RecoveryHandler interface.
func (m *Manager) StartAgent(ctx context.Context, agentName string) error {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return fmt.Errorf("manager not running")
	}

	// Get or create process entry
	proc, exists := m.processes[agentName]
	if !exists {
		proc = &Process{
			name:  agentName,
			state: StateIdle,
		}
		m.processes[agentName] = proc
	}
	m.mu.Unlock()

	// Lock the process for state transition
	proc.mu.Lock()
	defer proc.mu.Unlock()

	// If already running, stop first
	if proc.state == StateRunning && proc.cmd != nil && proc.cmd.Process != nil {
		proc.mu.Unlock()
		if err := m.StopAgent(ctx, agentName); err != nil {
			proc.mu.Lock()
			return fmt.Errorf("failed to stop existing process: %w", err)
		}
		proc.mu.Lock()
	}

	// Create new command
	cmd := m.factory(agentName)
	if cmd == nil {
		proc.state = StateFailed
		proc.lastError = fmt.Errorf("factory returned nil command")
		return proc.lastError
	}

	// Set process group for signal propagation
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Start the process
	if err := cmd.Start(); err != nil {
		proc.state = StateFailed
		proc.lastError = err
		return fmt.Errorf("failed to start process: %w", err)
	}

	proc.cmd = cmd
	proc.state = StateRunning
	proc.startedAt = time.Now()
	proc.lastError = nil

	// Monitor process in background
	go m.monitorProcess(agentName, cmd)

	return nil
}

// StopAgent stops an agent process gracefully.
// Implements healthmgr.RecoveryHandler interface.
func (m *Manager) StopAgent(ctx context.Context, agentName string) error {
	m.mu.RLock()
	proc, exists := m.processes[agentName]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("agent %s not found", agentName)
	}

	proc.mu.Lock()
	defer proc.mu.Unlock()

	// Already stopped or not running
	if proc.state != StateRunning || proc.cmd == nil || proc.cmd.Process == nil {
		return nil
	}

	proc.state = StateStopping

	// Send SIGTERM first
	if err := proc.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		// Process might already be dead
		if err == os.ErrProcessDone {
			proc.state = StateStopped
			proc.stoppedAt = time.Now()
			return nil
		}
		// Try SIGKILL directly
		return m.forceKill(proc)
	}

	// Wait for graceful shutdown with timeout
	done := make(chan error, 1)
	go func() {
		_, err := proc.cmd.Process.Wait()
		done <- err
	}()

	select {
	case <-done:
		proc.state = StateStopped
		proc.stoppedAt = time.Now()
		return nil
	case <-time.After(m.config.GracefulTimeout):
		// Timeout, force kill
		return m.forceKill(proc)
	case <-ctx.Done():
		// Context cancelled, force kill
		_ = m.forceKill(proc)
		return ctx.Err()
	}
}

// forceKill sends SIGKILL to the process.
// Must be called with proc.mu held.
func (m *Manager) forceKill(proc *Process) error {
	if proc.cmd == nil || proc.cmd.Process == nil {
		return nil
	}

	if err := proc.cmd.Process.Kill(); err != nil {
		if err == os.ErrProcessDone {
			proc.state = StateStopped
			proc.stoppedAt = time.Now()
			return nil
		}
		proc.state = StateFailed
		proc.lastError = err
		return fmt.Errorf("failed to kill process: %w", err)
	}

	proc.state = StateStopped
	proc.stoppedAt = time.Now()
	return nil
}

// monitorProcess monitors a running process and updates state on exit.
func (m *Manager) monitorProcess(agentName string, cmd *exec.Cmd) {
	// Wait for process to exit
	err := cmd.Wait()

	m.mu.RLock()
	proc, exists := m.processes[agentName]
	m.mu.RUnlock()

	if !exists {
		return
	}

	proc.mu.Lock()
	defer proc.mu.Unlock()

	// Only update if this is still the same command
	if proc.cmd != cmd {
		return
	}

	proc.stoppedAt = time.Now()
	if err != nil {
		proc.state = StateFailed
		proc.lastError = err
	} else {
		proc.state = StateStopped
	}
}

// GetProcess returns the process info for an agent.
func (m *Manager) GetProcess(agentName string) *Process {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.processes[agentName]
}

// GetAllProcesses returns all managed processes.
func (m *Manager) GetAllProcesses() map[string]*Process {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*Process, len(m.processes))
	for name, proc := range m.processes {
		result[name] = proc
	}
	return result
}

// IsRunning checks if the manager is running.
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}
