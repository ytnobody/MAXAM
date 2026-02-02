package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ytnobody/MAXAM/internal/agent"
	"github.com/ytnobody/MAXAM/internal/taskboard"
)

// runTaskboard handles the "task" subcommand for task board operations
func runTaskboard() {
	if len(os.Args) < 3 {
		printTaskboardUsage()
		os.Exit(1)
	}

	subCmd := os.Args[2]

	switch subCmd {
	case "add":
		runTaskAdd()
	case "list":
		runTaskList()
	case "delete", "rm":
		runTaskDelete()
	case "status":
		runTaskStatus()
	default:
		fmt.Fprintf(os.Stderr, "Unknown task command: %s\n", subCmd)
		printTaskboardUsage()
		os.Exit(1)
	}
}

func printTaskboardUsage() {
	fmt.Println(`Usage: maxam task <command> [arguments]

Commands:
  add <title> [--assignee <name>] [--desc <description>]
                         Add a new task to the taskboard
  list                   List all tasks
  delete <id>            Delete a task by ID (alias: rm)
  status <id> <status>   Change task status (pending/in_progress/completed)

Examples:
  maxam task add "Implement login feature"
  maxam task add "Fix bug #42" --assignee yuki
  maxam task add "Update docs" --desc "Add API documentation"
  maxam task list
  maxam task delete 1
  maxam task status 1 in_progress
  maxam task status 1 completed`)
}

func runTaskAdd() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: maxam task add <title> [--assignee <name>] [--desc <description>]")
		os.Exit(1)
	}

	title := os.Args[3]
	assignee := ""
	description := ""

	// Parse optional flags
	for i := 4; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--assignee", "-a":
			if i+1 < len(os.Args) {
				assignee = os.Args[i+1]
				i++
			}
		case "--desc", "-d":
			if i+1 < len(os.Args) {
				description = os.Args[i+1]
				i++
			}
		}
	}

	// Create file store
	store, err := taskboard.NewFileStore("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing task store: %v\n", err)
		os.Exit(1)
	}

	// Create task
	task := &taskboard.Task{
		Title:       title,
		Assignee:    assignee,
		Description: description,
		Status:      taskboard.StatusPending,
	}

	created, err := store.Create(context.Background(), task)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating task: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Task #%d created: %s\n", created.ID, created.Title)
	if assignee != "" {
		fmt.Printf("  Assignee: %s\n", assignee)
	}
	fmt.Printf("  File: %s\n", store.FilePath())
}

func runTaskList() {
	store, err := taskboard.NewFileStore("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing task store: %v\n", err)
		os.Exit(1)
	}

	tasks, err := store.List(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing tasks: %v\n", err)
		os.Exit(1)
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found.")
		fmt.Printf("  File: %s\n", store.FilePath())
		return
	}

	fmt.Println("Tasks:")
	for _, t := range tasks {
		status := statusIcon(t.Status)
		assignee := ""
		if t.Assignee != "" {
			assignee = fmt.Sprintf(" (@%s)", t.Assignee)
		}
		fmt.Printf("  %s #%d: %s%s\n", status, t.ID, t.Title, assignee)
	}
	fmt.Printf("\n  File: %s\n", store.FilePath())
}

func statusIcon(s taskboard.Status) string {
	switch s {
	case taskboard.StatusPending:
		return "○"
	case taskboard.StatusInProgress:
		return "◐"
	case taskboard.StatusCompleted:
		return "●"
	default:
		return "?"
	}
}

func runTaskDelete() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: maxam task delete <id>")
		os.Exit(1)
	}

	id, err := strconv.Atoi(os.Args[3])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid task ID: %s\n", os.Args[3])
		os.Exit(1)
	}

	store, err := taskboard.NewFileStore("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing task store: %v\n", err)
		os.Exit(1)
	}

	if err := store.Delete(context.Background(), id); err != nil {
		if err == taskboard.ErrNotFound {
			fmt.Fprintf(os.Stderr, "Task #%d not found\n", id)
		} else {
			fmt.Fprintf(os.Stderr, "Error deleting task: %v\n", err)
		}
		os.Exit(1)
	}

	fmt.Printf("Task #%d deleted\n", id)
}

func runTaskStatus() {
	if len(os.Args) < 5 {
		fmt.Fprintln(os.Stderr, "Usage: maxam task status <id> <status>")
		fmt.Fprintln(os.Stderr, "Status: pending, in_progress, completed")
		os.Exit(1)
	}

	id, err := strconv.Atoi(os.Args[3])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid task ID: %s\n", os.Args[3])
		os.Exit(1)
	}

	statusStr := strings.ToLower(os.Args[4])
	var status taskboard.Status
	switch statusStr {
	case "pending", "todo":
		status = taskboard.StatusPending
	case "in_progress", "inprogress", "doing":
		status = taskboard.StatusInProgress
	case "completed", "done":
		status = taskboard.StatusCompleted
	default:
		fmt.Fprintf(os.Stderr, "Invalid status: %s\n", statusStr)
		fmt.Fprintln(os.Stderr, "Valid: pending, in_progress, completed")
		os.Exit(1)
	}

	store, err := taskboard.NewFileStore("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing task store: %v\n", err)
		os.Exit(1)
	}

	task, err := store.Get(context.Background(), id)
	if err != nil {
		if err == taskboard.ErrNotFound {
			fmt.Fprintf(os.Stderr, "Task #%d not found\n", id)
		} else {
			fmt.Fprintf(os.Stderr, "Error getting task: %v\n", err)
		}
		os.Exit(1)
	}

	task.Status = status
	if err := store.Update(context.Background(), task); err != nil {
		fmt.Fprintf(os.Stderr, "Error updating task: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Task #%d status changed to %s\n", id, status)
}

// runTask is the legacy task command (for implementation requests)
func runTaskLegacy() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: maxam task <description>")
		os.Exit(1)
	}

	// Check if it's a taskboard subcommand
	subCmd := strings.ToLower(os.Args[2])
	if subCmd == "add" || subCmd == "list" || subCmd == "delete" || subCmd == "rm" || subCmd == "status" {
		runTaskboard()
		return
	}

	// Legacy behavior: treat as task description for Yuki
	runTaskImpl()
}

// runTaskImpl is the original task implementation logic
func runTaskImpl() {
	task := strings.Join(os.Args[2:], " ")
	workDir := getWorkDir()

	fmt.Println("MAXAM Task Runner")
	fmt.Println("=================")
	fmt.Printf("Task: %s\n\n", task)

	agents := agent.NewAgents(workDir)
	developer, ok := agents.GetByRole("developer")
	if !ok {
		fmt.Fprintln(os.Stderr, "Error: no agent with 'developer' role found")
		os.Exit(1)
	}

	ctx := context.Background()
	fmt.Printf("[%s] Working on it...\n", developer.Name)

	result, err := developer.Run(ctx, task)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n[%s] Done.\n", developer.Name)
	fmt.Println("\n--- Output ---")
	fmt.Println(result)
}
