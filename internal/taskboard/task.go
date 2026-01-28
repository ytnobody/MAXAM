// Package taskboard provides lightweight task management for TUI
package taskboard

import "time"

// Status represents task status
type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
)

// Task represents a lightweight task for team status display
type Task struct {
	ID          int
	Title       string
	Assignee    string
	Status      Status
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
