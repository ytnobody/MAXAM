// Package heartbeat provides agent health monitoring.
package heartbeat

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AlertLevel represents the severity of an alert.
type AlertLevel int

const (
	AlertLevelInfo AlertLevel = iota
	AlertLevelWarn
	AlertLevelCritical
)

func (l AlertLevel) String() string {
	switch l {
	case AlertLevelInfo:
		return "INFO"
	case AlertLevelWarn:
		return "WARN"
	case AlertLevelCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// AlerterConfig holds alerter configuration.
type AlerterConfig struct {
	LogDir        string // Directory for log files (default: ~/.maxam/logs)
	LogFileName   string // Log file name (default: heartbeat.log)
	EnableStdout  bool   // Enable stdout alerts (default: true)
	EnableLogFile bool   // Enable log file alerts (default: true)
	EnableGitHub  bool   // Enable GitHub Issues (default: false, not implemented yet)
}

// DefaultAlerterConfig returns the default alerter configuration.
func DefaultAlerterConfig() AlerterConfig {
	homeDir, _ := os.UserHomeDir()
	return AlerterConfig{
		LogDir:        filepath.Join(homeDir, ".maxam", "logs"),
		LogFileName:   "heartbeat.log",
		EnableStdout:  true,
		EnableLogFile: true,
		EnableGitHub:  false,
	}
}

// Alerter handles alert notifications.
type Alerter struct {
	cfg     AlerterConfig
	logFile *os.File
	logger  *log.Logger
	mu      sync.Mutex
	closed  bool
}

// NewAlerter creates a new alerter.
func NewAlerter(cfg AlerterConfig) (*Alerter, error) {
	a := &Alerter{
		cfg: cfg,
	}

	if cfg.EnableLogFile {
		if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
			return nil, err
		}

		logPath := filepath.Join(cfg.LogDir, cfg.LogFileName)
		f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
		a.logFile = f

		// Create multi-writer for both stdout and file if both enabled
		var writers []io.Writer
		if cfg.EnableStdout {
			writers = append(writers, os.Stdout)
		}
		writers = append(writers, f)
		a.logger = log.New(io.MultiWriter(writers...), "", 0)
	} else if cfg.EnableStdout {
		a.logger = log.New(os.Stdout, "", 0)
	}

	return a, nil
}

// Alert sends an alert.
func (a *Alerter) Alert(level AlertLevel, agentName string, message string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("[%s] [%s] Agent '%s': %s", timestamp, level.String(), agentName, message)

	if a.logger != nil {
		a.logger.Println(line)
	}
}

// AlertHealth sends an alert based on agent health.
func (a *Alerter) AlertHealth(health AgentHealth, prevState AgentState) {
	var level AlertLevel
	var message string

	switch health.State {
	case StateDegraded:
		level = AlertLevelWarn
		message = fmt.Sprintf("is degraded (last heartbeat: %s ago)", formatDuration(health.SinceLastBeat))
	case StateUnresponsive:
		level = AlertLevelCritical
		message = fmt.Sprintf("is unresponsive (last heartbeat: %s ago). Consider restarting the agent or checking manually.", formatDuration(health.SinceLastBeat))
	default:
		return // No alert for healthy state
	}

	a.Alert(level, health.AgentName, message)
}

// Close closes the alerter and releases resources.
func (a *Alerter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return nil
	}
	a.closed = true

	if a.logFile != nil {
		return a.logFile.Close()
	}
	return nil
}

// CreateAlertCallback creates an AlertCallback for use with FileMonitor.
func (a *Alerter) CreateAlertCallback() AlertCallback {
	return func(health AgentHealth, prevState AgentState) {
		a.AlertHealth(health, prevState)
	}
}
