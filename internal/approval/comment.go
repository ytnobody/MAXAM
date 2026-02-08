// Package approval manages approval requests and detection
package approval

import (
	"regexp"
	"strings"
	"time"

	"github.com/google/go-github/v60/github"
)

// ApprovalKeywords defines the keywords that indicate approval
var ApprovalKeywords = []string{
	"LGTM",
	"lgtm",
	"OK",
	"ok",
	"承認",
	"approve",
	"Approve",
	"APPROVE",
	"approved",
	"Approved",
	"APPROVED",
	"/approve",
	":+1:",
	"👍",
}

// CommentChecker checks comments for approval signals
type CommentChecker struct {
	keywords []string
	patterns []*regexp.Regexp
}

// NewCommentChecker creates a new CommentChecker with default keywords
func NewCommentChecker() *CommentChecker {
	return &CommentChecker{
		keywords: ApprovalKeywords,
		patterns: compilePatterns(ApprovalKeywords),
	}
}

// NewCommentCheckerWithKeywords creates a CommentChecker with custom keywords
func NewCommentCheckerWithKeywords(keywords []string) *CommentChecker {
	return &CommentChecker{
		keywords: keywords,
		patterns: compilePatterns(keywords),
	}
}

// compilePatterns compiles keywords into regex patterns
// Matches keyword at word boundaries to avoid false positives
func compilePatterns(keywords []string) []*regexp.Regexp {
	patterns := make([]*regexp.Regexp, 0, len(keywords))
	for _, kw := range keywords {
		var pattern *regexp.Regexp
		// Check if keyword contains only ASCII alphanumeric characters
		if isASCIIAlphanumeric(kw) {
			// Word boundary match for alphanumeric keywords
			pattern = regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(kw) + `\b`)
		} else {
			// For non-ASCII keywords (Japanese, emoji, etc.), use simple contains
			pattern = regexp.MustCompile(regexp.QuoteMeta(kw))
		}
		patterns = append(patterns, pattern)
	}
	return patterns
}

// isASCIIAlphanumeric checks if a string contains only ASCII alphanumeric characters
func isASCIIAlphanumeric(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

// IsApprovalComment checks if a comment body contains approval keywords
func (c *CommentChecker) IsApprovalComment(body string) bool {
	body = strings.TrimSpace(body)
	if body == "" {
		return false
	}

	for _, pattern := range c.patterns {
		if pattern.MatchString(body) {
			return true
		}
	}
	return false
}

// ApprovalResult represents the result of approval detection
type ApprovalResult struct {
	Approved   bool
	Comment    *github.IssueComment
	ApprovedAt time.Time
	ApprovedBy string
	Keyword    string
}

// FindApprovalInComments searches through comments for an approval
func (c *CommentChecker) FindApprovalInComments(comments []*github.IssueComment) *ApprovalResult {
	for _, comment := range comments {
		if comment.Body == nil {
			continue
		}

		body := *comment.Body
		for i, pattern := range c.patterns {
			if pattern.MatchString(body) {
				result := &ApprovalResult{
					Approved: true,
					Comment:  comment,
					Keyword:  c.keywords[i],
				}

				if comment.CreatedAt != nil {
					result.ApprovedAt = comment.CreatedAt.Time
				}

				if comment.User != nil && comment.User.Login != nil {
					result.ApprovedBy = *comment.User.Login
				}

				return result
			}
		}
	}

	return &ApprovalResult{Approved: false}
}

// FindApprovalSince searches for approval in comments created after a specific time
func (c *CommentChecker) FindApprovalSince(comments []*github.IssueComment, since time.Time) *ApprovalResult {
	for _, comment := range comments {
		// Skip comments before the since time
		if comment.CreatedAt != nil && comment.CreatedAt.Time.Before(since) {
			continue
		}

		if comment.Body == nil {
			continue
		}

		body := *comment.Body
		for i, pattern := range c.patterns {
			if pattern.MatchString(body) {
				result := &ApprovalResult{
					Approved: true,
					Comment:  comment,
					Keyword:  c.keywords[i],
				}

				if comment.CreatedAt != nil {
					result.ApprovedAt = comment.CreatedAt.Time
				}

				if comment.User != nil && comment.User.Login != nil {
					result.ApprovedBy = *comment.User.Login
				}

				return result
			}
		}
	}

	return &ApprovalResult{Approved: false}
}
