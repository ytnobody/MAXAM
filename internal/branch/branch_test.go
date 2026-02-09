package branch

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSanitizeBranchName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple name",
			input:    "feature",
			expected: "feature",
		},
		{
			name:     "spaces to hyphens",
			input:    "my feature",
			expected: "my-feature",
		},
		{
			name:     "uppercase to lowercase",
			input:    "MyFeature",
			expected: "myfeature",
		},
		{
			name:     "underscores to hyphens",
			input:    "my_feature",
			expected: "my-feature",
		},
		{
			name:     "special characters removed",
			input:    "feature!@#$%test",
			expected: "feature-test",
		},
		{
			name:     "consecutive hyphens collapsed",
			input:    "feature---test",
			expected: "feature-test",
		},
		{
			name:     "leading trailing hyphens removed",
			input:    "-feature-",
			expected: "feature",
		},
		{
			name:     "slashes preserved for hierarchy",
			input:    "feature/sub",
			expected: "feature/sub",
		},
		{
			name:     "dots preserved",
			input:    "v1.0.0",
			expected: "v1.0.0",
		},
		{
			name:     "complex milestone name",
			input:    "Sprint 2024-Q1: UI Improvements",
			expected: "sprint-2024-q1-ui-improvements",
		},
		{
			name:     "Japanese characters removed",
			input:    "機能追加",
			expected: "",
		},
		{
			name:     "mixed Japanese and English",
			input:    "feature-機能",
			expected: "feature",
		},
		{
			name:     "consecutive slashes collapsed",
			input:    "feature//sub",
			expected: "feature/sub",
		},
		{
			name:     "leading trailing slashes removed",
			input:    "/feature/",
			expected: "feature",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only special characters",
			input:    "!@#$%",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeBranchName(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeBranchName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMilestoneBranchName(t *testing.T) {
	tests := []struct {
		name          string
		milestoneName string
		expected      string
	}{
		{
			name:          "simple milestone",
			milestoneName: "v1.0",
			expected:      "milestone/v1.0",
		},
		{
			name:          "milestone with spaces",
			milestoneName: "Sprint 1",
			expected:      "milestone/sprint-1",
		},
		{
			name:          "empty milestone defaults",
			milestoneName: "",
			expected:      "milestone/default",
		},
		{
			name:          "only special chars defaults",
			milestoneName: "!!!",
			expected:      "milestone/default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MilestoneBranchName(tt.milestoneName)
			if result != tt.expected {
				t.Errorf("MilestoneBranchName(%q) = %q, want %q", tt.milestoneName, result, tt.expected)
			}
		})
	}
}

// setupTestRepo creates a temporary git repository for testing
func setupTestRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Configure git user (required for commits)
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git config email failed: %v", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git config name failed: %v", err)
	}

	// Create initial commit
	dummyFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(dummyFile, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("write dummy file failed: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	return dir
}

func TestBranchExists(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoPath := setupTestRepo(t)

	tests := []struct {
		name       string
		branchName string
		setup      func()
		expected   bool
	}{
		{
			name:       "existing default branch",
			branchName: "master",
			setup:      func() {},
			expected:   true,
		},
		{
			name:       "non-existing branch",
			branchName: "feature/nonexistent",
			setup:      func() {},
			expected:   false,
		},
		{
			name:       "created branch exists",
			branchName: "feature/test",
			setup: func() {
				cmd := exec.Command("git", "branch", "feature/test")
				cmd.Dir = repoPath
				_ = cmd.Run()
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			exists, err := BranchExists(repoPath, tt.branchName)
			if err != nil {
				t.Fatalf("BranchExists() error = %v", err)
			}
			if exists != tt.expected {
				t.Errorf("BranchExists(%q) = %v, want %v", tt.branchName, exists, tt.expected)
			}
		})
	}
}

func TestCreateBranch(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoPath := setupTestRepo(t)

	// Test creating a new branch
	branchName := "feature/new-branch"
	err := CreateBranch(repoPath, branchName)
	if err != nil {
		t.Fatalf("CreateBranch() error = %v", err)
	}

	// Verify branch exists
	exists, err := BranchExists(repoPath, branchName)
	if err != nil {
		t.Fatalf("BranchExists() error = %v", err)
	}
	if !exists {
		t.Error("Created branch should exist")
	}

	// Test creating duplicate branch (should fail)
	err = CreateBranch(repoPath, branchName)
	if err == nil {
		t.Error("CreateBranch() should fail for duplicate branch")
	}
}

func TestCreateMilestoneBranch(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	t.Run("creates new milestone branch", func(t *testing.T) {
		repoPath := setupTestRepo(t)

		branchName, created, err := CreateMilestoneBranch(repoPath, "Sprint 1")
		if err != nil {
			t.Fatalf("CreateMilestoneBranch() error = %v", err)
		}
		if !created {
			t.Error("Branch should be newly created")
		}
		if branchName != "milestone/sprint-1" {
			t.Errorf("branchName = %q, want %q", branchName, "milestone/sprint-1")
		}

		// Verify branch exists
		exists, _ := BranchExists(repoPath, branchName)
		if !exists {
			t.Error("Created milestone branch should exist")
		}
	})

	t.Run("returns existing branch without creating", func(t *testing.T) {
		repoPath := setupTestRepo(t)

		// Create first time
		branchName1, created1, err := CreateMilestoneBranch(repoPath, "v1.0")
		if err != nil {
			t.Fatalf("First CreateMilestoneBranch() error = %v", err)
		}
		if !created1 {
			t.Error("First call should create branch")
		}

		// Try to create again
		branchName2, created2, err := CreateMilestoneBranch(repoPath, "v1.0")
		if err != nil {
			t.Fatalf("Second CreateMilestoneBranch() error = %v", err)
		}
		if created2 {
			t.Error("Second call should not create branch")
		}
		if branchName1 != branchName2 {
			t.Errorf("Branch names should match: %q vs %q", branchName1, branchName2)
		}
	})
}

func TestCreateMilestoneBranchFromBase(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoPath := setupTestRepo(t)

	// Create a base branch with a commit
	cmd := exec.Command("git", "checkout", "-b", "develop")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("create develop branch failed: %v", err)
	}

	// Add a commit to develop
	dummyFile := filepath.Join(repoPath, "develop.txt")
	if err := os.WriteFile(dummyFile, []byte("develop content\n"), 0644); err != nil {
		t.Fatalf("write dummy file failed: %v", err)
	}
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoPath
	_ = cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "Develop commit")
	cmd.Dir = repoPath
	_ = cmd.Run()

	// Create milestone branch from develop
	branchName, created, err := CreateMilestoneBranchFromBase(repoPath, "Feature Sprint", "develop")
	if err != nil {
		t.Fatalf("CreateMilestoneBranchFromBase() error = %v", err)
	}
	if !created {
		t.Error("Branch should be newly created")
	}
	if branchName != "milestone/feature-sprint" {
		t.Errorf("branchName = %q, want %q", branchName, "milestone/feature-sprint")
	}

	// Verify the new branch has the develop commit
	cmd = exec.Command("git", "checkout", branchName)
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("checkout milestone branch failed: %v", err)
	}

	// Check if develop.txt exists (inherited from develop)
	if _, err := os.Stat(dummyFile); os.IsNotExist(err) {
		t.Error("Milestone branch should have develop.txt from base branch")
	}
}

