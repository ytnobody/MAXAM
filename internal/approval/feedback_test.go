package approval

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-github/v60/github"
)

func TestFeedbackChecker_ClassifyComment(t *testing.T) {
	checker := NewFeedbackChecker()

	tests := []struct {
		name     string
		body     string
		expected CommentType
	}{
		// Approval cases (should be detected as approval)
		{"LGTM", "LGTM", CommentTypeApproval},
		{"OK", "OK", CommentTypeApproval},
		{"承認", "承認", CommentTypeApproval},
		{"thumbs up", "👍", CommentTypeApproval},

		// Question cases
		{"question mark English", "What does this do?", CommentTypeQuestion},
		{"question mark Japanese", "これは何ですか？", CommentTypeQuestion},
		{"why question", "Why did you use this approach?", CommentTypeQuestion},
		{"how question", "How does this work?", CommentTypeQuestion},
		{"what question", "What is the expected behavior?", CommentTypeQuestion},
		{"can you question", "Can you explain this?", CommentTypeQuestion},
		{"could you question", "Could you add a test?", CommentTypeQuestion},
		{"Japanese why", "なぜこうしたの？", CommentTypeQuestion},
		{"Japanese what", "何が問題？", CommentTypeQuestion},

		// Feedback cases
		{"fix request", "Please fix the typo", CommentTypeFeedback},
		{"change request", "Change this to use a different approach", CommentTypeFeedback},
		{"add request", "Add error handling here", CommentTypeFeedback},
		{"remove request", "Remove this unused code", CommentTypeFeedback},
		{"update request", "Update the documentation", CommentTypeFeedback},
		{"modify request", "Modify the function signature", CommentTypeFeedback},
		{"Japanese fix", "修正してください", CommentTypeFeedback},
		{"Japanese change", "変更が必要です", CommentTypeFeedback},
		{"Japanese add", "追加してください", CommentTypeFeedback},
		{"Japanese remove", "削除してください", CommentTypeFeedback},
		{"Japanese update", "更新してください", CommentTypeFeedback},

		// Unknown cases
		{"thanks", "Thanks for the explanation", CommentTypeUnknown},
		{"looks good no approval", "Looks good", CommentTypeUnknown},
		{"empty", "", CommentTypeUnknown},
		{"whitespace", "   ", CommentTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.ClassifyComment(tt.body)
			if result != tt.expected {
				t.Errorf("ClassifyComment(%q) = %v, want %v", tt.body, result, tt.expected)
			}
		})
	}
}

func TestFeedbackChecker_ClassifyComments(t *testing.T) {
	checker := NewFeedbackChecker()

	now := time.Now()
	user := &github.User{Login: ptr("testuser")}

	comments := []*github.IssueComment{
		{
			Body:      ptr("LGTM"),
			CreatedAt: &github.Timestamp{Time: now},
			User:      user,
		},
		{
			Body:      ptr("What does this do?"),
			CreatedAt: &github.Timestamp{Time: now},
			User:      user,
		},
		{
			Body:      ptr("Please fix the typo"),
			CreatedAt: &github.Timestamp{Time: now},
			User:      user,
		},
		{
			Body:      ptr("Thanks!"),
			CreatedAt: &github.Timestamp{Time: now},
			User:      user,
		},
		{
			Body:      nil,
			CreatedAt: &github.Timestamp{Time: now},
			User:      user,
		},
	}

	result := checker.ClassifyComments(comments)

	// Should have 4 results (nil body is skipped)
	if len(result) != 4 {
		t.Errorf("expected 4 results, got %d", len(result))
	}

	expectedTypes := []CommentType{
		CommentTypeApproval,
		CommentTypeQuestion,
		CommentTypeFeedback,
		CommentTypeUnknown,
	}

	for i, fc := range result {
		if fc.Type != expectedTypes[i] {
			t.Errorf("result[%d].Type = %v, want %v", i, fc.Type, expectedTypes[i])
		}
		if fc.CommentedBy != "testuser" {
			t.Errorf("result[%d].CommentedBy = %q, want %q", i, fc.CommentedBy, "testuser")
		}
	}
}

