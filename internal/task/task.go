// Package task provides task management functionality
package task

import (
	"time"

	"github.com/google/go-github/v60/github"
)

// Task represents a task with additional metadata
type Task struct {
	Issue     *github.Issue
	StaleDays int
}

// NewTask creates a Task from a GitHub issue
func NewTask(issue *github.Issue) *Task {
	staleDays := 0
	if issue.CreatedAt != nil {
		staleDays = int(time.Since(issue.CreatedAt.Time).Hours() / 24)
	}
	return &Task{
		Issue:     issue,
		StaleDays: staleDays,
	}
}

// Number returns the issue number
func (t *Task) Number() int {
	if t.Issue == nil || t.Issue.Number == nil {
		return 0
	}
	return *t.Issue.Number
}

// Title returns the issue title
func (t *Task) Title() string {
	if t.Issue == nil || t.Issue.Title == nil {
		return ""
	}
	return *t.Issue.Title
}

// State returns the issue state (open/closed)
func (t *Task) State() string {
	if t.Issue == nil || t.Issue.State == nil {
		return ""
	}
	return *t.Issue.State
}

// Assignee returns the assignee login name
func (t *Task) Assignee() string {
	if t.Issue == nil || t.Issue.Assignee == nil || t.Issue.Assignee.Login == nil {
		return ""
	}
	return *t.Issue.Assignee.Login
}

// Assignees returns all assignee login names
func (t *Task) Assignees() []string {
	if t.Issue == nil {
		return nil
	}
	names := make([]string, 0, len(t.Issue.Assignees))
	for _, a := range t.Issue.Assignees {
		if a.Login != nil {
			names = append(names, *a.Login)
		}
	}
	return names
}

// Labels returns the issue labels
func (t *Task) Labels() []string {
	if t.Issue == nil {
		return nil
	}
	labels := make([]string, 0, len(t.Issue.Labels))
	for _, l := range t.Issue.Labels {
		if l.Name != nil {
			labels = append(labels, *l.Name)
		}
	}
	return labels
}

// IsPullRequest returns true if this issue is actually a PR
func (t *Task) IsPullRequest() bool {
	return t.Issue != nil && t.Issue.PullRequestLinks != nil
}
