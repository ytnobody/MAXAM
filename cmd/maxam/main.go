package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ytnobody/MAXAM/internal/agent"
	"github.com/ytnobody/MAXAM/internal/logger"
	"github.com/ytnobody/MAXAM/internal/summary"
	"github.com/ytnobody/MAXAM/internal/workflow"
)

func main() {
	// Ensure CLAUDE.summary.md is up to date before any command
	workDir, _ := os.Getwd()
	if err := summary.EnsureUpToDate(workDir); err != nil {
		// Non-fatal: just log and continue
		fmt.Fprintf(os.Stderr, "Warning: failed to update summary: %v\n", err)
	}

	if len(os.Args) < 2 {
		// No arguments: launch team chat TUI
		runTeamChat()
		return
	}

	cmd := os.Args[1]

	switch cmd {
	case "init":
		runInit()
	case "task":
		runTaskLegacy()
	case "team":
		runTeam()
	case "review":
		runReview()
	case "ask":
		runAsk()
	case "chat":
		runChat()
	case "analyze":
		runAnalyze()
	case "status":
		showStatus()
	case "watch":
		runWatch()
	case "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`MAXAM - AI Development Team CLI

Usage:
  maxam                Launch team chat (TUI)
  maxam <command> [arguments]

Commands:
  init                 Initialize MAXAM in current project
  (none)               Launch team chat TUI (default)
  chat <agent>         Interactive chat with an agent (mei/yuki/rin/shiori/priya/amara/team)
  chat team --daemon   Daemon mode: wait for input continuously (for external integration)
  task <description>   Submit a task to Yuki for implementation
  task add <title>     Add a task to the taskboard
  task list            List all tasks on the taskboard
  task delete <id>     Delete a task by ID
  task status <id> <s> Change task status (pending/in_progress/completed)
  team init            Initialize team configuration interactively
  team add <n> <role>  Add a team member
  team list            List team members
  team remove <name>   Remove a team member
  review <description> Run a full review cycle (Yuki → Priya)
  ask <agent> <prompt> Ask a specific agent a question
  analyze              Run Amara's weekly analysis
  watch <owner/repo>   Watch repository for new Issues/PRs
  status               Show team status
  help                 Show this help

Agents:
  mei    - PM, requirements
  yuki   - Backend, infrastructure
  rin    - Frontend
  shiori - Test, documentation
  priya  - Review, security, QA
  amara  - Analysis

Examples:
  maxam                                           # Start team chat
  maxam task add "Implement user authentication"  # Add task to board
  maxam task list                                 # List all tasks
  maxam task delete 1                             # Delete task #1
  maxam task status 1 in_progress                 # Change task #1 to in_progress
  maxam task "Add a login button to the header"  # Delegate to Yuki
  maxam review "Create unit tests for the auth module"
  maxam ask yuki "How would you implement caching here?"
  maxam watch ytnobody/MAXAM              # Watch for new Issues/PRs
  maxam watch ytnobody/MAXAM --interval 30s`)
}

func getWorkDir() string {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
		os.Exit(1)
	}
	return dir
}

func runReview() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: maxam review <description>")
		os.Exit(1)
	}

	task := strings.Join(os.Args[2:], " ")
	workDir := getWorkDir()

	fmt.Println("MAXAM Review Cycle")
	fmt.Println("==================")
	fmt.Printf("Task: %s\n", task)

	agents := agent.NewAgents(workDir)
	logMgr := logger.NewManager(logger.GetDefaultLogDir(), workDir)
	defer logMgr.Close()

	reviewCycle := workflow.NewReviewCycle(agents, logMgr)

	ctx := context.Background()
	result, err := reviewCycle.Run(ctx, task)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n=== Summary ===")
	if result.Approved {
		fmt.Println("Status: APPROVED")
	} else if result.Escalated {
		fmt.Println("Status: ESCALATED (needs human review)")
	} else {
		fmt.Println("Status: NOT APPROVED")
	}
	fmt.Printf("Iterations: %d\n", result.Iterations)
}

func runAsk() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: maxam ask <agent> <prompt>")
		fmt.Fprintln(os.Stderr, "Agents: mei, yuki, rin, shiori, priya, amara")
		os.Exit(1)
	}

	agentName := strings.ToLower(os.Args[2])
	prompt := strings.Join(os.Args[3:], " ")
	workDir := getWorkDir()

	agents := agent.NewAgents(workDir)
	runner, ok := agents.Get(agentName)
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown agent: %s\n", agentName)
		fmt.Fprintln(os.Stderr, "Available: mei, yuki, rin, shiori, priya, amara")
		os.Exit(1)
	}

	fmt.Printf("[%s] Thinking...\n\n", agentName)

	ctx := context.Background()
	result, err := runner.Run(ctx, prompt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(result)
}

func runAnalyze() {
	workDir := getWorkDir()

	fmt.Println("MAXAM Weekly Analysis")
	fmt.Println("=====================")

	agents := agent.NewAgents(workDir)
	logMgr := logger.NewManager(logger.GetDefaultLogDir(), workDir)
	defer logMgr.Close()

	analysisCycle := workflow.NewAnalysisCycle(agents, logMgr, workDir)

	ctx := context.Background()
	result, err := analysisCycle.Run(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n=== Analysis Report ===")
	fmt.Println(result.Report)

	if len(result.Recommendations) > 0 {
		fmt.Println("\n=== Recommendations ===")
		for i, rec := range result.Recommendations {
			fmt.Printf("%d. %s\n", i+1, rec)
		}
	}
}

func showStatus() {
	workDir := getWorkDir()

	fmt.Println("MAXAM Team Status")
	fmt.Println("=================")
	fmt.Println()

	// Check CLAUDE.md files
	fmt.Println("Agents:")
	agentNames := []string{"mei", "yuki", "rin", "shiori", "priya", "amara"}
	agentRoles := map[string]string{
		"mei":    "PM / Requirements",
		"yuki":   "Backend / Infrastructure",
		"rin":    "Frontend",
		"shiori": "Test / Documentation",
		"priya":  "Review / QA",
		"amara":  "Analysis",
	}

	for _, name := range agentNames {
		claudeMD := filepath.Join(workDir, "agents", name, "CLAUDE.md")
		status := "Ready"
		if _, err := os.Stat(claudeMD); os.IsNotExist(err) {
			status = "Missing CLAUDE.md"
		}
		fmt.Printf("  %-8s %-20s [%s]\n", name, agentRoles[name], status)
	}

	// Check logs
	fmt.Println("\nLogs:")
	projectName := logger.ProjectNameFromPath(workDir)
	logsDir := filepath.Join(logger.GetDefaultLogDir(), projectName)
	for _, name := range agentNames {
		agentLogDir := filepath.Join(logsDir, name)
		if entries, err := os.ReadDir(agentLogDir); err == nil {
			fmt.Printf("  %s: %d log files\n", name, len(entries))
		}
	}
	fmt.Printf("  (location: %s)\n", logsDir)
}
