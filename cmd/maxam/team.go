package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/ytnobody/MAXAM/internal/config"
)

// runTeam handles the "team" subcommand
func runTeam() {
	if len(os.Args) < 3 {
		printTeamUsage()
		os.Exit(1)
	}

	subCmd := os.Args[2]

	switch subCmd {
	case "init":
		runTeamInit()
	case "add":
		runTeamAdd()
	case "list":
		runTeamList()
	case "remove", "rm":
		runTeamRemove()
	default:
		fmt.Fprintf(os.Stderr, "Unknown team command: %s\n", subCmd)
		printTeamUsage()
		os.Exit(1)
	}
}

func printTeamUsage() {
	fmt.Println(`Usage: maxam team <command> [arguments]

Commands:
  init                     Initialize team configuration interactively
  add <name> <role>        Add a new team member
  list                     List current team members
  remove <name>            Remove a team member (alias: rm)

Examples:
  maxam team init
  maxam team add koji "Frontend / Backend"
  maxam team list
  maxam team remove koji`)
}

func runTeamInit() {
	workDir := getWorkDir()
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("MAXAM Team Configuration")
	fmt.Println("========================")
	fmt.Println()
	fmt.Println("Add team members interactively.")
	fmt.Println("Enter member details, or press Enter with empty name to finish.")
	fmt.Println()

	var agents []config.AgentConfig

	for {
		fmt.Print("Name (e.g., koji): ")
		name, _ := reader.ReadString('\n')
		name = strings.TrimSpace(name)

		if name == "" {
			break
		}

		fmt.Print("Full name (e.g., Koji Yamamoto): ")
		fullName, _ := reader.ReadString('\n')
		fullName = strings.TrimSpace(fullName)
		if fullName == "" {
			fullName = name
		}

		fmt.Print("Role (e.g., Backend / Frontend): ")
		role, _ := reader.ReadString('\n')
		role = strings.TrimSpace(role)

		agents = append(agents, config.AgentConfig{
			Name:     strings.ToLower(name),
			FullName: fullName,
			Role:     role,
		})

		fmt.Printf("  Added: %s (%s) - %s\n\n", name, fullName, role)
	}

	if len(agents) == 0 {
		fmt.Println("No members added. Exiting.")
		return
	}

	// Load existing project config or create new
	cfg, err := config.LoadProjectConfig(workDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Set default agent if not set
	if cfg.DefaultAgent == "" && len(agents) > 0 {
		cfg.DefaultAgent = agents[0].Name
	}

	cfg.Agents = agents

	if err := config.SaveToProject(workDir, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("Team configuration saved to: %s\n", config.ProjectConfigDir(workDir)+"/config.yaml")
	fmt.Printf("Default agent: %s\n", cfg.DefaultAgent)
	fmt.Printf("Team size: %d members\n", len(agents))
}

func runTeamAdd() {
	if len(os.Args) < 5 {
		fmt.Fprintln(os.Stderr, "Usage: maxam team add <name> <role>")
		fmt.Fprintln(os.Stderr, "Example: maxam team add koji \"Frontend / Backend\"")
		os.Exit(1)
	}

	name := strings.ToLower(os.Args[3])
	role := os.Args[4]

	// Parse optional full name
	fullName := name
	for i := 5; i < len(os.Args); i++ {
		if os.Args[i] == "--full-name" && i+1 < len(os.Args) {
			fullName = os.Args[i+1]
			break
		}
	}

	workDir := getWorkDir()

	cfg, err := config.LoadProjectConfig(workDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Check if already exists
	for _, a := range cfg.Agents {
		if a.Name == name {
			fmt.Fprintf(os.Stderr, "Member '%s' already exists\n", name)
			os.Exit(1)
		}
	}

	cfg.Agents = append(cfg.Agents, config.AgentConfig{
		Name:     name,
		FullName: fullName,
		Role:     role,
	})

	// Set default if first member
	if cfg.DefaultAgent == "" {
		cfg.DefaultAgent = name
	}

	if err := config.SaveToProject(workDir, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Added: %s (%s)\n", name, role)
	fmt.Printf("Config: %s\n", config.ProjectConfigDir(workDir)+"/config.yaml")
}

func runTeamList() {
	workDir := getWorkDir()

	cfg, err := config.LoadProjectConfig(workDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if len(cfg.Agents) == 0 {
		fmt.Println("No team members configured.")
		fmt.Println("Use 'maxam team init' or 'maxam team add' to add members.")
		return
	}

	fmt.Println("Team Members:")
	fmt.Println()
	for _, a := range cfg.Agents {
		defaultMark := ""
		if a.Name == cfg.DefaultAgent {
			defaultMark = " (default)"
		}
		fmt.Printf("  @%-12s %-20s %s%s\n", a.Name, a.FullName, a.Role, defaultMark)
	}
	fmt.Println()
	fmt.Printf("Config: %s\n", config.ProjectConfigDir(workDir)+"/config.yaml")
}

func runTeamRemove() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: maxam team remove <name>")
		os.Exit(1)
	}

	name := strings.ToLower(os.Args[3])
	workDir := getWorkDir()

	cfg, err := config.LoadProjectConfig(workDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	found := false
	var newAgents []config.AgentConfig
	for _, a := range cfg.Agents {
		if a.Name == name {
			found = true
		} else {
			newAgents = append(newAgents, a)
		}
	}

	if !found {
		fmt.Fprintf(os.Stderr, "Member '%s' not found\n", name)
		os.Exit(1)
	}

	cfg.Agents = newAgents

	// Update default if removed
	if cfg.DefaultAgent == name {
		if len(newAgents) > 0 {
			cfg.DefaultAgent = newAgents[0].Name
		} else {
			cfg.DefaultAgent = ""
		}
	}

	if err := config.SaveToProject(workDir, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Removed: %s\n", name)
}
