package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/anthropics/maxam/internal/agent"
	"github.com/anthropics/maxam/internal/comms"
	gh "github.com/anthropics/maxam/internal/github"
	"github.com/anthropics/maxam/internal/logger"
	sl "github.com/anthropics/maxam/internal/slack"
)

type Orchestrator struct {
	workDir string
	agents  *agent.Agents
	router  *comms.Router
	logMgr  *logger.Manager
	watcher *comms.Watcher
	webhook *gh.WebhookHandler

	// GitHub client (optional)
	ghClient *gh.Client

	// Slack client (optional)
	slackClient  *sl.Client
	slackHandler *sl.Handler
}

func main() {
	fmt.Println("MAXAM Orchestrator v0.1.0")
	fmt.Println("=========================")

	// Get base directory
	workDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "get working dir: %v\n", err)
		os.Exit(1)
	}

	orch := &Orchestrator{
		workDir: workDir,
	}

	if err := orch.init(); err != nil {
		fmt.Fprintf(os.Stderr, "init: %v\n", err)
		os.Exit(1)
	}
	defer orch.cleanup()

	// Start HTTP server for webhooks
	go orch.startServer()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("\nOrchestrator running.")
	fmt.Println("- Webhook endpoint: http://localhost:8080/webhook")
	fmt.Println("- Press Ctrl+C to stop")

	// Main event loop
	for {
		select {
		case event := <-orch.watcher.Events:
			orch.handleCommEvent(event)

		case err := <-orch.watcher.Errors:
			fmt.Fprintf(os.Stderr, "watcher error: %v\n", err)

		case <-sigCh:
			fmt.Println("\nShutting down...")
			return
		}
	}
}

func (o *Orchestrator) init() error {
	fmt.Println("Initializing...")

	// Initialize agents
	o.agents = agent.NewAgents(o.workDir)
	fmt.Println("  Agents: Ready (mei, yuki, priya, amara)")

	// Initialize communication router
	commsDir := filepath.Join(o.workDir, "comms")
	o.router = comms.NewRouter(commsDir)
	fmt.Println("  Router: Ready")

	// Initialize logger manager
	o.logMgr = logger.NewManager(filepath.Join(o.workDir, "logs"))
	fmt.Println("  Logger: Ready")

	// Initialize file watcher
	watcher, err := comms.NewWatcher(commsDir)
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	o.watcher = watcher
	o.watcher.Start()
	fmt.Println("  Watcher: Ready")

	// Initialize GitHub client (optional)
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		owner := os.Getenv("GITHUB_OWNER")
		repo := os.Getenv("GITHUB_REPO")
		if owner != "" && repo != "" {
			o.ghClient = gh.NewClientWithToken(owner, repo, token)
			fmt.Printf("  GitHub: Ready (%s/%s)\n", owner, repo)
		}
	}

	// Initialize webhook handler
	secret := os.Getenv("WEBHOOK_SECRET")
	o.webhook = gh.NewWebhookHandler(secret)
	o.setupWebhookHandlers()
	fmt.Println("  Webhook: Ready")

	// Initialize Slack client (optional)
	if botToken := os.Getenv("SLACK_BOT_TOKEN"); botToken != "" {
		appToken := os.Getenv("SLACK_APP_TOKEN")
		if appToken != "" {
			waitSecs := 120
			if w := os.Getenv("SLACK_WAIT_SECONDS"); w != "" {
				if parsed, err := strconv.Atoi(w); err == nil {
					waitSecs = parsed
				}
			}

			slackClient, err := sl.NewClient(sl.Config{
				BotToken:    botToken,
				AppToken:    appToken,
				WaitSeconds: waitSecs,
			})
			if err != nil {
				fmt.Printf("  Slack: Failed (%v)\n", err)
			} else {
				o.slackClient = slackClient
				o.slackHandler = sl.NewHandler(slackClient, o.agents, o.router, o.logMgr)

				// Start Slack in background
				ctx := context.Background()
				o.slackClient.Start(ctx, o.slackHandler.HandleMessages)
				go o.slackClient.Run()

				fmt.Printf("  Slack: Ready (wait %ds)\n", waitSecs)
			}
		}
	}

	return nil
}

func (o *Orchestrator) cleanup() {
	if o.watcher != nil {
		o.watcher.Stop()
	}
	if o.logMgr != nil {
		o.logMgr.Close()
	}
}

func (o *Orchestrator) startServer() {
	mux := http.NewServeMux()
	mux.Handle("/webhook", o.webhook)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("/status", o.handleStatus)

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
	}
}

