package heartbeat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultAlerterConfig(t *testing.T) {
	cfg := DefaultAlerterConfig()

	if cfg.LogFileName != "heartbeat.log" {
		t.Errorf("expected LogFileName 'heartbeat.log', got %s", cfg.LogFileName)
	}
	if !cfg.EnableStdout {
		t.Error("expected EnableStdout true")
	}
	if !cfg.EnableLogFile {
		t.Error("expected EnableLogFile true")
	}
	if cfg.EnableGitHub {
		t.Error("expected EnableGitHub false")
	}
}

func TestAlertLevel_String(t *testing.T) {
	tests := []struct {
		level AlertLevel
		want  string
	}{
		{AlertLevelInfo, "INFO"},
		{AlertLevelWarn, "WARN"},
		{AlertLevelCritical, "CRITICAL"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.level.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAlerter_LogFileOnly(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := AlerterConfig{
		LogDir:        tmpDir,
		LogFileName:   "test.log",
		EnableStdout:  false,
		EnableLogFile: true,
	}

	a, err := NewAlerter(cfg)
	if err != nil {
		t.Fatalf("NewAlerter failed: %v", err)
	}
	defer a.Close()

	a.Alert(AlertLevelWarn, "test-agent", "test message")

	// Read log file
	logPath := filepath.Join(tmpDir, "test.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "[WARN]") {
		t.Errorf("log should contain [WARN]: %s", content)
	}
	if !strings.Contains(content, "test-agent") {
		t.Errorf("log should contain agent name: %s", content)
	}
	if !strings.Contains(content, "test message") {
		t.Errorf("log should contain message: %s", content)
	}
}

func TestAlerter_AlertHealth_Degraded(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := AlerterConfig{
		LogDir:        tmpDir,
		LogFileName:   "test.log",
		EnableStdout:  false,
		EnableLogFile: true,
	}

	a, err := NewAlerter(cfg)
	if err != nil {
		t.Fatalf("NewAlerter failed: %v", err)
	}
	defer a.Close()

	health := AgentHealth{
		AgentName:     "degraded-agent",
		State:         StateDegraded,
		SinceLastBeat: 90 * time.Second,
	}
	a.AlertHealth(health, StateHealthy)

	// Read log file
	logPath := filepath.Join(tmpDir, "test.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "[WARN]") {
		t.Errorf("log should contain [WARN]: %s", content)
	}
	if !strings.Contains(content, "degraded") {
		t.Errorf("log should contain 'degraded': %s", content)
	}
}

func TestAlerter_AlertHealth_Unresponsive(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := AlerterConfig{
		LogDir:        tmpDir,
		LogFileName:   "test.log",
		EnableStdout:  false,
		EnableLogFile: true,
	}

	a, err := NewAlerter(cfg)
	if err != nil {
		t.Fatalf("NewAlerter failed: %v", err)
	}
	defer a.Close()

	health := AgentHealth{
		AgentName:     "unresponsive-agent",
		State:         StateUnresponsive,
		SinceLastBeat: 150 * time.Second,
	}
	a.AlertHealth(health, StateDegraded)

	// Read log file
	logPath := filepath.Join(tmpDir, "test.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "[CRITICAL]") {
		t.Errorf("log should contain [CRITICAL]: %s", content)
	}
	if !strings.Contains(content, "unresponsive") {
		t.Errorf("log should contain 'unresponsive': %s", content)
	}
}

func TestAlerter_AlertHealth_Healthy_NoAlert(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := AlerterConfig{
		LogDir:        tmpDir,
		LogFileName:   "test.log",
		EnableStdout:  false,
		EnableLogFile: true,
	}

	a, err := NewAlerter(cfg)
	if err != nil {
		t.Fatalf("NewAlerter failed: %v", err)
	}
	defer a.Close()

	health := AgentHealth{
		AgentName:     "healthy-agent",
		State:         StateHealthy,
		SinceLastBeat: 10 * time.Second,
	}
	a.AlertHealth(health, StateUnknown)

	// Log file should not exist or be empty
	logPath := filepath.Join(tmpDir, "test.log")
	data, _ := os.ReadFile(logPath)
	if len(data) > 0 {
		t.Errorf("log should be empty for healthy state: %s", string(data))
	}
}

func TestAlerter_CreateAlertCallback(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := AlerterConfig{
		LogDir:        tmpDir,
		LogFileName:   "test.log",
		EnableStdout:  false,
		EnableLogFile: true,
	}

	a, err := NewAlerter(cfg)
	if err != nil {
		t.Fatalf("NewAlerter failed: %v", err)
	}
	defer a.Close()

	callback := a.CreateAlertCallback()

	health := AgentHealth{
		AgentName:     "callback-agent",
		State:         StateDegraded,
		SinceLastBeat: 90 * time.Second,
	}
	callback(health, StateHealthy)

	// Read log file
	logPath := filepath.Join(tmpDir, "test.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "callback-agent") {
		t.Errorf("log should contain agent name: %s", content)
	}
}

func TestAlerter_Close_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := AlerterConfig{
		LogDir:        tmpDir,
		LogFileName:   "test.log",
		EnableLogFile: true,
	}

	a, err := NewAlerter(cfg)
	if err != nil {
		t.Fatalf("NewAlerter failed: %v", err)
	}

	// Multiple closes should be safe
	a.Close()
	a.Close()
	a.Close()
}

func TestAlerter_DirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "nested", "dirs", "logs")
	cfg := AlerterConfig{
		LogDir:        nestedDir,
		LogFileName:   "test.log",
		EnableLogFile: true,
	}

	a, err := NewAlerter(cfg)
	if err != nil {
		t.Fatalf("NewAlerter failed with nested dir: %v", err)
	}
	defer a.Close()

	// Verify directory was created
	if _, err := os.Stat(nestedDir); os.IsNotExist(err) {
		t.Fatalf("nested directory not created: %s", nestedDir)
	}
}

func TestAlerter_DisabledLogFile(t *testing.T) {
	cfg := AlerterConfig{
		EnableStdout:  false,
		EnableLogFile: false,
	}

	a, err := NewAlerter(cfg)
	if err != nil {
		t.Fatalf("NewAlerter failed: %v", err)
	}
	defer a.Close()

	// Should not panic when alerting with no output
	a.Alert(AlertLevelWarn, "test", "test")
}
