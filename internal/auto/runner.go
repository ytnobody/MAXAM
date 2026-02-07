// Package auto implements the automatic execution mode runner
package auto

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// TaskState represents the state of a task
type TaskState string

const (
	// TaskStatePending means the task is waiting to be executed
	TaskStatePending TaskState = "pending"
	// TaskStateRunning means the task is currently being executed
	TaskStateRunning TaskState = "running"
	// TaskStateCompleted means the task finished successfully
	TaskStateCompleted TaskState = "completed"
	// TaskStateFailed means the task failed
	TaskStateFailed TaskState = "failed"
)

// Task represents a task to be executed
type Task struct {
	IssueNumber int
	Title       string
	State       TaskState
	StartedAt   time.Time
	CompletedAt time.Time
	Error       error
	PRNumber    int // PR number created after completion
}

// TaskExecutor is the interface for executing tasks
// Implementations can be stubs or actual Claude agent integrations
type TaskExecutor interface {
	// Execute runs the task implementation
	// Returns the PR number on success, or an error on failure
	Execute(ctx context.Context, issueNumber int, title string) (prNumber int, err error)
}

// RunnerState represents the state of the runner
type RunnerState string

const (
	// RunnerStateIdle means the runner is not running
	RunnerStateIdle RunnerState = "idle"
	// RunnerStateRunning means the runner is executing tasks
	RunnerStateRunning RunnerState = "running"
	// RunnerStatePaused means the runner is paused
	RunnerStatePaused RunnerState = "paused"
	// RunnerStateCompleted means all tasks are done
	RunnerStateCompleted RunnerState = "completed"
)

// Runner manages the automatic execution of approved tasks
type Runner struct {
	mu sync.RWMutex

	executor TaskExecutor
	tasks    []*Task
	state    RunnerState

	// callbacks
	onTaskStart    func(task *Task)
	onTaskComplete func(task *Task)
	onAllComplete  func(tasks []*Task)

	// config
	logger *log.Logger
}

// Config holds configuration for the Runner
type Config struct {
	Logger *log.Logger
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		Logger: log.Default(),
	}
}

// NewRunner creates a new auto mode runner
func NewRunner(executor TaskExecutor, cfg Config) *Runner {
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}
	return &Runner{
		executor: executor,
		tasks:    make([]*Task, 0),
		state:    RunnerStateIdle,
		logger:   cfg.Logger,
	}
}

// SetOnTaskStart sets the callback for when a task starts
func (r *Runner) SetOnTaskStart(fn func(task *Task)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onTaskStart = fn
}

// SetOnTaskComplete sets the callback for when a task completes
func (r *Runner) SetOnTaskComplete(fn func(task *Task)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onTaskComplete = fn
}

// SetOnAllComplete sets the callback for when all tasks are done
func (r *Runner) SetOnAllComplete(fn func(tasks []*Task)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onAllComplete = fn
}

// AddTask adds a task to be executed
func (r *Runner) AddTask(issueNumber int, title string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tasks = append(r.tasks, &Task{
		IssueNumber: issueNumber,
		Title:       title,
		State:       TaskStatePending,
	})
}

// AddTasks adds multiple tasks to be executed
func (r *Runner) AddTasks(issues []IssueInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, issue := range issues {
		r.tasks = append(r.tasks, &Task{
			IssueNumber: issue.Number,
			Title:       issue.Title,
			State:       TaskStatePending,
		})
	}
}

// IssueInfo holds issue information for batch addition
type IssueInfo struct {
	Number int
	Title  string
}

// State returns the current runner state
func (r *Runner) State() RunnerState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state
}

// Tasks returns a copy of all tasks
func (r *Runner) Tasks() []*Task {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Task, len(r.tasks))
	for i, t := range r.tasks {
		copied := *t
		result[i] = &copied
	}
	return result
}

