// Package heartbeat provides agent health monitoring.
package heartbeat

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// HeartbeatData represents the heartbeat file content.
type HeartbeatData struct {
	AgentName string    `json:"agent_name"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"` // "active", "idle", etc.
	PID       int       `json:"pid"`
}

// WriterConfig holds heartbeat writer configuration.
type WriterConfig struct {
	AgentName string
	Interval  time.Duration // Update interval (default: 30s)
	BaseDir   string        // Base directory for heartbeat files (default: ~/.maxam/heartbeat)
}

// DefaultWriterConfig returns the default writer configuration.
func DefaultWriterConfig(agentName string) WriterConfig {
	homeDir, _ := os.UserHomeDir()
	return WriterConfig{
		AgentName: agentName,
		Interval:  30 * time.Second,
		BaseDir:   filepath.Join(homeDir, ".maxam", "heartbeat"),
	}
}

// Writer writes heartbeat files periodically.
type Writer struct {
	cfg       WriterConfig
	status    string
	statusMu  sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	running   bool
	runningMu sync.Mutex
}

// NewWriter creates a new heartbeat writer.
func NewWriter(cfg WriterConfig) *Writer {
	if cfg.Interval <= 0 {
		cfg.Interval = DefaultWriterConfig("").Interval
	}
	if cfg.BaseDir == "" {
		cfg.BaseDir = DefaultWriterConfig("").BaseDir
	}

	return &Writer{
		cfg:    cfg,
		status: "active",
	}
}

// SetStatus updates the current status.
func (w *Writer) SetStatus(status string) {
	w.statusMu.Lock()
	defer w.statusMu.Unlock()
	w.status = status
}

// GetStatus returns the current status.
func (w *Writer) GetStatus() string {
	w.statusMu.RLock()
	defer w.statusMu.RUnlock()
	return w.status
}

// FilePath returns the path to the heartbeat file.
func (w *Writer) FilePath() string {
	return filepath.Join(w.cfg.BaseDir, w.cfg.AgentName+".json")
}

// Start begins the heartbeat writing loop.
func (w *Writer) Start() error {
	w.runningMu.Lock()
	if w.running {
		w.runningMu.Unlock()
		return nil
	}
	w.running = true
	w.ctx, w.cancel = context.WithCancel(context.Background())
	w.runningMu.Unlock()

	// Ensure directory exists
	if err := os.MkdirAll(w.cfg.BaseDir, 0755); err != nil {
		return err
	}

	// Write initial heartbeat
	if err := w.write(); err != nil {
		return err
	}

	w.wg.Add(1)
	go w.writeLoop()

	return nil
}

// Stop stops the heartbeat writing loop.
func (w *Writer) Stop() {
	w.runningMu.Lock()
	if !w.running {
		w.runningMu.Unlock()
		return
	}
	w.running = false
	w.cancel()
	w.runningMu.Unlock()
	w.wg.Wait()
}

// writeLoop runs periodic heartbeat writes.
func (w *Writer) writeLoop() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			_ = w.write() // Best effort
		}
	}
}

// write writes the heartbeat file.
func (w *Writer) write() error {
	data := HeartbeatData{
		AgentName: w.cfg.AgentName,
		Timestamp: time.Now(),
		Status:    w.GetStatus(),
		PID:       os.Getpid(),
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	filePath := w.FilePath()

	// Write to temp file first, then rename (atomic)
	tmpFile := filePath + ".tmp"
	if err := os.WriteFile(tmpFile, jsonData, 0644); err != nil {
		return err
	}

	return os.Rename(tmpFile, filePath)
}

// WriteOnce writes a single heartbeat (for external use).
func (w *Writer) WriteOnce() error {
	// Ensure directory exists
	if err := os.MkdirAll(w.cfg.BaseDir, 0755); err != nil {
		return err
	}
	return w.write()
}
