// Package mode manages MAXAM operating modes
package mode

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v60/github"
	"github.com/ytnobody/MAXAM/internal/approval"
	gh "github.com/ytnobody/MAXAM/internal/github"
)

// DefaultPollingInterval is the default interval for polling approval status
const DefaultPollingInterval = 30 * time.Second

// PlanModeConfig holds configuration for plan mode
type PlanModeConfig struct {
	PollingInterval time.Duration
}

// DefaultPlanModeConfig returns the default configuration
func DefaultPlanModeConfig() *PlanModeConfig {
	return &PlanModeConfig{
		PollingInterval: DefaultPollingInterval,
	}
}

// ProjectAnalysis contains simplified project analysis for plan mode
type ProjectAnalysis struct {
	TotalIssues      int
	OpenIssues       int
	UnassignedIssues int
	OldestOpenIssue  *IssueInfo
	LabelCounts      map[string]int
	Summary          string
}

// IssueInfo contains basic issue information
type IssueInfo struct {
	Number    int
	Title     string
	CreatedAt time.Time
	Labels    []string
}

// PlanIssue represents a plan issue created for approval
type PlanIssue struct {
	IssueNumber int
	Title       string
	Body        string
	CreatedAt   time.Time
	URL         string
}

// PlanExecutor handles plan mode execution
type PlanExecutor struct {
	ghClient *gh.Client
	config   *PlanModeConfig
	checker  *approval.CommentChecker
}

// NewPlanExecutor creates a new PlanExecutor
func NewPlanExecutor(ghClient *gh.Client, config *PlanModeConfig) *PlanExecutor {
	if config == nil {
		config = DefaultPlanModeConfig()
	}
	return &PlanExecutor{
		ghClient: ghClient,
		config:   config,
		checker:  approval.NewCommentChecker(),
	}
}

// AnalyzeProject performs a lightweight project analysis
func (pe *PlanExecutor) AnalyzeProject(ctx context.Context) (*ProjectAnalysis, error) {
	issues, err := pe.ghClient.ListIssues(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}

	analysis := &ProjectAnalysis{
		TotalIssues: len(issues),
		LabelCounts: make(map[string]int),
	}

	var oldestIssue *github.Issue
	for _, issue := range issues {
		// Skip pull requests (GitHub API returns PRs as issues too)
		if issue.PullRequestLinks != nil {
			continue
		}

		analysis.OpenIssues++

		if issue.Assignee == nil && len(issue.Assignees) == 0 {
			analysis.UnassignedIssues++
		}

		for _, label := range issue.Labels {
			if label.Name != nil {
				analysis.LabelCounts[*label.Name]++
			}
		}

		if oldestIssue == nil || issue.CreatedAt.Before(oldestIssue.CreatedAt.Time) {
			oldestIssue = issue
		}
	}

	if oldestIssue != nil {
		analysis.OldestOpenIssue = &IssueInfo{
			Number:    oldestIssue.GetNumber(),
			Title:     oldestIssue.GetTitle(),
			CreatedAt: oldestIssue.CreatedAt.Time,
		}
		for _, label := range oldestIssue.Labels {
			if label.Name != nil {
				analysis.OldestOpenIssue.Labels = append(analysis.OldestOpenIssue.Labels, *label.Name)
			}
		}
	}

	analysis.Summary = pe.generateSummary(analysis)

	return analysis, nil
}

// generateSummary creates a human-readable summary of the analysis
func (pe *PlanExecutor) generateSummary(analysis *ProjectAnalysis) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## プロジェクト状況サマリ\n\n"))
	sb.WriteString(fmt.Sprintf("- オープンIssue: %d件\n", analysis.OpenIssues))
	sb.WriteString(fmt.Sprintf("- 未アサイン: %d件\n", analysis.UnassignedIssues))

	if analysis.OldestOpenIssue != nil {
		age := time.Since(analysis.OldestOpenIssue.CreatedAt)
		days := int(age.Hours() / 24)
		sb.WriteString(fmt.Sprintf("- 最古のオープンIssue: #%d (%d日前)\n",
			analysis.OldestOpenIssue.Number, days))
	}

	if len(analysis.LabelCounts) > 0 {
		sb.WriteString("\n### ラベル別集計\n")
		for label, count := range analysis.LabelCounts {
			sb.WriteString(fmt.Sprintf("- %s: %d件\n", label, count))
		}
	}

	return sb.String()
}

