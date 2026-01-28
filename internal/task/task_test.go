package task

import (
	"testing"
	"time"

	"github.com/google/go-github/v60/github"
)

func ptr[T any](v T) *T {
	return &v
}

func TestNewTask(t *testing.T) {
	now := time.Now()
	fiveDaysAgo := now.Add(-5 * 24 * time.Hour)

	issue := &github.Issue{
		Number:    ptr(123),
		Title:     ptr("Test Issue"),
		State:     ptr("open"),
		CreatedAt: &github.Timestamp{Time: fiveDaysAgo},
		Assignee: &github.User{
			Login: ptr("yuki"),
		},
		Assignees: []*github.User{
			{Login: ptr("yuki")},
			{Login: ptr("rin")},
		},
		Labels: []*github.Label{
			{Name: ptr("bug")},
			{Name: ptr("high-priority")},
		},
	}

	task := NewTask(issue)

	if task.Number() != 123 {
		t.Errorf("Number() = %d, want 123", task.Number())
	}

	if task.Title() != "Test Issue" {
		t.Errorf("Title() = %s, want Test Issue", task.Title())
	}

	if task.State() != "open" {
		t.Errorf("State() = %s, want open", task.State())
	}

	if task.Assignee() != "yuki" {
		t.Errorf("Assignee() = %s, want yuki", task.Assignee())
	}

	assignees := task.Assignees()
	if len(assignees) != 2 {
		t.Errorf("Assignees() len = %d, want 2", len(assignees))
	}

	labels := task.Labels()
	if len(labels) != 2 {
		t.Errorf("Labels() len = %d, want 2", len(labels))
	}

	// StaleDays should be approximately 5
	if task.StaleDays < 4 || task.StaleDays > 6 {
		t.Errorf("StaleDays = %d, want ~5", task.StaleDays)
	}
}

func TestTask_NilValues(t *testing.T) {
	task := NewTask(&github.Issue{})

	if task.Number() != 0 {
		t.Errorf("Number() = %d, want 0", task.Number())
	}

	if task.Title() != "" {
		t.Errorf("Title() = %s, want empty", task.Title())
	}

	if task.Assignee() != "" {
		t.Errorf("Assignee() = %s, want empty", task.Assignee())
	}

	if len(task.Assignees()) != 0 {
		t.Errorf("Assignees() len = %d, want 0", len(task.Assignees()))
	}

	if len(task.Labels()) != 0 {
		t.Errorf("Labels() len = %d, want 0", len(task.Labels()))
	}
}

func TestTask_IsPullRequest(t *testing.T) {
	// Regular issue
	issue := &github.Issue{
		Number: ptr(1),
	}
	task := NewTask(issue)
	if task.IsPullRequest() {
		t.Error("IsPullRequest() = true for regular issue")
	}

	// PR
	pr := &github.Issue{
		Number: ptr(2),
		PullRequestLinks: &github.PullRequestLinks{
			URL: ptr("https://api.github.com/repos/owner/repo/pulls/2"),
		},
	}
	prTask := NewTask(pr)
	if !prTask.IsPullRequest() {
		t.Error("IsPullRequest() = false for PR")
	}
}
