package member

import (
	"testing"
)

func TestParseMemberTable(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []struct {
			shortName string
			fullName  string
			role      string
		}
	}{
		{
			name: "standard table",
			content: `# Team

| Name | Role |
|------|------|
| Mei Chen | PM |
| Yuki Tanaka | Backend |
`,
			expected: []struct {
				shortName string
				fullName  string
				role      string
			}{
				{"mei", "Mei Chen", "PM"},
				{"yuki", "Yuki Tanaka", "Backend"},
			},
		},
		{
			name: "japanese header",
			content: `## チームメンバー

| 名前 | 役割 |
|------|------|
| Alex Johnson | Frontend |
| Hana Kim | Designer |
`,
			expected: []struct {
				shortName string
				fullName  string
				role      string
			}{
				{"alex", "Alex Johnson", "Frontend"},
				{"hana", "Hana Kim", "Designer"},
			},
		},
		{
			name: "with extra columns",
			content: `| Name | Role | Notes |
|------|------|-------|
| Priya Sharma | QA | Expert |
`,
			expected: []struct {
				shortName string
				fullName  string
				role      string
			}{
				{"priya", "Priya Sharma", "QA"},
			},
		},
		{
			name: "no table",
			content: `# Just some markdown
No table here.
`,
			expected: nil,
		},
		{
			name: "MAXAM style table",
			content: `## チームメンバー

| 名前 | 役割 |
|------|------|
| Mei Chen | 要件定義 + PM |
| Yuki Tanaka | バックエンド + インフラ |
| Rin Sato | フロントエンド + バックエンド |
| Shiori Tanaka | テスト + ドキュメント |
| Priya Sharma | レビュー + セキュリティ + QA |
| Amara Okonkwo | 分析 + 法的基礎知識 |
`,
			expected: []struct {
				shortName string
				fullName  string
				role      string
			}{
				{"mei", "Mei Chen", "要件定義 + PM"},
				{"yuki", "Yuki Tanaka", "バックエンド + インフラ"},
				{"rin", "Rin Sato", "フロントエンド + バックエンド"},
				{"shiori", "Shiori Tanaka", "テスト + ドキュメント"},
				{"priya", "Priya Sharma", "レビュー + セキュリティ + QA"},
				{"amara", "Amara Okonkwo", "分析 + 法的基礎知識"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseMemberTable(tt.content)

			if len(result) != len(tt.expected) {
				t.Errorf("got %d members, want %d", len(result), len(tt.expected))
				return
			}

			for i, exp := range tt.expected {
				if result[i].Name != exp.shortName {
					t.Errorf("member[%d].Name = %q, want %q", i, result[i].Name, exp.shortName)
				}
				if result[i].FullName != exp.fullName {
					t.Errorf("member[%d].FullName = %q, want %q", i, result[i].FullName, exp.fullName)
				}
				if result[i].Role != exp.role {
					t.Errorf("member[%d].Role = %q, want %q", i, result[i].Role, exp.role)
				}
			}
		})
	}
}

func TestExtractShortName(t *testing.T) {
	tests := []struct {
		fullName  string
		shortName string
	}{
		{"Mei Chen", "mei"},
		{"Yuki Tanaka", "yuki"},
		{"Alex", "alex"},
		{"", ""},
		{"  Priya  Sharma  ", "priya"},
	}

	for _, tt := range tests {
		t.Run(tt.fullName, func(t *testing.T) {
			got := extractShortName(tt.fullName)
			if got != tt.shortName {
				t.Errorf("extractShortName(%q) = %q, want %q", tt.fullName, got, tt.shortName)
			}
		})
	}
}
