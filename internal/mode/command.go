package mode

import (
	"strings"
)

// Command represents a mode-related command
type Command string

const (
	// CommandPlan enters plan mode
	CommandPlan Command = "/plan"
	// CommandAuto enters auto mode directly (without plan)
	CommandAuto Command = "/auto"
	// CommandStop stops current mode and returns to interactive
	CommandStop Command = "/stop"
	// CommandStatus shows current mode status
	CommandStatus Command = "/status"
	// CommandResetAuto resets agent auto mode
	CommandResetAuto Command = "/resetauto"
)

// ParseResult contains the result of parsing user input for commands
type ParseResult struct {
	IsCommand bool
	Command   Command
	Args      string // remaining text after command
}

// Parse parses user input and extracts any mode command
func Parse(input string) ParseResult {
	trimmed := strings.TrimSpace(input)

	// Check for known commands
	commands := []Command{CommandPlan, CommandAuto, CommandStop, CommandStatus, CommandResetAuto}
	for _, cmd := range commands {
		cmdStr := string(cmd)
		if trimmed == cmdStr {
			return ParseResult{
				IsCommand: true,
				Command:   cmd,
				Args:      "",
			}
		}
		if strings.HasPrefix(trimmed, cmdStr+" ") {
			return ParseResult{
				IsCommand: true,
				Command:   cmd,
				Args:      strings.TrimSpace(trimmed[len(cmdStr):]),
			}
		}
	}

	return ParseResult{IsCommand: false}
}

// IsCommand checks if the input starts with a command prefix
func IsCommand(input string) bool {
	trimmed := strings.TrimSpace(input)
	return strings.HasPrefix(trimmed, "/")
}
