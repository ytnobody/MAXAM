// Package github provides GitHub API integration
package github

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-github/v60/github"
	"golang.org/x/oauth2"
)

// Client wraps the GitHub API client
type Client struct {
	client *github.Client
	owner  string
	repo   string
}

// NewClient creates a new GitHub client
func NewClient(owner, repo string) (*Client, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN environment variable not set")
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	return &Client{
		client: github.NewClient(tc),
		owner:  owner,
		repo:   repo,
	}, nil
}

// NewClientWithToken creates a client with explicit token
func NewClientWithToken(owner, repo, token string) *Client {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	return &Client{
		client: github.NewClient(tc),
		owner:  owner,
		repo:   repo,
	}
}

// Issue operations

// ptr is a helper to create pointers to values
func ptr[T any](v T) *T {
	return &v
}

// CreateIssue creates a new issue
func (c *Client) CreateIssue(ctx context.Context, title, body string, labels []string) (*github.Issue, error) {
	issue := &github.IssueRequest{
		Title:  ptr(title),
		Body:   ptr(body),
		Labels: &labels,
	}

	created, _, err := c.client.Issues.Create(ctx, c.owner, c.repo, issue)
	if err != nil {
		return nil, fmt.Errorf("create issue: %w", err)
	}

	return created, nil
}

// GetIssue retrieves an issue by number
func (c *Client) GetIssue(ctx context.Context, number int) (*github.Issue, error) {
	issue, _, err := c.client.Issues.Get(ctx, c.owner, c.repo, number)
	if err != nil {
		return nil, fmt.Errorf("get issue: %w", err)
	}
	return issue, nil
}

// ListIssues lists open issues
func (c *Client) ListIssues(ctx context.Context, labels []string) ([]*github.Issue, error) {
	return c.ListIssuesWithOptions(ctx, "open", "", labels)
}

// ListIssuesWithOptions lists issues with filtering options
func (c *Client) ListIssuesWithOptions(ctx context.Context, state, assignee string, labels []string) ([]*github.Issue, error) {
	if state == "" {
		state = "open"
	}

	opts := &github.IssueListByRepoOptions{
		State:  state,
		Labels: labels,
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	if assignee != "" {
		opts.Assignee = assignee
	}

	var allIssues []*github.Issue
	for {
		issues, resp, err := c.client.Issues.ListByRepo(ctx, c.owner, c.repo, opts)
		if err != nil {
			return nil, fmt.Errorf("list issues: %w", err)
		}
		allIssues = append(allIssues, issues...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allIssues, nil
}

// AssignIssue assigns an issue to users
func (c *Client) AssignIssue(ctx context.Context, number int, assignees []string) error {
	_, _, err := c.client.Issues.AddAssignees(ctx, c.owner, c.repo, number, assignees)
	if err != nil {
		return fmt.Errorf("assign issue: %w", err)
	}
	return nil
}

// CommentIssue adds a comment to an issue
func (c *Client) CommentIssue(ctx context.Context, number int, body string) error {
	comment := &github.IssueComment{
		Body: ptr(body),
	}

	_, _, err := c.client.Issues.CreateComment(ctx, c.owner, c.repo, number, comment)
	if err != nil {
		return fmt.Errorf("comment issue: %w", err)
	}
	return nil
}

// CloseIssue closes an issue
func (c *Client) CloseIssue(ctx context.Context, number int) error {
	_, _, err := c.client.Issues.Edit(ctx, c.owner, c.repo, number, &github.IssueRequest{
		State: ptr("closed"),
	})
	if err != nil {
		return fmt.Errorf("close issue: %w", err)
	}
	return nil
}

// PR operations

// CreatePR creates a new pull request
func (c *Client) CreatePR(ctx context.Context, title, body, head, base string) (*github.PullRequest, error) {
	pr := &github.NewPullRequest{
		Title: ptr(title),
		Body:  ptr(body),
		Head:  ptr(head),
		Base:  ptr(base),
	}

	created, _, err := c.client.PullRequests.Create(ctx, c.owner, c.repo, pr)
	if err != nil {
		return nil, fmt.Errorf("create PR: %w", err)
	}

	return created, nil
}

// GetPR retrieves a pull request by number
func (c *Client) GetPR(ctx context.Context, number int) (*github.PullRequest, error) {
	pr, _, err := c.client.PullRequests.Get(ctx, c.owner, c.repo, number)
	if err != nil {
		return nil, fmt.Errorf("get PR: %w", err)
	}
	return pr, nil
}

// ListPRs lists open pull requests
func (c *Client) ListPRs(ctx context.Context) ([]*github.PullRequest, error) {
	opts := &github.PullRequestListOptions{
		State: "open",
		ListOptions: github.ListOptions{
			PerPage: 30,
		},
	}

	prs, _, err := c.client.PullRequests.List(ctx, c.owner, c.repo, opts)
	if err != nil {
		return nil, fmt.Errorf("list PRs: %w", err)
	}

	return prs, nil
}

// ReviewPR submits a review on a PR
func (c *Client) ReviewPR(ctx context.Context, number int, body string, approve bool) error {
	event := "COMMENT"
	if approve {
		event = "APPROVE"
	} else if body != "" {
		event = "REQUEST_CHANGES"
	}

	review := &github.PullRequestReviewRequest{
		Body:  ptr(body),
		Event: ptr(event),
	}

	_, _, err := c.client.PullRequests.CreateReview(ctx, c.owner, c.repo, number, review)
	if err != nil {
		return fmt.Errorf("review PR: %w", err)
	}
	return nil
}

// MergePR merges a pull request
func (c *Client) MergePR(ctx context.Context, number int, message string) error {
	_, _, err := c.client.PullRequests.Merge(ctx, c.owner, c.repo, number, message, nil)
	if err != nil {
		return fmt.Errorf("merge PR: %w", err)
	}
	return nil
}

// GetPRFiles returns files changed in a PR
func (c *Client) GetPRFiles(ctx context.Context, number int) ([]*github.CommitFile, error) {
	files, _, err := c.client.PullRequests.ListFiles(ctx, c.owner, c.repo, number, nil)
	if err != nil {
		return nil, fmt.Errorf("list PR files: %w", err)
	}
	return files, nil
}
