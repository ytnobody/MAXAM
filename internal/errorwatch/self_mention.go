// Package errorwatch provides error detection and automatic follow-up for agent errors.
package errorwatch

import (
	"fmt"
	"regexp"
	"strings"
)

// SelfMentionErrorPrefix is the prefix for self-mention error messages.
// This prefix is used to identify self-mention errors in IsRecoverableError.
const SelfMentionErrorPrefix = "[SELF_MENTION]"

// SelfMentionViolation represents a detected self-mention violation.
type SelfMentionViolation struct {
	AgentName string
	Mentions  []string
	Message   string
}

// Error returns the error message for the violation.
func (v *SelfMentionViolation) Error() string {
	return fmt.Sprintf("%s agent %s mentioned self: %v", SelfMentionErrorPrefix, v.AgentName, v.Mentions)
}

// mentionPattern matches @mentions in messages.
var mentionPattern = regexp.MustCompile(`@([a-zA-Z][\w-]*)`)

// DetectSelfMention checks if a message contains self-mentions.
// agentName is the name of the agent sending the message.
// message is the content to check.
// Returns a SelfMentionViolation if self-mention is detected, nil otherwise.
func DetectSelfMention(agentName, message string) *SelfMentionViolation {
	if agentName == "" || message == "" {
		return nil
	}

	// Normalize agent name for comparison (case-insensitive)
	normalizedAgent := strings.ToLower(agentName)
	// Handle instance suffixes (e.g., "yuki-1" should match "@yuki")
	baseAgentName := normalizedAgent
	if idx := strings.LastIndex(normalizedAgent, "-"); idx > 0 {
		// Check if suffix is numeric (instance number)
		suffix := normalizedAgent[idx+1:]
		if isNumeric(suffix) {
			baseAgentName = normalizedAgent[:idx]
		}
	}

	matches := mentionPattern.FindAllStringSubmatch(message, -1)
	if matches == nil {
		return nil
	}

	var selfMentions []string
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		mentionedName := strings.ToLower(match[1])
		// Check both full name (yuki-1) and base name (yuki)
		if mentionedName == normalizedAgent || mentionedName == baseAgentName {
			selfMentions = append(selfMentions, match[0])
		}
	}

	if len(selfMentions) == 0 {
		return nil
	}

	return &SelfMentionViolation{
		AgentName: agentName,
		Mentions:  selfMentions,
		Message:   message,
	}
}

// isNumeric checks if a string contains only numeric characters.
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// IsSelfMentionError checks if an error message is a self-mention error.
// This is used by IsRecoverableError to identify self-mention violations.
func IsSelfMentionError(errMsg string) bool {
	return strings.Contains(errMsg, SelfMentionErrorPrefix)
}