// PendingCount returns the number of pending tasks
func (r *Runner) PendingCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, t := range r.tasks {
		if t.State == TaskStatePending {
			count++
		}
	}
	return count
}

// CompletedCount returns the number of completed tasks
func (r *Runner) CompletedCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, t := range r.tasks {
		if t.State == TaskStateCompleted {
			count++
		}
	}
	return count
}

// FailedCount returns the number of failed tasks
func (r *Runner) FailedCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, t := range r.tasks {
		if t.State == TaskStateFailed {
			count++
		}
	}
	return count
}

// Run starts executing all pending tasks
// Blocks until all tasks are done or context is cancelled
func (r *Runner) Run(ctx context.Context) error {
	r.mu.Lock()
	if r.state == RunnerStateRunning {
		r.mu.Unlock()
		return fmt.Errorf("runner is already running")
	}
	r.state = RunnerStateRunning
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		if r.state == RunnerStateRunning {
			r.state = RunnerStateCompleted
		}
		r.mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		task := r.nextPendingTask()
		if task == nil {
			// All tasks done
			break
		}

		r.executeTask(ctx, task)
	}

	// Call completion callback
	r.mu.RLock()
	onAllComplete := r.onAllComplete
	tasks := make([]*Task, len(r.tasks))
	for i, t := range r.tasks {
		copied := *t
		tasks[i] = &copied
	}
	r.mu.RUnlock()

	if onAllComplete != nil {
		onAllComplete(tasks)
	}

	return nil
}

// nextPendingTask returns the next pending task, or nil if none
func (r *Runner) nextPendingTask() *Task {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, t := range r.tasks {
		if t.State == TaskStatePending {
			return t
		}
	}
	return nil
}

// executeTask executes a single task
func (r *Runner) executeTask(ctx context.Context, task *Task) {
	// Mark as running
	r.mu.Lock()
	task.State = TaskStateRunning
	task.StartedAt = time.Now()
	onTaskStart := r.onTaskStart
	r.mu.Unlock()

	if onTaskStart != nil {
		onTaskStart(task)
	}

	r.logger.Printf("[auto] Starting task: #%d %s", task.IssueNumber, task.Title)

	// Execute the task
	prNumber, err := r.executor.Execute(ctx, task.IssueNumber, task.Title)

	// Update state
	r.mu.Lock()
	task.CompletedAt = time.Now()
	if err != nil {
		task.State = TaskStateFailed
		task.Error = err
		r.logger.Printf("[auto] Task failed: #%d - %v", task.IssueNumber, err)
	} else {
		task.State = TaskStateCompleted
		task.PRNumber = prNumber
		r.logger.Printf("[auto] Task completed: #%d -> PR #%d", task.IssueNumber, prNumber)
	}
	onTaskComplete := r.onTaskComplete
	r.mu.Unlock()

	if onTaskComplete != nil {
		onTaskComplete(task)
	}
}

// Stop stops the runner
func (r *Runner) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state = RunnerStateIdle
}

// Pause pauses the runner
func (r *Runner) Pause() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == RunnerStateRunning {
		r.state = RunnerStatePaused
	}
}

// Resume resumes a paused runner
func (r *Runner) Resume() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == RunnerStatePaused {
		r.state = RunnerStateRunning
	}
}

// Reset clears all tasks and returns to idle state
func (r *Runner) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks = make([]*Task, 0)
	r.state = RunnerStateIdle
}

// Summary returns a summary string of the runner state
func (r *Runner) Summary() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	pending := 0
	running := 0
	completed := 0
	failed := 0

	for _, t := range r.tasks {
		switch t.State {
		case TaskStatePending:
			pending++
		case TaskStateRunning:
			running++
		case TaskStateCompleted:
			completed++
		case TaskStateFailed:
			failed++
		}
	}

	return fmt.Sprintf("Auto Mode: %s | Tasks: %d total, %d pending, %d running, %d completed, %d failed",
		r.state, len(r.tasks), pending, running, completed, failed)
}
