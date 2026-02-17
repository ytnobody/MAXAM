package coordinator

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/ytnobody/MAXAM/internal/taskqueue"
)

// API provides HTTP handlers for taskqueue operations.
type API struct {
	coordinator *Coordinator
}

// NewAPI creates a new API handler.
func NewAPI(coord *Coordinator) *API {
	return &API{
		coordinator: coord,
	}
}

// PendingTasksResponse represents the response for GET /api/tasks/pending
type PendingTasksResponse struct {
	Tasks []PendingTaskItem `json:"tasks"`
	Total int               `json:"total"`
}

// PendingTaskItem represents a task in the pending list
type PendingTaskItem struct {
	IssueNumber    int    `json:"issue_number"`
	OriginalAgent  string `json:"original_agent"`
	Priority       int    `json:"priority"`
	CreatedAt      string `json:"created_at"`
	ContextSummary string `json:"context_summary,omitempty"`
}

// HandoverResponse represents the response for GET /api/tasks/{id}/handover
type HandoverResponse struct {
	IssueNumber   int                       `json:"issue_number"`
	Status        string                    `json:"status"`
	OriginalAgent string                    `json:"original_agent"`
	AssignedAgent string                    `json:"assigned_agent,omitempty"`
	History       []taskqueue.HandoverEvent `json:"history"`
	Context       string                    `json:"context,omitempty"`
}

// AssignRequest represents the request body for POST /api/tasks/{id}/assign
type AssignRequest struct {
	TargetAgent string `json:"target_agent"`
}

// AssignResponse represents the response for POST /api/tasks/{id}/assign
type AssignResponse struct {
	Success     bool   `json:"success"`
	IssueNumber int    `json:"issue_number"`
	AssignedTo  string `json:"assigned_to"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// HandlePendingTasks handles GET /api/tasks/pending
func (a *API) HandlePendingTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	queue := a.coordinator.GetQueue()
	if queue == nil {
		a.writeError(w, http.StatusServiceUnavailable, "task queue not available")
		return
	}

	tasks, err := queue.ListPending()
	if err != nil {
		a.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]PendingTaskItem, len(tasks))
	for i, task := range tasks {
		items[i] = PendingTaskItem{
			IssueNumber:    task.IssueNumber,
			OriginalAgent:  task.OriginalAgent,
			Priority:       task.Priority,
			CreatedAt:      task.CreatedAt.Format("2006-01-02T15:04:05Z"),
			ContextSummary: task.ContextSummary,
		}
	}

	resp := PendingTasksResponse{
		Tasks: items,
		Total: len(items),
	}

	a.writeJSON(w, http.StatusOK, resp)
}

// HandleTaskHandover handles GET /api/tasks/{id}/handover
func (a *API) HandleTaskHandover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract issue number from path
	issueNumber, err := a.extractIssueNumber(r.URL.Path, "/api/tasks/", "/handover")
	if err != nil {
		a.writeError(w, http.StatusBadRequest, "invalid issue number")
		return
	}

	queue := a.coordinator.GetQueue()
	if queue == nil {
		a.writeError(w, http.StatusServiceUnavailable, "task queue not available")
		return
	}

	task, err := queue.Get(issueNumber)
	if err != nil {
		if err == taskqueue.ErrTaskNotFound {
			a.writeError(w, http.StatusNotFound, "task not found")
			return
		}
		a.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := HandoverResponse{
		IssueNumber:   task.IssueNumber,
		Status:        string(task.Status),
		OriginalAgent: task.OriginalAgent,
		AssignedAgent: task.AssignedAgent,
		History:       task.History,
		Context:       task.Context,
	}

	a.writeJSON(w, http.StatusOK, resp)
}

// HandleTaskAssign handles POST /api/tasks/{id}/assign
func (a *API) HandleTaskAssign(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract issue number from path
	issueNumber, err := a.extractIssueNumber(r.URL.Path, "/api/tasks/", "/assign")
	if err != nil {
		a.writeError(w, http.StatusBadRequest, "invalid issue number")
		return
	}

	// Parse request body
	var req AssignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.TargetAgent == "" {
		a.writeError(w, http.StatusBadRequest, "target_agent is required")
		return
	}

	queue := a.coordinator.GetQueue()
	if queue == nil {
		a.writeError(w, http.StatusServiceUnavailable, "task queue not available")
		return
	}

	err = queue.Assign(issueNumber, req.TargetAgent)
	if err != nil {
		if err == taskqueue.ErrTaskNotFound {
			a.writeError(w, http.StatusNotFound, "task not found")
			return
		}
		a.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := AssignResponse{
		Success:     true,
		IssueNumber: issueNumber,
		AssignedTo:  req.TargetAgent,
	}

	a.writeJSON(w, http.StatusOK, resp)
}

// extractIssueNumber extracts issue number from URL path
// e.g., /api/tasks/123/handover -> 123
func (a *API) extractIssueNumber(path, prefix, suffix string) (int, error) {
	// Remove prefix and suffix
	path = strings.TrimPrefix(path, prefix)
	path = strings.TrimSuffix(path, suffix)
	return strconv.Atoi(path)
}

// writeJSON writes a JSON response
func (a *API) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error response
func (a *API) writeError(w http.ResponseWriter, status int, message string) {
	a.writeJSON(w, status, ErrorResponse{Error: message})
}

// RegisterRoutes registers API routes with the given ServeMux
func (a *API) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/tasks/pending", a.HandlePendingTasks)

	// For path parameters, we need a catch-all handler
	// In production, use a proper router like chi or gorilla/mux
	mux.HandleFunc("/api/tasks/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, "/handover") {
			a.HandleTaskHandover(w, r)
		} else if strings.HasSuffix(path, "/assign") {
			a.HandleTaskAssign(w, r)
		} else {
			http.NotFound(w, r)
		}
	})
}
