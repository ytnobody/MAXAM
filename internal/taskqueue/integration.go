package taskqueue

import (
	"context"
	"fmt"
	"sync"
)

// TaskHandoverHandler is called when a task needs to be handed over.
type TaskHandoverHandler func(ctx context.Context, result HandoverResult) error

// NotifyHandler is called to send notifications.
type NotifyHandler func(ctx context.Context, target, message string) error

// Coordinator integrates task queue with health management.
type Coordinator struct {
	mu              sync.RWMutex
	queue           *Queue
	handoverHandler TaskHandoverHandler
	notifyHandler   NotifyHandler
	pmAgent         string
}

// CoordinatorOption is a functional option for Coordinator.
type CoordinatorOption func(*Coordinator)

// WithHandoverHandler sets the handover handler.
func WithHandoverHandler(h TaskHandoverHandler) CoordinatorOption {
	return func(c *Coordinator) {
		c.handoverHandler = h
	}
}

// WithNotifyHandler sets the notify handler.
func WithNotifyHandler(h NotifyHandler) CoordinatorOption {
	return func(c *Coordinator) {
		c.notifyHandler = h
	}
}

// WithPMAgent sets the PM agent name for notifications.
func WithPMAgent(pm string) CoordinatorOption {
	return func(c *Coordinator) {
		c.pmAgent = pm
	}
}

// NewCoordinator creates a new task coordinator.
func NewCoordinator(queue *Queue, opts ...CoordinatorOption) *Coordinator {
	c := &Coordinator{
		queue:   queue,
		pmAgent: "mei", // default PM
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// OnAgentFailure handles agent failure by attempting task handover.
// Returns list of tasks that were successfully handed over.
func (c *Coordinator) OnAgentFailure(ctx context.Context, agentName string, reason string) []HandoverResult {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Get handoverable tasks for the failed agent.
	tasks := c.queue.GetHandoverableTasks(agentName)
	if len(tasks) == 0 {
		return nil
	}

	var results []HandoverResult

	for _, task := range tasks {
		result := c.queue.Handover(HandoverRequest{
			TaskID:    task.ID,
			FromAgent: agentName,
			ToAgent:   "", // Auto-assign.
			Reason:    reason,
		})

		results = append(results, result)

		// Call handover handler if set.
		if result.Success && c.handoverHandler != nil {
			_ = c.handoverHandler(ctx, result)
		}
	}

	// Notify PM about handovers.
	c.notifyHandovers(ctx, agentName, results)

	return results
}

// notifyHandovers sends a summary notification to PM.
func (c *Coordinator) notifyHandovers(ctx context.Context, failedAgent string, results []HandoverResult) {
	if c.notifyHandler == nil || len(results) == 0 {
		return
	}

	successCount := 0
	failedCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		} else {
			failedCount++
		}
	}

	var message string
	if failedCount == 0 {
		message = fmt.Sprintf(
			"エージェント %s 障害発生。%d タスクを他エージェントに引き継ぎ完了。",
			failedAgent, successCount,
		)
	} else {
		message = fmt.Sprintf(
			"エージェント %s 障害発生。引き継ぎ: %d 成功, %d 失敗。手動対応が必要です。",
			failedAgent, successCount, failedCount,
		)
	}

	_ = c.notifyHandler(ctx, c.pmAgent, message)
}

// MarkAgentUnavailable marks an agent as unavailable for new tasks.
func (c *Coordinator) MarkAgentUnavailable(agentName string) error {
	return c.queue.SetAgentAvailability(agentName, false)
}

// MarkAgentAvailable marks an agent as available for new tasks.
func (c *Coordinator) MarkAgentAvailable(agentName string) error {
	return c.queue.SetAgentAvailability(agentName, true)
}

// GetQueue returns the underlying task queue.
func (c *Coordinator) GetQueue() *Queue {
	return c.queue
}

// RegisterTask registers a task and assigns it to an agent.
func (c *Coordinator) RegisterTask(task *Task, agentName string) error {
	c.queue.AddTask(task)
	return c.queue.AssignTask(task.ID, agentName)
}

// CompleteTask marks a task as completed.
func (c *Coordinator) CompleteTask(taskID string) error {
	return c.queue.CompleteTask(taskID)
}

// FailTask marks a task as failed.
func (c *Coordinator) FailTask(taskID, reason string) error {
	return c.queue.FailTask(taskID, reason)
}

// GetTasksForAgent returns all tasks assigned to an agent.
func (c *Coordinator) GetTasksForAgent(agentName string) []*Task {
	return c.queue.GetTasksForAgent(agentName)
}

// GetPendingTasks returns all pending tasks.
func (c *Coordinator) GetPendingTasks() []*Task {
	return c.queue.GetPendingTasks()
}
