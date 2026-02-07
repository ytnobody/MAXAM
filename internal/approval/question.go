// Package approval manages approval requests and detection
package approval

import (
	"context"
	"fmt"
)

// IssueCommenter is an interface for commenting on issues
type IssueCommenter interface {
	CommentIssue(ctx context.Context, number int, body string) error
}

// QuestionType represents the type of question being asked
type QuestionType string

const (
	// QuestionTypeClarification is for clarifying requirements
	QuestionTypeClarification QuestionType = "clarification"
	// QuestionTypeApproval is for requesting approval
	QuestionTypeApproval QuestionType = "approval"
	// QuestionTypeBlocked is for reporting a blocker
	QuestionTypeBlocked QuestionType = "blocked"
)

// Question represents a question to be asked to the owner
type Question struct {
	Type        QuestionType
	Title       string
	Description string
	Options     []string // Optional: if provided, owner can choose from these
	AgentName   string   // Who is asking
}

// QuestionPoster handles posting questions to GitHub issues
type QuestionPoster struct {
	client IssueCommenter
}

// NewQuestionPoster creates a new QuestionPoster
func NewQuestionPoster(client IssueCommenter) *QuestionPoster {
	return &QuestionPoster{client: client}
}

// PostQuestion posts a question to a GitHub issue
func (p *QuestionPoster) PostQuestion(ctx context.Context, issueNumber int, q Question) error {
	body := formatQuestion(q)
	return p.client.CommentIssue(ctx, issueNumber, body)
}

// formatQuestion formats a question for display in GitHub
func formatQuestion(q Question) string {
	var body string

	// Header with icon based on type
	switch q.Type {
	case QuestionTypeClarification:
		body = "## ❓ 質問があります\n\n"
	case QuestionTypeApproval:
		body = "## ✋ 承認をお願いします\n\n"
	case QuestionTypeBlocked:
		body = "## 🚫 ブロッカーが発生しました\n\n"
	default:
		body = "## 📝 お知らせ\n\n"
	}

	// Title
	if q.Title != "" {
		body += fmt.Sprintf("**%s**\n\n", q.Title)
	}

	// Description
	if q.Description != "" {
		body += q.Description + "\n\n"
	}

	// Options (if provided)
	if len(q.Options) > 0 {
		body += "### 選択肢\n\n"
		for i, opt := range q.Options {
			body += fmt.Sprintf("%d. %s\n", i+1, opt)
		}
		body += "\n選択肢の番号で回答するか、自由にコメントしてください。\n\n"
	}

	// How to respond
	body += "---\n"
	body += "回答は `/approve` または直接コメントしてください。\n"

	// Agent signature
	if q.AgentName != "" {
		body += fmt.Sprintf("\n*Asked by: %s (MAXAM Agent)*", q.AgentName)
	}

	return body
}

// PostPlanApprovalRequest posts a request for plan approval
func (p *QuestionPoster) PostPlanApprovalRequest(ctx context.Context, issueNumber int, planSummary, agentName string) error {
	q := Question{
		Type:        QuestionTypeApproval,
		Title:       "実装計画の承認",
		Description: planSummary,
		AgentName:   agentName,
	}
	return p.PostQuestion(ctx, issueNumber, q)
}

// PostClarificationRequest posts a clarification request
func (p *QuestionPoster) PostClarificationRequest(ctx context.Context, issueNumber int, question, agentName string) error {
	q := Question{
		Type:        QuestionTypeClarification,
		Title:       "要件の確認",
		Description: question,
		AgentName:   agentName,
	}
	return p.PostQuestion(ctx, issueNumber, q)
}

// PostBlockerNotification posts a blocker notification
func (p *QuestionPoster) PostBlockerNotification(ctx context.Context, issueNumber int, blocker, agentName string) error {
	q := Question{
		Type:        QuestionTypeBlocked,
		Title:       "作業がブロックされています",
		Description: blocker,
		AgentName:   agentName,
	}
	return p.PostQuestion(ctx, issueNumber, q)
}
