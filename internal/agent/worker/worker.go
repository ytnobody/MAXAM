package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ytnobody/MAXAM/internal/agent"
)

// ChatRequest represents a chat/conversation request
type ChatRequest struct {
	Input    string
	Response chan<- ChatResponse
}

// ChatResponse is the response to a chat request
type ChatResponse struct {
	Content string
	Elapsed time.Duration
	Err     error
}

// TaskRequest represents a work task request
type TaskRequest struct {
	Description string
	Prompt      string
	Response    chan<- TaskResponse
}

// TaskResponse is the response to a task request
type TaskResponse struct {
	Content string
	Elapsed time.Duration
	Err     error
}

// DefaultSessionTimeout is the maximum time a worker session can run before forced restart
const DefaultSessionTimeout = 3 * time.Minute

// Worker manages an agent's chat and task goroutines
type Worker struct {
	name   string
	runner *agent.Runner
	state  *AgentState

	chatChan chan ChatRequest
	taskChan chan TaskRequest

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Callback for building prompts with context
	buildPrompt func(agentName, input string) string

	// Session timeout management
	sessionStart   time.Time
	sessionTimeout time.Duration

	// Mutex for protecting buildPrompt and context fields during Restart
	mu sync.RWMutex
}

// NewWorker creates a new worker for an agent
func NewWorker(name string, runner *agent.Runner) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	return &Worker{
		name:           name,
		runner:         runner,
		state:          NewAgentState(),
		chatChan:       make(chan ChatRequest, 10),
		taskChan:       make(chan TaskRequest, 10),
		ctx:            ctx,
		cancel:         cancel,
		sessionTimeout: DefaultSessionTimeout,
	}
}

// SetSessionTimeout sets the session timeout duration
func (w *Worker) SetSessionTimeout(d time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.sessionTimeout = d
}

// SessionElapsed returns the time elapsed since the session started
func (w *Worker) SessionElapsed() time.Duration {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.sessionStart.IsZero() {
		return 0
	}
	return time.Since(w.sessionStart)
}

// IsSessionExpired returns true if the session has exceeded its timeout
func (w *Worker) IsSessionExpired() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.sessionStart.IsZero() || w.sessionTimeout == 0 {
		return false
	}
	return time.Since(w.sessionStart) > w.sessionTimeout
}

// SetPromptBuilder sets the callback for building prompts
func (w *Worker) SetPromptBuilder(fn func(agentName, input string) string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buildPrompt = fn
}

// Start begins the chat and task goroutines
func (w *Worker) Start() {
	w.mu.Lock()
	w.sessionStart = time.Now()
	w.mu.Unlock()
	w.wg.Add(2)
	go w.chatLoop()
	go w.taskLoop()
}

// Stop gracefully shuts down the worker
func (w *Worker) Stop() {
	w.cancel()
	close(w.chatChan)
	close(w.taskChan)
	w.wg.Wait()
}

// State returns the agent's current state
func (w *Worker) State() *AgentState {
	return w.state
}

// Name returns the worker's name
func (w *Worker) Name() string {
	return w.name
}

// Pause requests the worker to stop after completing current task
func (w *Worker) Pause() {
	w.state.RequestStop()
}

// Resume resumes a stopped worker
func (w *Worker) Resume() {
	w.state.Resume()
}

// IsStopped returns true if the worker is stopped
func (w *Worker) IsStopped() bool {
	return w.state.IsStopped()
}

// Kill forcefully terminates the worker by canceling its context
// This immediately stops any ongoing work without waiting for completion
func (w *Worker) Kill() {
	w.cancel()
	w.state.ForceStop()
}

// SendChat sends a chat request and returns immediately
// The response will be sent to the provided channel
func (w *Worker) SendChat(input string, response chan<- ChatResponse) {
	select {
	case w.chatChan <- ChatRequest{Input: input, Response: response}:
	case <-w.ctx.Done():
		response <- ChatResponse{Err: w.ctx.Err()}
	}
}

// SendTask sends a task request and returns immediately
func (w *Worker) SendTask(description, prompt string, response chan<- TaskResponse) {
	select {
	case w.taskChan <- TaskRequest{Description: description, Prompt: prompt, Response: response}:
	case <-w.ctx.Done():
		response <- TaskResponse{Err: w.ctx.Err()}
	}
}

// chatLoop handles chat requests - responds immediately even when working
func (w *Worker) chatLoop() {
	defer w.wg.Done()

	// Capture the context and channel at the start of this loop iteration.
	// This prevents races when Restart() modifies w.ctx and w.chatChan.
	w.mu.RLock()
	ctx := w.ctx
	chatChan := w.chatChan
	w.mu.RUnlock()

	for {
		select {
		case <-ctx.Done():
			return
		case req, ok := <-chatChan:
			if !ok {
				return
			}
			w.handleChat(req)
		}
	}
}

