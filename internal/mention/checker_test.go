package mention

import (
	"testing"
)

func TestCheck(t *testing.T) {
	// Setup: Set default checker with test agent names
	// This simulates the TUI initialization that sets up mention targets from config
	testAgents := []string{"mei", "yuki", "rin", "shiori", "priya", "amara", "オーナー"}
	SetDefaultChecker(NewChecker(testAgents))
	defer func() { defaultChecker = nil }() // Reset after test

	tests := []struct {
		name        string
		message     string
		wantWarning bool
		wantMention bool
	}{
		// 警告が必要なケース（メンションなし）
		{
			name:        "no mention - simple message",
			message:     "これお願い",
			wantWarning: true,
			wantMention: false,
		},
		{
			name:        "no mention - question",
			message:     "確認して",
			wantWarning: true,
			wantMention: false,
		},
		{
			name:        "no mention - statement",
			message:     "実装完了しました",
			wantWarning: true,
			wantMention: false,
		},
		{
			name:        "no mention - just text",
			message:     "これはただのコメントです",
			wantWarning: true,
			wantMention: false,
		},

		// 警告不要なケース（メンションあり）
		{
			name:        "with mention yuki",
			message:     "@yuki これお願い",
			wantWarning: false,
			wantMention: true,
		},
		{
			name:        "with mention priya",
			message:     "@priya レビューして",
			wantWarning: false,
			wantMention: true,
		},
		{
			name:        "with mention amara",
			message:     "@amara 分析してほしい",
			wantWarning: false,
			wantMention: true,
		},
		{
			name:        "with mention mei",
			message:     "@mei 確認お願いします",
			wantWarning: false,
			wantMention: true,
		},
		{
			name:        "with mention rin",
			message:     "@rin UI作成して",
			wantWarning: false,
			wantMention: true,
		},
		{
			name:        "with mention shiori",
			message:     "@shiori テストお願い",
			wantWarning: false,
			wantMention: true,
		},
		{
			name:        "multiple mentions",
			message:     "@yuki @priya これ確認してください",
			wantWarning: false,
			wantMention: true,
		},
		{
			name:        "uppercase mention",
			message:     "@Yuki お願い",
			wantWarning: false,
			wantMention: true,
		},
		{
			name:        "owner mention",
			message:     "@オーナー 確認お願いします",
			wantWarning: false,
			wantMention: true,
		},

		// 空メッセージは警告不要
		{
			name:        "empty message",
			message:     "",
			wantWarning: false,
			wantMention: false,
		},
		{
			name:        "whitespace only",
			message:     "   ",
			wantWarning: false,
			wantMention: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Check(tt.message)

			if result.NeedsWarning != tt.wantWarning {
				t.Errorf("Check(%q).NeedsWarning = %v, want %v", tt.message, result.NeedsWarning, tt.wantWarning)
			}
			if result.HasMention != tt.wantMention {
				t.Errorf("Check(%q).HasMention = %v, want %v", tt.message, result.HasMention, tt.wantMention)
			}
		})
	}
}

func TestFormatWarning(t *testing.T) {
	warning := FormatWarning()
	if warning == "" {
		t.Error("FormatWarning() returned empty string")
	}
	// 新仕様のメッセージを確認
	expected := "メンション先を明示してください（@名前）"
	if warning != expected {
		t.Errorf("FormatWarning() = %q, want %q", warning, expected)
	}
}

func TestNewChecker(t *testing.T) {
	tests := []struct {
		name        string
		agentNames  []string
		message     string
		wantMention bool
	}{
		{
			name:        "custom agents - match",
			agentNames:  []string{"alice", "bob"},
			message:     "@alice お願い",
			wantMention: true,
		},
		{
			name:        "custom agents - no match",
			agentNames:  []string{"alice", "bob"},
			message:     "@yuki お願い",
			wantMention: false,
		},
		{
			name:        "custom agents - title case",
			agentNames:  []string{"alice"},
			message:     "@Alice お願い",
			wantMention: true,
		},
		{
			name:        "empty agents - any mention matches",
			agentNames:  []string{},
			message:     "@anyone お願い",
			wantMention: true,
		},
		{
			name:        "owner mention - Japanese",
			agentNames:  []string{"オーナー"},
			message:     "@オーナー 確認お願い",
			wantMention: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewChecker(tt.agentNames)
			result := checker.Check(tt.message)
			if result.HasMention != tt.wantMention {
				t.Errorf("NewChecker(%v).Check(%q).HasMention = %v, want %v",
					tt.agentNames, tt.message, result.HasMention, tt.wantMention)
			}
		})
	}
}

func TestSetDefaultChecker(t *testing.T) {
	// Save original
	original := defaultChecker
	defer func() { defaultChecker = original }()

	// Set custom checker
	customChecker := NewChecker([]string{"custom"})
	SetDefaultChecker(customChecker)

	// Verify it's used
	result := Check("@custom お願い")
	if !result.HasMention {
		t.Error("SetDefaultChecker did not affect package-level Check")
	}

	result = Check("@yuki お願い")
	if result.HasMention {
		t.Error("Custom checker should not match @yuki")
	}
}
