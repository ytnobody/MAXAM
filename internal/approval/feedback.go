// Package approval manages approval requests and detection
package approval

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v60/github"
)

// CommentType represents the type of comment
type CommentType string

const (
	// CommentTypeApproval indicates an approval comment (LGTM, OK, etc.)
	CommentTypeApproval CommentType = "approval"
	// CommentTypeFeedback indicates feedback that requires plan updates
	CommentTypeFeedback CommentType = "feedback"
	// CommentTypeQuestion indicates a question from the owner
	CommentTypeQuestion CommentType = "question"
	// CommentTypeUnknown indicates an unclassified comment
	CommentTypeUnknown CommentType = "unknown"
)

// FeedbackKeywords defines keywords that indicate feedback
var FeedbackKeywords = []string{
	"修正",
	"変更",
	"追加",
	"削除",
	"更新",
	"fix",
	"change",
	"add",
	"remove",
	"update",
	"modify",
	"revise",
}

// QuestionPatterns defines patterns that indicate questions
var QuestionPatterns = []string{
	`\?$`,
	`？$`,
	`^なぜ`,
	`^どう`,
	`^何`,
	`^いつ`,
	`^誰`,
	`^どこ`,
	`^why\b`,
	`^how\b`,
	`^what\b`,
	`^when\b`,
	`^who\b`,
	`^where\b`,
	`^can you`,
	`^could you`,
	`^would you`,
}

// FeedbackComment represents a classified comment
type FeedbackComment struct {
	Comment     *github.IssueComment
	Type        CommentType
	Body        string
	CommentedAt time.Time
	CommentedBy string
}

// FeedbackChecker classifies comments
type FeedbackChecker struct {
	approvalChecker  *CommentChecker
	feedbackKeywords []string
	questionPatterns []*regexp.Regexp
}

// NewFeedbackChecker creates a new FeedbackChecker
func NewFeedbackChecker() *FeedbackChecker {
	patterns := make([]*regexp.Regexp, 0, len(QuestionPatterns))
	for _, p := range QuestionPatterns {
		patterns = append(patterns, regexp.MustCompile(`(?i)`+p))
	}

	return &FeedbackChecker{
		approvalChecker:  NewCommentChecker(),
		feedbackKeywords: FeedbackKeywords,
		questionPatterns: patterns,
	}
}

// ClassifyComment determines the type of a comment
func (fc *FeedbackChecker) ClassifyComment(body string) CommentType {
	body = strings.TrimSpace(body)
	if body == "" {
		return CommentTypeUnknown
	}

	// Check for approval first
	if fc.approvalChecker.IsApprovalComment(body) {
		return CommentTypeApproval
	}

	// Check for question patterns
	for _, pattern := range fc.questionPatterns {
		if pattern.MatchString(body) {
			return CommentTypeQuestion
		}
	}

	// Check for feedback keywords
	lowerBody := strings.ToLower(body)
	for _, kw := range fc.feedbackKeywords {
		if strings.Contains(lowerBody, strings.ToLower(kw)) {
			return CommentTypeFeedback
		}
	}

	return CommentTypeUnknown
}

// ClassifyComments classifies multiple comments
func (c *FeedbackChecker) ClassifyComments(comments []*github.IssueComment) []*FeedbackComment {
	result := make([]*FeedbackComment, 0, len(comments))

	for _, comment := range comments {
		if comment.Body == nil {
			continue
		}

		body := *comment.Body
		fc := &FeedbackComment{
			Comment: comment,
			Type:    c.ClassifyComment(body),
			Body:    body,
		}

		if comment.CreatedAt != nil {
			fc.CommentedAt = comment.CreatedAt.Time
		}
		if comment.User != nil && comment.User.Login != nil {
			fc.CommentedBy = *comment.User.Login
		}

		result = append(result, fc)
	}

	return result
}

// FeedbackEvent represents a feedback event detected
type FeedbackEvent struct {
	IssueNumber int
	Comment     *FeedbackComment
}

// FeedbackWatcher monitors issue comments for feedback
type FeedbackWatcher struct {
	mu       sync.RWMutex
	client   IssueCommentFetcher
	checker  *FeedbackChecker
	issues   map[int]*WatchedIssue
	interval time.Duration
}

// FeedbackWatcherConfig holds configuration for the FeedbackWatcher
type FeedbackWatcherConfig struct {
	Interval time.Duration
}

// DefaultFeedbackWatcherConfig returns the default configuration
func DefaultFeedbackWatcherConfig() FeedbackWatcherConfig {
	return FeedbackWatcherConfig{
		Interval: 30 * time.Second,
	}
}

// NewFeedbackWatcher creates a new feedback watcher
func NewFeedbackWatcher(client IssueCommentFetcher) *FeedbackWatcher {
	return NewFeedbackWatcherWithConfig(client, DefaultFeedbackWatcherConfig())
}

// NewFeedbackWatcherWithConfig creates a watcher with custom configuration
func NewFeedbackWatcherWithConfig(client IssueCommentFetcher, cfg FeedbackWatcherConfig) *FeedbackWatcher {
	if cfg.Interval == 0 {
		cfg.Interval = 30 * time.Second
	}

	return &FeedbackWatcher{
		client:   client,
		checker:  NewFeedbackChecker(),
		issues:   make(map[int]*WatchedIssue),
		interval: cfg.Interval,
	}
}

// Watch starts watching an issue for feedback comments
func (w *FeedbackWatcher) Watch(issueNumber int) {
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
func (w *FeedbackWatcher) Unwatch(issueNumber int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.issues, issueNumber)
}

// IsWatching returns true if the issue is being watched
func (w *FeedbackWatcher) IsWatching(issueNumber int) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	_, exists := w.issues[issueNumber]
	return exists
}

// CheckOnce checks all watched issues for feedback once
func (w *FeedbackWatcher) CheckOnce(ctx context.Context) ([]FeedbackEvent, error) {
	w.mu.RLock()
	issues := make(map[int]*WatchedIssue, len(w.issues))
	for k, v := range w.issues {
		issues[k] = v
	}
	w.mu.RUnlock()

	var events []FeedbackEvent

	for _, issue := range issues {
		comments, err := w.client.ListIssueCommentsSince(ctx, issue.Number, issue.WatchedAt)
		if err != nil {
			continue
		}

		for _, comment := range comments {
			if comment.Body == nil {
				continue
			}
			if comment.CreatedAt != nil && comment.CreatedAt.Time.Before(issue.WatchedAt) {
				continue
			}

			fc := &FeedbackComment{
				Comment: comment,
				Type:    w.checker.ClassifyComment(*comment.Body),
				Body:    *comment.Body,
			}

			if comment.CreatedAt != nil {
				fc.CommentedAt = comment.CreatedAt.Time
			}
			if comment.User != nil && comment.User.Login != nil {
				fc.CommentedBy = *comment.User.Login
			}

			// Only emit events for non-unknown comments
			if fc.Type != CommentTypeUnknown {
				events = append(events, FeedbackEvent{
					IssueNumber: issue.Number,
					Comment:     fc,
				})
			}
		}

		// Update watched time to avoid re-processing
		w.mu.Lock()
		if wi, exists := w.issues[issue.Number]; exists {
			wi.WatchedAt = time.Now()
		}
		w.mu.Unlock()
	}

	return events, nil
}

// Start starts the polling loop in a goroutine
func (w *FeedbackWatcher) Start(ctx context.Context) <-chan FeedbackEvent {
	eventCh := make(chan FeedbackEvent, 10)

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
func (w *FeedbackWatcher) Interval() time.Duration {
	return w.interval
}
