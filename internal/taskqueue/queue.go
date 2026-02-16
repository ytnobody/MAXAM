// Package taskqueue provides task handover queue management.
package taskqueue

import (
	"sync"
	"time"
)

// HandoverStatus represents the status of a pending task.
type HandoverStatus string

const (
	// HandoverStatusPending indicates the task is waiting for reassignment.
	HandoverStatusPending HandoverStatus = "pending"
	// HandoverStatusAssigned indicates the task has been assigned to another agent.
	HandoverStatusAssigned HandoverStatus = "assigned"
	// HandoverStatusCompleted indicates the handover was completed successfully.
	HandoverStatusCompleted HandoverStatus = "completed"
	// HandoverStatusFailed indicates the handover failed.
	HandoverStatusFailed HandoverStatus = "failed"
)

// HandoverEvent represents a single event in the handover history.
type HandoverEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Event     string    `json:"event"`
	From      string    `json:"from,omitempty"`
	To        string    `json:"to,omitempty"`
	Reason    string    `json:"reason,omitempty"`
}

// PendingTask represents a task waiting for handover.
type PendingTask struct {
	IssueNumber    int             `json:"issue_number"`
	OriginalAgent  string          `json:"original_agent"`
	AssignedAgent  string          `json:"assigned_agent,omitempty"`
	Context        string          `json:"context,omitempty"`
	ContextSummary string          `json:"context_summary,omitempty"`
	Priority       int             `json:"priority"`
	Status         HandoverStatus  `json:"status"`
	CreatedAt      time.Time       `json:"created_at"`
	History        []HandoverEvent `json:"history,omitempty"`
}

// TaskStore defines the interface for task storage.
// Phase 1: MemoryStore implementation
// Phase 2: FileStore implementation (future)
type TaskStore interface {
	// Add adds a task to the queue.
	Add(task PendingTask) error
	// Get retrieves a task by issue number.
	Get(issueNumber int) (*PendingTask, error)
	// List returns all pending tasks.
	List() ([]PendingTask, error)
	// Update updates a task in the queue.
	Update(task PendingTask) error
	// Remove removes a task from the queue.
	Remove(issueNumber int) error
}

// Queue manages pending tasks with thread-safe operations.
type Queue struct {
	store TaskStore
	mu    sync.RWMutex
}

// NewQueue creates a new task queue with the given store.
func NewQueue(store TaskStore) *Queue {
	return &Queue{
		store: store,
	}
}

// Add adds a new pending task to the queue.
func (q *Queue) Add(task PendingTask) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Set defaults
	if task.Status == "" {
		task.Status = HandoverStatusPending
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}

	// Add initial event to history
	task.History = append(task.History, HandoverEvent{
		Timestamp: time.Now(),
		Event:     "queued",
		From:      task.OriginalAgent,
		Reason:    "agent_unresponsive",
	})

	return q.store.Add(task)
}

// Get retrieves a task by issue number.
func (q *Queue) Get(issueNumber int) (*PendingTask, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.store.Get(issueNumber)
}

// List returns all pending tasks.
func (q *Queue) List() ([]PendingTask, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.store.List()
}

// ListPending returns only tasks with pending status.
func (q *Queue) ListPending() ([]PendingTask, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	allTasks, err := q.store.List()
	if err != nil {
		return nil, err
	}

	var pending []PendingTask
	for _, task := range allTasks {
		if task.Status == HandoverStatusPending {
			pending = append(pending, task)
		}
	}
	return pending, nil
}

// Assign assigns a pending task to a new agent.
func (q *Queue) Assign(issueNumber int, targetAgent string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, err := q.store.Get(issueNumber)
	if err != nil {
		return err
	}

	task.AssignedAgent = targetAgent
	task.Status = HandoverStatusAssigned
	task.History = append(task.History, HandoverEvent{
		Timestamp: time.Now(),
		Event:     "assigned",
		To:        targetAgent,
		Reason:    "auto_reassignment",
	})

	return q.store.Update(*task)
}

// Complete marks a task as completed.
func (q *Queue) Complete(issueNumber int) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, err := q.store.Get(issueNumber)
	if err != nil {
		return err
	}

	task.Status = HandoverStatusCompleted
	task.History = append(task.History, HandoverEvent{
		Timestamp: time.Now(),
		Event:     "completed",
		Reason:    "handover_success",
	})

	return q.store.Update(*task)
}

// Fail marks a task as failed.
func (q *Queue) Fail(issueNumber int, reason string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, err := q.store.Get(issueNumber)
	if err != nil {
		return err
	}

	task.Status = HandoverStatusFailed
	task.History = append(task.History, HandoverEvent{
		Timestamp: time.Now(),
		Event:     "failed",
		Reason:    reason,
	})

	return q.store.Update(*task)
}

// Remove removes a task from the queue.
func (q *Queue) Remove(issueNumber int) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.store.Remove(issueNumber)
}
