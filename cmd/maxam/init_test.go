package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ytnobody/MAXAM/internal/config"
	"gopkg.in/yaml.v3"
)

func TestLoadDefaultTeam(t *testing.T) {
	// Create temp home directory
	tempHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", originalHome)

	// Create ~/.maxam directory
	maxamDir := filepath.Join(tempHome, ".maxam")
	if err := os.MkdirAll(maxamDir, 0755); err != nil {
		t.Fatalf("failed to create .maxam dir: %v", err)
	}

	t.Run("no default-team.yaml", func(t *testing.T) {
		cfg, err := loadDefaultTeam()
		if err == nil {
			t.Error("expected error when no default-team.yaml exists")
		}
		if cfg != nil {
			t.Error("expected nil config when file doesn't exist")
		}
	})

	t.Run("with default-team.yaml", func(t *testing.T) {
		// Create default-team.yaml
		defaultTeam := &config.Config{
			Version:  "1",
			TeamName: "Test Team",
			Agents: []config.AgentConfig{
				{Name: "alice", FullName: "Alice Smith", Role: "Developer"},
				{Name: "bob", FullName: "Bob Jones", Role: "Reviewer"},
			},
		}

		data, err := yaml.Marshal(defaultTeam)
		if err != nil {
			t.Fatalf("failed to marshal config: %v", err)
		}

		if err := os.WriteFile(filepath.Join(maxamDir, "default-team.yaml"), data, 0644); err != nil {
			t.Fatalf("failed to write default-team.yaml: %v", err)
		}

		cfg, err := loadDefaultTeam()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.TeamName != "Test Team" {
			t.Errorf("expected team name 'Test Team', got '%s'", cfg.TeamName)
		}

		if len(cfg.Agents) != 2 {
			t.Errorf("expected 2 agents, got %d", len(cfg.Agents))
		}

		if cfg.Agents[0].Name != "alice" {
			t.Errorf("expected first agent 'alice', got '%s'", cfg.Agents[0].Name)
		}
	})
}

func TestInitProjectCreation(t *testing.T) {
	// Create temp directories
	tempHome := t.TempDir()
	tempProject := t.TempDir()

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", originalHome)

	// Create ~/.maxam directory
	maxamDir := filepath.Join(tempHome, ".maxam")
	if err := os.MkdirAll(maxamDir, 0755); err != nil {
		t.Fatalf("failed to create .maxam dir: %v", err)
	}

	t.Run("init creates .maxam directory", func(t *testing.T) {
		// Ensure project is not initialized
		if config.IsProjectInitialized(tempProject) {
			t.Error("project should not be initialized")
		}

		// Initialize
		if err := config.EnsureProjectInitialized(tempProject); err != nil {
			t.Fatalf("failed to initialize project: %v", err)
		}

		// Check files created
		configPath := filepath.Join(tempProject, ".maxam", "config.yaml")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Error("config.yaml not created")
		}

		claudeMDPath := filepath.Join(tempProject, ".maxam", "CLAUDE.md")
		if _, err := os.Stat(claudeMDPath); os.IsNotExist(err) {
			t.Error("CLAUDE.md not created")
		}

		// Check IsProjectInitialized returns true
		if !config.IsProjectInitialized(tempProject) {
			t.Error("project should be initialized")
		}
	})

	t.Run("init with default team loads agents", func(t *testing.T) {
		tempProject2 := t.TempDir()

		// Create default-team.yaml
		defaultTeam := &config.Config{
			Version:  "1",
			TeamName: "Default Team",
			Agents: []config.AgentConfig{
				{Name: "mei", FullName: "Mei Chen", Role: "PM"},
				{Name: "yuki", FullName: "Yuki Tanaka", Role: "Backend"},
			},
		}

		data, err := yaml.Marshal(defaultTeam)
		if err != nil {
			t.Fatalf("failed to marshal config: %v", err)
		}

		if err := os.WriteFile(filepath.Join(maxamDir, "default-team.yaml"), data, 0644); err != nil {
			t.Fatalf("failed to write default-team.yaml: %v", err)
		}

		// Initialize project
		if err := config.EnsureProjectInitialized(tempProject2); err != nil {
			t.Fatalf("failed to initialize project: %v", err)
		}

		// Load default team and apply
		team, err := loadDefaultTeam()
		if err != nil {
			t.Fatalf("failed to load default team: %v", err)
		}

		projectCfg, err := config.LoadProjectConfig(tempProject2)
		if err != nil {
			t.Fatalf("failed to load project config: %v", err)
		}

		projectCfg.Agents = team.Agents
		projectCfg.TeamName = team.TeamName

		if err := config.SaveToProject(tempProject2, projectCfg); err != nil {
			t.Fatalf("failed to save project config: %v", err)
		}

		// Verify agents are saved
		savedCfg, err := config.LoadProjectConfig(tempProject2)
		if err != nil {
			t.Fatalf("failed to load saved config: %v", err)
		}

		if len(savedCfg.Agents) != 2 {
			t.Errorf("expected 2 agents, got %d", len(savedCfg.Agents))
		}

		if savedCfg.TeamName != "Default Team" {
			t.Errorf("expected team name 'Default Team', got '%s'", savedCfg.TeamName)
		}
	})
}

func TestInitIdempotent(t *testing.T) {
	tempProject := t.TempDir()

	// First init
	if err := config.EnsureProjectInitialized(tempProject); err != nil {
		t.Fatalf("first init failed: %v", err)
	}

	// Modify config
	cfg, err := config.LoadProjectConfig(tempProject)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	cfg.TeamName = "Modified Team"
	if err := config.SaveToProject(tempProject, cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Second init should not overwrite
	if err := config.EnsureProjectInitialized(tempProject); err != nil {
		t.Fatalf("second init failed: %v", err)
	}

	// Verify modification preserved
	cfg2, err := config.LoadProjectConfig(tempProject)
	if err != nil {
		t.Fatalf("failed to load config after second init: %v", err)
	}

	if cfg2.TeamName != "Modified Team" {
		t.Errorf("expected team name 'Modified Team', got '%s'", cfg2.TeamName)
	}
}
