package github

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/go-github/v60/github"
)

// EventType represents the type of GitHub event
type EventType string

const (
	EventTypeIssue EventType = "issue"
	EventTypePR    EventType = "pr"
)

// Event represents a new Issue or PR creation event
type Event struct {
	Type      EventType
	Number    int
	Title     string
	Author    string
	URL       string
	CreatedAt time.Time
	Labels    []string
	Body      string
}

// EventWatcher monitors new Issue/PR creations
type EventWatcher struct {
	client    *Client
	seenIDs   map[string]bool
	mu        sync.RWMutex
	lastCheck time.Time
}

// NewEventWatcher creates a new event watcher
func NewEventWatcher(client *Client) *EventWatcher {
	return &EventWatcher{
		client:    client,
		seenIDs:   make(map[string]bool),
		lastCheck: time.Now(),
	}
}

// NewEventWatcherWithSince creates an event watcher with custom start time
func NewEventWatcherWithSince(client *Client, since time.Time) *EventWatcher {
	return &EventWatcher{
		client:    client,
		seenIDs:   make(map[string]bool),
		lastCheck: since,
	}
}

// CheckEvents fetches new Issue/PR events since last check
func (w *EventWatcher) CheckEvents(ctx context.Context) ([]Event, error) {
	var events []Event

	// Fetch new issues
	issues, err := w.fetchNewIssues(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch issues: %w", err)
	}
	events = append(events, issues...)

	// Fetch new PRs
	prs, err := w.fetchNewPRs(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch PRs: %w", err)
	}
	events = append(events, prs...)

	// Update last check time
	w.lastCheck = time.Now()

	return events, nil
}

// fetchNewIssues fetches issues created since lastCheck
func (w *EventWatcher) fetchNewIssues(ctx context.Context) ([]Event, error) {
	opts := &github.IssueListByRepoOptions{
		State:     "all",
		Sort:      "created",
		Direction: "desc",
		Since:     w.lastCheck,
		ListOptions: github.ListOptions{
			PerPage: 30,
		},
	}

	issues, _, err := w.client.client.Issues.ListByRepo(ctx, w.client.owner, w.client.repo, opts)
	if err != nil {
		return nil, err
	}

	var events []Event
	for _, issue := range issues {
		// Skip PRs (GitHub API returns PRs as issues too)
		if issue.PullRequestLinks != nil {
			continue
		}

		id := fmt.Sprintf("issue-%d", issue.GetNumber())

		w.mu.RLock()
		seen := w.seenIDs[id]
		w.mu.RUnlock()

		if seen {
			continue
		}

		// Only include if created after lastCheck
		if issue.GetCreatedAt().Time.Before(w.lastCheck) {
			continue
		}

		w.mu.Lock()
		w.seenIDs[id] = true
		w.mu.Unlock()

		labels := make([]string, 0, len(issue.Labels))
		for _, l := range issue.Labels {
			labels = append(labels, l.GetName())
		}

		events = append(events, Event{
			Type:      EventTypeIssue,
			Number:    issue.GetNumber(),
			Title:     issue.GetTitle(),
			Author:    issue.GetUser().GetLogin(),
			URL:       issue.GetHTMLURL(),
			CreatedAt: issue.GetCreatedAt().Time,
			Labels:    labels,
			Body:      issue.GetBody(),
		})
	}

	return events, nil
}

// fetchNewPRs fetches PRs created since lastCheck
func (w *EventWatcher) fetchNewPRs(ctx context.Context) ([]Event, error) {
	opts := &github.PullRequestListOptions{
		State:     "all",
		Sort:      "created",
		Direction: "desc",
		ListOptions: github.ListOptions{
			PerPage: 30,
		},
	}

	prs, _, err := w.client.client.PullRequests.List(ctx, w.client.owner, w.client.repo, opts)
	if err != nil {
		return nil, err
	}

	var events []Event
	for _, pr := range prs {
		id := fmt.Sprintf("pr-%d", pr.GetNumber())

		w.mu.RLock()
		seen := w.seenIDs[id]
		w.mu.RUnlock()

		if seen {
			continue
		}

		// Only include if created after lastCheck
		if pr.GetCreatedAt().Time.Before(w.lastCheck) {
			continue
		}

		w.mu.Lock()
		w.seenIDs[id] = true
		w.mu.Unlock()

		labels := make([]string, 0, len(pr.Labels))
		for _, l := range pr.Labels {
			labels = append(labels, l.GetName())
		}

		events = append(events, Event{
			Type:      EventTypePR,
			Number:    pr.GetNumber(),
			Title:     pr.GetTitle(),
			Author:    pr.GetUser().GetLogin(),
			URL:       pr.GetHTMLURL(),
			CreatedAt: pr.GetCreatedAt().Time,
			Labels:    labels,
			Body:      pr.GetBody(),
		})
	}

	return events, nil
}

// FormatEventNotification formats an event for display
func FormatEventNotification(e Event) string {
	switch e.Type {
	case EventTypeIssue:
		return fmt.Sprintf("📝 New Issue #%d: %s (by @%s)\n   %s", e.Number, e.Title, e.Author, e.URL)
	case EventTypePR:
		return fmt.Sprintf("🔀 New PR #%d: %s (by @%s)\n   %s", e.Number, e.Title, e.Author, e.URL)
	default:
		return fmt.Sprintf("Event #%d: %s", e.Number, e.Title)
	}
}

// MarkAsSeen marks an event as seen (useful for initialization)
func (w *EventWatcher) MarkAsSeen(eventType EventType, number int) {
	id := fmt.Sprintf("%s-%d", eventType, number)
	w.mu.Lock()
	w.seenIDs[id] = true
	w.mu.Unlock()
}

// Reset clears all seen IDs and resets lastCheck
func (w *EventWatcher) Reset() {
	w.mu.Lock()
	w.seenIDs = make(map[string]bool)
	w.lastCheck = time.Now()
	w.mu.Unlock()
}
