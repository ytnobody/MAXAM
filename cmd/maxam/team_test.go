package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ytnobody/MAXAM/internal/config"
)

func TestTeamAdd(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "maxam-team-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp dir
	oldDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to chdir: %v", err)
	}
	defer os.Chdir(oldDir)

	// Test adding a member
	cfg := &config.Config{
		Version: "1",
		Agents: []config.AgentConfig{
			{Name: "koji", FullName: "Koji Yamamoto", Role: "Backend"},
		},
		DefaultAgent: "koji",
	}

	if err := config.SaveToProject(tmpDir, cfg); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify config was saved
	loadedCfg, err := config.LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if len(loadedCfg.Agents) != 1 {
		t.Errorf("Expected 1 agent, got %d", len(loadedCfg.Agents))
	}

	if loadedCfg.Agents[0].Name != "koji" {
		t.Errorf("Expected agent name 'koji', got '%s'", loadedCfg.Agents[0].Name)
	}

	if loadedCfg.DefaultAgent != "koji" {
		t.Errorf("Expected default agent 'koji', got '%s'", loadedCfg.DefaultAgent)
	}
}

func TestTeamRemove(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "maxam-team-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Setup initial config with 2 members
	cfg := &config.Config{
		Version: "1",
		Agents: []config.AgentConfig{
			{Name: "koji", FullName: "Koji Yamamoto", Role: "Backend"},
			{Name: "yuki", FullName: "Yuki Tanaka", Role: "Frontend"},
		},
		DefaultAgent: "koji",
	}

	if err := config.SaveToProject(tmpDir, cfg); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Simulate removing koji
	loadedCfg, err := config.LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Remove koji
	var newAgents []config.AgentConfig
	for _, a := range loadedCfg.Agents {
		if a.Name != "koji" {
			newAgents = append(newAgents, a)
		}
	}
	loadedCfg.Agents = newAgents

	// Update default if removed
	if loadedCfg.DefaultAgent == "koji" && len(newAgents) > 0 {
		loadedCfg.DefaultAgent = newAgents[0].Name
	}

	if err := config.SaveToProject(tmpDir, loadedCfg); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify
	finalCfg, err := config.LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if len(finalCfg.Agents) != 1 {
		t.Errorf("Expected 1 agent after removal, got %d", len(finalCfg.Agents))
	}

	if finalCfg.Agents[0].Name != "yuki" {
		t.Errorf("Expected remaining agent 'yuki', got '%s'", finalCfg.Agents[0].Name)
	}

	if finalCfg.DefaultAgent != "yuki" {
		t.Errorf("Expected default agent updated to 'yuki', got '%s'", finalCfg.DefaultAgent)
	}
}

func TestConfigSaveToProject(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "maxam-config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Version:      "1",
		DefaultAgent: "test",
		Agents: []config.AgentConfig{
			{Name: "test", FullName: "Test User", Role: "Testing"},
		},
	}

	if err := config.SaveToProject(tmpDir, cfg); err != nil {
		t.Fatalf("SaveToProject failed: %v", err)
	}

	// Verify file exists
	configPath := filepath.Join(tmpDir, ".maxam", "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("Config file was not created at %s", configPath)
	}

	// Verify content
	loadedCfg, err := config.LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig failed: %v", err)
	}

	if loadedCfg.Version != "1" {
		t.Errorf("Expected version '1', got '%s'", loadedCfg.Version)
	}

	if loadedCfg.DefaultAgent != "test" {
		t.Errorf("Expected default agent 'test', got '%s'", loadedCfg.DefaultAgent)
	}
}

func TestLoadProjectConfigNotExist(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "maxam-config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Should return empty config when file doesn't exist
	cfg, err := config.LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig failed: %v", err)
	}

	if cfg.Version != "1" {
		t.Errorf("Expected version '1' for new config, got '%s'", cfg.Version)
	}

	if len(cfg.Agents) != 0 {
		t.Errorf("Expected 0 agents for new config, got %d", len(cfg.Agents))
	}
}
