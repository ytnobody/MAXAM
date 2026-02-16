// Package handover provides API client for task handover operations.
package handover

import "time"

// PendingTask represents a task waiting for handover.
type PendingTask struct {
	IssueNumber    int       `json:"issue_number"`
	Title          string    `json:"title"`
	OriginalAgent  string    `json:"original_agent"`
	Priority       int       `json:"priority"`
	CreatedAt      time.Time `json:"created_at"`
	ContextSummary string    `json:"context_summary"`
}

// PendingTasksResponse is the response for list pending tasks.
type PendingTasksResponse struct {
	Tasks []PendingTask `json:"tasks"`
	Total int           `json:"total"`
}

// AssignRequest is the request body for task assignment.
type AssignRequest struct {
	TargetAgent string `json:"target_agent"`
}

// AssignResult is the response for task assignment.
type AssignResult struct {
	Success     bool   `json:"success"`
	IssueNumber int    `json:"issue_number"`
	AssignedTo  string `json:"assigned_to"`
	Message     string `json:"message,omitempty"`
}

// HandoverDetail contains full handover information.
type HandoverDetail struct {
	IssueNumber  int             `json:"issue_number"`
	Title        string          `json:"title"`
	History      []HandoverEvent `json:"history"`
	CurrentState string          `json:"current_state"`
}

// HandoverEvent represents a single handover event in history.
type HandoverEvent struct {
	Timestamp time.Time `json:"timestamp"`
	FromAgent string    `json:"from_agent"`
	ToAgent   string    `json:"to_agent,omitempty"`
	Reason    string    `json:"reason"`
	Context   string    `json:"context"`
}
