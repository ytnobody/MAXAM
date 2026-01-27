package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ytnobody/MAXAM/internal/agent"
	"github.com/ytnobody/MAXAM/internal/comms"
	"github.com/ytnobody/MAXAM/internal/logger"
	"github.com/ytnobody/MAXAM/internal/workflow"
)

func main() {
	fmt.Println("MAXAM Workflow Test")
	fmt.Println("===================")

	workDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "get workdir: %v\n", err)
		os.Exit(1)
	}

	// Initialize components
	agents := agent.NewAgents(workDir)
	router := comms.NewRouter(filepath.Join(workDir, "comms"))
	logMgr := logger.NewManager(filepath.Join(workDir, "logs"))
	defer logMgr.Close()

	// Create review cycle workflow
	reviewCycle := workflow.NewReviewCycle(agents, router, logMgr)

	// Test task
	task := `test/ ディレクトリに簡単なGoのユニットテストを作成してください。
内容は、2つの数を足す関数 Add(a, b int) int のテストです。

1. test/math.go に Add 関数を実装
2. test/math_test.go にテストを作成
3. go test が通ることを確認`

	fmt.Printf("\nTask:\n%s\n", task)

	ctx := context.Background()
	result, err := reviewCycle.Run(ctx, task)
	if err != nil {
		fmt.Fprintf(os.Stderr, "workflow error: %v\n", err)
		os.Exit(1)
	}

	// Print result
	fmt.Println("\n=== Result ===")
	fmt.Printf("Approved: %v\n", result.Approved)
	fmt.Printf("Iterations: %d\n", result.Iterations)
	fmt.Printf("Escalated: %v\n", result.Escalated)

	fmt.Println("\n=== History ===")
	for _, round := range result.History {
		fmt.Printf("\n--- Round %d ---\n", round.Round)
		fmt.Printf("Approved: %v\n", round.Approved)
		fmt.Printf("Tags: %v\n", round.Tags)
		fmt.Println("\nImplementation (truncated):")
		fmt.Println(truncate(round.Implementation, 500))
		fmt.Println("\nReview (truncated):")
		fmt.Println(truncate(round.Review, 500))
	}

	fmt.Println("\nDone.")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n...(truncated)"
}
