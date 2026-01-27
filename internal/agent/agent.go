// Package agent manages AI agent processes
package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ytnobody/MAXAM/internal/mcp"
)

// Agent represents an AI agent with a specific persona
type Agent struct {
	Name        string
	Role        string
	WorkDir     string
	ClaudeMDDir string

	cmd    *exec.Cmd
	client *mcp.Client
}

// Config holds agent configuration
type Config struct {
	Name        string
	Role        string
	WorkDir     string
	ClaudeMDDir string
}

// New creates a new agent
func New(cfg Config) *Agent {
	return &Agent{
		Name:        cfg.Name,
		Role:        cfg.Role,
		WorkDir:     cfg.WorkDir,
		ClaudeMDDir: cfg.ClaudeMDDir,
	}
}

// Start launches the Claude Code process
func (a *Agent) Start() error {
	// Find claude command
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude not found: %w", err)
	}

	// Prepare environment with agent-specific CLAUDE.md
	env := os.Environ()
	if a.ClaudeMDDir != "" {
		claudeMDPath := filepath.Join(a.ClaudeMDDir, "CLAUDE.md")
		if _, err := os.Stat(claudeMDPath); err == nil {
			// Agent has its own CLAUDE.md
			env = append(env, fmt.Sprintf("CLAUDE_MD=%s", claudeMDPath))
		}
	}

	// Start claude in MCP mode
	a.cmd = exec.Command(claudePath, "mcp", "serve")
	a.cmd.Dir = a.WorkDir
	a.cmd.Env = env

	stdin, err := a.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := a.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	// Stderr goes to os.Stderr for debugging
	a.cmd.Stderr = os.Stderr

	if err := a.cmd.Start(); err != nil {
		return fmt.Errorf("start claude: %w", err)
	}

	// Create MCP client
	a.client = mcp.NewClient(stdin, stdout)
	a.client.Start()

	// Initialize MCP connection
	result, err := a.client.Initialize()
	if err != nil {
		a.Stop()
		return fmt.Errorf("initialize mcp: %w", err)
	}

	fmt.Printf("[%s] Connected to %s %s\n", a.Name, result.ServerInfo.Name, result.ServerInfo.Version)
	return nil
}

// Stop terminates the agent process
func (a *Agent) Stop() error {
	if a.client != nil {
		a.client.Close()
	}
	if a.cmd != nil && a.cmd.Process != nil {
		return a.cmd.Process.Kill()
	}
	return nil
}

// SendTask sends a task to the agent
func (a *Agent) SendTask(prompt string) (string, error) {
	if a.client == nil {
		return "", fmt.Errorf("agent not started")
	}

	// Use the prompt tool to send task
	result, err := a.client.GetPrompt("task", map[string]string{
		"instruction": prompt,
	})
	if err != nil {
		return "", err
	}

	if len(result.Messages) > 0 {
		return result.Messages[0].Content.Text, nil
	}

	return "", nil
}

// CallTool calls a specific tool on the agent
func (a *Agent) CallTool(name string, args map[string]interface{}) (string, error) {
	if a.client == nil {
		return "", fmt.Errorf("agent not started")
	}

	result, err := a.client.CallTool(name, args)
	if err != nil {
		return "", err
	}

	if result.IsError {
		return "", fmt.Errorf("tool error")
	}

	var text string
	for _, block := range result.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}

	return text, nil
}
