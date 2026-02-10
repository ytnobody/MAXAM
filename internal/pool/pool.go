// Package pool manages agent worker pools for concurrent request handling
package pool

import (
	"context"
	"errors"
	"sync"

	"github.com/ytnobody/MAXAM/internal/agent"
)

var (
	// ErrNoWorkerAvailable is returned when all workers are busy and queue is full
	ErrNoWorkerAvailable = errors.New("no worker available")
	// ErrAgentNotFound is returned when the requested agent doesn't exist
	ErrAgentNotFound = errors.New("agent not found")
	// ErrPoolClosed is returned when pool is closed
	ErrPoolClosed = errors.New("pool is closed")
)

// Task represents a queued task
type Task struct {
	AgentName string
	Prompt    string
	Result    chan Result
}

// Result represents the result of a task execution
type Result struct {
	Output string
	Err    error
}

// Worker represents a single worker for an agent
type Worker struct {
	ID        int
	AgentName string
	Runner    *agent.Runner
	busy      bool
	mu        sync.Mutex
}

// IsBusy returns whether the worker is currently busy
func (w *Worker) IsBusy() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.busy
}

// setBusy sets the worker's busy state
func (w *Worker) setBusy(busy bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.busy = busy
}

// Pool manages workers for each agent
type Pool struct {
	workers    map[string][]*Worker // agent name -> workers
	queues     map[string]chan Task // agent name -> task queue
	agents     *agent.Agents
	workersNum int // workers per agent
	queueSize  int
	mu         sync.RWMutex
	closed     bool
	wg         sync.WaitGroup
}

// Config holds pool configuration
type Config struct {
	WorkersPerAgent int // Number of workers per agent (default: 1)
	QueueSize       int // Size of task queue per agent (default: 10)
}

// DefaultConfig returns default pool configuration
func DefaultConfig() Config {
	return Config{
		WorkersPerAgent: 1,
		QueueSize:       10,
	}
}

// New creates a new agent pool
func New(agents *agent.Agents, cfg Config) *Pool {
	if cfg.WorkersPerAgent <= 0 {
		cfg.WorkersPerAgent = 1
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 10
	}

	return &Pool{
		workers:    make(map[string][]*Worker),
		queues:     make(map[string]chan Task),
		agents:     agents,
		workersNum: cfg.WorkersPerAgent,
		queueSize:  cfg.QueueSize,
	}
}

// Initialize sets up workers for an agent
func (p *Pool) Initialize(agentName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return ErrPoolClosed
	}

	// Already initialized
	if _, exists := p.workers[agentName]; exists {
		return nil
	}

	// Check if agents is nil
	if p.agents == nil {
		return ErrAgentNotFound
	}

	runner, ok := p.agents.Get(agentName)
	if !ok {
		return ErrAgentNotFound
	}

	// Create workers
	workers := make([]*Worker, p.workersNum)
	for i := 0; i < p.workersNum; i++ {
		workers[i] = &Worker{
			ID:        i,
			AgentName: agentName,
			Runner:    runner,
		}
	}
	p.workers[agentName] = workers

	// Create queue and start worker goroutines
	queue := make(chan Task, p.queueSize)
	p.queues[agentName] = queue

	for _, w := range workers {
		p.wg.Add(1)
		go p.runWorker(w, queue)
	}

	return nil
}

// runWorker processes tasks from the queue
func (p *Pool) runWorker(w *Worker, queue <-chan Task) {
	defer p.wg.Done()

	for task := range queue {
		w.setBusy(true)
		output, err := w.Runner.Run(context.Background(), task.Prompt)
		w.setBusy(false)

		task.Result <- Result{
			Output: output,
			Err:    err,
		}
	}
}

// Dispatch sends a task to an agent's pool
func (p *Pool) Dispatch(ctx context.Context, agentName, prompt string) (string, error) {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return "", ErrPoolClosed
	}

	queue, exists := p.queues[agentName]
	p.mu.RUnlock()

	if !exists {
		// Initialize on first use
		if err := p.Initialize(agentName); err != nil {
			return "", err
		}
		p.mu.RLock()
		queue = p.queues[agentName]
		p.mu.RUnlock()
	}

	resultChan := make(chan Result, 1)
	task := Task{
		AgentName: agentName,
		Prompt:    prompt,
		Result:    resultChan,
	}

	select {
	case queue <- task:
		// Task queued
	case <-ctx.Done():
		return "", ctx.Err()
	default:
		return "", ErrNoWorkerAvailable
	}

	select {
	case result := <-resultChan:
		return result.Output, result.Err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Stats returns pool statistics
type Stats struct {
	AgentName    string
	TotalWorkers int
	BusyWorkers  int
	QueueLength  int
}

// GetStats returns statistics for an agent
func (p *Pool) GetStats(agentName string) (Stats, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	workers, exists := p.workers[agentName]
	if !exists {
		return Stats{}, ErrAgentNotFound
	}

	busyCount := 0
	for _, w := range workers {
		if w.IsBusy() {
			busyCount++
		}
	}

	queueLen := 0
	if queue, ok := p.queues[agentName]; ok {
		queueLen = len(queue)
	}

	return Stats{
		AgentName:    agentName,
		TotalWorkers: len(workers),
		BusyWorkers:  busyCount,
		QueueLength:  queueLen,
	}, nil
}

// AllStats returns statistics for all agents
func (p *Pool) AllStats() []Stats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var stats []Stats
	for agentName := range p.workers {
		s, _ := p.GetStats(agentName)
		stats = append(stats, s)
	}
	return stats
}

// Close shuts down all workers
func (p *Pool) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true

	// Close all queues
	for _, queue := range p.queues {
		close(queue)
	}
	p.mu.Unlock()

	// Wait for all workers to finish
	p.wg.Wait()
}
