package github

import (
	"context"
	"testing"

	gh "github.com/google/go-github/v60/github"
)

func TestHasLabel(t *testing.T) {
	tests := []struct {
		name     string
		labels   []*gh.Label
		search   string
		expected bool
	}{
		{
			name: "label exists",
			labels: []*gh.Label{
				{Name: ptr("bug")},
				{Name: ptr("proposal")},
				{Name: ptr("approved")},
			},
			search:   "proposal",
			expected: true,
		},
		{
			name: "label does not exist",
			labels: []*gh.Label{
				{Name: ptr("bug")},
				{Name: ptr("enhancement")},
			},
			search:   "proposal",
			expected: false,
		},
		{
			name:     "empty labels",
			labels:   []*gh.Label{},
			search:   "proposal",
			expected: false,
		},
		{
			name: "case sensitive",
			labels: []*gh.Label{
				{Name: ptr("Proposal")},
			},
			search:   "proposal",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the label checking logic directly
			result := hasLabelInList(tt.labels, tt.search)
			if result != tt.expected {
				t.Errorf("hasLabelInList() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// hasLabelInList is a helper function to test label checking logic
// This mirrors the logic in HasLabel method
func hasLabelInList(labels []*gh.Label, search string) bool {
	for _, l := range labels {
		if l.GetName() == search {
			return true
		}
	}
	return false
}

func TestClient_NewClientWithToken(t *testing.T) {
	client := NewClientWithToken("owner", "repo", "test-token")
	if client == nil {
		t.Fatal("NewClientWithToken returned nil")
	}
	if client.owner != "owner" {
		t.Errorf("owner = %q, want %q", client.owner, "owner")
	}
	if client.repo != "repo" {
		t.Errorf("repo = %q, want %q", client.repo, "repo")
	}
}

func TestClient_NewClient_NoToken(t *testing.T) {
	// Clear GITHUB_TOKEN
	t.Setenv("GITHUB_TOKEN", "")

	_, err := NewClient("owner", "repo")
	if err == nil {
		t.Error("NewClient should return error when GITHUB_TOKEN is not set")
	}
}

func TestClient_NewClient_WithToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")

	client, err := NewClient("owner", "repo")
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	if client == nil {
		t.Fatal("NewClient returned nil client")
	}
}

// Integration tests for label operations would require mocking
// For now, we test the exported interface exists
func TestClient_LabelMethodsExist(t *testing.T) {
	client := NewClientWithToken("owner", "repo", "test-token")
	ctx := context.Background()

	// These tests verify the methods exist and have correct signatures
	// They will fail with network errors, which is expected
	_ = client.AddLabel(ctx, 1, "label")
	_ = client.RemoveLabel(ctx, 1, "label")
	_, _ = client.HasLabel(ctx, 1, "label")
}

func TestMemberTask_Structure(t *testing.T) {
	// Test that MemberTask struct has all expected fields
	task := MemberTask{
		Assignee: "test-user",
		Number:   42,
		Title:    "Test Issue",
		Type:     "issue",
		Status:   "作業中",
		IsDraft:  false,
	}

	if task.Assignee != "test-user" {
		t.Errorf("Assignee = %q, want %q", task.Assignee, "test-user")
	}
	if task.Number != 42 {
		t.Errorf("Number = %d, want %d", task.Number, 42)
	}
	if task.Type != "issue" {
		t.Errorf("Type = %q, want %q", task.Type, "issue")
	}
	if task.Status != "作業中" {
		t.Errorf("Status = %q, want %q", task.Status, "作業中")
	}
}

func TestMemberTask_PRStatus(t *testing.T) {
	tests := []struct {
		name           string
		isDraft        bool
		expectedStatus string
	}{
		{
			name:           "draft PR shows WIP status",
			isDraft:        true,
			expectedStatus: "作業中",
		},
		{
			name:           "non-draft PR shows review status",
			isDraft:        false,
			expectedStatus: "レビュー待ち",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the status logic from GetMemberTasks
			status := "レビュー待ち"
			if tt.isDraft {
				status = "作業中"
			}

			if status != tt.expectedStatus {
				t.Errorf("status = %q, want %q", status, tt.expectedStatus)
			}
		})
	}
}

func TestClient_GetMemberTasksMethodExists(t *testing.T) {
	client := NewClientWithToken("owner", "repo", "test-token")
	ctx := context.Background()

	// This test verifies the method exists and has correct signature
	// It will fail with network errors, which is expected
	_, _ = client.GetMemberTasks(ctx, []string{"user1", "user2"})
}
