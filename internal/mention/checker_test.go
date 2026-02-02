package mention

import (
	"testing"
)

func TestCheck(t *testing.T) {
	tests := []struct {
		name        string
		message     string
		wantWarning bool
		wantRequest bool
		wantMention bool
	}{
		// 警告が必要なケース（メンションなし）- 新仕様：全メッセージにメンション必須
		{
			name:        "no mention - request pattern",
			message:     "これお願い",
			wantWarning: true,
			wantRequest: true,
			wantMention: false,
		},
		{
			name:        "no mention - simple statement",
			message:     "これはただのコメントです",
			wantWarning: true,
			wantRequest: false,
			wantMention: false,
		},
		{
			name:        "no mention - question",
			message:     "どう思う？",
			wantWarning: true,
			wantRequest: false,
			wantMention: false,
		},
		{
			name:        "no mention - completion report",
			message:     "実装完了しました",
			wantWarning: true,
			wantRequest: false,
			wantMention: false,
		},

		// 警告不要なケース（メンションあり）
		{
			name:        "with mention yuki",
			message:     "@yuki これお願い",
			wantWarning: false,
			wantRequest: true,
			wantMention: true,
		},
		{
			name:        "with mention priya",
			message:     "@priya レビューして",
			wantWarning: false,
			wantRequest: true,
			wantMention: true,
		},
		{
			name:        "with mention amara",
			message:     "@amara 分析してほしい",
			wantWarning: false,
			wantRequest: true,
			wantMention: true,
		},
		{
			name:        "with mention mei",
			message:     "@mei 確認お願いします",
			wantWarning: false,
			wantRequest: true,
			wantMention: true,
		},
		{
			name:        "with mention rin",
			message:     "@rin UI作成して",
			wantWarning: false,
			wantRequest: true,
			wantMention: true,
		},
		{
			name:        "with mention shiori",
			message:     "@shiori テストお願い",
			wantWarning: false,
			wantRequest: true,
			wantMention: true,
		},
		{
			name:        "with mention - no request pattern",
			message:     "@yuki 完了したよ",
			wantWarning: false,
			wantRequest: false,
			wantMention: true,
		},
		{
			name:        "with owner mention",
			message:     "@オーナー 確認お願いします",
			wantWarning: false,
			wantRequest: true,
			wantMention: true,
		},

		// エッジケース
		{
			name:        "empty message",
			message:     "",
			wantWarning: true, // 空でもメンションがないので警告
			wantRequest: false,
			wantMention: false,
		},
		{
			name:        "multiple mentions with request",
			message:     "@yuki @priya これ確認してください",
			wantWarning: false,
			wantRequest: true,
			wantMention: true,
		},
		{
			name:        "uppercase mention",
			message:     "@Yuki お願い",
			wantWarning: false,
			wantRequest: true,
			wantMention: true,
		},
		{
			name:        "owner and member mention",
			message:     "@オーナー @Mei 相談があります",
			wantWarning: false,
			wantRequest: false,
			wantMention: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Check(tt.message)

			if result.NeedsWarning != tt.wantWarning {
				t.Errorf("Check(%q).NeedsWarning = %v, want %v", tt.message, result.NeedsWarning, tt.wantWarning)
			}
			if result.HasRequestPattern != tt.wantRequest {
				t.Errorf("Check(%q).HasRequestPattern = %v, want %v", tt.message, result.HasRequestPattern, tt.wantRequest)
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
			name:        "owner mention always works",
			agentNames:  []string{"alice", "bob"},
			message:     "@オーナー お願い",
			wantMention: true,
		},
		{
			name:        "owner mention with custom agents",
			agentNames:  []string{"custom"},
			message:     "@オーナー 確認",
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
