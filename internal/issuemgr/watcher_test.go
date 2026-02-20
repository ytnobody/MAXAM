package issuemgr

import (
	"context"
	"sync"
	"testing"
	"time"
)

// mockChecker implements IssueChecker for testing.
type mockChecker struct {
	mu    sync.Mutex
	count int
	err   error
}

func (m *mockChecker) CountOpenIssues(_ context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.count, m.err
}

func (m *mockChecker) setCount(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.count = n
}

func TestNewWatcher(t *testing.T) {
	checker := &mockChecker{}
	w := NewWatcher(checker)

	if w.checker != checker {
		t.Error("checker not set correctly")
	}
	if w.interval != DefaultInterval {
		t.Errorf("expected interval %v, got %v", DefaultInterval, w.interval)
	}
}

func TestWatcherWithOptions(t *testing.T) {
	checker := &mockChecker{}
	customInterval := 5 * time.Minute
	notifyCalled := false

	w := NewWatcher(checker,
		WithInterval(customInterval),
		WithNotifyFunc(func(_ context.Context, _ int) {
			notifyCalled = true
		}),
	)

	if w.interval != customInterval {
		t.Errorf("expected interval %v, got %v", customInterval, w.interval)
	}

	// Trigger notification manually.
	checker.setCount(1)
	w.check(context.Background())

	if !notifyCalled {
		t.Error("notify function not called")
	}
}

func TestWatcherNoNotifyOnZeroIssues(t *testing.T) {
	checker := &mockChecker{count: 0}
	notifyCalled := false

	w := NewWatcher(checker,
		WithNotifyFunc(func(_ context.Context, _ int) {
			notifyCalled = true
		}),
	)

	w.check(context.Background())

	if notifyCalled {
		t.Error("notify should not be called when no open issues")
	}
}

func TestWatcherStartStop(t *testing.T) {
	checker := &mockChecker{}
	w := NewWatcher(checker, WithInterval(100*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if w.IsRunning() {
		t.Error("watcher should not be running before Start")
	}

	err := w.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !w.IsRunning() {
		t.Error("watcher should be running after Start")
	}

	// Start again should be no-op.
	err = w.Start(ctx)
	if err != nil {
		t.Fatalf("second Start failed: %v", err)
	}

	w.Stop()

	// Give goroutine time to stop.
	time.Sleep(50 * time.Millisecond)

	if w.IsRunning() {
		t.Error("watcher should not be running after Stop")
	}
}

func TestWatcherContextCancel(t *testing.T) {
	checker := &mockChecker{}
	w := NewWatcher(checker, WithInterval(100*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())

	err := w.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Cancel context.
	cancel()

	// Give goroutine time to handle cancellation.
	time.Sleep(50 * time.Millisecond)

	if w.IsRunning() {
		t.Error("watcher should stop when context is cancelled")
	}
}

func TestWatcherNotifiesWithCorrectCount(t *testing.T) {
	checker := &mockChecker{count: 5}
	var receivedCount int
	var mu sync.Mutex

	w := NewWatcher(checker,
		WithNotifyFunc(func(_ context.Context, count int) {
			mu.Lock()
			receivedCount = count
			mu.Unlock()
		}),
	)

	w.check(context.Background())

	mu.Lock()
	if receivedCount != 5 {
		t.Errorf("expected count 5, got %d", receivedCount)
	}
	mu.Unlock()
}

func TestWatcherPeriodicCheck(t *testing.T) {
	checker := &mockChecker{count: 1}
	checkCount := 0
	var mu sync.Mutex

	w := NewWatcher(checker,
		WithInterval(50*time.Millisecond),
		WithNotifyFunc(func(_ context.Context, _ int) {
			mu.Lock()
			checkCount++
			mu.Unlock()
		}),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := w.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for a few intervals.
	time.Sleep(130 * time.Millisecond)

	w.Stop()

	mu.Lock()
	// Should have at least initial check + 2 periodic checks.
	if checkCount < 3 {
		t.Errorf("expected at least 3 checks, got %d", checkCount)
	}
	mu.Unlock()
}

func TestWatcherCheckNow(t *testing.T) {
	checker := &mockChecker{count: 2}
	var receivedCount int
	var mu sync.Mutex

	w := NewWatcher(checker,
		WithInterval(1*time.Hour), // Long interval to ensure no periodic check.
		WithNotifyFunc(func(_ context.Context, count int) {
			mu.Lock()
			receivedCount = count
			mu.Unlock()
		}),
	)

	// Manual check without starting.
	w.CheckNow(context.Background())

	mu.Lock()
	if receivedCount != 2 {
		t.Errorf("expected count 2, got %d", receivedCount)
	}
	mu.Unlock()
}
