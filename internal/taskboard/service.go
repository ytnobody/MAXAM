package taskboard

import "context"

// Service defines the interface for task management
type Service interface {
	// List returns all tasks sorted by ID (ascending)
	List(ctx context.Context) ([]*Task, error)

	// Get returns a task by ID
	Get(ctx context.Context, id int) (*Task, error)

	// Create creates a new task and returns it with assigned ID
	Create(ctx context.Context, task *Task) (*Task, error)

	// Update updates an existing task
	Update(ctx context.Context, task *Task) error

	// Delete removes a task by ID
	Delete(ctx context.Context, id int) error
}
