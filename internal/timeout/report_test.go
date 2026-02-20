package timeout

import (
	"strings"
	"testing"
	"time"
)

func TestReportFormat(t *testing.T) {
	now := time.Now()
	lastActivity := now.Add(-3 * time.Minute)

	report := Report{
		AgentName:    "yuki-1",
		IssueNumber:  367,
		TaskSummary:  "タイムアウト検知機能",
		Progress:     "monitor.go 実装中（50%）",
		NextAction:   "テストの作成",
		LastActivity: lastActivity,
		TimeoutAt:    now,
	}

	formatted := report.Format()

	// Check required elements
	checks := []string{
		"タイムアウト検知: yuki-1",
		"#367",
		"タイムアウト検知機能",
		"monitor.go 実装中（50%）",
		"テストの作成",
		"エージェント再起動を開始します",
	}

	for _, check := range checks {
		if !strings.Contains(formatted, check) {
			t.Errorf("Format() missing %q\nGot:\n%s", check, formatted)
		}
	}
}

func TestReportFormatShort(t *testing.T) {
	report := Report{
		AgentName:   "priya-1",
		IssueNumber: 100,
		TaskSummary: "セキュリティレビュー",
		Progress:    "コード確認中",
	}

	short := report.FormatShort()

	if !strings.Contains(short, "[TIMEOUT]") {
		t.Error("FormatShort() should contain [TIMEOUT] tag")
	}
	if !strings.Contains(short, "priya-1") {
		t.Error("FormatShort() should contain agent name")
	}
	if !strings.Contains(short, "#100") {
		t.Error("FormatShort() should contain issue number")
	}
}

func TestReportFormatWithZeroValues(t *testing.T) {
	// Test with zero values
	report := Report{
		AgentName: "test-agent",
	}

	formatted := report.Format()

	if !strings.Contains(formatted, "test-agent") {
		t.Error("Format() should contain agent name even with zero values")
	}
	if !strings.Contains(formatted, "#0") {
		t.Error("Format() should contain issue number (0 for unset)")
	}
}
