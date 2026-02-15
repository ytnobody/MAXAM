package heartbeat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultWriterConfig(t *testing.T) {
	cfg := DefaultWriterConfig("test-agent")

	if cfg.AgentName != "test-agent" {
		t.Errorf("expected agent name 'test-agent', got %s", cfg.AgentName)
	}
	if cfg.Interval != 30*time.Second {
		t.Errorf("expected interval 30s, got %v", cfg.Interval)
	}
	if cfg.BaseDir == "" {
		t.Error("expected non-empty BaseDir")
	}
}

func TestWriter_FilePath(t *testing.T) {
	cfg := WriterConfig{
		AgentName: "test-agent",
		BaseDir:   "/tmp/test-heartbeat",
	}
	w := NewWriter(cfg)

	expected := "/tmp/test-heartbeat/test-agent.json"
	if w.FilePath() != expected {
		t.Errorf("expected %s, got %s", expected, w.FilePath())
	}
}

func TestWriter_SetGetStatus(t *testing.T) {
	w := NewWriter(WriterConfig{AgentName: "test"})

	if w.GetStatus() != "active" {
		t.Errorf("expected default status 'active', got %s", w.GetStatus())
	}

	w.SetStatus("idle")
	if w.GetStatus() != "idle" {
		t.Errorf("expected status 'idle', got %s", w.GetStatus())
	}
}

func TestWriter_WriteOnce(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := WriterConfig{
		AgentName: "test-agent",
		BaseDir:   tmpDir,
	}
	w := NewWriter(cfg)

	err := w.WriteOnce()
	if err != nil {
		t.Fatalf("WriteOnce failed: %v", err)
	}

	// Verify file exists
	filePath := w.FilePath()
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("heartbeat file not created: %s", filePath)
	}

	// Verify content
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read heartbeat file: %v", err)
	}

	var hb HeartbeatData
	if err := json.Unmarshal(data, &hb); err != nil {
		t.Fatalf("failed to parse heartbeat file: %v", err)
	}

	if hb.AgentName != "test-agent" {
		t.Errorf("expected agent name 'test-agent', got %s", hb.AgentName)
	}
	if hb.Status != "active" {
		t.Errorf("expected status 'active', got %s", hb.Status)
	}
	if hb.PID <= 0 {
		t.Errorf("expected positive PID, got %d", hb.PID)
	}
	if time.Since(hb.Timestamp) > 5*time.Second {
		t.Errorf("timestamp too old: %v", hb.Timestamp)
	}
}

func TestWriter_StartStop(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := WriterConfig{
		AgentName: "test-agent",
		BaseDir:   tmpDir,
		Interval:  50 * time.Millisecond,
	}
	w := NewWriter(cfg)

	err := w.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for a few heartbeats
	time.Sleep(150 * time.Millisecond)

	w.Stop()

	// Verify file exists
	filePath := w.FilePath()
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("heartbeat file not created: %s", filePath)
	}
}

func TestWriter_StartStop_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := WriterConfig{
		AgentName: "test-agent",
		BaseDir:   tmpDir,
	}
	w := NewWriter(cfg)

	// Multiple starts should be safe
	_ = w.Start()
	_ = w.Start()
	_ = w.Start()

	// Multiple stops should be safe
	w.Stop()
	w.Stop()
	w.Stop()
}

func TestWriter_StatusUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := WriterConfig{
		AgentName: "test-agent",
		BaseDir:   tmpDir,
		Interval:  50 * time.Millisecond,
	}
	w := NewWriter(cfg)

	err := w.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Change status
	w.SetStatus("processing")

	// Wait for heartbeat to update
	time.Sleep(100 * time.Millisecond)

	w.Stop()

	// Verify status in file
	data, err := os.ReadFile(w.FilePath())
	if err != nil {
		t.Fatalf("failed to read heartbeat file: %v", err)
	}

	var hb HeartbeatData
	if err := json.Unmarshal(data, &hb); err != nil {
		t.Fatalf("failed to parse heartbeat file: %v", err)
	}

	if hb.Status != "processing" {
		t.Errorf("expected status 'processing', got %s", hb.Status)
	}
}

func TestWriter_DirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "nested", "dirs", "heartbeat")
	cfg := WriterConfig{
		AgentName: "test-agent",
		BaseDir:   nestedDir,
	}
	w := NewWriter(cfg)

	err := w.WriteOnce()
	if err != nil {
		t.Fatalf("WriteOnce failed with nested dir: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(nestedDir); os.IsNotExist(err) {
		t.Fatalf("nested directory not created: %s", nestedDir)
	}
}

func TestWriter_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := WriterConfig{
		AgentName: "test-agent",
		BaseDir:   tmpDir,
	}
	w := NewWriter(cfg)

	err := w.WriteOnce()
	if err != nil {
		t.Fatalf("WriteOnce failed: %v", err)
	}

	info, err := os.Stat(w.FilePath())
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}

	// Check file permissions (should be 0644)
	perm := info.Mode().Perm()
	if perm != 0644 {
		t.Errorf("expected permission 0644, got %04o", perm)
	}
}
