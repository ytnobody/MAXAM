package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const basePath = "/tmp/maxam"

var agentNames = []string{"mei", "yuki", "rin", "shiori", "priya", "amara"}

// CleanupForBranch removes all worktrees associated with the given branch
// Silently ignores errors (best-effort cleanup)
func CleanupForBranch(repoPath, branchName string) {
	for _, agent := range agentNames {
		worktrees := findWorktreesForAgent(agent, branchName)
		for _, wt := range worktrees {
			removeWorktree(repoPath, wt)
		}
	}
}

// findWorktreesForAgent finds worktrees for an agent that match the branch
func findWorktreesForAgent(agent, branchName string) []string {
	var result []string
	agentDir := filepath.Join(basePath, agent)

	entries, err := os.ReadDir(agentDir)
	if err != nil {
		return result
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		wtPath := filepath.Join(agentDir, entry.Name())
		branch := getWorktreeBranch(wtPath)
		if branch == branchName {
			result = append(result, wtPath)
		}
	}

	return result
}

// getWorktreeBranch returns the branch name of a worktree
func getWorktreeBranch(wtPath string) string {
	gitPath := filepath.Join(wtPath, ".git")
	content, err := os.ReadFile(gitPath)
	if err != nil {
		return ""
	}

	// .git file contains: gitdir: /path/to/repo/.git/worktrees/name
	gitdir := strings.TrimPrefix(strings.TrimSpace(string(content)), "gitdir: ")
	headPath := filepath.Join(gitdir, "HEAD")
	headContent, err := os.ReadFile(headPath)
	if err != nil {
		return ""
	}

	head := strings.TrimSpace(string(headContent))
	// HEAD is either a commit hash or ref: refs/heads/branch-name
	if branch, found := strings.CutPrefix(head, "ref: refs/heads/"); found {
		return branch
	}

	return "" // detached HEAD
}

// removeWorktree removes a worktree silently
func removeWorktree(repoPath, wtPath string) {
	// First try git worktree remove
	cmd := exec.Command("git", "worktree", "remove", "--force", wtPath)
	cmd.Dir = repoPath
	_ = cmd.Run()

	// Also clean up the directory if it still exists
	_ = os.RemoveAll(wtPath)

	// Prune any stale worktree entries
	pruneCmd := exec.Command("git", "worktree", "prune")
	pruneCmd.Dir = repoPath
	_ = pruneCmd.Run()
}

// ListAll returns all worktrees under /tmp/maxam
func ListAll() map[string][]string {
	result := make(map[string][]string)

	for _, agent := range agentNames {
		agentDir := filepath.Join(basePath, agent)
		entries, err := os.ReadDir(agentDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				wtPath := filepath.Join(agentDir, entry.Name())
				branch := getWorktreeBranch(wtPath)
				if branch == "" {
					branch = "(detached)"
				}
				result[agent] = append(result[agent], fmt.Sprintf("%s [%s]", entry.Name(), branch))
			}
		}
	}

	return result
}
