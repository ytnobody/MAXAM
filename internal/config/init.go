package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultAgents is the list of default agents to install
var DefaultAgents = []string{"mei", "yuki", "rin", "shiori", "priya", "amara"}

// EnsureInitialized sets up ~/.maxam/ if not present
// and copies default agents from the embedded data
func EnsureInitialized(embeddedAgents map[string]string) error {
	agentsDir, err := AgentsDir()
	if err != nil {
		return err
	}

	// Create agents directory if not exists
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return fmt.Errorf("create agents dir: %w", err)
	}

	// Check if config.yaml exists
	configDir, err := ConfigDir()
	if err != nil {
		return err
	}

	configPath := filepath.Join(configDir, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// First time setup - create config and copy agents
		cfg := DefaultConfig()
		if err := Save(cfg); err != nil {
			return fmt.Errorf("save default config: %w", err)
		}

		// Copy embedded agents
		for name, content := range embeddedAgents {
			if err := installAgent(agentsDir, name, content); err != nil {
				return fmt.Errorf("install agent %s: %w", name, err)
			}
		}
	}

	return nil
}

// installAgent writes an agent's CLAUDE.md to ~/.maxam/agents/{name}/
func installAgent(agentsDir, name, content string) error {
	agentDir := filepath.Join(agentsDir, name)
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return err
	}

	claudeMDPath := filepath.Join(agentDir, "CLAUDE.md")

	// Don't overwrite if already exists
	if _, err := os.Stat(claudeMDPath); err == nil {
		return nil
	}

	return os.WriteFile(claudeMDPath, []byte(content), 0644)
}

// ListAgents returns agent names from ~/.maxam/agents/
func ListAgents() ([]string, error) {
	agentsDir, err := AgentsDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read agents dir: %w", err)
	}

	var agents []string
	for _, entry := range entries {
		if entry.IsDir() {
			// Check if CLAUDE.md exists
			claudeMD := filepath.Join(agentsDir, entry.Name(), "CLAUDE.md")
			if _, err := os.Stat(claudeMD); err == nil {
				agents = append(agents, entry.Name())
			}
		}
	}

	return agents, nil
}

// GetAgentClaudeMD returns the path to an agent's CLAUDE.md
func GetAgentClaudeMD(name string) (string, error) {
	agentsDir, err := AgentsDir()
	if err != nil {
		return "", err
	}

	claudeMD := filepath.Join(agentsDir, name, "CLAUDE.md")
	if _, err := os.Stat(claudeMD); err != nil {
		return "", fmt.Errorf("agent %s not found: %w", name, err)
	}

	return claudeMD, nil
}

// GetAgentDir returns the directory for an agent
func GetAgentDir(name string) (string, error) {
	agentsDir, err := AgentsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(agentsDir, name), nil
}

// ListAgentsWithProject returns agent names with project-local priority
// Priority: project/.maxam/agents/ > ~/.maxam/agents/
func ListAgentsWithProject(projectDir string) ([]string, error) {
	agentSet := make(map[string]bool)

	// 1. Get global agents first
	globalAgents, err := ListAgents()
	if err != nil {
		return nil, err
	}
	for _, name := range globalAgents {
		agentSet[name] = true
	}

	// 2. Get project-local agents (these take priority)
	projectAgentsDir := ProjectAgentsDir(projectDir)
	entries, err := os.ReadDir(projectAgentsDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				claudeMD := filepath.Join(projectAgentsDir, entry.Name(), "CLAUDE.md")
				if _, err := os.Stat(claudeMD); err == nil {
					agentSet[entry.Name()] = true
				}
			}
		}
	}

	// Convert to slice
	agents := make([]string, 0, len(agentSet))
	for name := range agentSet {
		agents = append(agents, name)
	}
	return agents, nil
}

// GetAgentClaudeMDWithProject returns agent's CLAUDE.md path with project priority
// Priority: project/.maxam/agents/{name}/ > ~/.maxam/agents/{name}/
func GetAgentClaudeMDWithProject(projectDir, name string) (string, error) {
	// 1. Try project-local first
	projectClaudeMD := filepath.Join(ProjectAgentsDir(projectDir), name, "CLAUDE.md")
	if _, err := os.Stat(projectClaudeMD); err == nil {
		return projectClaudeMD, nil
	}

	// 2. Fall back to global
	return GetAgentClaudeMD(name)
}

// GetAgentDirWithProject returns agent directory with project priority
// Priority: project/.maxam/agents/{name}/ > ~/.maxam/agents/{name}/
func GetAgentDirWithProject(projectDir, name string) (string, error) {
	// 1. Try project-local first
	projectAgentDir := filepath.Join(ProjectAgentsDir(projectDir), name)
	if _, err := os.Stat(filepath.Join(projectAgentDir, "CLAUDE.md")); err == nil {
		return projectAgentDir, nil
	}

	// 2. Fall back to global
	return GetAgentDir(name)
}

// DefaultProjectConfigTemplate is the default config.yaml template for projects
const DefaultProjectConfigTemplate = `# MAXAM Project Configuration
# See: https://github.com/ytnobody/MAXAM

version: "1"

# Team members (run 'maxam team init' to set up)
# agents:
#   - name: mei
#     full_name: Mei Chen
#     role: PM + Architect
`

// DefaultProjectClaudeMDTemplate is the default CLAUDE.md template for projects
const DefaultProjectClaudeMDTemplate = `# Project Settings

> This file manages project-specific settings and learning. Tracked by git.

## Team Members

| Name | Role |
|------|------|
| (Run 'maxam team init' to set up) | |

## Learning

(Team learnings will be recorded here)

---

*This file is continuously updated through team learning*
`

// EnsureProjectInitialized sets up project/.maxam/ if not present
// Creates default config.yaml and CLAUDE.md templates
// Does not overwrite existing files
func EnsureProjectInitialized(projectDir string) error {
	configDir := ProjectConfigDir(projectDir)

	// Create .maxam directory if not exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create project config dir: %w", err)
	}

	// Create config.yaml if not exists
	configPath := filepath.Join(configDir, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := os.WriteFile(configPath, []byte(DefaultProjectConfigTemplate), 0644); err != nil {
			return fmt.Errorf("write project config: %w", err)
		}
	}

	// Create CLAUDE.md if not exists
	claudeMDPath := filepath.Join(configDir, "CLAUDE.md")
	if _, err := os.Stat(claudeMDPath); os.IsNotExist(err) {
		if err := os.WriteFile(claudeMDPath, []byte(DefaultProjectClaudeMDTemplate), 0644); err != nil {
			return fmt.Errorf("write project CLAUDE.md: %w", err)
		}
	}

	return nil
}

// IsProjectInitialized returns true if project/.maxam/ exists with config files
func IsProjectInitialized(projectDir string) bool {
	configDir := ProjectConfigDir(projectDir)
	configPath := filepath.Join(configDir, "config.yaml")
	_, err := os.Stat(configPath)
	return err == nil
}
