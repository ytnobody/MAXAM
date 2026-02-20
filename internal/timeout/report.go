// Package timeout provides agent timeout detection and reporting.
package timeout

import (
	"fmt"
	"time"
)

// Report contains information about an agent timeout event.
type Report struct {
	AgentName    string    // e.g., "yuki-1"
	IssueNumber  int       // e.g., 375
	TaskSummary  string    // e.g., "セルフメンション検知機能"
	Progress     string    // e.g., "detector.go 実装中（50%）"
	NextAction   string    // e.g., "TestDetectSelfMention の実装"
	LastActivity time.Time // Last activity timestamp
	TimeoutAt    time.Time // When timeout was detected
}

// Format returns a formatted string representation of the timeout report.
func (r Report) Format() string {
	elapsed := r.TimeoutAt.Sub(r.LastActivity).Truncate(time.Second)

	msg := fmt.Sprintf(`⏰ **タイムアウト検知: %s**

| 項目 | 内容 |
|------|------|
| エージェント | %s |
| 担当Issue | #%d |
| タスク | %s |
| 進捗 | %s |
| 次のアクション | %s |
| 最終アクティビティ | %s（%s前） |

→ エージェント再起動を開始します...`,
		r.AgentName,
		r.AgentName,
		r.IssueNumber,
		r.TaskSummary,
		r.Progress,
		r.NextAction,
		r.LastActivity.Format("15:04:05"),
		elapsed,
	)

	return msg
}

// FormatShort returns a compact representation for logs.
func (r Report) FormatShort() string {
	return fmt.Sprintf("[TIMEOUT] %s: Issue #%d, task=%s, progress=%s",
		r.AgentName, r.IssueNumber, r.TaskSummary, r.Progress)
}
