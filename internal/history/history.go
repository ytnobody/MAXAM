package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	MaxMessages   = 5000
	DefaultDir    = ".maxam"
	DefaultFile   = "history.json"
)

// Message represents a single chat message
type Message struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// History manages chat history persistence
type History struct {
	Messages  []Message `json:"messages"`
	UpdatedAt time.Time `json:"updated_at"`
	filePath  string
	mu        sync.Mutex
}

// New creates a new History instance
func New(customPath string) (*History, error) {
	path := customPath
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		dir := filepath.Join(home, DefaultDir)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
		path = filepath.Join(dir, DefaultFile)
	}

	h := &History{
		Messages: make([]Message, 0),
		filePath: path,
	}

	// Load existing history if exists
	if err := h.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return h, nil
}

// Add adds a message and persists to disk
func (h *History) Add(role, content string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	msg := Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}

	h.Messages = append(h.Messages, msg)

	// Trim if over limit
	if len(h.Messages) > MaxMessages {
		h.Messages = h.Messages[len(h.Messages)-MaxMessages:]
	}

	h.UpdatedAt = time.Now()

	return h.save()
}

// GetAll returns all messages
func (h *History) GetAll() []Message {
	h.mu.Lock()
	defer h.mu.Unlock()

	result := make([]Message, len(h.Messages))
	copy(result, h.Messages)
	return result
}

// Clear clears all history
func (h *History) Clear() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.Messages = make([]Message, 0)
	h.UpdatedAt = time.Now()

	return h.save()
}

// load reads history from disk
func (h *History) load() error {
	data, err := os.ReadFile(h.filePath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, h)
}

// save writes history to disk atomically
func (h *History) save() error {
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}

	// Write to temp file first, then rename for atomic operation
	tmpPath := h.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, h.filePath)
}

// Path returns the history file path
func (h *History) Path() string {
	return h.filePath
}
