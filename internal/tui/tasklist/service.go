package tasklist

// TaskService defines the interface for task management.
// Yuki will implement the actual backend (in-memory for now).
type TaskService interface {
	// List returns all tasks sorted by ID (ascending).
	List() []Task

	// Add creates a new task and returns it with assigned ID.
	Add(title, description, assignee string) Task

	// UpdateStatus changes the status of a task.
	UpdateStatus(id int, status Status) error

	// Delete removes a task by ID.
	Delete(id int) error
}
