package tasklist

import (
	"errors"
	"sort"
	"sync"
)

// ErrTaskNotFound is returned when a task with the given ID is not found.
var ErrTaskNotFound = errors.New("task not found")

// MemoryService is an in-memory implementation of TaskService.
// This is useful for testing and development.
type MemoryService struct {
	mu     sync.RWMutex
	tasks  map[int]Task
	nextID int
}

// NewMemoryService creates a new in-memory task service.
func NewMemoryService() *MemoryService {
	return &MemoryService{
		tasks:  make(map[int]Task),
		nextID: 1,
	}
}

// List returns all tasks sorted by ID (ascending).
func (s *MemoryService) List() []Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].ID < tasks[j].ID
	})

	return tasks
}

// Add creates a new task and returns it with assigned ID.
func (s *MemoryService) Add(title, description, assignee string) Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	task := Task{
		ID:          s.nextID,
		Title:       title,
		Description: description,
		Assignee:    assignee,
		Status:      StatusPending,
	}

	s.tasks[task.ID] = task
	s.nextID++

	return task
}

// UpdateStatus changes the status of a task.
func (s *MemoryService) UpdateStatus(id int, status Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[id]
	if !ok {
		return ErrTaskNotFound
	}

	task.Status = status
	s.tasks[id] = task

	return nil
}

// Delete removes a task by ID.
func (s *MemoryService) Delete(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tasks[id]; !ok {
		return ErrTaskNotFound
	}

	delete(s.tasks, id)
	return nil
}

// Get returns a task by ID. Returns ErrTaskNotFound if not found.
func (s *MemoryService) Get(id int) (Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.tasks[id]
	if !ok {
		return Task{}, ErrTaskNotFound
	}

	return task, nil
}