func TestGetCurrentBranch(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoPath := setupTestRepo(t)

	// Get current branch (should be master or main depending on git version)
	branch, err := GetCurrentBranch(repoPath)
	if err != nil {
		t.Fatalf("GetCurrentBranch() error = %v", err)
	}
	if branch != "master" && branch != "main" {
		t.Errorf("GetCurrentBranch() = %q, want 'master' or 'main'", branch)
	}

	// Create and checkout a new branch
	newBranch := "feature/test"
	cmd := exec.Command("git", "checkout", "-b", newBranch)
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("checkout new branch failed: %v", err)
	}

	branch, err = GetCurrentBranch(repoPath)
	if err != nil {
		t.Fatalf("GetCurrentBranch() error = %v", err)
	}
	if branch != newBranch {
		t.Errorf("GetCurrentBranch() = %q, want %q", branch, newBranch)
	}
}

func TestCheckoutBranch(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoPath := setupTestRepo(t)

	// Create a test branch
	testBranch := "feature/checkout-test"
	if err := CreateBranch(repoPath, testBranch); err != nil {
		t.Fatalf("CreateBranch() error = %v", err)
	}

	// Checkout the new branch
	if err := CheckoutBranch(repoPath, testBranch); err != nil {
		t.Fatalf("CheckoutBranch() error = %v", err)
	}

	// Verify we're on the new branch
	current, _ := GetCurrentBranch(repoPath)
	if current != testBranch {
		t.Errorf("Current branch = %q, want %q", current, testBranch)
	}

	// Try to checkout non-existent branch
	if err := CheckoutBranch(repoPath, "nonexistent"); err == nil {
		t.Error("CheckoutBranch() should fail for non-existent branch")
	}
}
