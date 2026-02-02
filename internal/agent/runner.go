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
	"github.com/ytnobody/MAXAM/internal/retry"
)

// Runner executes Claude Code with agent persona
type Runner struct {
	Name        string
	WorkDir     string
	ClaudeMDDir string
	Timeout     time.Duration
	ContextMode config.ContextMode
	Model       Model // Model to use for this agent
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

// getClaudeMDFilename returns the appropriate CLAUDE.md filename based on context mode
func (r *Runner) getClaudeMDFilename() string {
	if r.ContextMode == config.ContextModeSummary {
		return "CLAUDE.summary.md"
	}
	return "CLAUDE.md"
}

// buildSystemPrompt reads the agent's CLAUDE.md and builds system prompt
func (r *Runner) buildSystemPrompt() (string, error) {
	var parts []string

	// 1. Embedded common rules (always present)
	if EmbeddedRules != "" {
		parts = append(parts, EmbeddedRules)
	}

	// 2. Project-specific CLAUDE.md (optional)
	// In summary mode, try CLAUDE.summary.md first, fallback to CLAUDE.md
	filename := r.getClaudeMDFilename()
	sharedPath := filepath.Join(r.WorkDir, filename)
	if data, err := os.ReadFile(sharedPath); err == nil {
		parts = append(parts, string(data))
	} else if r.ContextMode == config.ContextModeSummary {
		// Fallback to full version if summary not found
		sharedPath = filepath.Join(r.WorkDir, "CLAUDE.md")
		if data, err := os.ReadFile(sharedPath); err == nil {
			parts = append(parts, string(data))
		}
	}

	// 3. Project-local CLAUDE.md from .maxam/ directory (optional)
	// Priority: .maxam/CLAUDE.md > CLAUDE.local.md (deprecated)
	projectClaudeMD := config.ProjectClaudeMD(r.WorkDir)
	if data, err := os.ReadFile(projectClaudeMD); err == nil {
		parts = append(parts, string(data))
	} else {
		// Fallback to deprecated CLAUDE.local.md for backward compatibility
		localPath := filepath.Join(r.WorkDir, "CLAUDE.local.md")
		if data, err := os.ReadFile(localPath); err == nil {
			parts = append(parts, string(data))
		}
	}

	// 4. Agent-specific CLAUDE.md (optional)
	if r.ClaudeMDDir != "" {
		agentPath := filepath.Join(r.ClaudeMDDir, filename)
		if data, err := os.ReadFile(agentPath); err == nil {
			parts = append(parts, string(data))
		} else if r.ContextMode == config.ContextModeSummary {
			// Fallback to full version if summary not found
			agentPath = filepath.Join(r.ClaudeMDDir, "CLAUDE.md")
			if data, err := os.ReadFile(agentPath); err == nil {
				parts = append(parts, string(data))
			}
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
	}

	// Use agent's configured model if set
	if r.Model != ModelDefault {
		args = append(args, "--model", string(r.Model))
	}

	args = append(args, prompt)

	cfg := retry.DefaultConfig()

	return retry.DoWithResult(ctx, cfg, func() (string, error) {
		runCtx, cancel := context.WithTimeout(ctx, r.Timeout)
		defer cancel()

		cmd := exec.CommandContext(runCtx, "claude", args...)
		cmd.Dir = r.WorkDir

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			if runCtx.Err() == context.DeadlineExceeded {
				return "", fmt.Errorf("timeout after %v", r.Timeout)
			}
			return "", fmt.Errorf("run claude: %w\nstderr: %s", err, stderr.String())
		}

		return stdout.String(), nil
	})
}

// Model represents the Claude model to use
type Model string

const (
	ModelDefault Model = ""       // Use claude's default
	ModelHaiku   Model = "haiku"  // Fast, lightweight
	ModelSonnet  Model = "sonnet" // Balanced
	ModelOpus    Model = "opus"   // Most capable
)

// RunWithModel executes a task with a specific model
func (r *Runner) RunWithModel(ctx context.Context, prompt string, model Model) (string, error) {
	systemPrompt, err := r.buildSystemPrompt()
	if err != nil {
		return "", fmt.Errorf("build system prompt: %w", err)
	}

	args := []string{
		"--print",
		"--system-prompt", systemPrompt,
		"--permission-mode", "bypassPermissions",
	}

	if model != ModelDefault {
		args = append(args, "--model", string(model))
	}

	args = append(args, prompt)

	cfg := retry.DefaultConfig()

	return retry.DoWithResult(ctx, cfg, func() (string, error) {
		runCtx, cancel := context.WithTimeout(ctx, r.Timeout)
		defer cancel()

		cmd := exec.CommandContext(runCtx, "claude", args...)
		cmd.Dir = r.WorkDir

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			if runCtx.Err() == context.DeadlineExceeded {
				return "", fmt.Errorf("timeout after %v", r.Timeout)
			}
			return "", fmt.Errorf("run claude: %w\nstderr: %s", err, stderr.String())
		}

		return stdout.String(), nil
	})
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

	cfg := retry.DefaultConfig()

	return retry.DoWithResult(ctx, cfg, func() (string, error) {
		runCtx, cancel := context.WithTimeout(ctx, r.Timeout)
		defer cancel()

		cmd := exec.CommandContext(runCtx, "claude", args...)
		cmd.Dir = r.WorkDir

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			if runCtx.Err() == context.DeadlineExceeded {
				return "", fmt.Errorf("timeout after %v", r.Timeout)
			}
			return "", fmt.Errorf("run claude: %w\nstderr: %s", err, stderr.String())
		}

		return stdout.String(), nil
	})
}

// Agents provides pre-configured runners for each team member
type Agents struct {
	workDir string
	runners map[string]*Runner
}

// NewAgents creates the agent team with project-local priority
// Priority: project/.maxam/agents/ > ~/.maxam/agents/
func NewAgents(workDir string) *Agents {
	// Initialize ~/.maxam/ if needed
	embeddedAgents, _ := config.GetEmbeddedAgents()
	config.EnsureInitialized(embeddedAgents)

	// Load config with project overrides
	cfg, err := config.LoadWithProject(workDir)
	if err != nil {
		cfg = config.DefaultConfig()
	}

	runners := make(map[string]*Runner)

	// Get context mode
	contextMode := cfg.GetContextMode()

	// Create runners for configured agents
	for _, agentCfg := range cfg.Agents {
		name := agentCfg.Name
		fullName := agentCfg.FullName
		if fullName == "" {
			fullName = name // Use agent name if FullName not set
		}

		// Get agent directory with project priority
		agentDir, err := config.GetAgentDirWithProject(workDir, name)
		if err != nil {
			continue // Skip if agent not found
		}

		// Only add if CLAUDE.md exists
		if _, err := os.Stat(filepath.Join(agentDir, "CLAUDE.md")); err == nil {
			runner := NewRunner(fullName, workDir, agentDir)
			runner.ContextMode = contextMode
			// Set model from config if specified
			if agentCfg.Model != "" {
				runner.Model = Model(agentCfg.Model)
			}
			runners[name] = runner
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
