// Package logger handles structured logging for agents
package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Entry represents a log entry
type Entry struct {
	Timestamp time.Time
	Agent     string
	Input     string
	Thinking  string
	Output    string
	Duration  time.Duration
}

// Format returns the entry in markdown format
func (e *Entry) Format() string {
	ts := e.Timestamp.Format("2006-01-02 15:04:05")

	return fmt.Sprintf(`## [%s] %s

### Input
%s

### Thinking
%s

### Output
%s

### Duration
%s

---
`, ts, e.Agent, e.Input, e.Thinking, e.Output, e.Duration)
}

// Logger handles logging for a specific agent
type Logger struct {
	dir   string
	agent string
	file  *os.File
}

// New creates a new logger for an agent
func New(baseDir, agent string) (*Logger, error) {
	dir := filepath.Join(baseDir, agent)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}

	// Create daily log file
	filename := time.Now().Format("2006-01-02") + ".md"
	path := filepath.Join(dir, filename)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	return &Logger{
		dir:   dir,
		agent: agent,
		file:  f,
	}, nil
}

// Log writes an entry to the log
func (l *Logger) Log(entry *Entry) error {
	entry.Agent = l.agent
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	_, err := l.file.WriteString(entry.Format())
	return err
}

// LogSimple writes a simple log entry
func (l *Logger) LogSimple(input, output string, duration time.Duration) error {
	return l.Log(&Entry{
		Input:    input,
		Output:   output,
		Duration: duration,
	})
}

// Close closes the log file
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Manager manages loggers for all agents
type Manager struct {
	baseDir string
	loggers map[string]*Logger
}

// NewManager creates a new logger manager
func NewManager(baseDir string) *Manager {
	// Ensure base directory exists
	os.MkdirAll(baseDir, 0755)

	return &Manager{
		baseDir: baseDir,
		loggers: make(map[string]*Logger),
	}
}

// Get returns a logger for an agent, creating if needed
func (m *Manager) Get(agent string) (*Logger, error) {
	if l, ok := m.loggers[agent]; ok {
		return l, nil
	}

	l, err := New(m.baseDir, agent)
	if err != nil {
		return nil, err
	}

	m.loggers[agent] = l
	return l, nil
}

// Close closes all loggers
func (m *Manager) Close() {
	for _, l := range m.loggers {
		l.Close()
	}
}
