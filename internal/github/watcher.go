package github

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v60/github"
)

// PREvent represents a PR event (merge or close)
type PREvent struct {
	Number     int
	Title      string
	Author     string
	Action     PRAction
	MergedAt   *time.Time
	ClosedAt   *time.Time
	URL        string
	MergedBy   string
	CreatedAt  time.Time
	HeadBranch string // source branch name (e.g., "feature/xxx")
}

// PRAction represents the type of PR event
type PRAction string

const (
	PRActionMerged PRAction = "merged"
	PRActionClosed PRAction = "closed" // closed without merge (rejected/abandoned)
)

// Watcher monitors PR events
type Watcher struct {
	client    *Client
	lastCheck time.Time
}

// NewWatcher creates a new PR event watcher
func NewWatcher(client *Client) *Watcher {
	return &Watcher{
		client:    client,
		lastCheck: time.Now().Add(-24 * time.Hour), // Default: check last 24 hours
	}
}

// NewWatcherWithSince creates a watcher with custom start time
func NewWatcherWithSince(client *Client, since time.Time) *Watcher {
	return &Watcher{
		client:    client,
		lastCheck: since,
	}
}

// CheckEvents fetches recent PR events since last check
func (w *Watcher) CheckEvents(ctx context.Context) ([]PREvent, error) {
	events, err := w.fetchPREvents(ctx, w.lastCheck)
	if err != nil {
		return nil, err
	}

	// Update last check time
	w.lastCheck = time.Now()

	return events, nil
}

// CheckEventsSince fetches PR events since a specific time
func (w *Watcher) CheckEventsSince(ctx context.Context, since time.Time) ([]PREvent, error) {
	return w.fetchPREvents(ctx, since)
}

// fetchPREvents fetches merged and closed PRs since the given time
func (w *Watcher) fetchPREvents(ctx context.Context, since time.Time) ([]PREvent, error) {
	opts := &github.PullRequestListOptions{
		State:     "closed",
		Sort:      "updated",
		Direction: "desc",
		ListOptions: github.ListOptions{
			PerPage: 30,
		},
	}

	prs, _, err := w.client.client.PullRequests.List(ctx, w.client.owner, w.client.repo, opts)
	if err != nil {
		return nil, fmt.Errorf("list closed PRs: %w", err)
	}

	var events []PREvent
	for _, pr := range prs {
		// Skip PRs that were closed before our threshold
		var eventTime time.Time
		if pr.MergedAt != nil {
			eventTime = pr.MergedAt.Time
		} else if pr.ClosedAt != nil {
			eventTime = pr.ClosedAt.Time
		} else {
			continue
		}

		if eventTime.Before(since) {
			continue
		}

		event := PREvent{
			Number:     pr.GetNumber(),
			Title:      pr.GetTitle(),
			Author:     pr.GetUser().GetLogin(),
			URL:        pr.GetHTMLURL(),
			CreatedAt:  pr.GetCreatedAt().Time,
			HeadBranch: pr.GetHead().GetRef(),
		}

		if pr.MergedAt != nil {
			event.Action = PRActionMerged
			mergedAt := pr.MergedAt.Time
			event.MergedAt = &mergedAt
			if pr.MergedBy != nil {
				event.MergedBy = pr.MergedBy.GetLogin()
			}
		} else {
			event.Action = PRActionClosed
			closedAt := pr.ClosedAt.Time
			event.ClosedAt = &closedAt
		}

		events = append(events, event)
	}

	return events, nil
}

// FormatEvent formats a PREvent for display
func FormatEvent(e PREvent) string {
	switch e.Action {
	case PRActionMerged:
		mergedBy := e.MergedBy
		if mergedBy == "" {
			mergedBy = "unknown"
		}
		return fmt.Sprintf("🟢 PR #%d merged: %s (by %s)", e.Number, e.Title, mergedBy)
	case PRActionClosed:
		return fmt.Sprintf("🔴 PR #%d closed without merge: %s", e.Number, e.Title)
	default:
		return fmt.Sprintf("PR #%d: %s (%s)", e.Number, e.Title, e.Action)
	}
}
