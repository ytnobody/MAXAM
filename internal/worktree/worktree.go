package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ytnobody/MAXAM/internal/config"
)

const basePath = "/tmp/maxam"

// getAgentNames returns agent names from config, or detects from filesystem
func getAgentNames(workDir string) []string {
	// Try to get from config first
	if workDir != "" {
		cfg, err := config.LoadWithProject(workDir)
		if err == nil && cfg.HasAgents() {
			return cfg.GetUniqueAgentNames()
		}
	}

	// Fallback: detect from filesystem
	return detectAgentsFromFilesystem()
}

// detectAgentsFromFilesystem scans /tmp/maxam for existing agent directories
func detectAgentsFromFilesystem() []string {
	var agents []string
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return agents
	}

	for _, entry := range entries {
		if entry.IsDir() {
			agents = append(agents, entry.Name())
		}
	}
	return agents
}

// CleanupForBranch removes all worktrees associated with the given branch
// Silently ignores errors (best-effort cleanup)
func CleanupForBranch(repoPath, branchName string) {
	agents := getAgentNames(repoPath)
	for _, agent := range agents {
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
	return ListAllForProject("")
}

// ListAllForProject returns all worktrees under /tmp/maxam for a specific project
func ListAllForProject(workDir string) map[string][]string {
	result := make(map[string][]string)

	agents := getAgentNames(workDir)
	for _, agent := range agents {
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
