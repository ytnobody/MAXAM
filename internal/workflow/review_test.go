package workflow

import (
	"strings"
	"testing"

	"github.com/ytnobody/MAXAM/internal/agent"
)

func TestSelectReviewModel(t *testing.T) {
	tests := []struct {
		name  string
		round int
		impl  string
		want  agent.Model
	}{
		{
			name:  "round 1 always default",
			round: 1,
			impl:  "short impl",
			want:  agent.ModelDefault,
		},
		{
			name:  "round 1 with long impl still default",
			round: 1,
			impl:  strings.Repeat("line\n", 200),
			want:  agent.ModelDefault,
		},
		{
			name:  "round 2 with short impl uses haiku",
			round: 2,
			impl:  "short impl",
			want:  agent.ModelHaiku,
		},
		{
			name:  "round 2 with exactly threshold lines uses haiku",
			round: 2,
			impl:  strings.Repeat("line\n", lightweightThreshold-1),
			want:  agent.ModelHaiku,
		},
		{
			name:  "round 2 with over threshold uses default",
			round: 2,
			impl:  strings.Repeat("line\n", lightweightThreshold+50),
			want:  agent.ModelDefault,
		},
		{
			name:  "round 3 with short impl uses haiku",
			round: 3,
			impl:  strings.Repeat("line\n", 50),
			want:  agent.ModelHaiku,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectReviewModel(tt.round, tt.impl)
			if got != tt.want {
				t.Errorf("selectReviewModel(%d, impl) = %v, want %v", tt.round, got, tt.want)
			}
		})
	}
}

func TestParseTags(t *testing.T) {
	tests := []struct {
		review string
		want   []string
	}{
		{
			review: "APPROVED",
			want:   []string{},
		},
		{
			review: "[MINOR] typo fix needed",
			want:   []string{"MINOR"},
		},
		{
			review: "[DESIGN] needs refactor\n[MINOR] also typo",
			want:   []string{"MINOR", "DESIGN"},
		},
		{
			review: "[SEC-CRITICAL] security issue!",
			want:   []string{"SEC-CRITICAL"},
		},
		{
			review: "[REQUIREMENTS] unclear spec",
			want:   []string{"REQUIREMENTS"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.review[:min(len(tt.review), 20)], func(t *testing.T) {
			got := parseTags(tt.review)
			if len(got) != len(tt.want) {
				t.Errorf("parseTags() = %v, want %v", got, tt.want)
				return
			}
			// Check all expected tags are present
			for _, want := range tt.want {
				found := false
				for _, g := range got {
					if g == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("parseTags() missing tag %q, got %v", want, got)
				}
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
