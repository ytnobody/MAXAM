package github

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/go-github/v60/github"
)

// CIStatus represents the status of CI checks
type CIStatus string

const (
	CIStatusPending   CIStatus = "pending"
	CIStatusRunning   CIStatus = "running"
	CIStatusSuccess   CIStatus = "success"
	CIStatusFailure   CIStatus = "failure"
	CIStatusCancelled CIStatus = "cancelled"
)

// CIResult represents the result of CI check monitoring
type CIResult struct {
	PRNumber    int
	PRTitle     string
	PRURL       string
	PRAuthor    string
	Status      CIStatus
	TotalChecks int
	Passed      int
	Failed      int
	Pending     int
	Details     []CheckDetail
	CompletedAt time.Time
}

// CheckDetail represents details of a single check
type CheckDetail struct {
	Name       string
	Status     CIStatus
	Conclusion string
	URL        string
}

// CIWatcherConfig holds configuration for CI watcher
type CIWatcherConfig struct {
	// PollInterval is the interval between polling
	PollInterval time.Duration
	// Timeout is the maximum time to wait for CI completion
	Timeout time.Duration
}

// DefaultCIWatcherConfig returns default configuration
func DefaultCIWatcherConfig() CIWatcherConfig {
	return CIWatcherConfig{
		PollInterval: 30 * time.Second,
		Timeout:      24 * time.Hour,
	}
}

// CIWatcher monitors CI status for PRs
type CIWatcher struct {
	client   *Client
	config   CIWatcherConfig
	watching map[int]*watchEntry
	mu       sync.RWMutex
	notifyCh chan CIResult
}

type watchEntry struct {
	prNumber  int
	startTime time.Time
	cancel    context.CancelFunc
}

// NewCIWatcher creates a new CI watcher
func NewCIWatcher(client *Client, config CIWatcherConfig) *CIWatcher {
	return &CIWatcher{
		client:   client,
		config:   config,
		watching: make(map[int]*watchEntry),
		notifyCh: make(chan CIResult, 100),
	}
}

// NotifyCh returns the notification channel
func (w *CIWatcher) NotifyCh() <-chan CIResult {
	return w.notifyCh
}

// WatchPR starts watching CI status for a PR
func (w *CIWatcher) WatchPR(ctx context.Context, prNumber int) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Already watching
	if _, ok := w.watching[prNumber]; ok {
		return nil
	}

	watchCtx, cancel := context.WithCancel(ctx)
	entry := &watchEntry{
		prNumber:  prNumber,
		startTime: time.Now(),
		cancel:    cancel,
	}
	w.watching[prNumber] = entry

	go w.watchLoop(watchCtx, entry)
	return nil
}

// StopWatching stops watching a PR
func (w *CIWatcher) StopWatching(prNumber int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if entry, ok := w.watching[prNumber]; ok {
		entry.cancel()
		delete(w.watching, prNumber)
	}
}

// IsWatching returns true if the PR is being watched
func (w *CIWatcher) IsWatching(prNumber int) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	_, ok := w.watching[prNumber]
	return ok
}

// WatchingPRs returns the list of PRs being watched
func (w *CIWatcher) WatchingPRs() []int {
	w.mu.RLock()
	defer w.mu.RUnlock()

	prs := make([]int, 0, len(w.watching))
	for prNum := range w.watching {
		prs = append(prs, prNum)
	}
	return prs
}

// GetCIStatus gets the current CI status for a PR
func (w *CIWatcher) GetCIStatus(ctx context.Context, prNumber int) (*CIResult, error) {
	pr, err := w.client.GetPR(ctx, prNumber)
	if err != nil {
		return nil, fmt.Errorf("get PR: %w", err)
	}

	return w.checkCIStatus(ctx, pr)
}

func (w *CIWatcher) watchLoop(ctx context.Context, entry *watchEntry) {
	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()

	timeout := time.After(w.config.Timeout)

	for {
		select {
		case <-ctx.Done():
			return
		case <-timeout:
			// Timeout reached, send timeout result
			w.sendTimeoutResult(entry.prNumber)
			w.StopWatching(entry.prNumber)
			return
		case <-ticker.C:
			pr, err := w.client.GetPR(ctx, entry.prNumber)
			if err != nil {
				continue // Retry on error
			}

			result, err := w.checkCIStatus(ctx, pr)
			if err != nil {
				continue // Retry on error
			}

			// Check if CI is complete
			if result.Status == CIStatusSuccess || result.Status == CIStatusFailure || result.Status == CIStatusCancelled {
				w.notifyCh <- *result
				w.StopWatching(entry.prNumber)
				return
			}
		}
	}
}

