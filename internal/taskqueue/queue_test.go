package taskqueue

import (
	"testing"
	"time"
)

func TestQueue_Add(t *testing.T) {
	store := NewMemoryStore()
	queue := NewQueue(store)

	task := PendingTask{
		IssueNumber:   123,
		OriginalAgent: "yuki-1",
		Context:       "PR review in progress",
		Priority:      1,
	}

	err := queue.Add(task)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	// Verify task was added
	got, err := queue.Get(123)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got.IssueNumber != 123 {
		t.Errorf("IssueNumber = %d, want 123", got.IssueNumber)
	}
	if got.OriginalAgent != "yuki-1" {
		t.Errorf("OriginalAgent = %s, want yuki-1", got.OriginalAgent)
	}
	if got.Status != HandoverStatusPending {
		t.Errorf("Status = %s, want pending", got.Status)
	}
	if len(got.History) != 1 {
		t.Errorf("History length = %d, want 1", len(got.History))
	}
	if got.History[0].Event != "queued" {
		t.Errorf("History[0].Event = %s, want queued", got.History[0].Event)
	}
}

func TestQueue_AddDuplicate(t *testing.T) {
	store := NewMemoryStore()
	queue := NewQueue(store)

	task := PendingTask{
		IssueNumber:   123,
		OriginalAgent: "yuki-1",
	}

	_ = queue.Add(task)
	err := queue.Add(task)
	if err != ErrTaskAlreadyExists {
		t.Errorf("Add() duplicate error = %v, want ErrTaskAlreadyExists", err)
	}
}

func TestQueue_Assign(t *testing.T) {
	store := NewMemoryStore()
	queue := NewQueue(store)

	task := PendingTask{
		IssueNumber:   123,
		OriginalAgent: "yuki-1",
	}
	_ = queue.Add(task)

	err := queue.Assign(123, "yuki-2")
	if err != nil {
		t.Fatalf("Assign() error = %v", err)
	}

	got, _ := queue.Get(123)
	if got.AssignedAgent != "yuki-2" {
		t.Errorf("AssignedAgent = %s, want yuki-2", got.AssignedAgent)
	}
	if got.Status != HandoverStatusAssigned {
		t.Errorf("Status = %s, want assigned", got.Status)
	}
	if len(got.History) != 2 {
		t.Errorf("History length = %d, want 2", len(got.History))
	}
}

func TestQueue_Complete(t *testing.T) {
	store := NewMemoryStore()
	queue := NewQueue(store)

	task := PendingTask{
		IssueNumber:   123,
		OriginalAgent: "yuki-1",
	}
	_ = queue.Add(task)
	_ = queue.Assign(123, "yuki-2")

	err := queue.Complete(123)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	got, _ := queue.Get(123)
	if got.Status != HandoverStatusCompleted {
		t.Errorf("Status = %s, want completed", got.Status)
	}
}

func TestQueue_Fail(t *testing.T) {
	store := NewMemoryStore()
	queue := NewQueue(store)

	task := PendingTask{
		IssueNumber:   123,
		OriginalAgent: "yuki-1",
	}
	_ = queue.Add(task)

	err := queue.Fail(123, "no available agent")
	if err != nil {
		t.Fatalf("Fail() error = %v", err)
	}

	got, _ := queue.Get(123)
	if got.Status != HandoverStatusFailed {
		t.Errorf("Status = %s, want failed", got.Status)
	}
}

func TestQueue_ListPending(t *testing.T) {
	store := NewMemoryStore()
	queue := NewQueue(store)

	// Add multiple tasks
	_ = queue.Add(PendingTask{IssueNumber: 1, OriginalAgent: "yuki-1"})
	_ = queue.Add(PendingTask{IssueNumber: 2, OriginalAgent: "rin-1"})
	_ = queue.Add(PendingTask{IssueNumber: 3, OriginalAgent: "shiori-1"})

	// Assign one task
	_ = queue.Assign(2, "yuki-2")

	pending, err := queue.ListPending()
	if err != nil {
		t.Fatalf("ListPending() error = %v", err)
	}

	if len(pending) != 2 {
		t.Errorf("ListPending() count = %d, want 2", len(pending))
	}
}

func TestMemoryStore_PrioritySort(t *testing.T) {
	store := NewMemoryStore()

	now := time.Now()
	_ = store.Add(PendingTask{IssueNumber: 1, Priority: 1, CreatedAt: now})
	_ = store.Add(PendingTask{IssueNumber: 2, Priority: 3, CreatedAt: now.Add(time.Second)})
	_ = store.Add(PendingTask{IssueNumber: 3, Priority: 2, CreatedAt: now.Add(2 * time.Second)})

	tasks, _ := store.List()

	// Should be sorted by priority descending
	if tasks[0].IssueNumber != 2 {
		t.Errorf("First task should be issue 2 (priority 3), got %d", tasks[0].IssueNumber)
	}
	if tasks[1].IssueNumber != 3 {
		t.Errorf("Second task should be issue 3 (priority 2), got %d", tasks[1].IssueNumber)
	}
	if tasks[2].IssueNumber != 1 {
		t.Errorf("Third task should be issue 1 (priority 1), got %d", tasks[2].IssueNumber)
	}
}

func TestQueue_Remove(t *testing.T) {
	store := NewMemoryStore()
	queue := NewQueue(store)

	task := PendingTask{
		IssueNumber:   123,
		OriginalAgent: "yuki-1",
	}
	_ = queue.Add(task)

	err := queue.Remove(123)
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	_, err = queue.Get(123)
	if err != ErrTaskNotFound {
		t.Errorf("Get() after Remove should return ErrTaskNotFound, got %v", err)
	}
}

func TestQueue_GetNotFound(t *testing.T) {
	store := NewMemoryStore()
	queue := NewQueue(store)

	_, err := queue.Get(999)
	if err != ErrTaskNotFound {
		t.Errorf("Get() nonexistent = %v, want ErrTaskNotFound", err)
	}
}
