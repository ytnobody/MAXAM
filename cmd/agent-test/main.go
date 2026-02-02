package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ytnobody/MAXAM/internal/agent"
)

func main() {
	fmt.Println("MAXAM Agent Test")
	fmt.Println("================")

	workDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "get workdir: %v\n", err)
		os.Exit(1)
	}

	agents := agent.NewAgents(workDir)

	// Test Yuki
	fmt.Println("\n--- Testing Yuki (Implementation) ---")
	yuki, ok := agents.Get("yuki")
	if !ok {
		fmt.Fprintln(os.Stderr, "yuki agent not found")
		os.Exit(1)
	}
	yuki.Timeout = 2 * time.Minute

	ctx := context.Background()
	prompt := `簡単な自己紹介をしてください。あなたの名前、役割、性格を含めて3行程度で。`

	fmt.Printf("Prompt: %s\n\n", prompt)
	fmt.Println("Running...")

	start := time.Now()
	result, err := yuki.Run(ctx, prompt)
	elapsed := time.Since(start)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Yuki error: %v\n", err)
	} else {
		fmt.Printf("Yuki says:\n%s\n", result)
	}
	fmt.Printf("(took %v)\n", elapsed.Round(time.Millisecond))

	// Test Priya
	fmt.Println("\n--- Testing Priya (Review) ---")
	priya, ok := agents.Get("priya")
	if !ok {
		fmt.Fprintln(os.Stderr, "priya agent not found")
		os.Exit(1)
	}
	priya.Timeout = 2 * time.Minute

	prompt = `簡単な自己紹介をしてください。あなたの名前、役割、性格を含めて3行程度で。`

	fmt.Printf("Prompt: %s\n\n", prompt)
	fmt.Println("Running...")

	start = time.Now()
	result, err = priya.Run(ctx, prompt)
	elapsed = time.Since(start)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Priya error: %v\n", err)
	} else {
		fmt.Printf("Priya says:\n%s\n", result)
	}
	fmt.Printf("(took %v)\n", elapsed.Round(time.Millisecond))

	fmt.Println("\nDone.")
}
