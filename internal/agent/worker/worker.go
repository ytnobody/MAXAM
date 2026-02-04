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
}

// NewWorker creates a new worker for an agent
func NewWorker(name string, runner *agent.Runner) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	return &Worker{
		name:     name,
		runner:   runner,
		state:    NewAgentState(),
		chatChan: make(chan ChatRequest, 10),
		taskChan: make(chan TaskRequest, 10),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// SetPromptBuilder sets the callback for building prompts
func (w *Worker) SetPromptBuilder(fn func(agentName, input string) string) {
	w.buildPrompt = fn
}

// Start begins the chat and task goroutines
func (w *Worker) Start() {
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

	for {
		select {
		case <-w.ctx.Done():
			return
		case req, ok := <-w.chatChan:
			if !ok {
				return
			}
			w.handleChat(req)
		}
	}
}

// handleChat processes a single chat request
func (w *Worker) handleChat(req ChatRequest) {
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
	prompt := req.Input
	if w.buildPrompt != nil {
		prompt = w.buildPrompt(w.name, req.Input)
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

// taskLoop handles task requests
func (w *Worker) taskLoop() {
	defer w.wg.Done()

	for {
		select {
		case <-w.ctx.Done():
			return
		case req, ok := <-w.taskChan:
			if !ok {
				return
			}
			w.handleTask(req)
		}
	}
}

// handleTask processes a single task request
func (w *Worker) handleTask(req TaskRequest) {
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

// Pool manages multiple workers
type Pool struct {
	workers map[string]*Worker
	mu      sync.RWMutex
}

// NewPool creates a new worker pool
func NewPool() *Pool {
	return &Pool{
		workers: make(map[string]*Worker),
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

// StopAll stops all workers
func (p *Pool) StopAll() {
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
