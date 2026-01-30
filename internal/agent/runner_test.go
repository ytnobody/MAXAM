package agent

import "testing"

func TestModelConstants(t *testing.T) {
	tests := []struct {
		model Model
		want  string
	}{
		{ModelDefault, ""},
		{ModelHaiku, "haiku"},
		{ModelSonnet, "sonnet"},
		{ModelOpus, "opus"},
	}

	for _, tt := range tests {
		if got := string(tt.model); got != tt.want {
			t.Errorf("Model %v = %q, want %q", tt.model, got, tt.want)
		}
	}
}
