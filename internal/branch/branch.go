// Package branch provides git branch management utilities
package branch

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// SanitizeBranchName converts a milestone name to a valid git branch name
// Rules:
// - Spaces → hyphens
// - Special characters (except / and -) → removed or converted to hyphens
// - Consecutive hyphens → single hyphen
// - Leading/trailing hyphens → removed
// - Lowercase conversion
func SanitizeBranchName(name string) string {
	// Convert to lowercase
	result := strings.ToLower(name)

	// Replace spaces with hyphens
	result = strings.ReplaceAll(result, " ", "-")

	// Replace underscores with hyphens
	result = strings.ReplaceAll(result, "_", "-")

	// Remove or replace special characters (keep alphanumeric, /, -, .)
	// Git branch names can contain / for hierarchy but not at start/end
	re := regexp.MustCompile(`[^a-z0-9/.\-]`)
	result = re.ReplaceAllString(result, "-")

	// Collapse consecutive hyphens
	re = regexp.MustCompile(`-+`)
	result = re.ReplaceAllString(result, "-")

	// Remove leading/trailing hyphens and dots
	result = strings.Trim(result, "-.")

	// Remove consecutive slashes
	re = regexp.MustCompile(`/+`)
	result = re.ReplaceAllString(result, "/")

	// Remove leading/trailing slashes
	result = strings.Trim(result, "/")

	return result
}

// MilestoneBranchName creates a branch name for a milestone
// Format: milestone/{sanitized-name}
func MilestoneBranchName(milestoneName string) string {
	sanitized := SanitizeBranchName(milestoneName)
	if sanitized == "" {
		return "milestone/default"
	}
	return "milestone/" + sanitized
}

// BranchExists checks if a branch exists in the repository
func BranchExists(repoPath, branchName string) (bool, error) {
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branchName)
	cmd.Dir = repoPath
	err := cmd.Run()
	if err != nil {
		// Exit code 1 means branch doesn't exist
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("check branch exists: %w", err)
	}
	return true, nil
}

// CreateBranch creates a new branch from the current HEAD
func CreateBranch(repoPath, branchName string) error {
	cmd := exec.Command("git", "branch", branchName)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create branch %q: %s: %w", branchName, strings.TrimSpace(string(output)), err)
	}
	return nil
}

// CreateBranchFromBase creates a new branch from a specified base branch
func CreateBranchFromBase(repoPath, branchName, baseBranch string) error {
	cmd := exec.Command("git", "branch", branchName, baseBranch)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create branch %q from %q: %s: %w", branchName, baseBranch, strings.TrimSpace(string(output)), err)
	}
	return nil
}

// CreateMilestoneBranch creates a milestone branch if it doesn't exist
// Returns (branchName, created, error)
// - branchName: the actual branch name created
// - created: true if branch was newly created, false if already existed
func CreateMilestoneBranch(repoPath, milestoneName string) (string, bool, error) {
	branchName := MilestoneBranchName(milestoneName)

	// Check if branch already exists
	exists, err := BranchExists(repoPath, branchName)
	if err != nil {
		return "", false, fmt.Errorf("check existing branch: %w", err)
	}

	if exists {
		return branchName, false, nil
	}

	// Create new branch
	if err := CreateBranch(repoPath, branchName); err != nil {
		return "", false, err
	}

	return branchName, true, nil
}

// CreateMilestoneBranchFromBase creates a milestone branch from a specified base
// Returns (branchName, created, error)
func CreateMilestoneBranchFromBase(repoPath, milestoneName, baseBranch string) (string, bool, error) {
	branchName := MilestoneBranchName(milestoneName)

	// Check if branch already exists
	exists, err := BranchExists(repoPath, branchName)
	if err != nil {
		return "", false, fmt.Errorf("check existing branch: %w", err)
	}

	if exists {
		return branchName, false, nil
	}

	// Create new branch from base
	if err := CreateBranchFromBase(repoPath, branchName, baseBranch); err != nil {
		return "", false, err
	}

	return branchName, true, nil
}

// CheckoutBranch switches to the specified branch
func CheckoutBranch(repoPath, branchName string) error {
	cmd := exec.Command("git", "checkout", branchName)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("checkout branch %q: %s: %w", branchName, strings.TrimSpace(string(output)), err)
	}
	return nil
}

// GetCurrentBranch returns the current branch name
func GetCurrentBranch(repoPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get current branch: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}
