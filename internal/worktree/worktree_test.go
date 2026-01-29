package worktree

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetWorktreeBranch(t *testing.T) {
	// Create a temporary directory structure simulating a worktree
	tmpDir := t.TempDir()
	wtPath := filepath.Join(tmpDir, "test-worktree")
	os.MkdirAll(wtPath, 0755)

	// Create .git file pointing to worktree gitdir
	gitDir := filepath.Join(tmpDir, ".git", "worktrees", "test-worktree")
	os.MkdirAll(gitDir, 0755)

	gitFile := filepath.Join(wtPath, ".git")
	os.WriteFile(gitFile, []byte("gitdir: "+gitDir), 0644)

	t.Run("branch ref", func(t *testing.T) {
		headFile := filepath.Join(gitDir, "HEAD")
		os.WriteFile(headFile, []byte("ref: refs/heads/feature/test-branch\n"), 0644)

		branch := getWorktreeBranch(wtPath)
		if branch != "feature/test-branch" {
			t.Errorf("expected 'feature/test-branch', got '%s'", branch)
		}
	})

	t.Run("detached HEAD", func(t *testing.T) {
		headFile := filepath.Join(gitDir, "HEAD")
		os.WriteFile(headFile, []byte("abc123def456\n"), 0644)

		branch := getWorktreeBranch(wtPath)
		if branch != "" {
			t.Errorf("expected empty string for detached HEAD, got '%s'", branch)
		}
	})

	t.Run("no .git file", func(t *testing.T) {
		nonExistentPath := filepath.Join(tmpDir, "non-existent")
		branch := getWorktreeBranch(nonExistentPath)
		if branch != "" {
			t.Errorf("expected empty string for non-existent path, got '%s'", branch)
		}
	})
}

func TestFindWorktreesForAgent(t *testing.T) {
	// This test requires /tmp/maxam structure
	// Skip if running in CI without the structure
	agentDir := filepath.Join(basePath, "yuki")
	if _, err := os.Stat(agentDir); os.IsNotExist(err) {
		t.Skip("skipping: /tmp/maxam/yuki does not exist")
	}

	// Just verify it doesn't panic
	result := findWorktreesForAgent("yuki", "non-existent-branch-12345")
	if len(result) != 0 {
		t.Logf("found %d worktrees for non-existent branch (expected 0)", len(result))
	}
}

func TestListAll(t *testing.T) {
	// Just verify it doesn't panic
	result := ListAll()
	t.Logf("found worktrees for %d agents", len(result))
}

func TestCleanupForBranch(t *testing.T) {
	// Create a temporary test setup
	tmpDir := t.TempDir()

	// Create a fake git repo
	repoDir := filepath.Join(tmpDir, "repo")
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)

	// CleanupForBranch should not panic even with invalid paths
	CleanupForBranch(repoDir, "non-existent-branch")
}
