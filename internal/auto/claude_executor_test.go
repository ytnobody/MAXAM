package auto

import (
	"testing"
)

func TestClaudeExecutor_generateBranchName(t *testing.T) {
	e := &ClaudeExecutor{}

	tests := []struct {
		name        string
		issueNumber int
		title       string
		want        string
	}{
		{
			name:        "simple title",
			issueNumber: 123,
			title:       "Add new feature",
			want:        "auto/123-add-new-feature",
		},
		{
			name:        "title with special chars",
			issueNumber: 456,
			title:       "Fix: bug in API (critical)",
			want:        "auto/456-fix-bug-in-api-critical",
		},
		{
			name:        "long title truncated",
			issueNumber: 789,
			title:       "This is a very long title that should be truncated to a reasonable length",
			want:        "auto/789-this-is-a-very-long-title-that-should-be",
		},
		{
			name:        "title with numbers",
			issueNumber: 100,
			title:       "Update version to 2.0",
			want:        "auto/100-update-version-to-20",
		},
		{
			name:        "title ending with special char",
			issueNumber: 200,
			title:       "Add feature!",
			want:        "auto/200-add-feature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := e.generateBranchName(tt.issueNumber, tt.title)
			if got != tt.want {
				t.Errorf("generateBranchName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClaudeExecutor_buildPrompt(t *testing.T) {
	e := &ClaudeExecutor{}

	prompt := e.buildPrompt(123, "Add login feature", "Implement user login with OAuth", "auto/123-add-login-feature")

	// Check key elements are present
	if !contains(prompt, "Issue #123") {
		t.Error("prompt should contain issue number")
	}
	if !contains(prompt, "Add login feature") {
		t.Error("prompt should contain title")
	}
	if !contains(prompt, "Implement user login with OAuth") {
		t.Error("prompt should contain body")
	}
	if !contains(prompt, "auto/123-add-login-feature") {
		t.Error("prompt should contain branch name")
	}
	if !contains(prompt, "Closes #123") {
		t.Error("prompt should contain close reference")
	}
	if !contains(prompt, "gh pr create") {
		t.Error("prompt should contain PR creation instruction")
	}
}

func TestClaudeExecutor_Interface(t *testing.T) {
	// Compile-time check that ClaudeExecutor implements TaskExecutor
	var _ TaskExecutor = (*ClaudeExecutor)(nil)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
