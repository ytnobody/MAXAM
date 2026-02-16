package handover

import "errors"

// Client errors for handover operations.
var (
	// ErrNotFound is returned when the requested task is not found.
	ErrNotFound = errors.New("task not found")

	// ErrConflict is returned when the task is already assigned.
	ErrConflict = errors.New("task already assigned")

	// ErrInvalidTargetAgent is returned when target agent is empty.
	ErrInvalidTargetAgent = errors.New("target agent cannot be empty")

	// ErrInvalidTaskID is returned when task ID is invalid.
	ErrInvalidTaskID = errors.New("task ID must be positive")

	// ErrServerError is returned for server-side errors.
	ErrServerError = errors.New("server error")
)
