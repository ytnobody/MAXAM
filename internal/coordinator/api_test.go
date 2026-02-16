package coordinator

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ytnobody/MAXAM/internal/taskqueue"
)

func setupTestAPI() (*API, *taskqueue.Queue) {
	store := taskqueue.NewMemoryStore()
	queue := taskqueue.NewQueue(store)
	coord := NewCoordinator(DefaultConfig(), queue)
	api := NewAPI(coord)
	return api, queue
}

func TestAPI_HandlePendingTasks_Empty(t *testing.T) {
	api, _ := setupTestAPI()

	req := httptest.NewRequest(http.MethodGet, "/api/tasks/pending", nil)
	w := httptest.NewRecorder()

	api.HandlePendingTasks(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp PendingTasksResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Total != 0 {
		t.Errorf("Total = %d, want 0", resp.Total)
	}
}

func TestAPI_HandlePendingTasks_WithTasks(t *testing.T) {
	api, queue := setupTestAPI()

	// Add some tasks
	_ = queue.Add(taskqueue.PendingTask{
		IssueNumber:    123,
		OriginalAgent:  "yuki-1",
		Priority:       1,
		ContextSummary: "PR review",
	})
	_ = queue.Add(taskqueue.PendingTask{
		IssueNumber:    456,
		OriginalAgent:  "rin-1",
		Priority:       2,
		ContextSummary: "Bug fix",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/tasks/pending", nil)
	w := httptest.NewRecorder()

	api.HandlePendingTasks(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp PendingTasksResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Total != 2 {
		t.Errorf("Total = %d, want 2", resp.Total)
	}

	// Higher priority should come first
	if resp.Tasks[0].IssueNumber != 456 {
		t.Errorf("First task IssueNumber = %d, want 456 (higher priority)", resp.Tasks[0].IssueNumber)
	}
}

func TestAPI_HandlePendingTasks_MethodNotAllowed(t *testing.T) {
	api, _ := setupTestAPI()

	req := httptest.NewRequest(http.MethodPost, "/api/tasks/pending", nil)
	w := httptest.NewRecorder()

	api.HandlePendingTasks(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestAPI_HandleTaskHandover(t *testing.T) {
	api, queue := setupTestAPI()

	// Add a task
	_ = queue.Add(taskqueue.PendingTask{
		IssueNumber:   123,
		OriginalAgent: "yuki-1",
		Context:       "Working on PR #456",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/tasks/123/handover", nil)
	w := httptest.NewRecorder()

	api.HandleTaskHandover(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp HandoverResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.IssueNumber != 123 {
		t.Errorf("IssueNumber = %d, want 123", resp.IssueNumber)
	}
	if resp.OriginalAgent != "yuki-1" {
		t.Errorf("OriginalAgent = %s, want yuki-1", resp.OriginalAgent)
	}
	if resp.Status != "pending" {
		t.Errorf("Status = %s, want pending", resp.Status)
	}
	if len(resp.History) != 1 {
		t.Errorf("History length = %d, want 1", len(resp.History))
	}
}

func TestAPI_HandleTaskHandover_NotFound(t *testing.T) {
	api, _ := setupTestAPI()

	req := httptest.NewRequest(http.MethodGet, "/api/tasks/999/handover", nil)
	w := httptest.NewRecorder()

	api.HandleTaskHandover(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestAPI_HandleTaskAssign(t *testing.T) {
	api, queue := setupTestAPI()

	// Add a task
	_ = queue.Add(taskqueue.PendingTask{
		IssueNumber:   123,
		OriginalAgent: "yuki-1",
	})

	body := bytes.NewBufferString(`{"target_agent": "yuki-2"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/123/assign", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.HandleTaskAssign(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp AssignResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Error("Success = false, want true")
	}
	if resp.IssueNumber != 123 {
		t.Errorf("IssueNumber = %d, want 123", resp.IssueNumber)
	}
	if resp.AssignedTo != "yuki-2" {
		t.Errorf("AssignedTo = %s, want yuki-2", resp.AssignedTo)
	}

	// Verify task was updated
	task, _ := queue.Get(123)
	if task.Status != taskqueue.HandoverStatusAssigned {
		t.Errorf("Task status = %s, want assigned", task.Status)
	}
	if task.AssignedAgent != "yuki-2" {
		t.Errorf("Task AssignedAgent = %s, want yuki-2", task.AssignedAgent)
	}
}

func TestAPI_HandleTaskAssign_MissingTargetAgent(t *testing.T) {
	api, queue := setupTestAPI()

	_ = queue.Add(taskqueue.PendingTask{
		IssueNumber:   123,
		OriginalAgent: "yuki-1",
	})

	body := bytes.NewBufferString(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/123/assign", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.HandleTaskAssign(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestAPI_HandleTaskAssign_NotFound(t *testing.T) {
	api, _ := setupTestAPI()

	body := bytes.NewBufferString(`{"target_agent": "yuki-2"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/999/assign", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	api.HandleTaskAssign(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestAPI_RegisterRoutes(t *testing.T) {
	api, queue := setupTestAPI()

	// Add a task
	_ = queue.Add(taskqueue.PendingTask{
		IssueNumber:   123,
		OriginalAgent: "yuki-1",
	})

	mux := http.NewServeMux()
	api.RegisterRoutes(mux)

	// Test /api/tasks/pending
	req := httptest.NewRequest(http.MethodGet, "/api/tasks/pending", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /api/tasks/pending: Status = %d, want %d", w.Code, http.StatusOK)
	}

	// Test /api/tasks/123/handover
	req = httptest.NewRequest(http.MethodGet, "/api/tasks/123/handover", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /api/tasks/123/handover: Status = %d, want %d", w.Code, http.StatusOK)
	}

	// Test /api/tasks/123/assign
	body := bytes.NewBufferString(`{"target_agent": "rin-1"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/tasks/123/assign", body)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST /api/tasks/123/assign: Status = %d, want %d", w.Code, http.StatusOK)
	}
}
