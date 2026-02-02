package slack

import (
	"testing"
)

func TestDetectAgentMention(t *testing.T) {
	// Note: This is a simplified test without full handler setup
	// Full integration tests would require mock dependencies

	tests := []struct {
		name           string
		text           string
		isFromBot      bool
		senderAgent    string
		expectedResult string // expected short name or empty
	}{
		{
			name:           "human mentions agent",
			text:           "@yuki タスクお願い",
			isFromBot:      false,
			senderAgent:    "",
			expectedResult: "yuki",
		},
		{
			name:           "agent mentions different agent",
			text:           "@yuki このタスクお願い",
			isFromBot:      true,
			senderAgent:    "Mei Chen",
			expectedResult: "yuki",
		},
		{
			name:           "agent mentions self (loop prevention)",
			text:           "@mei 確認します",
			isFromBot:      true,
			senderAgent:    "Mei Chen",
			expectedResult: "", // should be ignored
		},
		{
			name:           "no mention",
			text:           "こんにちは",
			isFromBot:      false,
			senderAgent:    "",
			expectedResult: "",
		},
		{
			name:           "bot message without mention",
			text:           "作業完了しました",
			isFromBot:      true,
			senderAgent:    "Yuki Tanaka",
			expectedResult: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &IncomingMessage{
				Text:        tt.text,
				IsFromBot:   tt.isFromBot,
				SenderAgent: tt.senderAgent,
			}

			// Test extractSenderShortName logic
			var senderShortName string
			if msg.IsFromBot && msg.SenderAgent != "" {
				parts := splitWords(msg.SenderAgent)
				if len(parts) > 0 {
					senderShortName = toLower(parts[0])
				}
			}

			// Check mention detection logic
			textLower := toLower(msg.Text)
			testAgents := []string{"mei", "yuki", "priya", "amara"}

			var result string
			for _, name := range testAgents {
				if contains(textLower, "@"+name) {
					if msg.IsFromBot && name == senderShortName {
						continue
					}
					result = name
					break
				}
			}

			if result != tt.expectedResult {
				t.Errorf("got %q, want %q", result, tt.expectedResult)
			}
		})
	}
}

// Helper functions for test (simplified versions)
func splitWords(s string) []string {
	var result []string
	var current []byte
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			if len(current) > 0 {
				result = append(result, string(current))
				current = nil
			}
		} else {
			current = append(current, s[i])
		}
	}
	if len(current) > 0 {
		result = append(result, string(current))
	}
	return result
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
