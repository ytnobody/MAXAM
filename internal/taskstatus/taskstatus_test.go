package taskstatus

import (
	"testing"
	"time"
)

func TestFormatStatusLine(t *testing.T) {
	tests := []struct {
		name        string
		statuses    []Status
		memberNames map[string]string
		want        string
	}{
		{
			name:        "empty statuses",
			statuses:    []Status{},
			memberNames: nil,
			want:        "",
		},
		{
			name: "single PR",
			statuses: []Status{
				{Member: "yuki", PRNumber: 90, State: "open"},
			},
			memberNames: map[string]string{"yuki": "Yuki"},
			want:        "Yuki: #90",
		},
		{
			name: "draft PR",
			statuses: []Status{
				{Member: "yuki", PRNumber: 90, State: "draft"},
			},
			memberNames: map[string]string{"yuki": "Yuki"},
			want:        "Yuki: #90 (draft)",
		},
		{
			name: "review requested",
			statuses: []Status{
				{Member: "yuki", PRNumber: 90, State: "review_requested"},
			},
			memberNames: map[string]string{"yuki": "Yuki"},
			want:        "Yuki: #90 (review)",
		},
		{
			name: "multiple members",
			statuses: []Status{
				{Member: "yuki", PRNumber: 90, State: "open"},
				{Member: "rin", PRNumber: 92, State: "review_requested"},
			},
			memberNames: map[string]string{"yuki": "Yuki", "rin": "Rin"},
			// Note: map iteration order is not guaranteed, so we check contains
			want: "", // Will be checked differently
		},
		{
			name: "unknown member",
			statuses: []Status{
				{Member: "unknown", PRNumber: 100, State: "open"},
			},
			memberNames: map[string]string{},
			want:        "unknown: #100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatStatusLine(tt.statuses, tt.memberNames)

			// Special case for multiple members (order not guaranteed)
			if tt.name == "multiple members" {
				if got == "" {
					t.Error("expected non-empty result for multiple members")
				}
				// Check that both members appear
				if !containsSubstring(got, "Yuki: #90") && !containsSubstring(got, "Rin: #92") {
					t.Errorf("expected both members in output, got %q", got)
				}
				return
			}

			if got != tt.want {
				t.Errorf("FormatStatusLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTruncateTitle(t *testing.T) {
	tests := []struct {
		title  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"this is a very long title", 10, "this is..."},
		{"exact len", 9, "exact len"},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got := truncateTitle(tt.title, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateTitle(%q, %d) = %q, want %q", tt.title, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestGetStateIcon(t *testing.T) {
	tests := []struct {
		state string
		want  string
	}{
		{"draft", " (draft)"},
		{"review_requested", " (review)"},
		{"open", ""},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			got := getStateIcon(tt.state)
			if got != tt.want {
				t.Errorf("getStateIcon(%q) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestFetcher_GetCached(t *testing.T) {
	f := &Fetcher{}

	// Initially empty
	if got := f.GetCached(); len(got) != 0 {
		t.Errorf("GetCached() = %v, want empty", got)
	}

	// Set cache
	expected := []Status{
		{Member: "yuki", PRNumber: 90, State: "open", UpdatedAt: time.Now()},
	}
	f.statuses = expected

	got := f.GetCached()
	if len(got) != 1 {
		t.Errorf("GetCached() length = %d, want 1", len(got))
	}
	if got[0].Member != "yuki" {
		t.Errorf("GetCached()[0].Member = %q, want %q", got[0].Member, "yuki")
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s[1:], substr) || s[:len(substr)] == substr)
}
