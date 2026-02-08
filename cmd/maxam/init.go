package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ytnobody/MAXAM/internal/config"
)

func runInit() {
	workDir := getWorkDir()

	// Check if already initialized
	if config.IsProjectInitialized(workDir) {
		fmt.Println("Project already initialized. Skipping.")
		fmt.Printf("  Config: %s/config.yaml\n", config.ProjectConfigDir(workDir))
		fmt.Printf("  CLAUDE.md: %s/CLAUDE.md\n", config.ProjectConfigDir(workDir))
		return
	}

	fmt.Println("Initializing MAXAM project...")

	// 1. Create .maxam/ directory and default files
	if err := config.EnsureProjectInitialized(workDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// 2. Load default team from ~/.maxam/default-team.yaml if exists
	defaultTeam, err := loadDefaultTeam()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load default team: %v\n", err)
	}

	if defaultTeam != nil && len(defaultTeam.Agents) > 0 {
		// Load project config and add agents
		projectCfg, err := config.LoadProjectConfig(workDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading project config: %v\n", err)
			os.Exit(1)
		}

		projectCfg.Agents = defaultTeam.Agents
		if defaultTeam.TeamName != "" {
			projectCfg.TeamName = defaultTeam.TeamName
		}

		if err := config.SaveToProject(workDir, projectCfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving project config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Loaded default team (%d agents)\n", len(defaultTeam.Agents))
	}

	fmt.Println("Done!")
	fmt.Println()
	fmt.Printf("Created:\n")
	fmt.Printf("  %s/config.yaml\n", config.ProjectConfigDir(workDir))
	fmt.Printf("  %s/CLAUDE.md\n", config.ProjectConfigDir(workDir))
	fmt.Println()
	fmt.Println("Next steps:")
	if defaultTeam == nil || len(defaultTeam.Agents) == 0 {
		fmt.Println("  1. Run 'maxam team init' to set up team members")
		fmt.Println("  2. Or edit .maxam/config.yaml directly")
	} else {
		fmt.Println("  Team is ready! Run 'maxam' to start.")
	}
}

// loadDefaultTeam loads team configuration from ~/.maxam/default-team.yaml
func loadDefaultTeam() (*config.Config, error) {
	configDir, err := config.ConfigDir()
	if err != nil {
		return nil, err
	}

	defaultTeamPath := filepath.Join(configDir, "default-team.yaml")
	return config.LoadFromPath(defaultTeamPath)
}
