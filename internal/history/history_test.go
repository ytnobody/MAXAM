package history

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	// Create temp dir
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_history.json")

	h, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if h.filePath != path {
		t.Errorf("filePath = %s, want %s", h.filePath, path)
	}

	if len(h.Messages) != 0 {
		t.Errorf("Messages length = %d, want 0", len(h.Messages))
	}
}

func TestAddAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_history.json")

	h, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Add messages
	if err := h.Add("user", "Hello"); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	if err := h.Add("mei", "Hi there!"); err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	msgs := h.GetAll()
	if len(msgs) != 2 {
		t.Errorf("GetAll() length = %d, want 2", len(msgs))
	}

	if msgs[0].Role != "user" || msgs[0].Content != "Hello" {
		t.Errorf("msgs[0] = %+v, want role=user content=Hello", msgs[0])
	}
	if msgs[1].Role != "mei" || msgs[1].Content != "Hi there!" {
		t.Errorf("msgs[1] = %+v, want role=mei content=Hi there!", msgs[1])
	}

	// Check file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("history file was not created")
	}
}

func TestPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_history.json")

	// Create and add
	h1, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	h1.Add("user", "First message")
	h1.Add("yuki", "...ok")

	// Reload
	h2, err := New(path)
	if err != nil {
		t.Fatalf("New() reload error: %v", err)
	}

	msgs := h2.GetAll()
	if len(msgs) != 2 {
		t.Errorf("After reload, GetAll() length = %d, want 2", len(msgs))
	}

	if msgs[0].Content != "First message" {
		t.Errorf("msgs[0].Content = %s, want First message", msgs[0].Content)
	}
}

func TestClear(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_history.json")

	h, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	h.Add("user", "Message 1")
	h.Add("user", "Message 2")

	if err := h.Clear(); err != nil {
		t.Fatalf("Clear() error: %v", err)
	}

	msgs := h.GetAll()
	if len(msgs) != 0 {
		t.Errorf("After Clear(), GetAll() length = %d, want 0", len(msgs))
	}

	// Reload and verify still empty
	h2, err := New(path)
	if err != nil {
		t.Fatalf("New() reload error: %v", err)
	}

	msgs2 := h2.GetAll()
	if len(msgs2) != 0 {
		t.Errorf("After reload Clear(), GetAll() length = %d, want 0", len(msgs2))
	}
}

func TestMaxMessages(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_history.json")

	h, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Add more than MaxMessages
	for i := 0; i < MaxMessages+100; i++ {
		h.Add("user", "message")
	}

	msgs := h.GetAll()
	if len(msgs) != MaxMessages {
		t.Errorf("GetAll() length = %d, want %d", len(msgs), MaxMessages)
	}
}

func TestDefaultPath(t *testing.T) {
	// Skip if HOME not set
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("UserHomeDir not available")
	}

	h, err := New("")
	if err != nil {
		t.Fatalf("New() with default path error: %v", err)
	}

	expected := filepath.Join(home, DefaultDir, DefaultFile)
	if h.Path() != expected {
		t.Errorf("Path() = %s, want %s", h.Path(), expected)
	}
}

func TestTrimToSize(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_history.json")

	h, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Add 10 messages
	for i := 0; i < 10; i++ {
		h.Add("user", "message "+string(rune('0'+i)))
	}

	// Trim to 4 messages
	if err := h.TrimToSize(4); err != nil {
		t.Fatalf("TrimToSize() error: %v", err)
	}

	msgs := h.GetAll()
	if len(msgs) != 4 {
		t.Errorf("After TrimToSize(4), len = %d, want 4", len(msgs))
	}

	// Verify persistence
	h2, err := New(path)
	if err != nil {
		t.Fatalf("New() reload error: %v", err)
	}

	msgs2 := h2.GetAll()
	if len(msgs2) != 4 {
		t.Errorf("After reload, len = %d, want 4", len(msgs2))
	}

	// Verify it kept the most recent messages
	if msgs2[3].Content != "message 9" {
		t.Errorf("Last message = %s, want 'message 9'", msgs2[3].Content)
	}
}

func TestTrimToSizeEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_history.json")

	h, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	h.Add("user", "msg1")
	h.Add("user", "msg2")

	// Trim with count=0 (clear all)
	if err := h.TrimToSize(0); err != nil {
		t.Fatalf("TrimToSize(0) error: %v", err)
	}

	msgs := h.GetAll()
	if len(msgs) != 0 {
		t.Errorf("After TrimToSize(0), len = %d, want 0", len(msgs))
	}

	h.Add("user", "msg1")
	h.Add("user", "msg2")

	// Trim with count > current size (no change)
	if err := h.TrimToSize(100); err != nil {
		t.Fatalf("TrimToSize(100) error: %v", err)
	}

	msgs = h.GetAll()
	if len(msgs) != 2 {
		t.Errorf("After TrimToSize(100), len = %d, want 2", len(msgs))
	}
}
