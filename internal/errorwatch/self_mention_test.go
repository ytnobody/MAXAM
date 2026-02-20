package errorwatch

import (
	"testing"
)

func TestDetectSelfMention(t *testing.T) {
	tests := []struct {
		name          string
		agentName     string
		message       string
		expectViolate bool
		expectCount   int
	}{
		{
			name:          "no mentions",
			agentName:     "yuki",
			message:       "実装完了しました",
			expectViolate: false,
		},
		{
			name:          "mention other agent",
			agentName:     "yuki",
			message:       "@mei レビューお願いします",
			expectViolate: false,
		},
		{
			name:          "self mention",
			agentName:     "yuki",
			message:       "@yuki 完了しました",
			expectViolate: true,
			expectCount:   1,
		},
		{
			name:          "multiple self mentions",
			agentName:     "yuki",
			message:       "@yuki 完了。@yuki 次のタスクに着手",
			expectViolate: true,
			expectCount:   2,
		},
		{
			name:          "mixed mentions with self",
			agentName:     "yuki",
			message:       "@mei PRです。@yuki としては問題なさそう",
			expectViolate: true,
			expectCount:   1,
		},
		{
			name:          "case insensitive",
			agentName:     "yuki",
			message:       "@YUKI 完了しました",
			expectViolate: true,
			expectCount:   1,
		},
		{
			name:          "case insensitive agent name",
			agentName:     "YUKI",
			message:       "@yuki 完了しました",
			expectViolate: true,
			expectCount:   1,
		},
		{
			name:          "empty agent name",
			agentName:     "",
			message:       "@yuki hello",
			expectViolate: false,
		},
		{
			name:          "empty message",
			agentName:     "yuki",
			message:       "",
			expectViolate: false,
		},
		{
			name:          "agent with instance number - exact match",
			agentName:     "yuki-1",
			message:       "@yuki-1 完了",
			expectViolate: true,
			expectCount:   1,
		},
		{
			name:          "agent with instance number - base name match",
			agentName:     "yuki-1",
			message:       "@yuki 完了",
			expectViolate: true,
			expectCount:   1,
		},
		{
			name:          "different instance number",
			agentName:     "yuki-1",
			message:       "@yuki-2 お願い",
			expectViolate: false,
		},
		{
			name:          "mention without at sign",
			agentName:     "yuki",
			message:       "yuki 完了しました",
			expectViolate: false,
		},
		{
			name:          "mention in email-like string",
			agentName:     "yuki",
			message:       "送信先: yuki@example.com",
			expectViolate: false,
		},
		{
			name:          "hyphenated name not instance",
			agentName:     "mei-chen",
			message:       "@mei-chen 確認",
			expectViolate: true,
			expectCount:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violation := DetectSelfMention(tt.agentName, tt.message)

			if tt.expectViolate {
				if violation == nil {
					t.Errorf("expected violation but got nil")
					return
				}
				if len(violation.Mentions) != tt.expectCount {
					t.Errorf("expected %d mentions, got %d: %v", tt.expectCount, len(violation.Mentions), violation.Mentions)
				}
				if violation.AgentName != tt.agentName {
					t.Errorf("expected agent %s, got %s", tt.agentName, violation.AgentName)
				}
			} else {
				if violation != nil {
					t.Errorf("expected no violation but got: %v", violation)
				}
			}
		})
	}
}

func TestSelfMentionViolation_Error(t *testing.T) {
	v := &SelfMentionViolation{
		AgentName: "yuki",
		Mentions:  []string{"@yuki"},
		Message:   "@yuki 完了",
	}

	errMsg := v.Error()
	if !IsSelfMentionError(errMsg) {
		t.Errorf("error message should be identifiable as self-mention error: %s", errMsg)
	}
	if errMsg != "[SELF_MENTION] agent yuki mentioned self: [@yuki]" {
		t.Errorf("unexpected error message: %s", errMsg)
	}
}

func TestIsSelfMentionError(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		expected bool
	}{
		{
			name:     "self mention error",
			errMsg:   "[SELF_MENTION] agent yuki mentioned self: [@yuki]",
			expected: true,
		},
		{
			name:     "context deadline error",
			errMsg:   "context deadline exceeded",
			expected: false,
		},
		{
			name:     "empty string",
			errMsg:   "",
			expected: false,
		},
		{
			name:     "partial match",
			errMsg:   "error: [SELF_MENTION] detected",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSelfMentionError(tt.errMsg)
			if result != tt.expected {
				t.Errorf("IsSelfMentionError(%q) = %v, want %v", tt.errMsg, result, tt.expected)
			}
		})
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"1", true},
		{"123", true},
		{"0", true},
		{"", false},
		{"a", false},
		{"1a", false},
		{"a1", false},
		{"-1", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isNumeric(tt.input)
			if result != tt.expected {
				t.Errorf("isNumeric(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
