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
			name:        "/auto command",
			input:       "/auto",
			wantCommand: true,
			wantCmd:     CommandAuto,
			wantArgs:    "",
		},
		{
			name:        "/auto with args",
			input:       "/auto yuki",
			wantCommand: true,
			wantCmd:     CommandAuto,
			wantArgs:    "yuki",
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
		{
			name:        "/kill command",
			input:       "/kill",
			wantCommand: true,
			wantCmd:     CommandKill,
			wantArgs:    "",
		},
		{
			name:        "/kill with agent",
			input:       "/kill @mei",
			wantCommand: true,
			wantCmd:     CommandKill,
			wantArgs:    "@mei",
		},
		{
			name:        "/kill with multiple agents",
			input:       "/kill @mei @yuki",
			wantCommand: true,
			wantCmd:     CommandKill,
			wantArgs:    "@mei @yuki",
		},
		{
			name:        "/kill all",
			input:       "/kill all",
			wantCommand: true,
			wantCmd:     CommandKill,
			wantArgs:    "all",
		},
		{
			name:        "/reset command",
			input:       "/reset",
			wantCommand: true,
			wantCmd:     CommandReset,
			wantArgs:    "",
		},
		{
			name:        "/reset with agent",
			input:       "/reset @mei",
			wantCommand: true,
			wantCmd:     CommandReset,
			wantArgs:    "@mei",
		},
		{
			name:        "/reset with multiple agents",
			input:       "/reset @mei @yuki",
			wantCommand: true,
			wantCmd:     CommandReset,
			wantArgs:    "@mei @yuki",
		},
		{
			name:        "/reset all",
			input:       "/reset all",
			wantCommand: true,
			wantCmd:     CommandReset,
			wantArgs:    "all",
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