func (o *Orchestrator) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status":  "running",
		"version": "0.1.0",
		"agents": []string{"mei", "yuki", "priya", "amara"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (o *Orchestrator) setupWebhookHandlers() {
	// Handle new issues
	o.webhook.On(gh.EventIssues, func(payload json.RawMessage) error {
		event, err := gh.ParseIssuesEvent(payload)
		if err != nil {
			return err
		}

		if event.Action != "opened" && event.Action != "assigned" {
			return nil
		}

		fmt.Printf("[Webhook] Issue #%d: %s (%s)\n",
			event.Issue.Number, event.Issue.Title, event.Action)

		// Notify Mei about new issue
		o.router.Send("github", "mei", &comms.Message{
			Subject:  fmt.Sprintf("New Issue #%d: %s", event.Issue.Number, event.Issue.Title),
			Body:     event.Issue.Body,
			Action:   "Review and assign",
			IssueNum: event.Issue.Number,
		})

		return nil
	})

	// Handle PRs
	o.webhook.On(gh.EventPullRequest, func(payload json.RawMessage) error {
		event, err := gh.ParsePullRequestEvent(payload)
		if err != nil {
			return err
		}

		if event.Action != "opened" && event.Action != "synchronize" {
			return nil
		}

		fmt.Printf("[Webhook] PR #%d: %s (%s)\n",
			event.PullRequest.Number, event.PullRequest.Title, event.Action)

		// Notify Priya about new PR for review
		o.router.Send("github", "priya", &comms.Message{
			Subject: fmt.Sprintf("PR #%d needs review: %s", event.PullRequest.Number, event.PullRequest.Title),
			Body:    event.PullRequest.Body,
			Action:  "Review the PR",
			PRNum:   event.PullRequest.Number,
		})

		return nil
	})

	// Handle PR reviews
	o.webhook.On(gh.EventPullRequestReview, func(payload json.RawMessage) error {
		event, err := gh.ParsePullRequestReviewEvent(payload)
		if err != nil {
			return err
		}

		fmt.Printf("[Webhook] PR Review #%d: %s by %s\n",
			event.PullRequest.Number, event.Review.State, event.Review.User.Login)

		if event.Review.State == "changes_requested" {
			// Notify Yuki about requested changes
			o.router.Send("github", "yuki", &comms.Message{
				Subject: fmt.Sprintf("PR #%d needs changes", event.PullRequest.Number),
				Body:    event.Review.Body,
				Action:  "Address the review comments",
				PRNum:   event.PullRequest.Number,
			})
		}

		return nil
	})
}

func (o *Orchestrator) handleCommEvent(event comms.Event) {
	fmt.Printf("[Comms] Message: %s -> %s\n", event.From, event.To)

	// Read the message
	content, err := os.ReadFile(event.FilePath)
	if err != nil {
		fmt.Printf("Error reading message: %v\n", err)
		return
	}

	// Log the message
	if log, err := o.logMgr.Get(event.To); err == nil {
		log.LogSimple(string(content), "received", 0)
	}

	// Handle based on recipient
	switch event.To {
	case "yuki":
		go o.handleYukiTask(string(content))
	case "priya":
		go o.handlePriyaReview(string(content))
	case "mei":
		go o.handleMeiEscalation(string(content))
	}
}

func (o *Orchestrator) handleYukiTask(content string) {
	fmt.Println("[Yuki] Processing task...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	yuki := o.agents.Yuki()
	result, err := yuki.Run(ctx, content)
	if err != nil {
		fmt.Printf("[Yuki] Error: %v\n", err)
		return
	}

	// Log result
	if log, err := o.logMgr.Get("yuki"); err == nil {
		log.LogSimple(content, result, 0)
	}

	// Notify Priya for review
	o.router.Send("yuki", "priya", &comms.Message{
		Subject: "Implementation complete - needs review",
		Body:    result,
		Action:  "Please review",
	})

	fmt.Println("[Yuki] Task complete, sent to Priya for review")
}

func (o *Orchestrator) handlePriyaReview(content string) {
	fmt.Println("[Priya] Processing review...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	priya := o.agents.Priya()
	result, err := priya.Run(ctx, content)
	if err != nil {
		fmt.Printf("[Priya] Error: %v\n", err)
		return
	}

	// Log result
	if log, err := o.logMgr.Get("priya"); err == nil {
		log.LogSimple(content, result, 0)
	}

	// Check if approved
	approved := containsApproved(result)

	if approved {
		o.router.Send("priya", "yuki", &comms.Message{
			Subject: "Review Approved",
			Body:    result,
			Action:  "None - Good job",
		})
		fmt.Println("[Priya] Approved!")
	} else {
		// Check for escalation tags
		if containsTag(result, "DESIGN") || containsTag(result, "REQUIREMENTS") || containsTag(result, "SEC-CRITICAL") {
			o.router.Send("priya", "mei", &comms.Message{
				Subject: "Escalation needed",
				Body:    result,
				Action:  "Please review the situation",
			})
			fmt.Println("[Priya] Escalated to Mei")
		} else {
			o.router.Send("priya", "yuki", &comms.Message{
				Subject: "Changes requested",
				Body:    result,
				Action:  "Please address the issues",
			})
			fmt.Println("[Priya] Requested changes from Yuki")
		}
	}
}

func (o *Orchestrator) handleMeiEscalation(content string) {
	fmt.Println("[Mei] Processing escalation...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	mei := o.agents.Mei()
	result, err := mei.Run(ctx, content)
	if err != nil {
		fmt.Printf("[Mei] Error: %v\n", err)
		return
	}

	// Log result
	if log, err := o.logMgr.Get("mei"); err == nil {
		log.LogSimple(content, result, 0)
	}

	fmt.Println("[Mei] Escalation handled")
	fmt.Println(result)
}

func containsApproved(s string) bool {
	return contains(s, "APPROVED") || contains(s, "Approved") || contains(s, "approved")
}

func containsTag(s, tag string) bool {
	return contains(s, "["+tag+"]")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsString(s, substr))
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
