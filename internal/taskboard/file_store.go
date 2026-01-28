package taskboard

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// FileStore is a file-based implementation of Service
type FileStore struct {
	mu       sync.RWMutex
	filePath string
	tasks    map[int]*Task
	nextID   int
}

// fileData represents the JSON structure for persistence
type fileData struct {
	NextID int     `json:"next_id"`
	Tasks  []*Task `json:"tasks"`
}

// DefaultTaskFilePath returns the default path for task storage
func DefaultTaskFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	return filepath.Join(home, ".maxam", "tasks.json")
}

// NewFileStore creates a new file-based task store
func NewFileStore(filePath string) (*FileStore, error) {
	if filePath == "" {
		filePath = DefaultTaskFilePath()
	}

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	fs := &FileStore{
		filePath: filePath,
		tasks:    make(map[int]*Task),
		nextID:   1,
	}

	// Load existing data if file exists
	if err := fs.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return fs, nil
}

// FilePath returns the path to the task file
func (f *FileStore) FilePath() string {
	return f.filePath
}

// load reads tasks from the file
func (f *FileStore) load() error {
	data, err := os.ReadFile(f.filePath)
	if err != nil {
		return err
	}

	var fd fileData
	if err := json.Unmarshal(data, &fd); err != nil {
		return err
	}

	f.tasks = make(map[int]*Task)
	for _, t := range fd.Tasks {
		f.tasks[t.ID] = t
	}
	f.nextID = fd.NextID

	return nil
}

// save writes tasks to the file
func (f *FileStore) save() error {
	tasks := make([]*Task, 0, len(f.tasks))
	for _, t := range f.tasks {
		tasks = append(tasks, t)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].ID < tasks[j].ID
	})

	fd := fileData{
		NextID: f.nextID,
		Tasks:  tasks,
	}

	data, err := json.MarshalIndent(fd, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(f.filePath, data, 0644)
}

// Reload reloads tasks from the file (for external changes)
func (f *FileStore) Reload() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.load()
}

// List returns all tasks sorted by ID (ascending)
func (f *FileStore) List(ctx context.Context) ([]*Task, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	tasks := make([]*Task, 0, len(f.tasks))
	for _, t := range f.tasks {
		tasks = append(tasks, copyTask(t))
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].ID < tasks[j].ID
	})

	return tasks, nil
}

// Get returns a task by ID
func (f *FileStore) Get(ctx context.Context, id int) (*Task, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	t, ok := f.tasks[id]
	if !ok {
		return nil, ErrNotFound
	}

	return copyTask(t), nil
}

// Create creates a new task
func (f *FileStore) Create(ctx context.Context, task *Task) (*Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	now := time.Now()
	newTask := &Task{
		ID:          f.nextID,
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

	f.tasks[f.nextID] = newTask
	f.nextID++

	if err := f.save(); err != nil {
		return nil, err
	}

	return copyTask(newTask), nil
}

// Update updates an existing task
func (f *FileStore) Update(ctx context.Context, task *Task) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	existing, ok := f.tasks[task.ID]
	if !ok {
		return ErrNotFound
	}

	existing.Title = task.Title
	existing.Assignee = task.Assignee
	existing.Status = task.Status
	existing.Description = task.Description
	existing.UpdatedAt = time.Now()

	return f.save()
}

// Delete removes a task by ID
func (f *FileStore) Delete(ctx context.Context, id int) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.tasks[id]; !ok {
		return ErrNotFound
	}

	delete(f.tasks, id)
	return f.save()
}
