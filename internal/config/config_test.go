package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Version != "1" {
		t.Errorf("Version = %q, want %q", cfg.Version, "1")
	}
	if len(cfg.Agents) != 6 {
		t.Errorf("Agents count = %d, want 6", len(cfg.Agents))
	}
}

func TestConfigDir(t *testing.T) {
	dir, err := ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir() error: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".maxam")
	if dir != expected {
		t.Errorf("ConfigDir() = %q, want %q", dir, expected)
	}
}

func TestAgentsDir(t *testing.T) {
	dir, err := AgentsDir()
	if err != nil {
		t.Fatalf("AgentsDir() error: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".maxam", "agents")
	if dir != expected {
		t.Errorf("AgentsDir() = %q, want %q", dir, expected)
	}
}

func TestLoadNonExistent(t *testing.T) {
	// When config doesn't exist, should return default
	// Note: This test assumes ~/.maxam/config.yaml might or might not exist
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg == nil {
		t.Error("Load() returned nil config")
	}
}

func TestGetEmbeddedAgents(t *testing.T) {
	agents, err := GetEmbeddedAgents()
	if err != nil {
		t.Fatalf("GetEmbeddedAgents() error: %v", err)
	}

	// Check all default agents are embedded
	for _, name := range DefaultAgents {
		if _, ok := agents[name]; !ok {
			t.Errorf("Agent %q not found in embedded agents", name)
		}
	}
}

func TestEnsureInitialized(t *testing.T) {
	// Create a temp dir to simulate home
	tmpHome := t.TempDir()

	// Override HOME for this test
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", originalHome)

	// Get embedded agents
	agents, err := GetEmbeddedAgents()
	if err != nil {
		t.Fatalf("GetEmbeddedAgents() error: %v", err)
	}

	// Initialize
	if err := EnsureInitialized(agents); err != nil {
		t.Fatalf("EnsureInitialized() error: %v", err)
	}

	// Check config.yaml was created
	configPath := filepath.Join(tmpHome, ".maxam", "config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("config.yaml not created: %v", err)
	}

	// Check agents were installed
	for name := range agents {
		claudeMD := filepath.Join(tmpHome, ".maxam", "agents", name, "CLAUDE.md")
		if _, err := os.Stat(claudeMD); err != nil {
			t.Errorf("Agent %s CLAUDE.md not installed: %v", name, err)
		}
	}
}

func TestListAgents(t *testing.T) {
	// Create a temp dir
	tmpHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", originalHome)

	// Initialize with embedded agents
	agents, _ := GetEmbeddedAgents()
	EnsureInitialized(agents)

	// List agents
	list, err := ListAgents()
	if err != nil {
		t.Fatalf("ListAgents() error: %v", err)
	}

	if len(list) != len(DefaultAgents) {
		t.Errorf("ListAgents() returned %d agents, want %d", len(list), len(DefaultAgents))
	}
}
