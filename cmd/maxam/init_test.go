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

	t.Run("no default-team.yaml returns error", func(t *testing.T) {
		cfg, err := loadDefaultTeam()
		if err == nil {
			t.Error("expected error when no default-team.yaml exists")
		}
		if cfg != nil {
			t.Error("expected nil config when file doesn't exist")
		}
	})

	t.Run("fallback to builtin when no default-team.yaml", func(t *testing.T) {
		// This simulates the actual init behavior
		cfg, err := loadDefaultTeam()
		if err != nil {
			// Expected: no user-defined default team, use built-in
			cfg = config.BuiltinDefaultTeam()
		}

		if cfg == nil {
			t.Fatal("expected builtin team config")
		}

		if len(cfg.Agents) != 6 {
			t.Errorf("expected 6 agents from builtin team, got %d", len(cfg.Agents))
		}

		if cfg.TeamName != "MAXAM" {
			t.Errorf("expected team name 'MAXAM', got '%s'", cfg.TeamName)
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

	t.Run("init with user default-team.yaml takes priority", func(t *testing.T) {
		tempProject2 := t.TempDir()

		// Create user-defined default-team.yaml
		defaultTeam := &config.Config{
			Version:  "1",
			TeamName: "Custom Team",
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

		// Initialize project
		if err := config.EnsureProjectInitialized(tempProject2); err != nil {
			t.Fatalf("failed to initialize project: %v", err)
		}

		// Load default team (should use user-defined, not built-in)
		team, err := loadDefaultTeam()
		if err != nil {
			t.Fatalf("failed to load default team: %v", err)
		}

		// User-defined team should take priority
		if team.TeamName != "Custom Team" {
			t.Errorf("expected 'Custom Team' from user config, got '%s'", team.TeamName)
		}

		if len(team.Agents) != 2 {
			t.Errorf("expected 2 agents from user config, got %d", len(team.Agents))
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

		// Verify user-defined agents are saved
		savedCfg, err := config.LoadProjectConfig(tempProject2)
		if err != nil {
			t.Fatalf("failed to load saved config: %v", err)
		}

		if len(savedCfg.Agents) != 2 {
			t.Errorf("expected 2 agents, got %d", len(savedCfg.Agents))
		}

		if savedCfg.TeamName != "Custom Team" {
			t.Errorf("expected team name 'Custom Team', got '%s'", savedCfg.TeamName)
		}
	})

	t.Run("init with builtin team when no user config", func(t *testing.T) {
		tempProject3 := t.TempDir()

		// Remove user-defined default-team.yaml
		os.Remove(filepath.Join(maxamDir, "default-team.yaml"))

		// Initialize project
		if err := config.EnsureProjectInitialized(tempProject3); err != nil {
			t.Fatalf("failed to initialize project: %v", err)
		}

		// Load default team (should fall back to built-in)
		team, err := loadDefaultTeam()
		if err != nil {
			team = config.BuiltinDefaultTeam()
		}

		projectCfg, err := config.LoadProjectConfig(tempProject3)
		if err != nil {
			t.Fatalf("failed to load project config: %v", err)
		}

		projectCfg.Agents = team.Agents
		projectCfg.TeamName = team.TeamName

		if err := config.SaveToProject(tempProject3, projectCfg); err != nil {
			t.Fatalf("failed to save project config: %v", err)
		}

		// Verify builtin agents are saved
		savedCfg, err := config.LoadProjectConfig(tempProject3)
		if err != nil {
			t.Fatalf("failed to load saved config: %v", err)
		}

		if len(savedCfg.Agents) != 6 {
			t.Errorf("expected 6 agents from builtin, got %d", len(savedCfg.Agents))
		}

		if savedCfg.TeamName != "MAXAM" {
			t.Errorf("expected team name 'MAXAM' from builtin, got '%s'", savedCfg.TeamName)
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
