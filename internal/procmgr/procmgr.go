// Package procmgr provides process management for agent subprocesses.
package procmgr

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// ProcessState represents the state of a managed process.
type ProcessState int

const (
	// StateNotStarted means the process hasn't been started yet.
	StateNotStarted ProcessState = iota
	// StateRunning means the process is running.
	StateRunning
	// StateStopped means the process has stopped.
	StateStopped
	// StateRestarting means the process is being restarted.
	StateRestarting
)

func (s ProcessState) String() string {
	switch s {
	case StateNotStarted:
		return "not_started"
	case StateRunning:
		return "running"
	case StateStopped:
		return "stopped"
	case StateRestarting:
		return "restarting"
	default:
		return "unknown"
	}
}

// ProcessInfo holds information about a managed process.
type ProcessInfo struct {
	Name         string
	PID          int
	State        ProcessState
	StartedAt    time.Time
	RestartCount int
	LastError    error
	WorkDir      string
	Branch       string // Git branch for state restoration
}

// ProcessConfig holds configuration for starting a process.
type ProcessConfig struct {
	Name    string
	Command string
	Args    []string
	WorkDir string
	Env     []string
}

// Manager manages multiple agent processes.
type Manager struct {
	processes map[string]*ManagedProcess
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewManager creates a new process manager.
func NewManager() *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		processes: make(map[string]*ManagedProcess),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// ManagedProcess represents a single managed process.
type ManagedProcess struct {
	config       ProcessConfig
	cmd          *exec.Cmd
	info         ProcessInfo
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	onExit       func(name string, err error)
	restartDelay time.Duration
	waitOnce     sync.Once
	waitErr      error
	waitDone     chan struct{}
}

// Start starts a new process with the given configuration.
func (m *Manager) Start(cfg ProcessConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.processes[cfg.Name]; exists {
		return fmt.Errorf("process %s already exists", cfg.Name)
	}

	proc, err := m.startProcess(cfg)
	if err != nil {
		return err
	}

	m.processes[cfg.Name] = proc
	return nil
}

// startProcess creates and starts a new managed process.
func (m *Manager) startProcess(cfg ProcessConfig) (*ManagedProcess, error) {
	ctx, cancel := context.WithCancel(m.ctx)

	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	cmd.Dir = cfg.WorkDir
	if len(cfg.Env) > 0 {
		cmd.Env = append(os.Environ(), cfg.Env...)
	}

	// Redirect stdout/stderr if needed (for debugging)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	proc := &ManagedProcess{
		config: cfg,
		cmd:    cmd,
		info: ProcessInfo{
			Name:    cfg.Name,
			State:   StateNotStarted,
			WorkDir: cfg.WorkDir,
		},
		ctx:          ctx,
		cancel:       cancel,
		restartDelay: 5 * time.Second, // Default delay between restarts
		waitDone:     make(chan struct{}),
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start process %s: %w", cfg.Name, err)
	}

	proc.mu.Lock()
	proc.info.PID = cmd.Process.Pid
	proc.info.State = StateRunning
	proc.info.StartedAt = time.Now()
	proc.mu.Unlock()

	// Monitor process in background
	go proc.monitor()

	return proc, nil
}

// monitor watches the process and handles exit.
func (p *ManagedProcess) monitor() {
	p.waitOnce.Do(func() {
		p.waitErr = p.cmd.Wait()
		close(p.waitDone)
	})

	p.mu.Lock()
	p.info.State = StateStopped
	p.info.LastError = p.waitErr
	handler := p.onExit
	p.mu.Unlock()

	if handler != nil {
		handler(p.config.Name, p.waitErr)
	}
}

// Stop stops a process gracefully (SIGTERM, then SIGKILL).
func (m *Manager) Stop(name string) error {
	m.mu.RLock()
	proc, exists := m.processes[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("process %s not found", name)
	}

	return proc.stop()
}

// stop stops the process gracefully.
func (p *ManagedProcess) stop() error {
	p.mu.Lock()

	if p.info.State != StateRunning {
		p.mu.Unlock()
		return nil
	}

	// Try SIGTERM first
	if p.cmd.Process != nil {
		if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
			// Process might already be dead
			if !errors.Is(err, os.ErrProcessDone) {
				p.mu.Unlock()
				return fmt.Errorf("failed to send SIGTERM: %w", err)
			}
		}
	}

	// Release lock before waiting to avoid blocking
	waitDone := p.waitDone
	p.mu.Unlock()

	// Wait for process to exit (monitor goroutine will close waitDone)
	select {
	case <-waitDone:
		p.mu.Lock()
		p.info.State = StateStopped
		p.mu.Unlock()
		return nil
	case <-time.After(5 * time.Second):
		// Force kill
		p.mu.Lock()
		if p.cmd.Process != nil {
			p.cmd.Process.Kill()
		}
		p.info.State = StateStopped
		p.mu.Unlock()
		return nil
	}
}

