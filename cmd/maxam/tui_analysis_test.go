package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsNoIssueResponse(t *testing.T) {
	tests := []struct {
		name     string
		response string
		want     bool
	}{
		{
			name:     "特になし",
			response: "特になし",
			want:     true,
		},
		{
			name:     "特に問題なし",
			response: "特に問題なし。",
			want:     true,
		},
		{
			name:     "特に気になる点はない",
			response: "確認したけど、特に気になる点はないね。",
			want:     true,
		},
		{
			name:     "問題なし",
			response: "問題なし",
			want:     true,
		},
		{
			name:     "英語 nothing to report",
			response: "Nothing to report.",
			want:     true,
		},
		{
			name:     "英語 no issues",
			response: "No issues found.",
			want:     true,
		},
		{
			name:     "気づきあり",
			response: "差し戻しが2回あったね。Issueにしておく？",
			want:     false,
		},
		{
			name:     "コメントあり",
			response: "要件の確認が少し曖昧だったかも。",
			want:     false,
		},
		{
			name:     "空文字",
			response: "",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNoIssueResponse(tt.response)
			if got != tt.want {
				t.Errorf("isNoIssueResponse(%q) = %v, want %v", tt.response, got, tt.want)
			}
		})
	}
}

func TestFindGitRepos(t *testing.T) {
	// テスト用の一時ディレクトリを作成
	tmpDir, err := os.MkdirTemp("", "maxam-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// サブプロジェクト構造を作成
	// tmpDir/
	//   ├── project1/
	//   │   └── .git/
	//   ├── project2/
	//   │   ├── .git/
	//   │   └── README.md
	//   ├── nested/
	//   │   └── deep/
	//   │       └── .git/
	//   └── not-a-repo/

	// project1 (.git ディレクトリ)
	project1 := filepath.Join(tmpDir, "project1")
	os.MkdirAll(filepath.Join(project1, ".git"), 0755)

	// project2 (.git ディレクトリ + README)
	project2 := filepath.Join(tmpDir, "project2")
	os.MkdirAll(filepath.Join(project2, ".git"), 0755)
	os.WriteFile(filepath.Join(project2, "README.md"), []byte("# Project 2\nTest project"), 0644)
	os.WriteFile(filepath.Join(project2, "go.mod"), []byte("module example.com/project2"), 0644)

	// nested/deep (深いネスト)
	nested := filepath.Join(tmpDir, "nested", "deep")
	os.MkdirAll(filepath.Join(nested, ".git"), 0755)

	// not-a-repo (gitリポジトリではない)
	notRepo := filepath.Join(tmpDir, "not-a-repo")
	os.MkdirAll(notRepo, 0755)

	// テスト実行
	repos := findGitRepos(tmpDir)

	if len(repos) != 3 {
		t.Errorf("expected 3 repos, got %d", len(repos))
		for _, r := range repos {
			t.Logf("  found: %s", r.path)
		}
	}

	// 各リポジトリの検証
	pathMap := make(map[string]subProject)
	for _, r := range repos {
		pathMap[r.path] = r
	}

	// project1
	if _, ok := pathMap["project1"]; !ok {
		t.Error("project1 not found")
	}

	// project2 (README and go.mod)
	if p2, ok := pathMap["project2"]; ok {
		if p2.readme == "" {
			t.Error("project2 should have readme")
		}
		if !p2.hasGoMod {
			t.Error("project2 should have go.mod")
		}
	} else {
		t.Error("project2 not found")
	}

	// nested/deep
	nestedPath := filepath.Join("nested", "deep")
	if _, ok := pathMap[nestedPath]; !ok {
		t.Errorf("nested/deep not found, expected path: %s", nestedPath)
	}
}

func TestDetectAgentMentions(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		currentAgent string
		want         []string
	}{
		{
			name:         "単一メンション @yuki",
			text:         "@Yuki これやっといて",
			currentAgent: "mei",
			want:         []string{"yuki"},
		},
		{
			name:         "複数メンション @yuki @priya",
			text:         "@Yuki 実装して、@Priya レビューお願い",
			currentAgent: "mei",
			want:         []string{"yuki", "priya"},
		},
		{
			name:         "全員メンション",
			text:         "@Yuki @Priya @Amara @Rin @Shiori みんな集まって",
			currentAgent: "mei",
			want:         []string{"yuki", "priya", "amara", "rin", "shiori"},
		},
		{
			name:         "自分へのメンションは無視",
			text:         "@Mei 確認して",
			currentAgent: "mei",
			want:         []string{},
		},
		{
			name:         "メンションなし",
			text:         "これで完了です",
			currentAgent: "yuki",
			want:         []string{},
		},
		{
			name:         "日本語パターン ゆきさん",
			text:         "ゆきさんにお願いしますね",
			currentAgent: "mei",
			want:         []string{"yuki"},
		},
		{
			name:         "重複メンションは1回だけ",
			text:         "@Yuki やって、yukiさんよろしく",
			currentAgent: "mei",
			want:         []string{"yuki"},
		},
		{
			name:         "新人エージェント rin shiori",
			text:         "@Rin フロント、@Shiori テストよろしく",
			currentAgent: "mei",
			want:         []string{"rin", "shiori"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectAgentMentions(tt.text, tt.currentAgent)
			if len(got) != len(tt.want) {
				t.Errorf("detectAgentMentions(%q, %q) = %v, want %v", tt.text, tt.currentAgent, got, tt.want)
				return
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("detectAgentMentions(%q, %q)[%d] = %q, want %q", tt.text, tt.currentAgent, i, v, tt.want[i])
				}
			}
		})
	}
}

func TestGetWorktreePath(t *testing.T) {
	tests := []struct {
		name           string
		agentName      string
		rootDir        string
		subProjectPath string
		want           string
	}{
		{
			name:           "単純なサブプロジェクト",
			agentName:      "yuki",
			rootDir:        "/home/user/calcium-lang",
			subProjectPath: "calcium",
			want:           "/tmp/maxam/yuki/calcium-lang_calcium",
		},
		{
			name:           "深いネスト",
			agentName:      "yuki",
			rootDir:        "/home/user/parent",
			subProjectPath: filepath.Join("child", "grandchild"),
			want:           "/tmp/maxam/yuki/parent_child_grandchild",
		},
		{
			name:           "別のエージェント",
			agentName:      "priya",
			rootDir:        "/home/user/calcium-lang",
			subProjectPath: "boneyard",
			want:           "/tmp/maxam/priya/calcium-lang_boneyard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getWorktreePath(tt.agentName, tt.rootDir, tt.subProjectPath)
			if got != tt.want {
				t.Errorf("getWorktreePath(%q, %q, %q) = %q, want %q",
					tt.agentName, tt.rootDir, tt.subProjectPath, got, tt.want)
			}
		})
	}
}
