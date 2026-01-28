// Package tasklist provides a lightweight task board component for TUI.
package tasklist

// Status represents the current state of a task.
type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
)

// Task represents a single task in the task board.
type Task struct {
	ID          int
	Title       string
	Description string
	Assignee    string
	Status      Status
}

// String returns the status as a display string.
func (s Status) String() string {
	switch s {
	case StatusPending:
		return "Pending"
	case StatusInProgress:
		return "In Progress"
	case StatusDone:
		return "Done"
	default:
		return string(s)
	}
}
