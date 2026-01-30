package router

import (
	"testing"
)

func TestParseResult(t *testing.T) {
	agents := []AgentInfo{
		{Name: "mei", Role: "PM"},
		{Name: "yuki", Role: "Backend"},
		{Name: "priya", Role: "Review"},
		{Name: "amara", Role: "Analysis"},
		{Name: "rin", Role: "Frontend"},
		{Name: "shiori", Role: "Test/Docs"},
	}
	r := New(agents)

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "valid JSON",
			input:    `{"agents": ["yuki"]}`,
			expected: []string{"yuki"},
		},
		{
			name:     "multiple agents",
			input:    `{"agents": ["yuki", "priya"]}`,
			expected: []string{"yuki", "priya"},
		},
		{
			name:     "JSON with extra text",
			input:    `Here is the result: {"agents": ["amara"]} That's all.`,
			expected: []string{"amara"},
		},
		{
			name:     "fallback to name detection",
			input:    `I think Yuki should handle this.`,
			expected: []string{"yuki"},
		},
		{
			name:     "multiple names in text",
			input:    `Both Yuki and Priya should review this.`,
			expected: []string{"yuki", "priya"},
		},
		{
			name:     "invalid agent name filtered",
			input:    `{"agents": ["yuki", "unknown"]}`,
			expected: []string{"yuki"},
		},
		{
			name:     "empty result",
			input:    `{}`,
			expected: []string{},
		},
		{
			name:     "rin and shiori",
			input:    `{"agents": ["rin", "shiori"]}`,
			expected: []string{"rin", "shiori"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.parseResult(tt.input)
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

func TestValidateAgents(t *testing.T) {
	agents := []AgentInfo{
		{Name: "mei", Role: "PM"},
		{Name: "yuki", Role: "Backend"},
	}
	r := New(agents)

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "valid agents",
			input:    []string{"mei", "yuki"},
			expected: []string{"mei", "yuki"},
		},
		{
			name:     "filter invalid",
			input:    []string{"mei", "invalid", "yuki"},
			expected: []string{"mei", "yuki"},
		},
		{
			name:     "remove duplicates",
			input:    []string{"mei", "mei", "yuki"},
			expected: []string{"mei", "yuki"},
		},
		{
			name:     "case insensitive",
			input:    []string{"MEI", "Yuki"},
			expected: []string{"mei", "yuki"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.validateAgents(tt.input)
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

func TestBuildPrompt(t *testing.T) {
	agents := []AgentInfo{
		{Name: "mei", Role: "PM / 要件定義"},
		{Name: "yuki", Role: "バックエンド / インフラ"},
	}
	r := New(agents)

	history := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "mei", Content: "Hi!"},
	}

	prompt := r.buildPrompt("Test message", history)

	// Check prompt contains required sections
	if !contains(prompt, "mei") {
		t.Error("prompt should contain agent mei")
	}
	if !contains(prompt, "yuki") {
		t.Error("prompt should contain agent yuki")
	}
	if !contains(prompt, "Test message") {
		t.Error("prompt should contain the message")
	}
	if !contains(prompt, "Hello") {
		t.Error("prompt should contain history")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"this is a long string", 10, "this is a ..."},
		{"exact", 5, "exact"},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
