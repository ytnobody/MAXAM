package approval

import (
	"testing"
	"time"

	"github.com/google/go-github/v60/github"
)

// ptr is a helper to create pointers to values
func ptr[T any](v T) *T {
	return &v
}

func TestCommentChecker_IsApprovalComment(t *testing.T) {
	checker := NewCommentChecker()

	tests := []struct {
		name     string
		body     string
		expected bool
	}{
		// Approval cases
		{"LGTM uppercase", "LGTM", true},
		{"lgtm lowercase", "lgtm", true},
		{"LGTM with message", "LGTM! Great work.", true},
		{"OK uppercase", "OK", true},
		{"ok lowercase", "ok", true},
		{"OK with exclamation", "OK!", true},
		{"承認 Japanese", "承認", true},
		{"承認 with comment", "承認します！", true},
		{"approve lowercase", "approve", true},
		{"Approve titlecase", "Approve", true},
		{"APPROVE uppercase", "APPROVE", true},
		{"approved lowercase", "approved", true},
		{"Approved titlecase", "Approved", true},

		// Non-approval cases
		{"empty string", "", false},
		{"random text", "This is a random comment", false},
		{"question about ok", "Is this ok to merge?", true}, // "ok" matches as word boundary
		{"looks good", "Looks good!", false},              // "ok" in "Looks" doesn't match (not word boundary)
		{"nice work", "Nice work!", false},

		// Edge cases
		{"whitespace only", "   ", false},
		{"LGTM in sentence", "I think LGTM for this PR", true},
		{"multiple keywords", "LGTM, OK, 承認", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.IsApprovalComment(tt.body)
			if result != tt.expected {
				t.Errorf("IsApprovalComment(%q) = %v, want %v", tt.body, result, tt.expected)
			}
		})
	}
}

func TestCommentChecker_FindApprovalInComments(t *testing.T) {
	checker := NewCommentChecker()

	now := time.Now()
	user := &github.User{Login: ptr("testuser")}

	tests := []struct {
		name           string
		comments       []*github.IssueComment
		expectApproved bool
		expectUser     string
	}{
		{
			name:           "no comments",
			comments:       []*github.IssueComment{},
			expectApproved: false,
		},
		{
			name: "single approval",
			comments: []*github.IssueComment{
				{
					Body:      ptr("LGTM"),
					CreatedAt: &github.Timestamp{Time: now},
					User:      user,
				},
			},
			expectApproved: true,
			expectUser:     "testuser",
		},
		{
			name: "approval among other comments",
			comments: []*github.IssueComment{
				{
					Body:      ptr("This is a question"),
					CreatedAt: &github.Timestamp{Time: now.Add(-2 * time.Hour)},
					User:      &github.User{Login: ptr("user1")},
				},
				{
					Body:      ptr("OK, looks good"),
					CreatedAt: &github.Timestamp{Time: now.Add(-1 * time.Hour)},
					User:      &github.User{Login: ptr("approver")},
				},
				{
					Body:      ptr("Thanks!"),
					CreatedAt: &github.Timestamp{Time: now},
					User:      &github.User{Login: ptr("user1")},
				},
			},
			expectApproved: true,
			expectUser:     "approver",
		},
		{
			name: "no approval",
			comments: []*github.IssueComment{
				{
					Body:      ptr("Nice work"),
					CreatedAt: &github.Timestamp{Time: now},
					User:      user,
				},
				{
					Body:      ptr("Can you fix the typo?"),
					CreatedAt: &github.Timestamp{Time: now},
					User:      user,
				},
			},
			expectApproved: false,
		},
		{
			name: "nil body",
			comments: []*github.IssueComment{
				{
					Body:      nil,
					CreatedAt: &github.Timestamp{Time: now},
					User:      user,
				},
			},
			expectApproved: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.FindApprovalInComments(tt.comments)
			if result.Approved != tt.expectApproved {
				t.Errorf("Approved = %v, want %v", result.Approved, tt.expectApproved)
			}
			if tt.expectApproved && result.ApprovedBy != tt.expectUser {
				t.Errorf("ApprovedBy = %q, want %q", result.ApprovedBy, tt.expectUser)
			}
		})
	}
}

func TestCommentChecker_FindApprovalSince(t *testing.T) {
	checker := NewCommentChecker()

	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)
	twoHoursAgo := now.Add(-2 * time.Hour)

	user := &github.User{Login: ptr("testuser")}

	tests := []struct {
		name           string
		comments       []*github.IssueComment
		since          time.Time
		expectApproved bool
	}{
		{
			name: "approval after since",
			comments: []*github.IssueComment{
				{
					Body:      ptr("LGTM"),
					CreatedAt: &github.Timestamp{Time: now},
					User:      user,
				},
			},
			since:          oneHourAgo,
			expectApproved: true,
		},
		{
			name: "approval before since",
			comments: []*github.IssueComment{
				{
					Body:      ptr("LGTM"),
					CreatedAt: &github.Timestamp{Time: twoHoursAgo},
					User:      user,
				},
			},
			since:          oneHourAgo,
			expectApproved: false,
		},
		{
			name: "mixed comments",
			comments: []*github.IssueComment{
				{
					Body:      ptr("LGTM"),
					CreatedAt: &github.Timestamp{Time: twoHoursAgo},
					User:      &github.User{Login: ptr("old_approver")},
				},
				{
					Body:      ptr("Please fix"),
					CreatedAt: &github.Timestamp{Time: oneHourAgo.Add(30 * time.Minute)},
					User:      user,
				},
				{
					Body:      ptr("OK"),
					CreatedAt: &github.Timestamp{Time: now},
					User:      &github.User{Login: ptr("new_approver")},
				},
			},
			since:          oneHourAgo,
			expectApproved: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.FindApprovalSince(tt.comments, tt.since)
			if result.Approved != tt.expectApproved {
				t.Errorf("Approved = %v, want %v", result.Approved, tt.expectApproved)
			}
		})
	}
}

func TestNewCommentCheckerWithKeywords(t *testing.T) {
	customKeywords := []string{"GO", "SHIP", "🚀"}
	checker := NewCommentCheckerWithKeywords(customKeywords)

	tests := []struct {
		name     string
		body     string
		expected bool
	}{
		{"custom GO", "GO", true},
		{"custom SHIP", "SHIP it!", true},
		{"emoji", "🚀", true},
		{"default LGTM not matched", "LGTM", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.IsApprovalComment(tt.body)
			if result != tt.expected {
				t.Errorf("IsApprovalComment(%q) = %v, want %v", tt.body, result, tt.expected)
			}
		})
	}
}
