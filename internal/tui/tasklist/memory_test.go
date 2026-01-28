package tasklist

import (
	"testing"
)

func TestMemoryService_Add(t *testing.T) {
	s := NewMemoryService()

	task := s.Add("Test Task", "Description", "Rin")

	if task.ID != 1 {
		t.Errorf("expected ID 1, got %d", task.ID)
	}
	if task.Title != "Test Task" {
		t.Errorf("expected title 'Test Task', got '%s'", task.Title)
	}
	if task.Description != "Description" {
		t.Errorf("expected description 'Description', got '%s'", task.Description)
	}
	if task.Assignee != "Rin" {
		t.Errorf("expected assignee 'Rin', got '%s'", task.Assignee)
	}
	if task.Status != StatusPending {
		t.Errorf("expected status 'pending', got '%s'", task.Status)
	}
}

func TestMemoryService_List(t *testing.T) {
	s := NewMemoryService()

	// Empty list
	tasks := s.List()
	if len(tasks) != 0 {
		t.Errorf("expected empty list, got %d tasks", len(tasks))
	}

	// Add tasks
	s.Add("Task 1", "", "Yuki")
	s.Add("Task 2", "", "Rin")
	s.Add("Task 3", "", "Mei")

	tasks = s.List()
	if len(tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(tasks))
	}

	// Should be sorted by ID
	for i := 0; i < len(tasks)-1; i++ {
		if tasks[i].ID >= tasks[i+1].ID {
			t.Errorf("tasks not sorted by ID: %d >= %d", tasks[i].ID, tasks[i+1].ID)
		}
	}
}

func TestMemoryService_UpdateStatus(t *testing.T) {
	s := NewMemoryService()

	task := s.Add("Task", "", "Yuki")

	// Update to in_progress
	err := s.UpdateStatus(task.ID, StatusInProgress)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	updated, _ := s.Get(task.ID)
	if updated.Status != StatusInProgress {
		t.Errorf("expected status 'in_progress', got '%s'", updated.Status)
	}

	// Update to done
	err = s.UpdateStatus(task.ID, StatusDone)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	updated, _ = s.Get(task.ID)
	if updated.Status != StatusDone {
		t.Errorf("expected status 'done', got '%s'", updated.Status)
	}

	// Update non-existent task
	err = s.UpdateStatus(999, StatusPending)
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestMemoryService_Delete(t *testing.T) {
	s := NewMemoryService()

	task := s.Add("Task", "", "Rin")

	// Delete existing task
	err := s.Delete(task.ID)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	tasks := s.List()
	if len(tasks) != 0 {
		t.Errorf("expected empty list after delete, got %d tasks", len(tasks))
	}

	// Delete non-existent task
	err = s.Delete(999)
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestMemoryService_Get(t *testing.T) {
	s := NewMemoryService()

	task := s.Add("Task", "Desc", "Priya")

	// Get existing task
	got, err := s.Get(task.ID)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if got.Title != "Task" {
		t.Errorf("expected title 'Task', got '%s'", got.Title)
	}

	// Get non-existent task
	_, err = s.Get(999)
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestStatus_String(t *testing.T) {
	tests := []struct {
		status   Status
		expected string
	}{
		{StatusPending, "Pending"},
		{StatusInProgress, "In Progress"},
		{StatusDone, "Done"},
		{Status("unknown"), "unknown"},
	}

	for _, tt := range tests {
		got := tt.status.String()
		if got != tt.expected {
			t.Errorf("Status(%s).String() = %s, want %s", tt.status, got, tt.expected)
		}
	}
}
