// Package issuemgr provides issue monitoring functionality.
package issuemgr

import (
	"context"
	"sync"
	"time"
)

// IssueChecker interface for fetching open issues.
type IssueChecker interface {
	// CountOpenIssues returns the number of open issues.
	CountOpenIssues(ctx context.Context) (int, error)
}

// NotifyFunc is called when there are unattended open issues.
type NotifyFunc func(ctx context.Context, count int)

// Watcher monitors open issues and notifies when there are unattended issues.
type Watcher struct {
	mu       sync.RWMutex
	checker  IssueChecker
	interval time.Duration
	onNotify NotifyFunc
	running  bool
	stopCh   chan struct{}
}

// WatcherOption is a functional option for Watcher.
type WatcherOption func(*Watcher)

// DefaultInterval is the default monitoring interval (3 minutes).
const DefaultInterval = 3 * time.Minute

// NewWatcher creates a new issue watcher.
func NewWatcher(checker IssueChecker, opts ...WatcherOption) *Watcher {
	w := &Watcher{
		checker:  checker,
		interval: DefaultInterval,
	}

	for _, opt := range opts {
		opt(w)
	}

	return w
}

// WithInterval sets the monitoring interval.
func WithInterval(d time.Duration) WatcherOption {
	return func(w *Watcher) {
		w.interval = d
	}
}

// WithNotifyFunc sets the notification callback.
func WithNotifyFunc(fn NotifyFunc) WatcherOption {
	return func(w *Watcher) {
		w.onNotify = fn
	}
}

// Start begins the issue monitoring loop.
func (w *Watcher) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return nil
	}
	w.running = true
	w.stopCh = make(chan struct{})
	w.mu.Unlock()

	go w.run(ctx)
	return nil
}

// Stop stops the issue monitoring loop.
func (w *Watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	close(w.stopCh)
	w.running = false
}

// IsRunning returns whether the watcher is running.
func (w *Watcher) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

func (w *Watcher) run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Run initial check immediately.
	w.check(ctx)

	for {
		select {
		case <-ctx.Done():
			w.mu.Lock()
			w.running = false
			w.mu.Unlock()
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.check(ctx)
		}
	}
}

func (w *Watcher) check(ctx context.Context) {
	count, err := w.checker.CountOpenIssues(ctx)
	if err != nil {
		// Log error but continue; best effort.
		return
	}

	// Only notify if there are open issues.
	if count > 0 && w.onNotify != nil {
		w.onNotify(ctx, count)
	}
}

// CheckNow triggers an immediate check.
func (w *Watcher) CheckNow(ctx context.Context) {
	w.check(ctx)
}
