package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ytnobody/MAXAM/internal/github"
)

func runPRStatus() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: maxam pr-status <owner/repo>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Shows PRs that are awaiting review.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "A PR is considered 'awaiting review' if:")
		fmt.Fprintln(os.Stderr, "  - It has no reviews yet, OR")
		fmt.Fprintln(os.Stderr, "  - Someone was requested to review")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "PRs with CHANGES_REQUESTED are NOT included")
		fmt.Fprintln(os.Stderr, "(author needs to fix first)")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  maxam pr-status ytnobody/MAXAM")
		os.Exit(1)
	}

	// Parse owner/repo
	repoArg := os.Args[2]
	parts := strings.Split(repoArg, "/")
	if len(parts) != 2 {
		fmt.Fprintf(os.Stderr, "Invalid repository format: %s (expected owner/repo)\n", repoArg)
		os.Exit(1)
	}
	owner, repo := parts[0], parts[1]

	// Create GitHub client
	client, err := github.NewClient(owner, repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating GitHub client: %v\n", err)
		fmt.Fprintln(os.Stderr, "Make sure GITHUB_TOKEN environment variable is set")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Printf("Checking PRs awaiting review in %s/%s...\n\n", owner, repo)

	prs, err := client.ListPRsAwaitingReview(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing PRs: %v\n", err)
		os.Exit(1)
	}

	if len(prs) == 0 {
		fmt.Println("No PRs awaiting review.")
		return
	}

	fmt.Printf("Found %d PR(s) awaiting review:\n\n", len(prs))

	for _, prStatus := range prs {
		pr := prStatus.PR
		fmt.Printf("  #%d %s\n", pr.GetNumber(), pr.GetTitle())
		fmt.Printf("      Author: %s\n", pr.GetUser().GetLogin())
		fmt.Printf("      URL: %s\n", pr.GetHTMLURL())
		fmt.Printf("      Created: %s\n", pr.GetCreatedAt().Format("2006-01-02 15:04"))
		fmt.Println()
	}
}