// handleChat processes a single chat request
func (w *Worker) handleChat(req ChatRequest) {
	// If stopped, return stopped message
	if w.state.IsStopped() {
		req.Response <- ChatResponse{
			Content: "停止中です。resumeコマンドで再開してください。",
			Elapsed: 0,
			Err:     nil,
		}
		return
	}

	// If working, return status message without calling LLM
	if w.state.IsWorking() {
		status := w.state.GetStatus()
		req.Response <- ChatResponse{
			Content: fmt.Sprintf("今%sの作業中。完了したら対応する。", status),
			Elapsed: 0,
			Err:     nil,
		}
		return
	}

	// Available - process normally
	// Mark as working during chat processing
	desc := req.Input
	if len(desc) > 30 {
		desc = desc[:30] + "..."
	}
	w.state.StartTask(desc)
	defer w.state.CompleteTask()

	prompt := req.Input
	// Read buildPrompt under RLock to prevent race with SetPromptBuilder
	w.mu.RLock()
	buildFn := w.buildPrompt
	w.mu.RUnlock()
	if buildFn != nil {
		prompt = buildFn(w.name, req.Input)
	}

	ctx, cancel := context.WithTimeout(w.ctx, 5*time.Minute)
	defer cancel()

	start := time.Now()
	result, err := w.runner.Run(ctx, prompt)
	elapsed := time.Since(start)

	req.Response <- ChatResponse{
		Content: result,
		Elapsed: elapsed,
		Err:     err,
	}
}

// ErrAgentStopped is returned when attempting to send a task to a stopped agent
var ErrAgentStopped = fmt.Errorf("agent is stopped")

// taskLoop handles task requests
func (w *Worker) taskLoop() {
	defer w.wg.Done()

	// Capture the context and channel at the start of this loop iteration.
	// This prevents races when Restart() modifies w.ctx and w.taskChan.
	w.mu.RLock()
	ctx := w.ctx
	taskChan := w.taskChan
	w.mu.RUnlock()

	for {
		select {
		case <-ctx.Done():
			return
		case req, ok := <-taskChan:
			if !ok {
				return
			}
			// Reject task if stopped
			if w.state.IsStopped() {
				req.Response <- TaskResponse{
					Err: ErrAgentStopped,
				}
				continue
			}
			w.handleTask(req)
		}
	}
}

// handleTask processes a single task request
func (w *Worker) handleTask(req TaskRequest) {
	// If already working on another task, return status message
	if w.state.IsWorking() {
		status := w.state.GetStatus()
		req.Response <- TaskResponse{
			Content: fmt.Sprintf("今%sの作業中。完了したら対応する。", status),
			Elapsed: 0,
			Err:     nil,
		}
		return
	}

	// Mark as working
	w.state.StartTask(req.Description)
	defer w.state.CompleteTask()

	ctx, cancel := context.WithTimeout(w.ctx, 10*time.Minute)
	defer cancel()

	start := time.Now()
	result, err := w.runner.Run(ctx, req.Prompt)
	elapsed := time.Since(start)

	req.Response <- TaskResponse{
		Content: result,
		Elapsed: elapsed,
		Err:     err,
	}
}

// SessionTimeoutCallback is called when a worker's session times out
type SessionTimeoutCallback func(workerName string)

// SessionRestartEvent represents a session restart notification
type SessionRestartEvent struct {
	WorkerName string
}

// Pool manages multiple workers
type Pool struct {
	workers map[string]*Worker
	mu      sync.RWMutex

	// Session timeout monitoring
	monitorCtx       context.Context
	monitorCancel    context.CancelFunc
	monitorWg        sync.WaitGroup
	onSessionTimeout SessionTimeoutCallback

	// Session restart notification channel for TUI integration
	restartNotifyChan chan SessionRestartEvent
}

// NewPool creates a new worker pool
func NewPool() *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	return &Pool{
		workers:           make(map[string]*Worker),
		monitorCtx:        ctx,
		monitorCancel:     cancel,
		restartNotifyChan: make(chan SessionRestartEvent, 10),
	}
}

// GetRestartNotifyChannel returns the channel for session restart notifications
// TUI can subscribe to this channel to receive restart events
func (p *Pool) GetRestartNotifyChannel() <-chan SessionRestartEvent {
	return p.restartNotifyChan
}

// SetSessionTimeoutCallback sets the callback for session timeout events
func (p *Pool) SetSessionTimeoutCallback(fn SessionTimeoutCallback) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onSessionTimeout = fn
}

