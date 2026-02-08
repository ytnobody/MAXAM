package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ytnobody/MAXAM/internal/github"
)

func runWatch() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: maxam watch <owner/repo> [--interval <duration>]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  maxam watch ytnobody/MAXAM")
		fmt.Fprintln(os.Stderr, "  maxam watch ytnobody/MAXAM --interval 30s")
		fmt.Fprintln(os.Stderr, "  maxam watch ytnobody/MAXAM --interval 2m")
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

	// Parse interval (default: 1 minute)
	interval := time.Minute
	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--interval" && i+1 < len(os.Args) {
			d, err := time.ParseDuration(os.Args[i+1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid duration: %s\n", os.Args[i+1])
				os.Exit(1)
			}
			interval = d
		}
	}

	// Create GitHub client
	client, err := github.NewClient(owner, repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating GitHub client: %v\n", err)
		fmt.Fprintln(os.Stderr, "Make sure GITHUB_TOKEN environment variable is set")
		os.Exit(1)
	}

	watcher := github.NewEventWatcher(client)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Printf("🔍 Watching %s/%s for new Issues/PRs (interval: %s)\n", owner, repo, interval)
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	// Initial check to populate seen IDs
	events, err := watcher.CheckEvents(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: initial check failed: %v\n", err)
	} else if len(events) > 0 {
		fmt.Printf("Found %d existing event(s), marked as seen\n", len(events))
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-sigCh:
			fmt.Println("\n👋 Stopping watcher...")
			return
		case <-ticker.C:
			events, err := watcher.CheckEvents(ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error checking events: %v\n", err)
				continue
			}

			for _, e := range events {
				fmt.Println(github.FormatEventNotification(e))
				fmt.Println()
			}
		}
	}
}
