package taskboard

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileStore_CreateAndList(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "tasks.json")

	store, err := NewFileStore(filePath)
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}

	ctx := context.Background()

	// Create tasks
	task1, err := store.Create(ctx, &Task{Title: "Task 1", Assignee: "yuki"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if task1.ID != 1 {
		t.Errorf("Expected ID 1, got %d", task1.ID)
	}
	if task1.Status != StatusPending {
		t.Errorf("Expected status pending, got %s", task1.Status)
	}

	task2, err := store.Create(ctx, &Task{Title: "Task 2", Status: StatusInProgress})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if task2.ID != 2 {
		t.Errorf("Expected ID 2, got %d", task2.ID)
	}

	// List tasks
	tasks, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(tasks))
	}

	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Task file was not created")
	}
}

func TestFileStore_Get(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "tasks.json")

	store, err := NewFileStore(filePath)
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}

	ctx := context.Background()

	// Create a task
	created, _ := store.Create(ctx, &Task{Title: "Test Task"})

	// Get existing task
	task, err := store.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if task.Title != "Test Task" {
		t.Errorf("Expected 'Test Task', got %s", task.Title)
	}

	// Get non-existing task
	_, err = store.Get(ctx, 999)
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestFileStore_Update(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "tasks.json")

	store, err := NewFileStore(filePath)
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}

	ctx := context.Background()

	// Create a task
	created, _ := store.Create(ctx, &Task{Title: "Original"})

	// Update task
	created.Title = "Updated"
	created.Status = StatusCompleted
	err = store.Update(ctx, created)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify update
	task, _ := store.Get(ctx, created.ID)
	if task.Title != "Updated" {
		t.Errorf("Expected 'Updated', got %s", task.Title)
	}
	if task.Status != StatusCompleted {
		t.Errorf("Expected completed, got %s", task.Status)
	}
}

func TestFileStore_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "tasks.json")

	store, err := NewFileStore(filePath)
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}

	ctx := context.Background()

	// Create tasks
	task1, _ := store.Create(ctx, &Task{Title: "Task 1"})
	store.Create(ctx, &Task{Title: "Task 2"})

	// Delete first task
	err = store.Delete(ctx, task1.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deletion
	tasks, _ := store.List(ctx)
	if len(tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(tasks))
	}

	// Delete non-existing task
	err = store.Delete(ctx, 999)
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestFileStore_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "tasks.json")

	ctx := context.Background()

	// Create store and add tasks
	store1, _ := NewFileStore(filePath)
	store1.Create(ctx, &Task{Title: "Persistent Task", Assignee: "mei"})

	// Create new store instance (simulating restart)
	store2, err := NewFileStore(filePath)
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}

	// Verify tasks are loaded
	tasks, _ := store2.List(ctx)
	if len(tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Title != "Persistent Task" {
		t.Errorf("Expected 'Persistent Task', got %s", tasks[0].Title)
	}
	if tasks[0].Assignee != "mei" {
		t.Errorf("Expected 'mei', got %s", tasks[0].Assignee)
	}
}

func TestFileStore_Reload(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "tasks.json")

	ctx := context.Background()

	// Create two store instances
	store1, _ := NewFileStore(filePath)
	store2, _ := NewFileStore(filePath)

	// Add task via store1
	store1.Create(ctx, &Task{Title: "New Task"})

	// store2 doesn't see it yet
	tasks, _ := store2.List(ctx)
	if len(tasks) != 0 {
		t.Errorf("Expected 0 tasks before reload, got %d", len(tasks))
	}

	// Reload store2
	store2.Reload()

	// Now store2 should see the task
	tasks, _ = store2.List(ctx)
	if len(tasks) != 1 {
		t.Errorf("Expected 1 task after reload, got %d", len(tasks))
	}
}

func TestFileStore_DefaultPath(t *testing.T) {
	path := DefaultTaskFilePath()
	if path == "" {
		t.Error("DefaultTaskFilePath returned empty string")
	}
	if !filepath.IsAbs(path) {
		t.Errorf("Expected absolute path, got %s", path)
	}
}
