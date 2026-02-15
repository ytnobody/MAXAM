package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ytnobody/MAXAM/internal/config"
)

func TestBuildSystemPromptWithContextMode(t *testing.T) {
	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "agent_context_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create full CLAUDE.md
	fullContent := "# Full Rules\n\n## 学習事項\nLots of details..."
	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(fullContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create summary CLAUDE.md
	summaryContent := "# Summary Rules"
	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.summary.md"), []byte(summaryContent), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("full mode uses CLAUDE.md", func(t *testing.T) {
		r := NewRunner("Test", tmpDir, "")
		r.ContextMode = config.ContextModeFull
		prompt, err := r.buildSystemPrompt()
		if err != nil {
			t.Fatal(err)
		}

		if !containsStr(prompt, fullContent) {
			t.Errorf("prompt should contain full content")
		}
	})

	t.Run("summary mode uses CLAUDE.summary.md", func(t *testing.T) {
		r := NewRunner("Test", tmpDir, "")
		r.ContextMode = config.ContextModeSummary
		prompt, err := r.buildSystemPrompt()
		if err != nil {
			t.Fatal(err)
		}

		if !containsStr(prompt, summaryContent) {
			t.Errorf("prompt should contain summary content")
		}
		if containsStr(prompt, "学習事項") {
			t.Errorf("prompt should not contain full learning content in summary mode")
		}
	})

	t.Run("summary mode fallback to CLAUDE.md when no summary file", func(t *testing.T) {
		// Create a dir without summary file
		noSummaryDir, _ := os.MkdirTemp("", "no_summary")
		defer os.RemoveAll(noSummaryDir)
		if err := os.WriteFile(filepath.Join(noSummaryDir, "CLAUDE.md"), []byte(fullContent), 0644); err != nil {
			t.Fatal(err)
		}

		r := NewRunner("Test", noSummaryDir, "")
		r.ContextMode = config.ContextModeSummary
		prompt, err := r.buildSystemPrompt()
		if err != nil {
			t.Fatal(err)
		}

		// Should fallback to full content
		if !containsStr(prompt, fullContent) {
			t.Errorf("prompt should fallback to full content when no summary file")
		}
	})
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestBuildSystemPromptWithProjectLocalCLAUDE(t *testing.T) {
	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "agent_project_local_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create root CLAUDE.md
	rootContent := "# Root Rules"
	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(rootContent), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("reads .maxam/CLAUDE.md", func(t *testing.T) {
		// Create .maxam/CLAUDE.md
		maxamDir := filepath.Join(tmpDir, ".maxam")
		os.MkdirAll(maxamDir, 0755)
		projectLocalContent := "# Project Local Settings"
		if err := os.WriteFile(filepath.Join(maxamDir, "CLAUDE.md"), []byte(projectLocalContent), 0644); err != nil {
			t.Fatal(err)
		}

		r := NewRunner("Test", tmpDir, "")
		prompt, err := r.buildSystemPrompt()
		if err != nil {
			t.Fatal(err)
		}

		if !containsStr(prompt, projectLocalContent) {
			t.Errorf("prompt should contain .maxam/CLAUDE.md content")
		}
	})

	t.Run("fallback to CLAUDE.local.md when .maxam/CLAUDE.md not exists", func(t *testing.T) {
		// Create a dir without .maxam/CLAUDE.md but with CLAUDE.local.md
		fallbackDir, _ := os.MkdirTemp("", "fallback_test")
		defer os.RemoveAll(fallbackDir)

		// Create root CLAUDE.md
		if err := os.WriteFile(filepath.Join(fallbackDir, "CLAUDE.md"), []byte(rootContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create deprecated CLAUDE.local.md
		localContent := "# Local Learning (deprecated)"
		if err := os.WriteFile(filepath.Join(fallbackDir, "CLAUDE.local.md"), []byte(localContent), 0644); err != nil {
			t.Fatal(err)
		}

		r := NewRunner("Test", fallbackDir, "")
		prompt, err := r.buildSystemPrompt()
		if err != nil {
			t.Fatal(err)
		}

		// Should contain CLAUDE.local.md content as fallback
		if !containsStr(prompt, localContent) {
			t.Errorf("prompt should fallback to CLAUDE.local.md when .maxam/CLAUDE.md not exists")
		}
	})

	t.Run(".maxam/CLAUDE.md takes priority over CLAUDE.local.md", func(t *testing.T) {
		// Create a dir with both files
		priorityDir, _ := os.MkdirTemp("", "priority_test")
		defer os.RemoveAll(priorityDir)

		// Create root CLAUDE.md
		if err := os.WriteFile(filepath.Join(priorityDir, "CLAUDE.md"), []byte(rootContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create .maxam/CLAUDE.md
		maxamDir := filepath.Join(priorityDir, ".maxam")
		os.MkdirAll(maxamDir, 0755)
		newContent := "# New Project Local"
		if err := os.WriteFile(filepath.Join(maxamDir, "CLAUDE.md"), []byte(newContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Create deprecated CLAUDE.local.md
		oldContent := "# Old Local (should not be read)"
		if err := os.WriteFile(filepath.Join(priorityDir, "CLAUDE.local.md"), []byte(oldContent), 0644); err != nil {
			t.Fatal(err)
		}

		r := NewRunner("Test", priorityDir, "")
		prompt, err := r.buildSystemPrompt()
		if err != nil {
			t.Fatal(err)
		}

		// Should contain .maxam/CLAUDE.md content
		if !containsStr(prompt, newContent) {
			t.Errorf("prompt should contain .maxam/CLAUDE.md content")
		}

		// Should NOT contain CLAUDE.local.md content
		if containsStr(prompt, oldContent) {
			t.Errorf("prompt should not contain CLAUDE.local.md when .maxam/CLAUDE.md exists")
		}
	})
}

func TestModelConstants(t *testing.T) {
	tests := []struct {
		model Model
		want  string
	}{
		{ModelDefault, "opus"},
		{ModelHaiku, "haiku"},
		{ModelSonnet, "sonnet"},
		{ModelOpus, "opus"},
	}

	for _, tt := range tests {
		if got := string(tt.model); got != tt.want {
			t.Errorf("Model %v = %q, want %q", tt.model, got, tt.want)
		}
	}
}
