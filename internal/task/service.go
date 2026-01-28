// Package task provides task management functionality
package task

import (
	"context"
	"fmt"

	gh "github.com/ytnobody/MAXAM/internal/github"
)

// ListOptions specifies options for listing tasks
type ListOptions struct {
	State    string // "open", "closed", "all" (default: "open")
	Assignee string // filter by assignee login (empty = all)
	Labels   []string
}

// Service provides task management operations
type Service struct {
	client *gh.Client
}

// NewService creates a new task service
func NewService(client *gh.Client) *Service {
	return &Service{client: client}
}

// List returns tasks matching the options
func (s *Service) List(ctx context.Context, opts ListOptions) ([]*Task, error) {
	issues, err := s.client.ListIssuesWithOptions(ctx, opts.State, opts.Assignee, opts.Labels)
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}

	tasks := make([]*Task, 0, len(issues))
	for _, issue := range issues {
		// Skip PRs - we only want issues
		if issue.PullRequestLinks != nil {
			continue
		}
		tasks = append(tasks, NewTask(issue))
	}

	return tasks, nil
}

// ListByAssignee returns tasks for a specific assignee
func (s *Service) ListByAssignee(ctx context.Context, assignee string) ([]*Task, error) {
	return s.List(ctx, ListOptions{
		State:    "open",
		Assignee: assignee,
	})
}

// ListStale returns tasks older than specified days
func (s *Service) ListStale(ctx context.Context, days int) ([]*Task, error) {
	tasks, err := s.List(ctx, ListOptions{State: "open"})
	if err != nil {
		return nil, err
	}

	stale := make([]*Task, 0)
	for _, t := range tasks {
		if t.StaleDays >= days {
			stale = append(stale, t)
		}
	}

	return stale, nil
}
