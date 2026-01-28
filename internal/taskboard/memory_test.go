package taskboard

import (
	"context"
	"testing"
)

func TestMemoryStore_Create(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	task, err := store.Create(ctx, &Task{
		Title:    "Test task",
		Assignee: "yuki",
	})

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if task.ID != 1 {
		t.Errorf("ID = %d, want 1", task.ID)
	}

	if task.Title != "Test task" {
		t.Errorf("Title = %s, want 'Test task'", task.Title)
	}

	if task.Status != StatusPending {
		t.Errorf("Status = %s, want pending", task.Status)
	}

	if task.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
}

func TestMemoryStore_Get(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	created, _ := store.Create(ctx, &Task{Title: "Test"})

	got, err := store.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got.Title != "Test" {
		t.Errorf("Title = %s, want 'Test'", got.Title)
	}

	// Not found
	_, err = store.Get(ctx, 999)
	if err != ErrNotFound {
		t.Errorf("Get(999) error = %v, want ErrNotFound", err)
	}
}

func TestMemoryStore_List(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	store.Create(ctx, &Task{Title: "Task 1"})
	store.Create(ctx, &Task{Title: "Task 2"})
	store.Create(ctx, &Task{Title: "Task 3"})

	tasks, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(tasks) != 3 {
		t.Fatalf("len(tasks) = %d, want 3", len(tasks))
	}

	// Check sorted by ID
	for i := 0; i < len(tasks)-1; i++ {
		if tasks[i].ID >= tasks[i+1].ID {
			t.Errorf("tasks not sorted: %d >= %d", tasks[i].ID, tasks[i+1].ID)
		}
	}
}

func TestMemoryStore_Update(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	created, _ := store.Create(ctx, &Task{
		Title:    "Original",
		Status:   StatusPending,
		Assignee: "yuki",
	})

	err := store.Update(ctx, &Task{
		ID:       created.ID,
		Title:    "Updated",
		Status:   StatusInProgress,
		Assignee: "rin",
	})

	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, _ := store.Get(ctx, created.ID)
	if got.Title != "Updated" {
		t.Errorf("Title = %s, want 'Updated'", got.Title)
	}
	if got.Status != StatusInProgress {
		t.Errorf("Status = %s, want in_progress", got.Status)
	}
	if got.Assignee != "rin" {
		t.Errorf("Assignee = %s, want 'rin'", got.Assignee)
	}

	// Not found
	err = store.Update(ctx, &Task{ID: 999})
	if err != ErrNotFound {
		t.Errorf("Update(999) error = %v, want ErrNotFound", err)
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	created, _ := store.Create(ctx, &Task{Title: "To delete"})

	err := store.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err = store.Get(ctx, created.ID)
	if err != ErrNotFound {
		t.Errorf("Get after delete: error = %v, want ErrNotFound", err)
	}

	// Not found
	err = store.Delete(ctx, 999)
	if err != ErrNotFound {
		t.Errorf("Delete(999) error = %v, want ErrNotFound", err)
	}
}

func TestMemoryStore_Concurrency(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	done := make(chan bool)

	// Concurrent creates
	for i := 0; i < 10; i++ {
		go func(n int) {
			store.Create(ctx, &Task{Title: "Task"})
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	tasks, _ := store.List(ctx)
	if len(tasks) != 10 {
		t.Errorf("len(tasks) = %d, want 10", len(tasks))
	}
}
