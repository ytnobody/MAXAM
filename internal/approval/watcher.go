// Package approval manages approval requests and detection
package approval

import (
	"context"
	"sync"
	"time"

	"github.com/google/go-github/v60/github"
)

// IssueCommentFetcher is an interface for fetching issue comments
type IssueCommentFetcher interface {
	ListIssueCommentsSince(ctx context.Context, number int, since time.Time) ([]*github.IssueComment, error)
}

// WatchedIssue represents an issue being watched for approval
type WatchedIssue struct {
	Number    int
	WatchedAt time.Time
}

// Watcher monitors issue comments for approval
type Watcher struct {
	mu       sync.RWMutex
	client   IssueCommentFetcher
	checker  *CommentChecker
	issues   map[int]*WatchedIssue
	interval time.Duration
}

// WatcherConfig holds configuration for the Watcher
type WatcherConfig struct {
	Interval time.Duration
	Keywords []string // Custom keywords; if empty, uses defaults
}

// DefaultWatcherConfig returns the default watcher configuration
func DefaultWatcherConfig() WatcherConfig {
	return WatcherConfig{
		Interval: 30 * time.Second,
	}
}

// NewWatcher creates a new comment watcher
func NewWatcher(client IssueCommentFetcher) *Watcher {
	return NewWatcherWithConfig(client, DefaultWatcherConfig())
}

// NewWatcherWithConfig creates a watcher with custom configuration
func NewWatcherWithConfig(client IssueCommentFetcher, cfg WatcherConfig) *Watcher {
	var checker *CommentChecker
	if len(cfg.Keywords) > 0 {
		checker = NewCommentCheckerWithKeywords(cfg.Keywords)
	} else {
		checker = NewCommentChecker()
	}

	if cfg.Interval == 0 {
		cfg.Interval = 30 * time.Second
	}

	return &Watcher{
		client:   client,
		checker:  checker,
		issues:   make(map[int]*WatchedIssue),
		interval: cfg.Interval,
	}
}

// Watch starts watching an issue for approval comments
func (w *Watcher) Watch(issueNumber int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, exists := w.issues[issueNumber]; !exists {
		w.issues[issueNumber] = &WatchedIssue{
			Number:    issueNumber,
			WatchedAt: time.Now(),
		}
	}
}

// Unwatch stops watching an issue
func (w *Watcher) Unwatch(issueNumber int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.issues, issueNumber)
}

// IsWatching returns true if the issue is being watched
func (w *Watcher) IsWatching(issueNumber int) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	_, exists := w.issues[issueNumber]
	return exists
}

// WatchedIssues returns a copy of all watched issues
func (w *Watcher) WatchedIssues() []int {
	w.mu.RLock()
	defer w.mu.RUnlock()

	issues := make([]int, 0, len(w.issues))
	for num := range w.issues {
		issues = append(issues, num)
	}
	return issues
}

// ApprovalEvent represents an approval event detected
type ApprovalEvent struct {
	IssueNumber int
	Result      *ApprovalResult
}

// CheckOnce checks all watched issues for approval once
func (w *Watcher) CheckOnce(ctx context.Context) ([]ApprovalEvent, error) {
	w.mu.RLock()
	issues := make(map[int]*WatchedIssue, len(w.issues))
	for k, v := range w.issues {
		issues[k] = v
	}
	w.mu.RUnlock()

	var events []ApprovalEvent

	for _, issue := range issues {
		comments, err := w.client.ListIssueCommentsSince(ctx, issue.Number, issue.WatchedAt)
		if err != nil {
			// Log error but continue checking other issues
			continue
		}

		result := w.checker.FindApprovalSince(comments, issue.WatchedAt)
		if result.Approved {
			events = append(events, ApprovalEvent{
				IssueNumber: issue.Number,
				Result:      result,
			})

			// Stop watching this issue once approved
			w.Unwatch(issue.Number)
		}
	}

	return events, nil
}

// Start starts the polling loop in a goroutine
// Returns a channel that receives approval events
func (w *Watcher) Start(ctx context.Context) <-chan ApprovalEvent {
	eventCh := make(chan ApprovalEvent, 10)

	go func() {
		defer close(eventCh)

		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				events, err := w.CheckOnce(ctx)
				if err != nil {
					continue
				}

				for _, event := range events {
					select {
					case eventCh <- event:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	return eventCh
}

// Interval returns the current polling interval
func (w *Watcher) Interval() time.Duration {
	return w.interval
}
