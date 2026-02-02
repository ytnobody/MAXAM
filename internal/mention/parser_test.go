package mention

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseTableFromFile(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []Member
	}{
		{
			name: "standard MAXAM table",
			content: `# Project

## チームメンバー

| 名前 | 役割 |
|------|------|
| Mei Chen | 要件定義 + PM |
| Yuki Tanaka | 実装 + インフラ |
| Priya Sharma | レビュー + セキュリティ |
`,
			expected: []Member{
				{Name: "Mei Chen", Role: "要件定義 + PM"},
				{Name: "Yuki Tanaka", Role: "実装 + インフラ"},
				{Name: "Priya Sharma", Role: "レビュー + セキュリティ"},
			},
		},
		{
			name: "english header",
			content: `| Name | Role |
|------|------|
| Alex Johnson | Backend |
| Hana Kim | Frontend |
`,
			expected: []Member{
				{Name: "Alex Johnson", Role: "Backend"},
				{Name: "Hana Kim", Role: "Frontend"},
			},
		},
		{
			name: "extra columns",
			content: `| ID | Name | Role | Note |
|----|------|------|------|
| 1 | Bob Smith | DevOps | |
`,
			expected: []Member{
				{Name: "Bob Smith", Role: "DevOps"},
			},
		},
		{
			name: "no table",
			content: `# Project

Just some text without any table.
`,
			expected: nil,
		},
		{
			name: "member header variant",
			content: `| Member | 担当 |
|--------|------|
| Charlie Brown | QA |
`,
			expected: []Member{
				{Name: "Charlie Brown", Role: "QA"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "CLAUDE.md")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}

			members, err := parseTableFromFile(tmpFile)
			if err != nil {
				t.Fatalf("parseTableFromFile failed: %v", err)
			}

			if len(members) != len(tt.expected) {
				t.Errorf("got %d members, want %d", len(members), len(tt.expected))
				return
			}

			for i, m := range members {
				if m.Name != tt.expected[i].Name {
					t.Errorf("member[%d].Name = %q, want %q", i, m.Name, tt.expected[i].Name)
				}
				if m.Role != tt.expected[i].Role {
					t.Errorf("member[%d].Role = %q, want %q", i, m.Role, tt.expected[i].Role)
				}
			}
		})
	}
}

func TestExtractShortName(t *testing.T) {
	tests := []struct {
		fullName string
		expected string
	}{
		{"Mei Chen", "mei"},
		{"Yuki Tanaka", "yuki"},
		{"Alex", "alex"},
		{"", ""},
		{"  Bob  Smith  ", "bob"},
	}

	for _, tt := range tests {
		t.Run(tt.fullName, func(t *testing.T) {
			got := ExtractShortName(tt.fullName)
			if got != tt.expected {
				t.Errorf("ExtractShortName(%q) = %q, want %q", tt.fullName, got, tt.expected)
			}
		})
	}
}

func TestIsSeparatorRow(t *testing.T) {
	tests := []struct {
		line     string
		expected bool
	}{
		{"|---|---|", true},
		{"| --- | --- |", true},
		{"|:---|---:|", true},
		{"| Name | Role |", false},
		{"not a table", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := isSeparatorRow(tt.line)
			if got != tt.expected {
				t.Errorf("isSeparatorRow(%q) = %v, want %v", tt.line, got, tt.expected)
			}
		})
	}
}
