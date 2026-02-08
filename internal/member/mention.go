package member

import (
	"strings"
)

// DetectMentions extracts all mentioned member names from text
// Returns lowercase short names of mentioned members
// Also resolves aliases to their canonical names
func (m *Members) DetectMentions(text string) []string {
	lower := strings.ToLower(text)
	var mentioned []string
	seen := make(map[string]bool)

	// Check direct member names
	for name := range m.members {
		// Check @mention pattern
		pattern := "@" + name
		if strings.Contains(lower, pattern) {
			if !seen[name] {
				mentioned = append(mentioned, name)
				seen[name] = true
			}
		}
	}

	// Check aliases
	for alias, canonical := range m.aliases {
		pattern := "@" + alias
		if strings.Contains(lower, pattern) {
			if !seen[canonical] {
				mentioned = append(mentioned, canonical)
				seen[canonical] = true
			}
		}
	}

	// If no explicit mentions, return nil (caller should use default)
	return mentioned
}

// DetectMention returns the first mentioned member, or default if none
// Kept for backward compatibility
func (m *Members) DetectMention(text string, defaultMember string) string {
	mentions := m.DetectMentions(text)
	if len(mentions) == 0 {
		return defaultMember
	}
	return mentions[0]
}

// GetFullName returns the full name for a short name
func (m *Members) GetFullName(shortName string) string {
	if member, ok := m.Get(shortName); ok {
		return member.FullName
	}
	// Return capitalized version as fallback
	if len(shortName) > 0 {
		return strings.ToUpper(shortName[:1]) + shortName[1:]
	}
	return shortName
}
