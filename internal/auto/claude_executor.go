package auto

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/ytnobody/MAXAM/internal/agent"
)

// ClaudeExecutor executes tasks using Claude CLI via agent.Runner
type ClaudeExecutor struct {
	// WorkDir is the repository working directory
	WorkDir string
	// ClaudeMDDir is the directory containing CLAUDE.md for the executor
	ClaudeMDDir string
	// Model is the Claude model to use (optional)
	Model agent.Model
}

// NewClaudeExecutor creates a new ClaudeExecutor
func NewClaudeExecutor(workDir, claudeMDDir string) *ClaudeExecutor {
	return &ClaudeExecutor{
		WorkDir:     workDir,
		ClaudeMDDir: claudeMDDir,
	}
}

// Execute implements TaskExecutor interface
// It fetches issue details, runs Claude CLI to implement, and creates a PR
func (e *ClaudeExecutor) Execute(ctx context.Context, issueNumber int, title string) (int, error) {
	// 1. Fetch issue details
	issueBody, err := e.fetchIssueDetails(ctx, issueNumber)
	if err != nil {
		return 0, fmt.Errorf("fetch issue details: %w", err)
	}

	// 2. Create branch name
	branchName := e.generateBranchName(issueNumber, title)

	// 3. Build prompt for Claude
	prompt := e.buildPrompt(issueNumber, title, issueBody, branchName)

	// 4. Create and run agent.Runner
	runner := agent.NewRunner("auto-executor", e.WorkDir, e.ClaudeMDDir)
	if e.Model != agent.ModelDefault {
		runner.Model = e.Model
	}

	_, err = runner.Run(ctx, prompt)
	if err != nil {
		return 0, fmt.Errorf("claude execution: %w", err)
	}

	// 5. Extract PR number from created PR
	prNumber, err := e.findCreatedPR(ctx, branchName)
	if err != nil {
		return 0, fmt.Errorf("find created PR: %w", err)
	}

	return prNumber, nil
}

// fetchIssueDetails gets issue body from GitHub
func (e *ClaudeExecutor) fetchIssueDetails(ctx context.Context, issueNumber int) (string, error) {
	cmd := exec.CommandContext(ctx, "gh", "issue", "view", strconv.Itoa(issueNumber), "--json", "body", "--jq", ".body")
	cmd.Dir = e.WorkDir

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// generateBranchName creates a branch name from issue number and title
func (e *ClaudeExecutor) generateBranchName(issueNumber int, title string) string {
	// Sanitize title: lowercase, replace spaces with hyphens, remove special chars
	sanitized := strings.ToLower(title)
	sanitized = strings.ReplaceAll(sanitized, " ", "-")
	sanitized = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(sanitized, "")
	// Truncate if too long
	if len(sanitized) > 40 {
		sanitized = sanitized[:40]
	}
	// Remove trailing hyphens
	sanitized = strings.TrimRight(sanitized, "-")

	return fmt.Sprintf("auto/%d-%s", issueNumber, sanitized)
}

// buildPrompt creates the prompt for Claude CLI
func (e *ClaudeExecutor) buildPrompt(issueNumber int, title, body, branchName string) string {
	return fmt.Sprintf(`Implement the following GitHub issue and create a PR.

## Issue #%d: %s

%s

## Instructions

1. Create a new branch: %s
2. Implement the changes required by this issue
3. Commit with message format: "feat: <description>\n\nCloses #%d"
4. Push the branch
5. Create a PR using: gh pr create --title "<title>" --body "<body>"

Important:
- Follow the project's coding conventions
- Write tests if applicable
- Keep commits atomic and focused
`, issueNumber, title, body, branchName, issueNumber)
}

// findCreatedPR finds the PR number for the given branch
func (e *ClaudeExecutor) findCreatedPR(ctx context.Context, branchName string) (int, error) {
	cmd := exec.CommandContext(ctx, "gh", "pr", "list", "--head", branchName, "--json", "number", "--jq", ".[0].number")
	cmd.Dir = e.WorkDir

	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("list PRs: %w", err)
	}

	prNumStr := strings.TrimSpace(string(output))
	if prNumStr == "" || prNumStr == "null" {
		return 0, fmt.Errorf("no PR found for branch %s", branchName)
	}

	prNumber, err := strconv.Atoi(prNumStr)
	if err != nil {
		return 0, fmt.Errorf("parse PR number: %w", err)
	}

	return prNumber, nil
}

// Ensure ClaudeExecutor implements TaskExecutor
var _ TaskExecutor = (*ClaudeExecutor)(nil)