// Restart restarts a process.
func (m *Manager) Restart(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	proc, exists := m.processes[name]
	if !exists {
		return fmt.Errorf("process %s not found", name)
	}

	// Stop existing process
	if err := proc.stop(); err != nil {
		// Continue anyway, process might be dead
	}

	proc.mu.Lock()
	proc.info.State = StateRestarting
	proc.info.RestartCount++
	proc.mu.Unlock()

	// Small delay before restart
	time.Sleep(proc.restartDelay)

	// Start new process
	newProc, err := m.startProcess(proc.config)
	if err != nil {
		proc.mu.Lock()
		proc.info.State = StateStopped
		proc.info.LastError = err
		proc.mu.Unlock()
		return err
	}

	// Copy restart count
	newProc.info.RestartCount = proc.info.RestartCount
	newProc.onExit = proc.onExit

	m.processes[name] = newProc
	return nil
}

// Kill forcefully kills a process.
func (m *Manager) Kill(name string) error {
	m.mu.RLock()
	proc, exists := m.processes[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("process %s not found", name)
	}

	proc.mu.Lock()
	defer proc.mu.Unlock()

	if proc.cmd.Process != nil {
		proc.cmd.Process.Kill()
	}
	proc.info.State = StateStopped
	return nil
}

// GetInfo returns information about a process.
func (m *Manager) GetInfo(name string) (ProcessInfo, bool) {
	m.mu.RLock()
	proc, exists := m.processes[name]
	m.mu.RUnlock()

	if !exists {
		return ProcessInfo{}, false
	}

	proc.mu.RLock()
	defer proc.mu.RUnlock()
	return proc.info, true
}

// GetAllInfo returns information about all processes.
func (m *Manager) GetAllInfo() map[string]ProcessInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]ProcessInfo)
	for name, proc := range m.processes {
		proc.mu.RLock()
		result[name] = proc.info
		proc.mu.RUnlock()
	}
	return result
}

// SetOnExit sets a callback for when a process exits.
func (m *Manager) SetOnExit(name string, fn func(name string, err error)) error {
	m.mu.RLock()
	proc, exists := m.processes[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("process %s not found", name)
	}

	proc.mu.Lock()
	proc.onExit = fn
	proc.mu.Unlock()
	return nil
}

// IsRunning returns true if the process is running.
func (m *Manager) IsRunning(name string) bool {
	info, exists := m.GetInfo(name)
	if !exists {
		return false
	}
	return info.State == StateRunning
}

// StopAll stops all processes gracefully.
func (m *Manager) StopAll() {
	m.mu.RLock()
	names := make([]string, 0, len(m.processes))
	for name := range m.processes {
		names = append(names, name)
	}
	m.mu.RUnlock()

	for _, name := range names {
		m.Stop(name)
	}
}

// Close stops all processes and cleans up.
func (m *Manager) Close() {
	m.cancel()
	m.StopAll()
}

// Remove removes a stopped process from management.
func (m *Manager) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	proc, exists := m.processes[name]
	if !exists {
		return nil
	}

	proc.mu.RLock()
	state := proc.info.State
	proc.mu.RUnlock()

	if state == StateRunning {
		return fmt.Errorf("cannot remove running process %s", name)
	}

	delete(m.processes, name)
	return nil
}

// RecoveryAdapter implements healthmgr.RecoveryHandler interface.
// It wraps Manager to provide agent-based start/stop functionality.
type RecoveryAdapter struct {
	manager       *Manager
	configBuilder func(agentName string) ProcessConfig
}

// NewRecoveryAdapter creates a new recovery adapter.
// configBuilder is called to generate ProcessConfig for a given agent name.
func NewRecoveryAdapter(m *Manager, configBuilder func(agentName string) ProcessConfig) *RecoveryAdapter {
	return &RecoveryAdapter{
		manager:       m,
		configBuilder: configBuilder,
	}
}

// StartAgent starts or restarts an agent process.
func (r *RecoveryAdapter) StartAgent(ctx context.Context, agentName string) error {
	// Check if already running
	if r.manager.IsRunning(agentName) {
		// Restart existing process
		return r.manager.Restart(agentName)
	}

	// Start new process
	cfg := r.configBuilder(agentName)
	return r.manager.Start(cfg)
}

// StopAgent stops an agent process.
func (r *RecoveryAdapter) StopAgent(ctx context.Context, agentName string) error {
	if !r.manager.IsRunning(agentName) {
		return nil // Already stopped
	}
	return r.manager.Stop(agentName)
}
