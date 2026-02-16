// Package taskqueue provides task management and handover functionality for agent coordination.
package taskqueue

import "time"

// TaskStatus represents the current status of a task.
type TaskStatus string

const (
	// TaskStatusPending indicates the task is waiting to be processed.
	TaskStatusPending TaskStatus = "pending"
	// TaskStatusInProgress indicates the task is currently being processed.
	TaskStatusInProgress TaskStatus = "in_progress"
	// TaskStatusCompleted indicates the task has been completed.
	TaskStatusCompleted TaskStatus = "completed"
	// TaskStatusFailed indicates the task has failed.
	TaskStatusFailed TaskStatus = "failed"
	// TaskStatusHandover indicates the task is being handed over to another agent.
	TaskStatusHandover TaskStatus = "handover"
)

// Task represents a unit of work assigned to an agent.
type Task struct {
	// ID is the unique identifier for the task.
	ID string
	// Description is a human-readable description of the task.
	Description string
	// AssignedTo is the name of the agent currently assigned to this task.
	AssignedTo string
	// Status is the current status of the task.
	Status TaskStatus
	// CreatedAt is when the task was created.
	CreatedAt time.Time
	// StartedAt is when the task was started.
	StartedAt time.Time
	// CompletedAt is when the task was completed or failed.
	CompletedAt time.Time
	// Handoverable indicates if this task can be handed over to another agent.
	Handoverable bool
	// HandoverCount tracks how many times this task has been handed over.
	HandoverCount int
	// MaxHandovers is the maximum number of handovers allowed (0 = unlimited).
	MaxHandovers int
	// Metadata holds additional task-specific data.
	Metadata map[string]string
	// PreviousAgents tracks agents who previously worked on this task.
	PreviousAgents []string
}

// HandoverRequest represents a request to hand over a task to another agent.
type HandoverRequest struct {
	// TaskID is the ID of the task to hand over.
	TaskID string
	// FromAgent is the agent handing over the task.
	FromAgent string
	// ToAgent is the target agent (empty for auto-assignment).
	ToAgent string
	// Reason explains why the handover is happening.
	Reason string
}

// HandoverResult represents the outcome of a handover attempt.
type HandoverResult struct {
	// Success indicates if the handover was successful.
	Success bool
	// TaskID is the ID of the task that was handed over.
	TaskID string
	// FromAgent is the agent who handed over the task.
	FromAgent string
	// ToAgent is the agent who received the task.
	ToAgent string
	// Error contains the error message if handover failed.
	Error string
}

// AgentCapability describes what an agent can handle.
type AgentCapability struct {
	// AgentName is the name of the agent.
	AgentName string
	// Available indicates if the agent is available for new tasks.
	Available bool
	// CurrentTasks is the number of tasks currently assigned.
	CurrentTasks int
	// MaxTasks is the maximum number of concurrent tasks.
	MaxTasks int
	// Tags are capability tags for matching tasks.
	Tags []string
}
