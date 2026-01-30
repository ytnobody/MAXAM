package router

import (
	"testing"
)

func TestRoute(t *testing.T) {
	agents := []AgentInfo{
		{Name: "mei", Role: "PM"},
		{Name: "yuki", Role: "Backend"},
		{Name: "priya", Role: "Review"},
		{Name: "amara", Role: "Analysis"},
		{Name: "rin", Role: "Frontend"},
		{Name: "shiori", Role: "Test/Docs"},
	}

	tests := []struct {
		name         string
		message      string
		defaultAgent string
		expected     []string
	}{
		{
			name:         "single mention",
			message:      "@yuki 実装お願い",
			defaultAgent: "mei",
			expected:     []string{"yuki"},
		},
		{
			name:         "multiple mentions",
			message:      "@yuki と @priya に確認お願い",
			defaultAgent: "mei",
			expected:     []string{"yuki", "priya"},
		},
		{
			name:         "case insensitive",
			message:      "@YUKI 実装して",
			defaultAgent: "mei",
			expected:     []string{"yuki"},
		},
		{
			name:         "invalid mention ignored",
			message:      "@unknown この人いない",
			defaultAgent: "mei",
			expected:     []string{"mei"},
		},
		{
			name:         "no mention - use default",
			message:      "こんにちは",
			defaultAgent: "mei",
			expected:     []string{"mei"},
		},
		{
			name:         "no mention - custom default",
			message:      "レビューお願い",
			defaultAgent: "priya",
			expected:     []string{"priya"},
		},
		{
			name:         "mention at end",
			message:      "実装お願い @yuki",
			defaultAgent: "mei",
			expected:     []string{"yuki"},
		},
		{
			name:         "mention in middle",
			message:      "ちょっと @priya さんに聞きたい",
			defaultAgent: "mei",
			expected:     []string{"priya"},
		},
		{
			name:         "duplicate mentions",
			message:      "@yuki @yuki 二回書いた",
			defaultAgent: "mei",
			expected:     []string{"yuki"},
		},
		{
			name:         "all agents mentioned",
			message:      "@mei @yuki @priya @amara @rin @shiori 全員集合",
			defaultAgent: "mei",
			expected:     []string{"mei", "yuki", "priya", "amara", "rin", "shiori"},
		},
		{
			name:         "mixed valid and invalid",
			message:      "@yuki @invalid @priya お願い",
			defaultAgent: "mei",
			expected:     []string{"yuki", "priya"},
		},
		{
			name:         "empty default fallback to mei",
			message:      "誰かお願い",
			defaultAgent: "",
			expected:     []string{"mei"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New(agents, tt.defaultAgent)
			result := r.Route(tt.message)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
				return
			}
			for i, agent := range result {
				if agent != tt.expected[i] {
					t.Errorf("expected %v, got %v", tt.expected, result)
					return
				}
			}
		})
	}
}

func TestExtractMentions(t *testing.T) {
	agents := []AgentInfo{
		{Name: "mei", Role: "PM"},
		{Name: "yuki", Role: "Backend"},
	}
	r := New(agents, "mei")

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "no mentions",
			input:    "普通のメッセージ",
			expected: nil,
		},
		{
			name:     "one mention",
			input:    "@yuki",
			expected: []string{"yuki"},
		},
		{
			name:     "mention with text",
			input:    "Hello @mei how are you",
			expected: []string{"mei"},
		},
		{
			name:     "invalid mention only",
			input:    "@unknown",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.extractMentions(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
				return
			}
			for i, agent := range result {
				if agent != tt.expected[i] {
					t.Errorf("expected %v, got %v", tt.expected, result)
					return
				}
			}
		})
	}
}

func TestGetValidNames(t *testing.T) {
	agents := []AgentInfo{
		{Name: "Mei", Role: "PM"},
		{Name: "YUKI", Role: "Backend"},
	}
	r := New(agents, "mei")

	names := r.getValidNames()
	expected := []string{"mei", "yuki"}

	if len(names) != len(expected) {
		t.Errorf("expected %v, got %v", expected, names)
		return
	}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("expected %v, got %v", expected, names)
			return
		}
	}
}
