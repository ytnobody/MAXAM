package mention

import (
	"testing"
)

func TestCheck(t *testing.T) {
	tests := []struct {
		name         string
		message      string
		wantWarning  bool
		wantRequest  bool
		wantMention  bool
	}{
		// 警告が必要なケース（依頼あり、メンションなし）
		{
			name:        "request without mention - onegai",
			message:     "これお願い",
			wantWarning: true,
			wantRequest: true,
			wantMention: false,
		},
		{
			name:        "request without mention - shite",
			message:     "確認して",
			wantWarning: true,
			wantRequest: true,
			wantMention: false,
		},
		{
			name:        "request without mention - review",
			message:     "レビューしてください",
			wantWarning: true,
			wantRequest: true,
			wantMention: false,
		},
		{
			name:        "request without mention - yoroshiku",
			message:     "よろしくお願いします",
			wantWarning: true,
			wantRequest: true,
			wantMention: false,
		},
		{
			name:        "request without mention - tanomu",
			message:     "実装頼む",
			wantWarning: true,
			wantRequest: true,
			wantMention: false,
		},
		{
			name:        "request without mention - dekiru",
			message:     "これできる?",
			wantWarning: true,
			wantRequest: true,
			wantMention: false,
		},

		// 警告不要なケース（依頼あり、メンションあり）
		{
			name:        "request with mention yuki",
			message:     "@yuki これお願い",
			wantWarning: false,
			wantRequest: true,
			wantMention: true,
		},
		{
			name:        "request with mention priya",
			message:     "@priya レビューして",
			wantWarning: false,
			wantRequest: true,
			wantMention: true,
		},
		{
			name:        "request with mention amara",
			message:     "@amara 分析してほしい",
			wantWarning: false,
			wantRequest: true,
			wantMention: true,
		},
		{
			name:        "request with mention mei",
			message:     "@mei 確認お願いします",
			wantWarning: false,
			wantRequest: true,
			wantMention: true,
		},
		{
			name:        "request with mention rin",
			message:     "@rin UI作成して",
			wantWarning: false,
			wantRequest: true,
			wantMention: true,
		},
		{
			name:        "request with mention shiori",
			message:     "@shiori テストお願い",
			wantWarning: false,
			wantRequest: true,
			wantMention: true,
		},

		// 警告不要なケース（依頼なし）
		{
			name:        "no request pattern",
			message:     "これはただのコメントです",
			wantWarning: false,
			wantRequest: false,
			wantMention: false,
		},
		{
			name:        "question without request",
			message:     "どう思う？",
			wantWarning: false,
			wantRequest: false,
			wantMention: false,
		},
		{
			name:        "statement",
			message:     "実装完了しました",
			wantWarning: false,
			wantRequest: false,
			wantMention: false,
		},
		{
			name:        "mention without request",
			message:     "@yuki 完了したよ",
			wantWarning: false,
			wantRequest: false,
			wantMention: true,
		},

		// エッジケース
		{
			name:        "empty message",
			message:     "",
			wantWarning: false,
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
		name       string
		agentNames []string
		message    string
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
