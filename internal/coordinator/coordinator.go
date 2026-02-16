// Package coordinator provides unified coordination between
// heartbeat monitoring, recovery handling, and task handover.
package coordinator

import (
	"context"
	"sync"
	"time"

	"github.com/ytnobody/MAXAM/internal/taskqueue"
)

// AgentState represents the health state of an agent.
type AgentState string

const (
	// StateHealthy indicates the agent is responding normally.
	StateHealthy AgentState = "healthy"
	// StateUnresponsive indicates the agent is not responding.
	StateUnresponsive AgentState = "unresponsive"
	// StateRecovering indicates recovery is in progress.
	StateRecovering AgentState = "recovering"
	// StateFailed indicates recovery failed after max retries.
	StateFailed AgentState = "failed"
)

// Config holds coordinator configuration.
type Config struct {
	HealthCheckInterval time.Duration // Interval between health checks
	ResponseTimeout     time.Duration // Timeout for agent responses
	MaxRetries          int           // Maximum retry attempts
	BackoffBase         time.Duration // Base duration for exponential backoff
	BackoffMax          time.Duration // Maximum backoff duration
}

// DefaultConfig returns the default coordinator configuration.
func DefaultConfig() Config {
	return Config{
		HealthCheckInterval: 5 * time.Second,
		ResponseTimeout:     10 * time.Second,
		MaxRetries:          3,
		BackoffBase:         5 * time.Second,
		BackoffMax:          60 * time.Second,
	}
}

// AgentInfo holds information about an agent's state.
type AgentInfo struct {
	Name         string
	State        AgentState
	FailCount    int
	LastCheck    time.Time
	LastResponse time.Time
	Tasks        []int // Issue numbers assigned to this agent
}

// RecoverFunc is called to attempt recovery of an agent.
type RecoverFunc func(ctx context.Context, agentName string) error

// NotifyFunc is called to send notifications.
type NotifyFunc func(ctx context.Context, target, message string) error

// GetTasksFunc returns the tasks assigned to an agent.
type GetTasksFunc func(agentName string) []int

// Coordinator manages the coordination between heartbeat, recovery, and task queue.
type Coordinator struct {
	cfg        Config
	queue      *taskqueue.Queue
	recoverFn  RecoverFunc
	notifyFn   NotifyFunc
	getTasksFn GetTasksFunc
	agents     map[string]*AgentInfo
	mu         sync.RWMutex
}

// NewCoordinator creates a new coordinator.
func NewCoordinator(cfg Config, queue *taskqueue.Queue) *Coordinator {
	if cfg.HealthCheckInterval <= 0 {
		cfg.HealthCheckInterval = DefaultConfig().HealthCheckInterval
	}
	if cfg.ResponseTimeout <= 0 {
		cfg.ResponseTimeout = DefaultConfig().ResponseTimeout
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = DefaultConfig().MaxRetries
	}
	if cfg.BackoffBase <= 0 {
		cfg.BackoffBase = DefaultConfig().BackoffBase
	}
	if cfg.BackoffMax <= 0 {
		cfg.BackoffMax = DefaultConfig().BackoffMax
	}

	return &Coordinator{
		cfg:    cfg,
		queue:  queue,
		agents: make(map[string]*AgentInfo),
	}
}

// SetRecoverFunc sets the recovery function.
func (c *Coordinator) SetRecoverFunc(fn RecoverFunc) {
	c.recoverFn = fn
}

// SetNotifyFunc sets the notification function.
func (c *Coordinator) SetNotifyFunc(fn NotifyFunc) {
	c.notifyFn = fn
}

// SetGetTasksFunc sets the function to get tasks for an agent.
func (c *Coordinator) SetGetTasksFunc(fn GetTasksFunc) {
	c.getTasksFn = fn
}

// RegisterAgent registers an agent for monitoring.
func (c *Coordinator) RegisterAgent(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.agents[name] = &AgentInfo{
		Name:         name,
		State:        StateHealthy,
		LastCheck:    time.Now(),
		LastResponse: time.Now(),
	}
}

// UnregisterAgent removes an agent from monitoring.
func (c *Coordinator) UnregisterAgent(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.agents, name)
}

// HandleHeartbeat processes a successful heartbeat from an agent.
func (c *Coordinator) HandleHeartbeat(agentName string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	agent, exists := c.agents[agentName]
	if !exists {
		// Auto-register if not exists
		agent = &AgentInfo{
			Name: agentName,
		}
		c.agents[agentName] = agent
	}

	agent.State = StateHealthy
	agent.FailCount = 0
	agent.LastCheck = time.Now()
	agent.LastResponse = time.Now()
}

// HandleFailure processes a failure detection for an agent.
// Returns true if recovery was successful.
func (c *Coordinator) HandleFailure(ctx context.Context, agentName string) bool {
	c.mu.Lock()
	agent, exists := c.agents[agentName]
	if !exists {
		c.mu.Unlock()
		return false
	}

	agent.FailCount++
	agent.LastCheck = time.Now()
	agent.State = StateUnresponsive
	failCount := agent.FailCount
	c.mu.Unlock()

	// Step 1: Try recovery with exponential backoff
	if failCount <= c.cfg.MaxRetries {
		agent.State = StateRecovering

		backoff := c.calculateBackoff(failCount)
		select {
		case <-ctx.Done():
			return false
		case <-time.After(backoff):
		}

		if c.recoverFn != nil {
			if err := c.recoverFn(ctx, agentName); err == nil {
				c.HandleHeartbeat(agentName) // Reset state on success
				return true
			}
		}
	}

	// Step 2: Recovery failed - move tasks to queue
	if failCount >= c.cfg.MaxRetries {
		c.mu.Lock()
		agent.State = StateFailed
		c.mu.Unlock()

		c.handoverTasks(ctx, agentName)
	}

	return false
}

// calculateBackoff calculates the backoff duration using exponential formula.
// backoff = base * 2^(attempt-1), capped at backoff_max
func (c *Coordinator) calculateBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return c.cfg.BackoffBase
	}

	backoff := c.cfg.BackoffBase
	for i := 1; i < attempt; i++ {
		backoff *= 2
		if backoff > c.cfg.BackoffMax {
			return c.cfg.BackoffMax
		}
	}
	return backoff
}

// handoverTasks moves tasks from a failed agent to the queue.
func (c *Coordinator) handoverTasks(ctx context.Context, agentName string) {
	if c.getTasksFn == nil || c.queue == nil {
		return
	}

	tasks := c.getTasksFn(agentName)
	for _, issueNumber := range tasks {
		task := taskqueue.PendingTask{
			IssueNumber:    issueNumber,
			OriginalAgent:  agentName,
			Priority:       1, // Default priority
			ContextSummary: "Handed over due to agent failure",
		}
		_ = c.queue.Add(task) // Best effort
	}

	// Notify PM about the handover
	if c.notifyFn != nil {
		message := "Agent " + agentName + " failed. Tasks queued for reassignment."
		_ = c.notifyFn(ctx, "mei", message)
	}
}

// GetAgentState returns the current state of an agent.
func (c *Coordinator) GetAgentState(agentName string) (AgentInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	agent, exists := c.agents[agentName]
	if !exists {
		return AgentInfo{}, false
	}
	return *agent, true
}

// GetAllAgents returns all registered agents.
func (c *Coordinator) GetAllAgents() map[string]AgentInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]AgentInfo, len(c.agents))
	for name, agent := range c.agents {
		result[name] = *agent
	}
	return result
}

// GetQueue returns the task queue.
func (c *Coordinator) GetQueue() *taskqueue.Queue {
	return c.queue
}
