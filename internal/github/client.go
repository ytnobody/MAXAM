// Package github provides GitHub API integration
package github

import (
	"context"
	"fmt"
	"os"
	"time"

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

// CountOpenIssues returns the number of open issues
// Implements the auto.IssueChecker interface
func (c *Client) CountOpenIssues(ctx context.Context) (int, error) {
	issues, err := c.ListIssues(ctx, nil)
	if err != nil {
		return 0, err
	}
	return len(issues), nil
}

// EditIssue updates an issue's title and/or body
func (c *Client) EditIssue(ctx context.Context, number int, title, body *string) (*github.Issue, error) {
	req := &github.IssueRequest{}
	if title != nil {
		req.Title = title
	}
	if body != nil {
		req.Body = body
	}

	issue, _, err := c.client.Issues.Edit(ctx, c.owner, c.repo, number, req)
	if err != nil {
		return nil, fmt.Errorf("edit issue: %w", err)
	}
	return issue, nil
}

// ListIssueComments lists comments on an issue
func (c *Client) ListIssueComments(ctx context.Context, number int) ([]*github.IssueComment, error) {
	opts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	var allComments []*github.IssueComment
	for {
		comments, resp, err := c.client.Issues.ListComments(ctx, c.owner, c.repo, number, opts)
		if err != nil {
			return nil, fmt.Errorf("list issue comments: %w", err)
		}
		allComments = append(allComments, comments...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allComments, nil
}

// ListIssueCommentsSince lists comments on an issue created after a specific time
func (c *Client) ListIssueCommentsSince(ctx context.Context, number int, since time.Time) ([]*github.IssueComment, error) {
	opts := &github.IssueListCommentsOptions{
		Since: &since,
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	var allComments []*github.IssueComment
	for {
		comments, resp, err := c.client.Issues.ListComments(ctx, c.owner, c.repo, number, opts)
		if err != nil {
			return nil, fmt.Errorf("list issue comments: %w", err)
		}
		allComments = append(allComments, comments...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allComments, nil
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

// PRReviewStatus represents the review status of a pull request
type PRReviewStatus string

const (
	// PRReviewStatusPending means no reviews yet or review requested
	PRReviewStatusPending PRReviewStatus = "PENDING"
	// PRReviewStatusApproved means the PR has been approved
	PRReviewStatusApproved PRReviewStatus = "APPROVED"
	// PRReviewStatusChangesRequested means changes have been requested
	PRReviewStatusChangesRequested PRReviewStatus = "CHANGES_REQUESTED"
)

// PRWithReviewStatus contains a PR and its review status
type PRWithReviewStatus struct {
	PR           *github.PullRequest
	ReviewStatus PRReviewStatus
}

// GetPRReviewStatus returns the current review status of a PR
func (c *Client) GetPRReviewStatus(ctx context.Context, number int) (PRReviewStatus, error) {
	reviews, _, err := c.client.PullRequests.ListReviews(ctx, c.owner, c.repo, number, nil)
	if err != nil {
		return PRReviewStatusPending, fmt.Errorf("list PR reviews: %w", err)
	}

	if len(reviews) == 0 {
		return PRReviewStatusPending, nil
	}

	// Find the latest review state (most recent review wins)
	var latestReview *github.PullRequestReview
	for _, review := range reviews {
		state := review.GetState()
		// Skip COMMENTED and PENDING states as they don't affect approval
		if state == "APPROVED" || state == "CHANGES_REQUESTED" {
			if latestReview == nil || review.GetSubmittedAt().After(latestReview.GetSubmittedAt().Time) {
				latestReview = review
			}
		}
	}

	if latestReview == nil {
		return PRReviewStatusPending, nil
	}

	switch latestReview.GetState() {
	case "APPROVED":
		return PRReviewStatusApproved, nil
	case "CHANGES_REQUESTED":
		return PRReviewStatusChangesRequested, nil
	default:
		return PRReviewStatusPending, nil
	}
}

// ListPRsAwaitingReview returns PRs that are waiting for review
// A PR is considered "awaiting review" if:
// - It has no reviews yet, OR
// - It has review_requested status (someone was requested to review)
// PRs with CHANGES_REQUESTED are NOT included (author needs to fix first)
func (c *Client) ListPRsAwaitingReview(ctx context.Context) ([]*PRWithReviewStatus, error) {
	// Get all open PRs
	prs, err := c.ListPRs(ctx)
	if err != nil {
		return nil, err
	}

	var awaitingReview []*PRWithReviewStatus
	for _, pr := range prs {
		status, err := c.GetPRReviewStatus(ctx, pr.GetNumber())
		if err != nil {
			// Log error but continue with other PRs
			continue
		}

		// Only include PRs that are pending review (not approved, not changes requested)
		if status == PRReviewStatusPending {
			awaitingReview = append(awaitingReview, &PRWithReviewStatus{
				PR:           pr,
				ReviewStatus: status,
			})
		}
	}

	return awaitingReview, nil
}

// Label operations

// AddLabel adds a label to an issue
func (c *Client) AddLabel(ctx context.Context, number int, label string) error {
	_, _, err := c.client.Issues.AddLabelsToIssue(ctx, c.owner, c.repo, number, []string{label})
	if err != nil {
		return fmt.Errorf("add label: %w", err)
	}
	return nil
}

// RemoveLabel removes a label from an issue
func (c *Client) RemoveLabel(ctx context.Context, number int, label string) error {
	_, err := c.client.Issues.RemoveLabelForIssue(ctx, c.owner, c.repo, number, label)
	if err != nil {
		return fmt.Errorf("remove label: %w", err)
	}
	return nil
}

// HasLabel checks if an issue has a specific label
func (c *Client) HasLabel(ctx context.Context, number int, label string) (bool, error) {
	issue, _, err := c.client.Issues.Get(ctx, c.owner, c.repo, number)
	if err != nil {
		return false, fmt.Errorf("get issue: %w", err)
	}

	for _, l := range issue.Labels {
		if l.GetName() == label {
			return true, nil
		}
	}
	return false, nil
}

// GetUnderlyingClient returns the underlying go-github client
func (c *Client) GetUnderlyingClient() *github.Client {
	return c.client
}
