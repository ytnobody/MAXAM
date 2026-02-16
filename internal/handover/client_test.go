package handover

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_ListPending(t *testing.T) {
	tests := []struct {
		name       string
		response   PendingTasksResponse
		statusCode int
		wantErr    bool
		wantCount  int
	}{
		{
			name: "success with tasks",
			response: PendingTasksResponse{
				Tasks: []PendingTask{
					{IssueNumber: 123, Title: "Task 1", OriginalAgent: "yuki"},
					{IssueNumber: 456, Title: "Task 2", OriginalAgent: "rin"},
				},
				Total: 2,
			},
			statusCode: http.StatusOK,
			wantErr:    false,
			wantCount:  2,
		},
		{
			name: "success with no tasks",
			response: PendingTasksResponse{
				Tasks: []PendingTask{},
				Total: 0,
			},
			statusCode: http.StatusOK,
			wantErr:    false,
			wantCount:  0,
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/tasks/pending" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				if r.Method != http.MethodGet {
					t.Errorf("unexpected method: %s", r.Method)
				}

				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					json.NewEncoder(w).Encode(tt.response)
				}
			}))
			defer server.Close()

			client := NewClient(server.URL, WithTimeout(5*time.Second))
			tasks, err := client.ListPending(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("ListPending() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(tasks) != tt.wantCount {
				t.Errorf("ListPending() got %d tasks, want %d", len(tasks), tt.wantCount)
			}
		})
	}
}

func TestClient_Assign(t *testing.T) {
	tests := []struct {
		name        string
		taskID      int
		targetAgent string
		response    AssignResult
		statusCode  int
		wantErr     error
	}{
		{
			name:        "success",
			taskID:      123,
			targetAgent: "rin",
			response: AssignResult{
				Success:     true,
				IssueNumber: 123,
				AssignedTo:  "rin",
			},
			statusCode: http.StatusOK,
			wantErr:    nil,
		},
		{
			name:        "invalid task ID",
			taskID:      0,
			targetAgent: "rin",
			wantErr:     ErrInvalidTaskID,
		},
		{
			name:        "empty target agent",
			taskID:      123,
			targetAgent: "",
			wantErr:     ErrInvalidTargetAgent,
		},
		{
			name:        "not found",
			taskID:      999,
			targetAgent: "rin",
			statusCode:  http.StatusNotFound,
			wantErr:     ErrNotFound,
		},
		{
			name:        "conflict",
			taskID:      123,
			targetAgent: "rin",
			statusCode:  http.StatusConflict,
			wantErr:     ErrConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					json.NewEncoder(w).Encode(tt.response)
				}
			}))
			defer server.Close()

			client := NewClient(server.URL, WithTimeout(5*time.Second))
			result, err := client.Assign(context.Background(), tt.taskID, tt.targetAgent)

			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("Assign() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("Assign() unexpected error: %v", err)
				return
			}

			if result.IssueNumber != tt.response.IssueNumber {
				t.Errorf("Assign() IssueNumber = %d, want %d", result.IssueNumber, tt.response.IssueNumber)
			}
			if result.AssignedTo != tt.response.AssignedTo {
				t.Errorf("Assign() AssignedTo = %s, want %s", result.AssignedTo, tt.response.AssignedTo)
			}
		})
	}
}

func TestClient_GetHandoverDetail(t *testing.T) {
	tests := []struct {
		name       string
		taskID     int
		response   HandoverDetail
		statusCode int
		wantErr    error
	}{
		{
			name:   "success",
			taskID: 123,
			response: HandoverDetail{
				IssueNumber:  123,
				Title:        "Test Task",
				CurrentState: "pending",
				History: []HandoverEvent{
					{
						Timestamp: time.Now(),
						FromAgent: "yuki",
						Reason:    "timeout",
						Context:   "Working on API",
					},
				},
			},
			statusCode: http.StatusOK,
			wantErr:    nil,
		},
		{
			name:    "invalid task ID",
			taskID:  -1,
			wantErr: ErrInvalidTaskID,
		},
		{
			name:       "not found",
			taskID:     999,
			statusCode: http.StatusNotFound,
			wantErr:    ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					json.NewEncoder(w).Encode(tt.response)
				}
			}))
			defer server.Close()

			client := NewClient(server.URL, WithTimeout(5*time.Second))
			result, err := client.GetHandoverDetail(context.Background(), tt.taskID)

			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("GetHandoverDetail() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("GetHandoverDetail() unexpected error: %v", err)
				return
			}

			if result.IssueNumber != tt.response.IssueNumber {
				t.Errorf("GetHandoverDetail() IssueNumber = %d, want %d", result.IssueNumber, tt.response.IssueNumber)
			}
			if result.Title != tt.response.Title {
				t.Errorf("GetHandoverDetail() Title = %s, want %s", result.Title, tt.response.Title)
			}
		})
	}
}

func TestNewClient_Options(t *testing.T) {
	client := NewClient("http://localhost:8080",
		WithTimeout(10*time.Second),
	)

	if client.baseURL != "http://localhost:8080" {
		t.Errorf("baseURL = %s, want http://localhost:8080", client.baseURL)
	}
	if client.httpClient.Timeout != 10*time.Second {
		t.Errorf("timeout = %v, want 10s", client.httpClient.Timeout)
	}
}
