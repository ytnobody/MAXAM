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
