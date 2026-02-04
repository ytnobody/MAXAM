package summary

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNeedsRegeneration(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(dir string) error
		expected bool
	}{
		{
			name: "no source file",
			setup: func(dir string) error {
				// Don't create anything
				return nil
			},
			expected: false,
		},
		{
			name: "source exists, no summary",
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, SourceFile), []byte("# Test"), 0644)
			},
			expected: true,
		},
		{
			name: "source newer than summary",
			setup: func(dir string) error {
				summaryPath := filepath.Join(dir, SummaryFile)
				if err := os.WriteFile(summaryPath, []byte("# Summary"), 0644); err != nil {
					return err
				}
				// Set summary to older time
				oldTime := time.Now().Add(-1 * time.Hour)
				if err := os.Chtimes(summaryPath, oldTime, oldTime); err != nil {
					return err
				}
				// Create source (will be newer)
				return os.WriteFile(filepath.Join(dir, SourceFile), []byte("# Test"), 0644)
			},
			expected: true,
		},
		{
			name: "summary newer than source",
			setup: func(dir string) error {
				sourcePath := filepath.Join(dir, SourceFile)
				if err := os.WriteFile(sourcePath, []byte("# Test"), 0644); err != nil {
					return err
				}
				// Set source to older time
				oldTime := time.Now().Add(-1 * time.Hour)
				if err := os.Chtimes(sourcePath, oldTime, oldTime); err != nil {
					return err
				}
				// Create summary (will be newer)
				return os.WriteFile(filepath.Join(dir, SummaryFile), []byte("# Summary"), 0644)
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := tt.setup(dir); err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			got := NeedsRegeneration(dir)
			if got != tt.expected {
				t.Errorf("NeedsRegeneration() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestExtractSections(t *testing.T) {
	content := `# MAXAM 共通規約

## プロジェクト概要

MAXAMは複数のAIエージェントが協調して開発業務を遂行するシステム。

## チームメンバー

チームメンバー情報は管理されます。

## 設定管理の指針

これはスキップされるセクション。

## コーディング規約

### 全般
- シンプルに保つ
- テストを書く

### Go
- gofmtでフォーマット

## 別のセクション

これもスキップ。

## エスカレーションルール

1. ルール1
2. ルール2
`

	sections := []string{
		"プロジェクト概要",
		"チームメンバー",
		"コーディング規約",
		"エスカレーションルール",
	}

	result := ExtractSections(content, sections)

	// Check that target sections are included
	if !contains(result, "## プロジェクト概要") {
		t.Error("missing プロジェクト概要 section")
	}
	if !contains(result, "## チームメンバー") {
		t.Error("missing チームメンバー section")
	}
	if !contains(result, "## コーディング規約") {
		t.Error("missing コーディング規約 section")
	}
	if !contains(result, "## エスカレーションルール") {
		t.Error("missing エスカレーションルール section")
	}

	// Check that non-target sections are excluded
	if contains(result, "## 設定管理の指針") {
		t.Error("should not include 設定管理の指針 section")
	}
	if contains(result, "## 別のセクション") {
		t.Error("should not include 別のセクション section")
	}

	// Check subsections are included
	if !contains(result, "### 全般") {
		t.Error("missing subsection 全般")
	}
}

func TestGenerate(t *testing.T) {
	dir := t.TempDir()

	// Create source file
	sourceContent := `# MAXAM 共通規約

## プロジェクト概要

MAXAMはテストシステム。

## チームメンバー

テストチーム。

## コーディング規約

### 全般
- シンプル

## 通信規約

チャットで連絡。

## エスカレーションルール

PMへ。

## Worktree運用

/tmp/maxam/
`

	sourcePath := filepath.Join(dir, SourceFile)
	if err := os.WriteFile(sourcePath, []byte(sourceContent), 0644); err != nil {
		t.Fatalf("failed to write source: %v", err)
	}

	// Generate summary
	if err := Generate(dir); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Check summary was created
	summaryPath := filepath.Join(dir, SummaryFile)
	data, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("failed to read summary: %v", err)
	}

	summary := string(data)

	// Check header
	if !contains(summary, "# MAXAM 共通規約（要約版）") {
		t.Error("missing summary header")
	}

	// Check footer
	if !contains(summary, "詳細はCLAUDE.md（フル版）を参照") {
		t.Error("missing summary footer")
	}

	// Check sections are present
	if !contains(summary, "## プロジェクト概要") {
		t.Error("missing プロジェクト概要")
	}
	if !contains(summary, "MAXAMはテストシステム") {
		t.Error("missing content from プロジェクト概要")
	}
}

func TestEnsureUpToDate(t *testing.T) {
	dir := t.TempDir()

	// Create source file
	sourcePath := filepath.Join(dir, SourceFile)
	if err := os.WriteFile(sourcePath, []byte("# Test\n\n## プロジェクト概要\n\nテスト"), 0644); err != nil {
		t.Fatalf("failed to write source: %v", err)
	}

	// First call should generate
	if err := EnsureUpToDate(dir); err != nil {
		t.Fatalf("EnsureUpToDate() error = %v", err)
	}

	// Check summary exists
	summaryPath := filepath.Join(dir, SummaryFile)
	if _, err := os.Stat(summaryPath); err != nil {
		t.Error("summary file should exist after EnsureUpToDate")
	}

	// Get summary mtime
	info1, _ := os.Stat(summaryPath)
	mtime1 := info1.ModTime()

	// Wait a bit and call again - should not regenerate
	time.Sleep(10 * time.Millisecond)
	if err := EnsureUpToDate(dir); err != nil {
		t.Fatalf("second EnsureUpToDate() error = %v", err)
	}

	info2, _ := os.Stat(summaryPath)
	mtime2 := info2.ModTime()

	if !mtime1.Equal(mtime2) {
		t.Error("summary should not be regenerated when up to date")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
