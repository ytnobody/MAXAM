// Package mention provides mention leak detection for team chat messages.
package mention

import (
	"regexp"
	"strings"
)

// Result represents the result of a mention check
type Result struct {
	HasRequestPattern bool     // 依頼パターンが検出されたか
	HasMention        bool     // メンションが含まれているか
	NeedsWarning      bool     // 警告が必要か
	Patterns          []string // 検出された依頼パターン
}

// 依頼パターンの正規表現
var requestPatterns = []*regexp.Regexp{
	// 「〜お願い」系
	regexp.MustCompile(`お願い(します|しま|)|おねがい`),
	// 「〜して」系
	regexp.MustCompile(`(確認|チェック|レビュー|実装|修正|追加|削除|更新|作成|調査|分析|対応|検討|共有|報告|連絡|送信)して`),
	// 「〜してください」系
	regexp.MustCompile(`して(ください|くれ|ほしい|もらえ)`),
	// 「〜やって」系
	regexp.MustCompile(`やって(ください|くれ|ほしい|もらえ|)`),
	// 「〜頼む」系
	regexp.MustCompile(`頼(む|みます|んだ)`),
	// 「〜できる？」系（依頼っぽい疑問）
	regexp.MustCompile(`(できる|やれる|可能)[?？]`),
	// 「〜よろしく」系
	regexp.MustCompile(`よろしく(お願い|ね|)`),
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
		lower := strings.ToLower(name)
		// Add both lowercase and title case
		parts = append(parts, lower)
		if len(lower) > 0 {
			titled := strings.ToUpper(string(lower[0])) + lower[1:]
			if titled != lower {
				parts = append(parts, titled)
			}
		}
	}

	pattern := `@(` + strings.Join(parts, "|") + `)`
	return &Checker{
		mentionPattern: regexp.MustCompile(pattern),
	}
}

// Check analyzes a message for mention leaks
func (c *Checker) Check(message string) Result {
	result := Result{
		Patterns: make([]string, 0),
	}

	// メンションの検出
	result.HasMention = c.mentionPattern.MatchString(message)

	// 依頼パターンの検出
	lower := strings.ToLower(message)
	for _, pattern := range requestPatterns {
		if pattern.MatchString(lower) || pattern.MatchString(message) {
			result.HasRequestPattern = true
			// マッチしたパターンを記録（デバッグ用）
			matches := pattern.FindAllString(message, -1)
			if len(matches) == 0 {
				matches = pattern.FindAllString(lower, -1)
			}
			result.Patterns = append(result.Patterns, matches...)
		}
	}

	// 依頼パターンがあるのにメンションがない場合は警告
	result.NeedsWarning = result.HasRequestPattern && !result.HasMention

	return result
}

// defaultChecker is used by the package-level Check function
var defaultChecker *Checker

// SetDefaultChecker sets the default checker for package-level Check function
func SetDefaultChecker(c *Checker) {
	defaultChecker = c
}

// Check analyzes a message for mention leaks using the default checker
// For backward compatibility
func Check(message string) Result {
	if defaultChecker == nil {
		// fallback to hardcoded names for backward compatibility
		defaultChecker = NewChecker([]string{"mei", "yuki", "rin", "shiori", "priya", "amara"})
	}
	return defaultChecker.Check(message)
}

// FormatWarning returns a warning message for display
func FormatWarning() string {
	return "宛先が不明確かも（@名前 で指定してね）"
}