// StartSessionMonitor starts the session timeout monitoring goroutine
// It checks all workers every checkInterval and triggers restart for expired sessions
func (p *Pool) StartSessionMonitor(checkInterval time.Duration) {
	p.monitorWg.Add(1)
	go p.sessionMonitorLoop(checkInterval)
}

// sessionMonitorLoop periodically checks for expired sessions
func (p *Pool) sessionMonitorLoop(checkInterval time.Duration) {
	defer p.monitorWg.Done()
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.monitorCtx.Done():
			return
		case <-ticker.C:
			p.checkAndRestartExpiredSessions()
		}
	}
}

// checkAndRestartExpiredSessions checks all workers and restarts those with expired sessions
func (p *Pool) checkAndRestartExpiredSessions() {
	p.mu.RLock()
	expiredWorkers := make([]string, 0)
	for name, w := range p.workers {
		if w.IsSessionExpired() {
			expiredWorkers = append(expiredWorkers, name)
		}
	}
	callback := p.onSessionTimeout
	p.mu.RUnlock()

	// Restart expired workers (outside the read lock)
	for _, name := range expiredWorkers {
		// Kill and restart
		p.Kill(name)
		p.Restart(name)

		// Notify callback if set
		if callback != nil {
			callback(name)
		}

		// Send restart notification to TUI (non-blocking)
		select {
		case p.restartNotifyChan <- SessionRestartEvent{WorkerName: name}:
		default:
			// Channel full, skip notification
		}
	}
}

// Add adds a worker to the pool
func (p *Pool) Add(w *Worker) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.workers[w.name] = w
}

// Get returns a worker by name
func (p *Pool) Get(name string) (*Worker, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	w, ok := p.workers[name]
	return w, ok
}

// StartAll starts all workers
func (p *Pool) StartAll() {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, w := range p.workers {
		w.Start()
	}
}

// StopAll stops all workers and the session monitor
func (p *Pool) StopAll() {
	// Stop the session monitor first
	p.monitorCancel()
	p.monitorWg.Wait()

	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, w := range p.workers {
		w.Stop()
	}
}

// GetStatus returns the status of all workers
func (p *Pool) GetStatus() map[string]string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	status := make(map[string]string)
	for name, w := range p.workers {
		status[name] = w.state.GetStatus()
	}
	return status
}

// All returns all worker names
func (p *Pool) All() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	names := make([]string, 0, len(p.workers))
	for name := range p.workers {
		names = append(names, name)
	}
	return names
}

// PauseAll pauses all workers
func (p *Pool) PauseAll() {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, w := range p.workers {
		w.Pause()
	}
}

// ResumeAll resumes all workers
func (p *Pool) ResumeAll() {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, w := range p.workers {
		w.Resume()
	}
}

// Pause pauses a specific worker by name
func (p *Pool) Pause(name string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if w, ok := p.workers[name]; ok {
		w.Pause()
		return true
	}
	return false
}

// Resume resumes a specific worker by name
func (p *Pool) Resume(name string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if w, ok := p.workers[name]; ok {
		w.Resume()
		return true
	}
	return false
}

// GetStoppedWorkers returns names of stopped workers
func (p *Pool) GetStoppedWorkers() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	stopped := make([]string, 0)
	for name, w := range p.workers {
		if w.IsStopped() {
			stopped = append(stopped, name)
		}
	}
	return stopped
}

// Kill forcefully terminates a specific worker by name
// Returns true if the worker was found and killed
func (p *Pool) Kill(name string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if w, ok := p.workers[name]; ok {
		w.Kill()
		return true
	}
	return false
}

// Restart restarts a killed worker by creating a new context
func (p *Pool) Restart(name string) bool {
	p.mu.Lock()
	w, ok := p.workers[name]
	p.mu.Unlock()

	if !ok {
		return false
	}

	// Wait for old goroutines to finish
	w.wg.Wait()

	p.mu.Lock()
	defer p.mu.Unlock()
	// Re-check worker still exists after releasing lock
	if _, ok := p.workers[name]; !ok {
		return false
	}

	// Create new context and restart goroutines
	ctx, cancel := context.WithCancel(context.Background())
	w.mu.Lock()
	w.ctx = ctx
	w.cancel = cancel
	w.chatChan = make(chan ChatRequest, 10)
	w.taskChan = make(chan TaskRequest, 10)
	w.sessionStart = time.Now() // Reset session start time
	w.mu.Unlock()
	w.state.Resume()
	w.wg.Add(2)
	go w.chatLoop()
	go w.taskLoop()
	return true
}
