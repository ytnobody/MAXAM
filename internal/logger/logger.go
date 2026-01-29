// Package logger handles structured logging for agents
package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// GetDefaultLogDir returns the default log directory (~/.maxam/logs)
func GetDefaultLogDir() string {
	var home string
	if runtime.GOOS == "windows" {
		home = os.Getenv("USERPROFILE")
	} else {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".maxam", "logs")
}

// ProjectNameFromPath converts a directory path to a project name.
// Example: /home/user/my-project -> home_user_my-project
func ProjectNameFromPath(path string) string {
	// Clean and get absolute path
	cleaned := filepath.Clean(path)
	// Remove leading slash and convert to underscore-separated
	trimmed := strings.TrimPrefix(cleaned, string(filepath.Separator))
	return strings.ReplaceAll(trimmed, string(filepath.Separator), "_")
}

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

// NewManager creates a new logger manager with project-specific subdirectory.
// baseDir: logs directory (e.g., ~/.maxam/logs)
// projectDir: working directory for the project (used to generate project name)
func NewManager(baseDir, projectDir string) *Manager {
	projectName := ProjectNameFromPath(projectDir)
	fullDir := filepath.Join(baseDir, projectName)

	// Migrate from project-local logs/ to ~/.maxam/logs/
	migrateFromProjectLocal(projectDir, fullDir)

	// Migrate existing logs if needed (old style in baseDir)
	migrateExistingLogs(baseDir, projectName)

	// Ensure directory exists
	os.MkdirAll(fullDir, 0755)

	return &Manager{
		baseDir: fullDir,
		loggers: make(map[string]*Logger),
	}
}

// migrateFromProjectLocal moves logs from project/logs/ to ~/.maxam/logs/{project}/
func migrateFromProjectLocal(projectDir, targetDir string) {
	localLogsDir := filepath.Join(projectDir, "logs")

	// Check if local logs directory exists
	info, err := os.Stat(localLogsDir)
	if err != nil || !info.IsDir() {
		return
	}

	// Ensure target directory exists
	os.MkdirAll(targetDir, 0755)

	agents := []string{"mei", "yuki", "priya", "amara"}
	projectName := ProjectNameFromPath(projectDir)

	for _, agent := range agents {
		// Check both direct agent dir and project-named subdir
		paths := []string{
			filepath.Join(localLogsDir, agent),
			filepath.Join(localLogsDir, projectName, agent),
		}

		for _, oldPath := range paths {
			info, err := os.Stat(oldPath)
			if err != nil || !info.IsDir() {
				continue
			}

			newPath := filepath.Join(targetDir, agent)
			if _, err := os.Stat(newPath); os.IsNotExist(err) {
				// Move directory
				if err := os.Rename(oldPath, newPath); err != nil {
					// If rename fails, try copying files
					copyDir(oldPath, newPath)
				}
			} else {
				// Merge: copy files that don't exist in target
				mergeDir(oldPath, newPath)
			}
		}
	}
}

// copyDir copies all files from src to dst
func copyDir(src, dst string) {
	os.MkdirAll(dst, 0755)
	entries, err := os.ReadDir(src)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if data, err := os.ReadFile(srcPath); err == nil {
			os.WriteFile(dstPath, data, 0644)
		}
	}
}

// mergeDir copies files from src to dst that don't exist in dst
func mergeDir(src, dst string) {
	entries, err := os.ReadDir(src)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if _, err := os.Stat(dstPath); os.IsNotExist(err) {
			if data, err := os.ReadFile(srcPath); err == nil {
				os.WriteFile(dstPath, data, 0644)
			}
		}
	}
}

// migrateExistingLogs moves old agent directories to project-specific directory
func migrateExistingLogs(baseDir, projectName string) {
	agents := []string{"mei", "yuki", "priya", "amara"}
	targetDir := filepath.Join(baseDir, projectName)

	for _, agent := range agents {
		oldPath := filepath.Join(baseDir, agent)
		newPath := filepath.Join(targetDir, agent)

		// Check if old-style directory exists and new-style doesn't
		if info, err := os.Stat(oldPath); err == nil && info.IsDir() {
			if _, err := os.Stat(newPath); os.IsNotExist(err) {
				// Create target directory and move
				os.MkdirAll(targetDir, 0755)
				if err := os.Rename(oldPath, newPath); err != nil {
					// If rename fails (cross-device), skip
					continue
				}
			}
		}
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
