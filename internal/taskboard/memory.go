package taskboard

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

var (
	ErrNotFound = errors.New("task not found")
)

// MemoryStore is an in-memory implementation of Service
type MemoryStore struct {
	mu     sync.RWMutex
	tasks  map[int]*Task
	nextID int
}

// NewMemoryStore creates a new in-memory task store
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		tasks:  make(map[int]*Task),
		nextID: 1,
	}
}

// List returns all tasks sorted by ID (ascending)
func (m *MemoryStore) List(ctx context.Context) ([]*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*Task, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, copyTask(t))
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].ID < tasks[j].ID
	})

	return tasks, nil
}

// Get returns a task by ID
func (m *MemoryStore) Get(ctx context.Context, id int) (*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.tasks[id]
	if !ok {
		return nil, ErrNotFound
	}

	return copyTask(t), nil
}

// Create creates a new task
func (m *MemoryStore) Create(ctx context.Context, task *Task) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	newTask := &Task{
		ID:          m.nextID,
		Title:       task.Title,
		Assignee:    task.Assignee,
		Status:      task.Status,
		Description: task.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if newTask.Status == "" {
		newTask.Status = StatusPending
	}

	m.tasks[m.nextID] = newTask
	m.nextID++

	return copyTask(newTask), nil
}

// Update updates an existing task
func (m *MemoryStore) Update(ctx context.Context, task *Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.tasks[task.ID]
	if !ok {
		return ErrNotFound
	}

	existing.Title = task.Title
	existing.Assignee = task.Assignee
	existing.Status = task.Status
	existing.Description = task.Description
	existing.UpdatedAt = time.Now()

	return nil
}

// Delete removes a task by ID
func (m *MemoryStore) Delete(ctx context.Context, id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tasks[id]; !ok {
		return ErrNotFound
	}

	delete(m.tasks, id)
	return nil
}

func copyTask(t *Task) *Task {
	return &Task{
		ID:          t.ID,
		Title:       t.Title,
		Assignee:    t.Assignee,
		Status:      t.Status,
		Description: t.Description,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}
