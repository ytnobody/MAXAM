package plan

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ytnobody/MAXAM/internal/github"
)

// ApprovalStatus represents the approval state
type ApprovalStatus string

const (
	// StatusPending means waiting for approval
	StatusPending ApprovalStatus = "pending"
	// StatusApproved means the plan was approved
	StatusApproved ApprovalStatus = "approved"
	// StatusRejected means the plan was rejected
	StatusRejected ApprovalStatus = "rejected"
)

// ApprovalKeywords for detecting approval
var ApprovalKeywords = []string{
	"approved",
	"approve",
	"lgtm",
	"looks good",
	"go ahead",
}

// RejectionKeywords for detecting rejection
var RejectionKeywords = []string{
	"rejected",
	"reject",
	"cancel",
	"don't proceed",
	"stop",
}

// ApprovalChecker checks for approval on issues
type ApprovalChecker struct {
	client        *github.Client
	approvedLabel string
}

// NewApprovalChecker creates a new approval checker
func NewApprovalChecker(client *github.Client) *ApprovalChecker {
	return &ApprovalChecker{
		client:        client,
		approvedLabel: "approved",
	}
}

// ApprovalResult contains the result of checking approval
type ApprovalResult struct {
	Status     ApprovalStatus
	ApprovedBy string
	Comment    string
	Timestamp  time.Time
}

// CheckIssueApproval checks if an issue has been approved
func (c *ApprovalChecker) CheckIssueApproval(ctx context.Context, issueNumber int) (*ApprovalResult, error) {
	// First check for approved label
	hasLabel, err := c.client.HasLabel(ctx, issueNumber, c.approvedLabel)
	if err != nil {
		return nil, fmt.Errorf("check label: %w", err)
	}

	if hasLabel {
		return &ApprovalResult{
			Status:     StatusApproved,
			ApprovedBy: "label",
			Comment:    "Approved via label",
			Timestamp:  time.Now(),
		}, nil
	}

	// Check comments for approval keywords
	comments, err := c.getIssueComments(ctx, issueNumber)
	if err != nil {
		return nil, fmt.Errorf("get comments: %w", err)
	}

	// Check comments in reverse order (newest first)
	for i := len(comments) - 1; i >= 0; i-- {
		comment := comments[i]
		body := strings.ToLower(comment.Body)

		// Check for rejection first (takes priority if both found)
		for _, keyword := range RejectionKeywords {
			if strings.Contains(body, keyword) {
				return &ApprovalResult{
					Status:     StatusRejected,
					ApprovedBy: comment.Author,
					Comment:    comment.Body,
					Timestamp:  comment.CreatedAt,
				}, nil
			}
		}

		// Check for approval
		for _, keyword := range ApprovalKeywords {
			if strings.Contains(body, keyword) {
				return &ApprovalResult{
					Status:     StatusApproved,
					ApprovedBy: comment.Author,
					Comment:    comment.Body,
					Timestamp:  comment.CreatedAt,
				}, nil
			}
		}
	}

	return &ApprovalResult{
		Status: StatusPending,
	}, nil
}

// IssueComment represents a comment on an issue
type IssueComment struct {
	ID        int64
	Author    string
	Body      string
	CreatedAt time.Time
}

// getIssueComments fetches comments for an issue
func (c *ApprovalChecker) getIssueComments(ctx context.Context, issueNumber int) ([]IssueComment, error) {
	// Use gh CLI to get comments since the go-github API doesn't expose this directly
	// This is a simplified implementation - in production you'd use the full API
	comments, err := c.client.ListIssueComments(ctx, issueNumber)
	if err != nil {
		return nil, err
	}

	var result []IssueComment
	for _, comment := range comments {
		result = append(result, IssueComment{
			ID:        comment.GetID(),
			Author:    comment.GetUser().GetLogin(),
			Body:      comment.GetBody(),
			CreatedAt: comment.GetCreatedAt().Time,
		})
	}

	return result, nil
}

// WaitForApproval polls for approval status
func (c *ApprovalChecker) WaitForApproval(ctx context.Context, issueNumber int, pollInterval time.Duration) (*ApprovalResult, error) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			result, err := c.CheckIssueApproval(ctx, issueNumber)
			if err != nil {
				// Log error but continue polling
				continue
			}

			if result.Status != StatusPending {
				return result, nil
			}
		}
	}
}