// CreatePlanIssue creates an issue with the plan proposal
func (pe *PlanExecutor) CreatePlanIssue(ctx context.Context, analysis *ProjectAnalysis, proposal string) (*PlanIssue, error) {
	title := "[Plan] プロジェクト方針の提案"

	body := fmt.Sprintf(`%s

## 提案内容

%s

---

**承認する場合は、このIssueにコメントで「OK」「LGTM」「承認」などと返信してください。**

_このIssueはMAXAM Plan Modeによって自動生成されました。_
`, analysis.Summary, proposal)

	issue, err := pe.ghClient.CreateIssue(ctx, title, body, []string{"plan-mode"})
	if err != nil {
		return nil, fmt.Errorf("create plan issue: %w", err)
	}

	return &PlanIssue{
		IssueNumber: issue.GetNumber(),
		Title:       issue.GetTitle(),
		Body:        issue.GetBody(),
		CreatedAt:   issue.CreatedAt.Time,
		URL:         issue.GetHTMLURL(),
	}, nil
}

// WaitForApproval polls the issue for approval comments
func (pe *PlanExecutor) WaitForApproval(ctx context.Context, issueNumber int) (*approval.ApprovalResult, error) {
	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Get comments since start time
		comments, err := pe.ghClient.ListIssueCommentsSince(ctx, issueNumber, startTime)
		if err != nil {
			return nil, fmt.Errorf("list comments: %w", err)
		}

		result := pe.checker.FindApprovalSince(comments, startTime)
		if result.Approved {
			return result, nil
		}

		// Wait before next poll
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pe.config.PollingInterval):
		}
	}
}

// CheckApprovalOnce checks for approval without blocking
func (pe *PlanExecutor) CheckApprovalOnce(ctx context.Context, issueNumber int, since time.Time) (*approval.ApprovalResult, error) {
	comments, err := pe.ghClient.ListIssueCommentsSince(ctx, issueNumber, since)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}

	return pe.checker.FindApprovalSince(comments, since), nil
}

// PlanLabel is the label used to identify plan issues
const PlanLabel = "maxam-plan"

// FindExistingPlanIssue searches for an existing open plan issue by label
func (pe *PlanExecutor) FindExistingPlanIssue(ctx context.Context) (*PlanIssue, error) {
	issues, err := pe.ghClient.ListIssuesWithOptions(ctx, "open", "", []string{PlanLabel})
	if err != nil {
		return nil, fmt.Errorf("list issues with label: %w", err)
	}

	// Skip PRs and find the most recent plan issue
	var latestIssue *github.Issue
	for _, issue := range issues {
		if issue.PullRequestLinks != nil {
			continue
		}
		if latestIssue == nil || issue.CreatedAt.After(latestIssue.CreatedAt.Time) {
			latestIssue = issue
		}
	}

	if latestIssue == nil {
		return nil, nil
	}

	return &PlanIssue{
		IssueNumber: latestIssue.GetNumber(),
		Title:       latestIssue.GetTitle(),
		Body:        latestIssue.GetBody(),
		CreatedAt:   latestIssue.CreatedAt.Time,
		URL:         latestIssue.GetHTMLURL(),
	}, nil
}

// GetOrCreatePlanIssue finds an existing plan issue or creates a new one
func (pe *PlanExecutor) GetOrCreatePlanIssue(ctx context.Context, analysis *ProjectAnalysis, proposal string) (*PlanIssue, bool, error) {
	existing, err := pe.FindExistingPlanIssue(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("find existing plan issue: %w", err)
	}

	if existing != nil {
		return existing, false, nil
	}

	// Create new issue
	issue, err := pe.CreatePlanIssueWithLabel(ctx, analysis, proposal)
	if err != nil {
		return nil, false, fmt.Errorf("create plan issue: %w", err)
	}

	return issue, true, nil
}

// CreatePlanIssueWithLabel creates an issue with the plan proposal and maxam-plan label
func (pe *PlanExecutor) CreatePlanIssueWithLabel(ctx context.Context, analysis *ProjectAnalysis, proposal string) (*PlanIssue, error) {
	title := "[Plan] プロジェクト方針の提案"

	body := fmt.Sprintf(`%s

## 提案内容

%s

---

**承認する場合は、このIssueにコメントで「OK」「LGTM」「承認」「👍」などと返信してください。**

_このIssueはMAXAM Plan Modeによって自動生成されました。_
`, analysis.Summary, proposal)

	issue, err := pe.ghClient.CreateIssue(ctx, title, body, []string{PlanLabel})
	if err != nil {
		return nil, fmt.Errorf("create plan issue: %w", err)
	}

	return &PlanIssue{
		IssueNumber: issue.GetNumber(),
		Title:       issue.GetTitle(),
		Body:        issue.GetBody(),
		CreatedAt:   issue.CreatedAt.Time,
		URL:         issue.GetHTMLURL(),
	}, nil
}

// PostPlanComment posts a plan proposal as a comment to an existing issue
func (pe *PlanExecutor) PostPlanComment(ctx context.Context, issueNumber int, proposal string) error {
	body := fmt.Sprintf(`## 新しいプラン提案

%s

---

**承認する場合は「OK」「LGTM」「承認」「👍」などと返信してください。**

_このコメントはMAXAM Plan Modeによって自動生成されました。_
`, proposal)

	return pe.ghClient.CommentIssue(ctx, issueNumber, body)
}
