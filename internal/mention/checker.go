// Package mention provides mention leak detection for team chat messages.
package mention

import (
	"regexp"
	"strings"
)

// Result represents the result of a mention check
type Result struct {
	HasMention   bool // メンションが含まれているか
	NeedsWarning bool // 警告が必要か（メンションなし）
}

// Checker detects mention leaks in messages
type Checker struct {
	mentionPattern *regexp.Regexp
}

// NewChecker creates a Checker with the given agent names
func NewChecker(agentNames []string) *Checker {
	if len(agentNames) == 0 {
		// fallback: allow any @mention
		return &Checker{
			mentionPattern: regexp.MustCompile(`@\w+`),
		}
	}

	// Build pattern: @(name1|name2|Name1|Name2|...)
	var parts []string
	for _, name := range agentNames {
		// Escape special regex characters
		escaped := regexp.QuoteMeta(name)
		parts = append(parts, escaped)

		// For ASCII names, also add lowercase and title case variants
		lower := strings.ToLower(name)
		if lower != name && isASCII(lower) {
			parts = append(parts, regexp.QuoteMeta(lower))
		}
		if len(lower) > 0 && isASCII(lower) {
			titled := strings.ToUpper(string(lower[0])) + lower[1:]
			if titled != name && titled != lower {
				parts = append(parts, regexp.QuoteMeta(titled))
			}
		}
	}

	pattern := `@(` + strings.Join(parts, "|") + `)`
	return &Checker{
		mentionPattern: regexp.MustCompile(pattern),
	}
}

// isASCII checks if a string contains only ASCII characters
func isASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}

// Check analyzes a message for mention leaks
// 新仕様: すべてのメッセージにメンション必須。メンションなしは警告対象。
func (c *Checker) Check(message string) Result {
	result := Result{}

	// 空メッセージは警告不要
	if strings.TrimSpace(message) == "" {
		return result
	}

	// メンションの検出
	result.HasMention = c.mentionPattern.MatchString(message)

	// メンションがなければ警告
	result.NeedsWarning = !result.HasMention

	return result
}

// defaultChecker is used by the package-level Check function
var defaultChecker *Checker

// SetDefaultChecker sets the default checker for package-level Check function
func SetDefaultChecker(c *Checker) {
	defaultChecker = c
}

// Check analyzes a message for mention leaks using the default checker
// IMPORTANT: Call SetDefaultChecker first to set valid mention targets from config
func Check(message string) Result {
	if defaultChecker == nil {
		// No hardcoded names - use empty list (any @mention is valid) as fallback
		defaultChecker = NewChecker([]string{})
	}
	return defaultChecker.Check(message)
}

// FormatWarning returns a warning message for display
func FormatWarning() string {
	return "メンション先を明示してください（@名前）"
}
