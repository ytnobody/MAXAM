package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ytnobody/MAXAM/internal/config"
)

// Runner executes Claude Code with agent persona
type Runner struct {
	Name        string
	WorkDir     string
	ClaudeMDDir string
	Timeout     time.Duration
}

// NewRunner creates a new agent runner
func NewRunner(name, workDir, claudeMDDir string) *Runner {
	return &Runner{
		Name:        name,
		WorkDir:     workDir,
		ClaudeMDDir: claudeMDDir,
		Timeout:     10 * time.Minute,
	}
}

// buildSystemPrompt reads the agent's CLAUDE.md and builds system prompt
func (r *Runner) buildSystemPrompt() (string, error) {
	var parts []string

	// 1. Embedded common rules (always present)
	if EmbeddedRules != "" {
		parts = append(parts, EmbeddedRules)
	}

	// 2. Project-specific CLAUDE.md (optional)
	sharedPath := filepath.Join(r.WorkDir, "CLAUDE.md")
	if data, err := os.ReadFile(sharedPath); err == nil {
		parts = append(parts, string(data))
	}

	// 3. Agent-specific CLAUDE.md (optional)
	if r.ClaudeMDDir != "" {
		agentPath := filepath.Join(r.ClaudeMDDir, "CLAUDE.md")
		if data, err := os.ReadFile(agentPath); err == nil {
			parts = append(parts, string(data))
		}
	}

	if len(parts) == 0 {
		return "", fmt.Errorf("no CLAUDE.md found")
	}

	return strings.Join(parts, "\n\n---\n\n"), nil
}

// Run executes a task with the agent
func (r *Runner) Run(ctx context.Context, prompt string) (string, error) {
	systemPrompt, err := r.buildSystemPrompt()
	if err != nil {
		return "", fmt.Errorf("build system prompt: %w", err)
	}

	// Build command
	args := []string{
		"--print",
		"--system-prompt", systemPrompt,
		"--permission-mode", "bypassPermissions",
		prompt,
	}

	ctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = r.WorkDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("timeout after %v", r.Timeout)
		}
		return "", fmt.Errorf("run claude: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// RunWithAllowedTools executes with specific tools enabled
func (r *Runner) RunWithAllowedTools(ctx context.Context, prompt string, tools []string) (string, error) {
	systemPrompt, err := r.buildSystemPrompt()
	if err != nil {
		return "", fmt.Errorf("build system prompt: %w", err)
	}

	args := []string{
		"--print",
		"--system-prompt", systemPrompt,
		"--permission-mode", "bypassPermissions",
	}

	if len(tools) > 0 {
		args = append(args, "--allowedTools")
		args = append(args, tools...)
	}

	args = append(args, prompt)

	ctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = r.WorkDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("timeout after %v", r.Timeout)
		}
		return "", fmt.Errorf("run claude: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// Agents provides pre-configured runners for each team member
type Agents struct {
	workDir string
	runners map[string]*Runner
}

// NewAgents creates the agent team from ~/.maxam/agents/
func NewAgents(workDir string) *Agents {
	// Initialize ~/.maxam/ if needed
	embeddedAgents, _ := config.GetEmbeddedAgents()
	config.EnsureInitialized(embeddedAgents)

	// Get agents directory (~/.maxam/agents/)
	agentsDir, err := config.AgentsDir()
	if err != nil {
		// Fallback to local agents dir
		agentsDir = filepath.Join(workDir, "agents")
	}

	// Load agent config
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	runners := make(map[string]*Runner)
	agentNames := map[string]string{
		"mei":    "Mei Chen",
		"yuki":   "Yuki Tanaka",
		"rin":    "Rin Sato",
		"shiori": "Shiori Tanaka",
		"priya":  "Priya Sharma",
		"amara":  "Amara Okonkwo",
	}

	// Create runners for configured agents
	for _, agentCfg := range cfg.Agents {
		name := agentCfg.Name
		fullName := agentNames[name]
		if fullName == "" {
			fullName = name // Use agent name if not in map
		}

		agentDir := filepath.Join(agentsDir, name)
		// Only add if CLAUDE.md exists
		if _, err := os.Stat(filepath.Join(agentDir, "CLAUDE.md")); err == nil {
			runners[name] = NewRunner(fullName, workDir, agentDir)
		}
	}

	return &Agents{
		workDir: workDir,
		runners: runners,
	}
}

// Get returns a runner by name
func (a *Agents) Get(name string) (*Runner, bool) {
	r, ok := a.runners[name]
	return r, ok
}

// Yuki returns the implementation agent
func (a *Agents) Yuki() *Runner {
	return a.runners["yuki"]
}

// Priya returns the review agent
func (a *Agents) Priya() *Runner {
	return a.runners["priya"]
}

// Mei returns the PM agent
func (a *Agents) Mei() *Runner {
	return a.runners["mei"]
}

// Amara returns the analysis agent
func (a *Agents) Amara() *Runner {
	return a.runners["amara"]
}

// Rin returns the frontend agent
func (a *Agents) Rin() *Runner {
	return a.runners["rin"]
}

// Shiori returns the test/docs agent
func (a *Agents) Shiori() *Runner {
	return a.runners["shiori"]
}
