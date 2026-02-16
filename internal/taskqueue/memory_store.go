package taskqueue

import (
	"errors"
	"sort"
)

var (
	// ErrTaskNotFound is returned when a task is not found.
	ErrTaskNotFound = errors.New("task not found")
	// ErrTaskAlreadyExists is returned when adding a task that already exists.
	ErrTaskAlreadyExists = errors.New("task already exists")
)

// MemoryStore provides an in-memory implementation of TaskStore.
// Phase 1 implementation - tasks are lost on restart.
type MemoryStore struct {
	tasks map[int]PendingTask
}

// NewMemoryStore creates a new in-memory task store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		tasks: make(map[int]PendingTask),
	}
}

// Add adds a task to the store.
func (s *MemoryStore) Add(task PendingTask) error {
	if _, exists := s.tasks[task.IssueNumber]; exists {
		return ErrTaskAlreadyExists
	}
	s.tasks[task.IssueNumber] = task
	return nil
}

// Get retrieves a task by issue number.
func (s *MemoryStore) Get(issueNumber int) (*PendingTask, error) {
	task, exists := s.tasks[issueNumber]
	if !exists {
		return nil, ErrTaskNotFound
	}
	return &task, nil
}

// List returns all tasks sorted by priority (higher first) and creation time.
func (s *MemoryStore) List() ([]PendingTask, error) {
	tasks := make([]PendingTask, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}

	// Sort by priority (descending) then by created_at (ascending)
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Priority != tasks[j].Priority {
			return tasks[i].Priority > tasks[j].Priority
		}
		return tasks[i].CreatedAt.Before(tasks[j].CreatedAt)
	})

	return tasks, nil
}

// Update updates a task in the store.
func (s *MemoryStore) Update(task PendingTask) error {
	if _, exists := s.tasks[task.IssueNumber]; !exists {
		return ErrTaskNotFound
	}
	s.tasks[task.IssueNumber] = task
	return nil
}

// Remove removes a task from the store.
func (s *MemoryStore) Remove(issueNumber int) error {
	if _, exists := s.tasks[issueNumber]; !exists {
		return ErrTaskNotFound
	}
	delete(s.tasks, issueNumber)
	return nil
}

// Clear removes all tasks from the store.
func (s *MemoryStore) Clear() {
	s.tasks = make(map[int]PendingTask)
}

// Count returns the number of tasks in the store.
func (s *MemoryStore) Count() int {
	return len(s.tasks)
}
