package mode

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantCommand bool
		wantCmd     Command
		wantArgs    string
	}{
		{
			name:        "/plan command",
			input:       "/plan",
			wantCommand: true,
			wantCmd:     CommandPlan,
			wantArgs:    "",
		},
		{
			name:        "/plan with args",
			input:       "/plan some project analysis",
			wantCommand: true,
			wantCmd:     CommandPlan,
			wantArgs:    "some project analysis",
		},
		{
			name:        "/stop command",
			input:       "/stop",
			wantCommand: true,
			wantCmd:     CommandStop,
			wantArgs:    "",
		},
		{
			name:        "/status command",
			input:       "/status",
			wantCommand: true,
			wantCmd:     CommandStatus,
			wantArgs:    "",
		},
		{
			name:        "regular message",
			input:       "hello team",
			wantCommand: false,
		},
		{
			name:        "unknown command",
			input:       "/unknown",
			wantCommand: false,
		},
		{
			name:        "empty input",
			input:       "",
			wantCommand: false,
		},
		{
			name:        "/plan with whitespace",
			input:       "  /plan  ",
			wantCommand: true,
			wantCmd:     CommandPlan,
			wantArgs:    "",
		},
		{
			name:        "text containing /plan but not starting with it",
			input:       "let's /plan this",
			wantCommand: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Parse(tt.input)
			if result.IsCommand != tt.wantCommand {
				t.Errorf("IsCommand = %v, want %v", result.IsCommand, tt.wantCommand)
			}
			if tt.wantCommand {
				if result.Command != tt.wantCmd {
					t.Errorf("Command = %q, want %q", result.Command, tt.wantCmd)
				}
				if result.Args != tt.wantArgs {
					t.Errorf("Args = %q, want %q", result.Args, tt.wantArgs)
				}
			}
		})
	}
}

func TestIsCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "starts with slash",
			input:    "/plan",
			expected: true,
		},
		{
			name:     "slash with space prefix",
			input:    "  /plan",
			expected: true,
		},
		{
			name:     "regular text",
			input:    "hello",
			expected: false,
		},
		{
			name:     "slash in middle",
			input:    "hello/world",
			expected: false,
		},
		{
			name:     "empty",
			input:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsCommand(tt.input)
			if got != tt.expected {
				t.Errorf("IsCommand(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
