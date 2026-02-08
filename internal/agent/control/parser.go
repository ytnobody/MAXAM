package control

import (
	"regexp"
	"strings"
)

// CommandType represents the type of control command
type CommandType int

const (
	// CommandUnknown represents an unrecognized command
	CommandUnknown CommandType = iota
	// CommandStop represents a stop command
	CommandStop
	// CommandResume represents a resume command
	CommandResume
	// CommandStopAll represents a stop all command
	CommandStopAll
	// CommandResumeAll represents a resume all command
	CommandResumeAll
)

func (c CommandType) String() string {
	switch c {
	case CommandStop:
		return "stop"
	case CommandResume:
		return "resume"
	case CommandStopAll:
		return "stop_all"
	case CommandResumeAll:
		return "resume_all"
	default:
		return "unknown"
	}
}

// Command represents a parsed control command
type Command struct {
	Type   CommandType
	Target string // Agent name for individual commands, empty for all commands
}

// Parser parses control commands from input text
type Parser struct {
	// mentionPattern matches @agent stop or @agent resume
	mentionPattern *regexp.Regexp
	// globalPattern matches "stop all" or "resume all"
	globalPattern *regexp.Regexp
}

// NewParser creates a new control command parser
func NewParser() *Parser {
	return &Parser{
		// Match @agentname stop or @agentname resume (case insensitive)
		mentionPattern: regexp.MustCompile(`(?i)^@(\w+)\s+(stop|resume)\s*$`),
		// Match "stop all" or "resume all" (case insensitive)
		globalPattern: regexp.MustCompile(`(?i)^(stop|resume)\s+all\s*$`),
	}
}

// Parse parses the input text and returns a Command if recognized
func (p *Parser) Parse(input string) *Command {
	input = strings.TrimSpace(input)

	// Check for global commands first (stop all, resume all)
	if matches := p.globalPattern.FindStringSubmatch(input); len(matches) == 2 {
		action := strings.ToLower(matches[1])
		if action == "stop" {
			return &Command{Type: CommandStopAll}
		}
		return &Command{Type: CommandResumeAll}
	}

	// Check for mention commands (@agent stop, @agent resume)
	if matches := p.mentionPattern.FindStringSubmatch(input); len(matches) == 3 {
		target := matches[1]
		action := strings.ToLower(matches[2])
		if action == "stop" {
			return &Command{Type: CommandStop, Target: target}
		}
		return &Command{Type: CommandResume, Target: target}
	}

	return nil
}

// IsControlCommand returns true if the input is a control command
func (p *Parser) IsControlCommand(input string) bool {
	return p.Parse(input) != nil
}
