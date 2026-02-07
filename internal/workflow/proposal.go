// Package workflow implements agent collaboration workflows
package workflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ytnobody/MAXAM/internal/agent"
	"github.com/ytnobody/MAXAM/internal/github"
	"github.com/ytnobody/MAXAM/internal/logger"
)

// Label constants for proposal workflow
const (
	LabelProposal      = "proposal"
	LabelProposalReady = "proposal-ready"
)

// ProposalWorkflow represents the proposal analysis and generation workflow
type ProposalWorkflow struct {
	agents   *agent.Agents
	logMgr   *logger.Manager
	ghClient *github.Client
}

// NewProposalWorkflow creates a new proposal workflow
func NewProposalWorkflow(agents *agent.Agents, logMgr *logger.Manager, ghClient *github.Client) *ProposalWorkflow {
	return &ProposalWorkflow{
		agents:   agents,
		logMgr:   logMgr,
		ghClient: ghClient,
	}
}

// ProposalResult contains the outcome of the proposal workflow
type ProposalResult struct {
	IssueNumber int
	Analysis    string
	Proposal    string
	Success     bool
	Error       error
}

// Run executes the proposal workflow for a given issue
func (pw *ProposalWorkflow) Run(ctx context.Context, issueNumber int) (*ProposalResult, error) {
	result := &ProposalResult{
		IssueNumber: issueNumber,
	}

	fmt.Printf("[ProposalWorkflow] Starting for issue #%d\n", issueNumber)

	// Get issue details
	issue, err := pw.ghClient.GetIssue(ctx, issueNumber)
	if err != nil {
		return nil, fmt.Errorf("get issue: %w", err)
	}

	// Get analyst agent (Amara)
	analyst, ok := pw.agents.GetByRole("analyst")
	if !ok {
		return nil, fmt.Errorf("no agent with 'analyst' role found")
	}

	// Build analysis prompt
	prompt := pw.buildAnalysisPrompt(issue.GetTitle(), issue.GetBody())

	// Run analysis
	fmt.Printf("[%s] Analyzing issue...\n", analyst.Name)
	start := time.Now()
	analysis, err := analyst.Run(ctx, prompt)
	elapsed := time.Since(start)

	if err != nil {
		result.Error = fmt.Errorf("analysis: %w", err)
		return result, nil
	}
	result.Analysis = analysis
	fmt.Printf("[%s] Analysis complete (%v)\n", analyst.Name, elapsed.Round(time.Millisecond))

	// Log analyst's work
	if log, err := pw.logMgr.Get(strings.ToLower(strings.Split(analyst.Name, " ")[0])); err == nil {
		log.LogSimple(prompt, analysis, elapsed)
	}

	// Generate proposal from analysis
	proposal := pw.buildProposal(issue.GetTitle(), analysis)
	result.Proposal = proposal

	// Post proposal as comment
	fmt.Printf("[ProposalWorkflow] Posting proposal to issue #%d\n", issueNumber)
	if err := pw.ghClient.CommentIssue(ctx, issueNumber, proposal); err != nil {
		result.Error = fmt.Errorf("post comment: %w", err)
		return result, nil
	}

	// Add proposal-ready label
	if err := pw.ghClient.AddLabel(ctx, issueNumber, LabelProposalReady); err != nil {
		result.Error = fmt.Errorf("add label: %w", err)
		return result, nil
	}

	// Remove proposal label (workflow completed)
	if err := pw.ghClient.RemoveLabel(ctx, issueNumber, LabelProposal); err != nil {
		// Non-critical, just log
		fmt.Printf("[ProposalWorkflow] Warning: could not remove '%s' label: %v\n", LabelProposal, err)
	}

	result.Success = true
	fmt.Printf("[ProposalWorkflow] Completed for issue #%d\n", issueNumber)

	return result, nil
}

func (pw *ProposalWorkflow) buildAnalysisPrompt(title, body string) string {
	return fmt.Sprintf(`以下のIssueを分析し、実装提案を作成してください。

## Issue タイトル
%s

## Issue 本文
%s

## 分析観点
1. 要件の明確化（曖昧な点の指摘）
2. 技術的な実現可能性
3. 影響範囲の特定
4. リスクの洗い出し
5. 実装方針の提案

## 出力形式
### 要件サマリ
（Issue の要件を箇条書きで整理）

### 技術的考察
（実現方法、選択肢、トレードオフ）

### 影響範囲
（変更が必要なファイル、モジュール）

### リスク・懸念事項
（注意すべき点）

### 推奨実装方針
（具体的な進め方）
`, title, body)
}

func (pw *ProposalWorkflow) buildProposal(title, analysis string) string {
	return fmt.Sprintf(`## 提案: %s

%s

---
_この提案は MAXAM の ProposalWorkflow によって自動生成されました。_
`, title, analysis)
}

// HandleLabeledEvent processes an issue labeled event and triggers workflow if applicable
func (pw *ProposalWorkflow) HandleLabeledEvent(ctx context.Context, event *github.IssuesEvent) error {
	if event.Action != "labeled" {
		return nil
	}

	// Check if the proposal label was added
	hasProposalLabel := false
	for _, label := range event.Issue.Labels {
		if label.Name == LabelProposal {
			hasProposalLabel = true
			break
		}
	}

	if !hasProposalLabel {
		return nil
	}

	// Run the proposal workflow
	result, err := pw.Run(ctx, event.Issue.Number)
	if err != nil {
		return fmt.Errorf("proposal workflow: %w", err)
	}

	if result.Error != nil {
		return fmt.Errorf("proposal workflow result: %w", result.Error)
	}

	return nil
}
