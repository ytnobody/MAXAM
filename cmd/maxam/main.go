package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ytnobody/MAXAM/internal/agent"
	"github.com/ytnobody/MAXAM/internal/comms"
	"github.com/ytnobody/MAXAM/internal/logger"
	"github.com/ytnobody/MAXAM/internal/workflow"
)

func main() {
	if len(os.Args) < 2 {
		// No arguments: launch team chat TUI
		runTeamChat()
		return
	}

	cmd := os.Args[1]

	switch cmd {
	case "task":
		runTask()
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
  (none)               Launch team chat TUI (default)
  chat <agent>         Interactive chat with an agent (mei/yuki/priya/amara/team)
  task <description>   Submit a task to Yuki for implementation
  review <description> Run a full review cycle (Yuki → Priya)
  ask <agent> <prompt> Ask a specific agent a question
  analyze              Run Amara's weekly analysis
  status               Show team status
  help                 Show this help

Agents:
  mei    - PM, requirements
  yuki   - Implementation, infrastructure
  priya  - Review, security, QA
  amara  - Analysis

Examples:
  maxam                                           # Start team chat
  maxam task "Add a login button to the header"
  maxam review "Create unit tests for the auth module"
  maxam ask yuki "How would you implement caching here?"`)
}

func getWorkDir() string {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
		os.Exit(1)
	}
	return dir
}

func runTask() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: maxam task <description>")
		os.Exit(1)
	}

	task := strings.Join(os.Args[2:], " ")
	workDir := getWorkDir()

	fmt.Println("MAXAM Task Runner")
	fmt.Println("=================")
	fmt.Printf("Task: %s\n\n", task)

	agents := agent.NewAgents(workDir)
	yuki := agents.Yuki()

	ctx := context.Background()
	fmt.Println("[Yuki] Working on it...")

	result, err := yuki.Run(ctx, task)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n[Yuki] Done.")
	fmt.Println("\n--- Output ---")
	fmt.Println(result)
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
	router := comms.NewRouter(filepath.Join(workDir, "comms"))
	logMgr := logger.NewManager(filepath.Join(workDir, "logs"))
	defer logMgr.Close()

	reviewCycle := workflow.NewReviewCycle(agents, router, logMgr)

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
		fmt.Fprintln(os.Stderr, "Agents: mei, yuki, priya, amara")
		os.Exit(1)
	}

	agentName := strings.ToLower(os.Args[2])
	prompt := strings.Join(os.Args[3:], " ")
	workDir := getWorkDir()

	agents := agent.NewAgents(workDir)
	runner, ok := agents.Get(agentName)
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown agent: %s\n", agentName)
		fmt.Fprintln(os.Stderr, "Available: mei, yuki, priya, amara")
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
	router := comms.NewRouter(filepath.Join(workDir, "comms"))
	logMgr := logger.NewManager(filepath.Join(workDir, "logs"))
	defer logMgr.Close()

	analysisCycle := workflow.NewAnalysisCycle(agents, router, logMgr, workDir)

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
	agentNames := []string{"mei", "yuki", "priya", "amara"}
	agentRoles := map[string]string{
		"mei":   "PM / Requirements",
		"yuki":  "Implementation",
		"priya": "Review / QA",
		"amara": "Analysis",
	}

	for _, name := range agentNames {
		claudeMD := filepath.Join(workDir, "agents", name, "CLAUDE.md")
		status := "Ready"
		if _, err := os.Stat(claudeMD); os.IsNotExist(err) {
			status = "Missing CLAUDE.md"
		}
		fmt.Printf("  %-8s %-20s [%s]\n", name, agentRoles[name], status)
	}

	// Check comms
	fmt.Println("\nCommunications:")
	commsDir := filepath.Join(workDir, "comms")
	if entries, err := os.ReadDir(commsDir); err == nil {
		if len(entries) == 0 {
			fmt.Println("  (no messages)")
		}
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".md") {
				info, _ := e.Info()
				fmt.Printf("  %s (%d bytes)\n", e.Name(), info.Size())
			}
		}
	} else {
		fmt.Println("  (comms directory not found)")
	}

	// Check logs
	fmt.Println("\nLogs:")
	logsDir := filepath.Join(workDir, "logs")
	for _, name := range agentNames {
		agentLogDir := filepath.Join(logsDir, name)
		if entries, err := os.ReadDir(agentLogDir); err == nil {
			fmt.Printf("  %s: %d log files\n", name, len(entries))
		}
	}
}
