// Package taskstatus provides task status tracking from GitHub PRs
package taskstatus

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v60/github"
)

// Status represents a member's current task status
type Status struct {
	Member    string
	PRNumber  int
	PRTitle   string
	State     string // "open", "draft", "review_requested"
	UpdatedAt time.Time
}

// Fetcher fetches task status from GitHub PRs
type Fetcher struct {
	client *github.Client
	owner  string
	repo   string

	mu       sync.RWMutex
	statuses []Status
}

// NewFetcher creates a new task status fetcher
func NewFetcher(client *github.Client, owner, repo string) *Fetcher {
	return &Fetcher{
		client: client,
		owner:  owner,
		repo:   repo,
	}
}

// Fetch fetches current task status from GitHub PRs
func (f *Fetcher) Fetch(ctx context.Context) ([]Status, error) {
	opts := &github.PullRequestListOptions{
		State: "open",
		ListOptions: github.ListOptions{
			PerPage: 30,
		},
	}

	prs, _, err := f.client.PullRequests.List(ctx, f.owner, f.repo, opts)
	if err != nil {
		return nil, fmt.Errorf("list PRs: %w", err)
	}

	statuses := make([]Status, 0)
	for _, pr := range prs {
		// Skip PRs without assignees
		if len(pr.Assignees) == 0 && pr.User == nil {
			continue
		}

		// Get assignee or author
		var member string
		if len(pr.Assignees) > 0 {
			member = pr.Assignees[0].GetLogin()
		} else if pr.User != nil {
			member = pr.User.GetLogin()
		}

		// Determine state
		state := "open"
		if pr.GetDraft() {
			state = "draft"
		} else if pr.RequestedReviewers != nil && len(pr.RequestedReviewers) > 0 {
			state = "review_requested"
		}

		statuses = append(statuses, Status{
			Member:    member,
			PRNumber:  pr.GetNumber(),
			PRTitle:   truncateTitle(pr.GetTitle(), 30),
			State:     state,
			UpdatedAt: pr.GetUpdatedAt().Time,
		})
	}

	// Cache the result
	f.mu.Lock()
	f.statuses = statuses
	f.mu.Unlock()

	return statuses, nil
}

// GetCached returns cached statuses
func (f *Fetcher) GetCached() []Status {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.statuses
}

// FormatStatusLine formats statuses for status bar display
func FormatStatusLine(statuses []Status, memberNames map[string]string) string {
	if len(statuses) == 0 {
		return ""
	}

	// Group by member
	memberPRs := make(map[string][]Status)
	for _, s := range statuses {
		memberPRs[s.Member] = append(memberPRs[s.Member], s)
	}

	var parts []string
	for member, prs := range memberPRs {
		displayName := member
		if name, ok := memberNames[member]; ok {
			displayName = name
		}

		// Show first PR for each member
		if len(prs) > 0 {
			pr := prs[0]
			stateIcon := getStateIcon(pr.State)
			parts = append(parts, fmt.Sprintf("%s: #%d%s", displayName, pr.PRNumber, stateIcon))
		}
	}

	return strings.Join(parts, " | ")
}

func getStateIcon(state string) string {
	switch state {
	case "draft":
		return " (draft)"
	case "review_requested":
		return " (review)"
	default:
		return ""
	}
}

func truncateTitle(title string, maxLen int) string {
	if len(title) <= maxLen {
		return title
	}
	return title[:maxLen-3] + "..."
}