// mockCommentFetcher implements IssueCommentFetcher for testing
type mockCommentFetcher struct {
	comments map[int][]*github.IssueComment
}

func (m *mockCommentFetcher) ListIssueCommentsSince(_ context.Context, number int, since time.Time) ([]*github.IssueComment, error) {
	all := m.comments[number]
	var filtered []*github.IssueComment
	for _, c := range all {
		if c.CreatedAt != nil && !c.CreatedAt.Time.Before(since) {
			filtered = append(filtered, c)
		}
	}
	return filtered, nil
}

func TestFeedbackWatcher_Watch(t *testing.T) {
	fetcher := &mockCommentFetcher{comments: make(map[int][]*github.IssueComment)}
	watcher := NewFeedbackWatcher(fetcher)

	// Initially not watching
	if watcher.IsWatching(1) {
		t.Error("expected not watching issue 1")
	}

	// Start watching
	watcher.Watch(1)
	if !watcher.IsWatching(1) {
		t.Error("expected watching issue 1")
	}

	// Stop watching
	watcher.Unwatch(1)
	if watcher.IsWatching(1) {
		t.Error("expected not watching issue 1 after unwatch")
	}
}

func TestFeedbackWatcher_CheckOnce(t *testing.T) {
	now := time.Now()
	user := &github.User{Login: ptr("testuser")}

	fetcher := &mockCommentFetcher{
		comments: map[int][]*github.IssueComment{
			1: {
				{
					Body:      ptr("What does this do?"),
					CreatedAt: &github.Timestamp{Time: now.Add(1 * time.Second)},
					User:      user,
				},
				{
					Body:      ptr("Please fix this"),
					CreatedAt: &github.Timestamp{Time: now.Add(2 * time.Second)},
					User:      user,
				},
			},
		},
	}

	watcher := NewFeedbackWatcher(fetcher)
	watcher.Watch(1)

	events, err := watcher.CheckOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}

	// Verify event types
	foundQuestion := false
	foundFeedback := false
	for _, e := range events {
		if e.Comment.Type == CommentTypeQuestion {
			foundQuestion = true
		}
		if e.Comment.Type == CommentTypeFeedback {
			foundFeedback = true
		}
	}

	if !foundQuestion {
		t.Error("expected to find question event")
	}
	if !foundFeedback {
		t.Error("expected to find feedback event")
	}
}

func TestFeedbackWatcher_SkipsUnknownComments(t *testing.T) {
	now := time.Now()
	user := &github.User{Login: ptr("testuser")}

	fetcher := &mockCommentFetcher{
		comments: map[int][]*github.IssueComment{
			1: {
				{
					Body:      ptr("Thanks!"),
					CreatedAt: &github.Timestamp{Time: now.Add(1 * time.Second)},
					User:      user,
				},
			},
		},
	}

	watcher := NewFeedbackWatcher(fetcher)
	watcher.Watch(1)

	events, err := watcher.CheckOnce(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Unknown comments should not produce events
	if len(events) != 0 {
		t.Errorf("expected 0 events for unknown comments, got %d", len(events))
	}
}

func TestFeedbackWatcher_Interval(t *testing.T) {
	fetcher := &mockCommentFetcher{comments: make(map[int][]*github.IssueComment)}

	// Default interval
	watcher := NewFeedbackWatcher(fetcher)
	if watcher.Interval() != 30*time.Second {
		t.Errorf("expected default interval 30s, got %v", watcher.Interval())
	}

	// Custom interval
	cfg := FeedbackWatcherConfig{Interval: 10 * time.Second}
	watcher = NewFeedbackWatcherWithConfig(fetcher, cfg)
	if watcher.Interval() != 10*time.Second {
		t.Errorf("expected custom interval 10s, got %v", watcher.Interval())
	}
}
