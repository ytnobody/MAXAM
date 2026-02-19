package errorwatch

import (
	"testing"
	"time"
)

func TestIsRecoverableError(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		expected bool
	}{
		{
			name:     "context deadline exceeded",
			errMsg:   "context deadline exceeded",
			expected: true,
		},
		{
			name:     "context deadline exceeded with prefix",
			errMsg:   "error running agent: context deadline exceeded",
			expected: true,
		},
		{
			name:     "context canceled",
			errMsg:   "context canceled",
			expected: true,
		},
		{
			name:     "timeout error",
			errMsg:   "request timeout",
			expected: true,
		},
		{
			name:     "connection refused",
			errMsg:   "dial tcp: connection refused",
			expected: true,
		},
		{
			name:     "connection reset",
			errMsg:   "connection reset by peer",
			expected: true,
		},
		{
			name:     "case insensitive",
			errMsg:   "CONTEXT DEADLINE EXCEEDED",
			expected: true,
		},
		{
			name:     "unknown error",
			errMsg:   "something went wrong",
			expected: false,
		},
		{
			name:     "empty error",
			errMsg:   "",
			expected: false,
		},
		{
			name:     "syntax error",
			errMsg:   "syntax error: unexpected token",
			expected: false,
		},
		// LLM API errors
		{
			name:     "rate limit error",
			errMsg:   "rate limit exceeded",
			expected: true,
		},
		{
			name:     "rate_limit with underscore",
			errMsg:   "error: rate_limit_exceeded",
			expected: true,
		},
		{
			name:     "429 error",
			errMsg:   "HTTP 429: Too Many Requests",
			expected: true,
		},
		{
			name:     "503 error",
			errMsg:   "503 Service Unavailable",
			expected: true,
		},
		{
			name:     "overloaded",
			errMsg:   "The model is currently overloaded",
			expected: true,
		},
		{
			name:     "capacity",
			errMsg:   "No capacity available",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRecoverableError(tt.errMsg)
			if result != tt.expected {
				t.Errorf("IsRecoverableError(%q) = %v, want %v", tt.errMsg, result, tt.expected)
			}
		})
	}
}

func TestDetectorShouldFollowUp(t *testing.T) {
	t.Run("first error should trigger follow-up", func(t *testing.T) {
		d := DefaultDetector()
		if !d.ShouldFollowUp("yuki", "context deadline exceeded") {
			t.Error("first error should trigger follow-up")
		}
	})

	t.Run("non-recoverable error should not trigger follow-up", func(t *testing.T) {
		d := DefaultDetector()
		if d.ShouldFollowUp("yuki", "syntax error") {
			t.Error("non-recoverable error should not trigger follow-up")
		}
	})

	t.Run("second error within cooldown should not trigger follow-up", func(t *testing.T) {
		d := NewDetector(100 * time.Millisecond)

		// First error
		result := d.CheckAndGenerateFollowUp("yuki", "context deadline exceeded")
		if !result.ShouldSend {
			t.Error("first error should trigger follow-up")
		}

		// Second error immediately
		if d.ShouldFollowUp("yuki", "context deadline exceeded") {
			t.Error("second error within cooldown should not trigger follow-up")
		}
	})

	t.Run("error after cooldown should trigger follow-up", func(t *testing.T) {
		d := NewDetector(50 * time.Millisecond)

		// First error
		result := d.CheckAndGenerateFollowUp("yuki", "context deadline exceeded")
		if !result.ShouldSend {
			t.Error("first error should trigger follow-up")
		}

		// Wait for cooldown
		time.Sleep(60 * time.Millisecond)

		// Second error after cooldown
		if !d.ShouldFollowUp("yuki", "context deadline exceeded") {
			t.Error("error after cooldown should trigger follow-up")
		}
	})

	t.Run("different agents have separate cooldowns", func(t *testing.T) {
		d := NewDetector(100 * time.Millisecond)

		// First error for yuki
		result := d.CheckAndGenerateFollowUp("yuki", "context deadline exceeded")
		if !result.ShouldSend {
			t.Error("first error for yuki should trigger follow-up")
		}

		// First error for rin (should still trigger)
		if !d.ShouldFollowUp("rin", "context deadline exceeded") {
			t.Error("first error for rin should trigger follow-up")
		}
	})
}

func TestFollowUpMessage(t *testing.T) {
	tests := []struct {
		agentName string
		expected  string
	}{
		{"yuki", "@yuki 大丈夫？"},
		{"rin", "@rin 大丈夫？"},
		{"mei", "@mei 大丈夫？"},
	}

	for _, tt := range tests {
		t.Run(tt.agentName, func(t *testing.T) {
			result := FollowUpMessage(tt.agentName)
			if result != tt.expected {
				t.Errorf("FollowUpMessage(%q) = %q, want %q", tt.agentName, result, tt.expected)
			}
		})
	}
}

func TestCheckAndGenerateFollowUp(t *testing.T) {
	t.Run("generates follow-up for recoverable error", func(t *testing.T) {
		d := DefaultDetector()
		result := d.CheckAndGenerateFollowUp("yuki", "context deadline exceeded")

		if !result.ShouldSend {
			t.Error("should send follow-up for recoverable error")
		}
		if result.AgentName != "yuki" {
			t.Errorf("AgentName = %q, want %q", result.AgentName, "yuki")
		}
		if result.Message != "@yuki 大丈夫？" {
			t.Errorf("Message = %q, want %q", result.Message, "@yuki 大丈夫？")
		}
	})

	t.Run("does not generate follow-up for non-recoverable error", func(t *testing.T) {
		d := DefaultDetector()
		result := d.CheckAndGenerateFollowUp("yuki", "syntax error")

		if result.ShouldSend {
			t.Error("should not send follow-up for non-recoverable error")
		}
	})

	t.Run("records follow-up to prevent duplicates", func(t *testing.T) {
		d := NewDetector(100 * time.Millisecond)

		// First call
		result1 := d.CheckAndGenerateFollowUp("yuki", "context deadline exceeded")
		if !result1.ShouldSend {
			t.Error("first call should send follow-up")
		}

		// Second call immediately
		result2 := d.CheckAndGenerateFollowUp("yuki", "context deadline exceeded")
		if result2.ShouldSend {
			t.Error("second call within cooldown should not send follow-up")
		}
	})
}

func TestReset(t *testing.T) {
	d := NewDetector(1 * time.Hour) // Long cooldown

	// Record follow-up
	d.CheckAndGenerateFollowUp("yuki", "context deadline exceeded")

	// Should not trigger (within cooldown)
	if d.ShouldFollowUp("yuki", "context deadline exceeded") {
		t.Error("should not trigger within cooldown")
	}

	// Reset
	d.Reset("yuki")

	// Should trigger again
	if !d.ShouldFollowUp("yuki", "context deadline exceeded") {
		t.Error("should trigger after reset")
	}
}

func TestResetAll(t *testing.T) {
	d := NewDetector(1 * time.Hour) // Long cooldown

	// Record follow-ups for multiple agents
	d.CheckAndGenerateFollowUp("yuki", "context deadline exceeded")
	d.CheckAndGenerateFollowUp("rin", "context deadline exceeded")

	// Reset all
	d.ResetAll()

	// Both should trigger again
	if !d.ShouldFollowUp("yuki", "context deadline exceeded") {
		t.Error("yuki should trigger after reset all")
	}
	if !d.ShouldFollowUp("rin", "context deadline exceeded") {
		t.Error("rin should trigger after reset all")
	}
}
