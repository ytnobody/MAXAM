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

// メンションパターン
var mentionPattern = regexp.MustCompile(`@(yuki|priya|amara|mei|rin|shiori|Yuki|Priya|Amara|Mei|Rin|Shiori)`)

// Check analyzes a message for mention leaks
// Returns a Result indicating if the message needs a warning
func Check(message string) Result {
	result := Result{
		Patterns: make([]string, 0),
	}

	// メンションの検出
	result.HasMention = mentionPattern.MatchString(message)

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

// FormatWarning returns a warning message for display
func FormatWarning() string {
	return "宛先が不明確かも（@名前 で指定してね）"
}