func (w *CIWatcher) checkCIStatus(ctx context.Context, pr *github.PullRequest) (*CIResult, error) {
	ref := pr.GetHead().GetSHA()

	// Get combined status (for status checks)
	combinedStatus, _, err := w.client.client.Repositories.GetCombinedStatus(ctx, w.client.owner, w.client.repo, ref, nil)
	if err != nil {
		return nil, fmt.Errorf("get combined status: %w", err)
	}

	// Get check runs (for GitHub Actions)
	checkRuns, _, err := w.client.client.Checks.ListCheckRunsForRef(ctx, w.client.owner, w.client.repo, ref, nil)
	if err != nil {
		return nil, fmt.Errorf("list check runs: %w", err)
	}

	result := &CIResult{
		PRNumber: pr.GetNumber(),
		PRTitle:  pr.GetTitle(),
		PRURL:    pr.GetHTMLURL(),
		PRAuthor: pr.GetUser().GetLogin(),
		Details:  make([]CheckDetail, 0),
	}

	// Process status checks
	for _, status := range combinedStatus.Statuses {
		detail := CheckDetail{
			Name:   status.GetContext(),
			URL:    status.GetTargetURL(),
			Status: mapStatusState(status.GetState()),
		}
		result.Details = append(result.Details, detail)

		switch detail.Status {
		case CIStatusSuccess:
			result.Passed++
		case CIStatusFailure:
			result.Failed++
		default:
			result.Pending++
		}
	}

	// Process check runs
	for _, run := range checkRuns.CheckRuns {
		detail := CheckDetail{
			Name:       run.GetName(),
			URL:        run.GetHTMLURL(),
			Status:     mapCheckRunStatus(run.GetStatus(), run.GetConclusion()),
			Conclusion: run.GetConclusion(),
		}
		result.Details = append(result.Details, detail)

		switch detail.Status {
		case CIStatusSuccess:
			result.Passed++
		case CIStatusFailure, CIStatusCancelled:
			result.Failed++
		default:
			result.Pending++
		}
	}

	result.TotalChecks = len(result.Details)

	// Determine overall status
	if result.TotalChecks == 0 {
		result.Status = CIStatusPending
	} else if result.Pending > 0 {
		result.Status = CIStatusRunning
	} else if result.Failed > 0 {
		result.Status = CIStatusFailure
	} else {
		result.Status = CIStatusSuccess
		result.CompletedAt = time.Now()
	}

	return result, nil
}

func (w *CIWatcher) sendTimeoutResult(prNumber int) {
	w.notifyCh <- CIResult{
		PRNumber: prNumber,
		Status:   CIStatusCancelled,
	}
}

func mapStatusState(state string) CIStatus {
	switch state {
	case "success":
		return CIStatusSuccess
	case "failure", "error":
		return CIStatusFailure
	case "pending":
		return CIStatusPending
	default:
		return CIStatusPending
	}
}

func mapCheckRunStatus(status, conclusion string) CIStatus {
	if status != "completed" {
		if status == "in_progress" {
			return CIStatusRunning
		}
		return CIStatusPending
	}

	switch conclusion {
	case "success":
		return CIStatusSuccess
	case "failure":
		return CIStatusFailure
	case "cancelled", "skipped":
		return CIStatusCancelled
	case "neutral":
		return CIStatusSuccess // neutral is treated as success
	default:
		return CIStatusFailure
	}
}

// FormatCIResult formats a CI result for display
func FormatCIResult(r CIResult) string {
	var icon string
	switch r.Status {
	case CIStatusSuccess:
		icon = "✅"
	case CIStatusFailure:
		icon = "❌"
	case CIStatusRunning:
		icon = "🔄"
	case CIStatusCancelled:
		icon = "⏹️"
	default:
		icon = "⏳"
	}

	return fmt.Sprintf("%s CI %s for PR #%d: %s (%d/%d passed)\n   %s",
		icon, r.Status, r.PRNumber, r.PRTitle, r.Passed, r.TotalChecks, r.PRURL)
}
