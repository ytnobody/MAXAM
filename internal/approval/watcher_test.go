package approval

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/go-github/v60/github"
)

// mockFetcher is a mock implementation of IssueCommentFetcher
type mockFetcher struct {
	mu       sync.Mutex
	comments map[int][]*github.IssueComment
}

func newMockFetcher() *mockFetcher {
	return &mockFetcher{
		comments: make(map[int][]*github.IssueComment),
	}
}

func (m *mockFetcher) SetComments(issueNumber int, comments []*github.IssueComment) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.comments[issueNumber] = comments
}

func (m *mockFetcher) ListIssueCommentsSince(ctx context.Context, number int, since time.Time) ([]*github.IssueComment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	allComments := m.comments[number]
	var filtered []*github.IssueComment
	for _, c := range allComments {
		if c.CreatedAt != nil && !c.CreatedAt.Time.Before(since) {
			filtered = append(filtered, c)
		}
	}
	return filtered, nil
}

func TestWatcher_WatchUnwatch(t *testing.T) {
	fetcher := newMockFetcher()
	w := NewWatcher(fetcher)

	// Initially not watching
	if w.IsWatching(123) {
		t.Error("Should not be watching issue 123 initially")
	}

	// Start watching
	w.Watch(123)
	if !w.IsWatching(123) {
		t.Error("Should be watching issue 123 after Watch()")
	}

	// Check watched issues
	issues := w.WatchedIssues()
	if len(issues) != 1 || issues[0] != 123 {
		t.Errorf("WatchedIssues() = %v, want [123]", issues)
	}

	// Stop watching
	w.Unwatch(123)
	if w.IsWatching(123) {
		t.Error("Should not be watching issue 123 after Unwatch()")
	}
}

func TestWatcher_CheckOnce_NoApproval(t *testing.T) {
	fetcher := newMockFetcher()
	w := NewWatcher(fetcher)

	now := time.Now()
	fetcher.SetComments(123, []*github.IssueComment{
		{
			Body:      ptrStr("This is just a question"),
			CreatedAt: &github.Timestamp{Time: now},
			User:      &github.User{Login: ptrStr("user1")},
		},
	})

	w.Watch(123)

	events, err := w.CheckOnce(context.Background())
	if err != nil {
		t.Fatalf("CheckOnce() error = %v", err)
	}

	if len(events) != 0 {
		t.Errorf("CheckOnce() returned %d events, want 0", len(events))
	}

	// Should still be watching
	if !w.IsWatching(123) {
		t.Error("Should still be watching issue 123")
	}
}

func TestWatcher_CheckOnce_WithApproval(t *testing.T) {
	fetcher := newMockFetcher()
	w := NewWatcher(fetcher)

	now := time.Now()
	fetcher.SetComments(123, []*github.IssueComment{
		{
			Body:      ptrStr("/approve"),
			CreatedAt: &github.Timestamp{Time: now.Add(1 * time.Second)},
			User:      &github.User{Login: ptrStr("owner")},
		},
	})

	w.Watch(123)

	events, err := w.CheckOnce(context.Background())
	if err != nil {
		t.Fatalf("CheckOnce() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("CheckOnce() returned %d events, want 1", len(events))
	}

	event := events[0]
	if event.IssueNumber != 123 {
		t.Errorf("IssueNumber = %d, want 123", event.IssueNumber)
	}
	if !event.Result.Approved {
		t.Error("Result.Approved = false, want true")
	}
	if event.Result.ApprovedBy != "owner" {
		t.Errorf("ApprovedBy = %q, want %q", event.Result.ApprovedBy, "owner")
	}

	// Should stop watching after approval
	if w.IsWatching(123) {
		t.Error("Should not be watching issue 123 after approval")
	}
}

func TestWatcher_CheckOnce_MultipleIssues(t *testing.T) {
	fetcher := newMockFetcher()
	w := NewWatcher(fetcher)

	now := time.Now()

	// Issue 100: Has approval
	fetcher.SetComments(100, []*github.IssueComment{
		{
			Body:      ptrStr("LGTM"),
			CreatedAt: &github.Timestamp{Time: now.Add(1 * time.Second)},
			User:      &github.User{Login: ptrStr("approver1")},
		},
	})

	// Issue 200: No approval
	fetcher.SetComments(200, []*github.IssueComment{
		{
			Body:      ptrStr("Still working on it"),
			CreatedAt: &github.Timestamp{Time: now.Add(1 * time.Second)},
			User:      &github.User{Login: ptrStr("dev")},
		},
	})

	// Issue 300: Has approval
	fetcher.SetComments(300, []*github.IssueComment{
		{
			Body:      ptrStr("承認"),
			CreatedAt: &github.Timestamp{Time: now.Add(1 * time.Second)},
			User:      &github.User{Login: ptrStr("approver2")},
		},
	})

	w.Watch(100)
	w.Watch(200)
	w.Watch(300)

	events, err := w.CheckOnce(context.Background())
	if err != nil {
		t.Fatalf("CheckOnce() error = %v", err)
	}

	if len(events) != 2 {
		t.Errorf("CheckOnce() returned %d events, want 2", len(events))
	}

	// Issue 200 should still be watched
	if !w.IsWatching(200) {
		t.Error("Should still be watching issue 200")
	}

	// Issues 100 and 300 should not be watched
	if w.IsWatching(100) {
		t.Error("Should not be watching issue 100")
	}
	if w.IsWatching(300) {
		t.Error("Should not be watching issue 300")
	}
}

func TestWatcher_Interval(t *testing.T) {
	fetcher := newMockFetcher()

	// Default config
	w1 := NewWatcher(fetcher)
	if w1.Interval() != 30*time.Second {
		t.Errorf("Default interval = %v, want 30s", w1.Interval())
	}

	// Custom config
	cfg := WatcherConfig{Interval: 1 * time.Minute}
	w2 := NewWatcherWithConfig(fetcher, cfg)
	if w2.Interval() != 1*time.Minute {
		t.Errorf("Custom interval = %v, want 1m", w2.Interval())
	}
}

func TestWatcher_CustomKeywords(t *testing.T) {
	fetcher := newMockFetcher()

	cfg := WatcherConfig{
		Keywords: []string{"SHIPIT", "🚀"},
	}
	w := NewWatcherWithConfig(fetcher, cfg)

	now := time.Now()

	// Default keyword should not match
	fetcher.SetComments(100, []*github.IssueComment{
		{
			Body:      ptrStr("LGTM"),
			CreatedAt: &github.Timestamp{Time: now.Add(1 * time.Second)},
			User:      &github.User{Login: ptrStr("user")},
		},
	})

	// Custom keyword should match
	fetcher.SetComments(200, []*github.IssueComment{
		{
			Body:      ptrStr("SHIPIT"),
			CreatedAt: &github.Timestamp{Time: now.Add(1 * time.Second)},
			User:      &github.User{Login: ptrStr("user")},
		},
	})

	w.Watch(100)
	w.Watch(200)

	events, err := w.CheckOnce(context.Background())
	if err != nil {
		t.Fatalf("CheckOnce() error = %v", err)
	}

	if len(events) != 1 {
		t.Errorf("CheckOnce() returned %d events, want 1", len(events))
	}

	if len(events) > 0 && events[0].IssueNumber != 200 {
		t.Errorf("IssueNumber = %d, want 200", events[0].IssueNumber)
	}
}

// ptrStr is a helper to create string pointers
func ptrStr(s string) *string {
	return &s
}
