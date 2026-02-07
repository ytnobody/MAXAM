package mode

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultPlanModeConfig(t *testing.T) {
	config := DefaultPlanModeConfig()

	if config.PollingInterval != DefaultPollingInterval {
		t.Errorf("PollingInterval = %v, want %v", config.PollingInterval, DefaultPollingInterval)
	}

	if config.PollingInterval != 30*time.Second {
		t.Errorf("DefaultPollingInterval should be 30 seconds, got %v", config.PollingInterval)
	}
}

func TestProjectAnalysis_Summary(t *testing.T) {
	// Test that summary generation includes expected fields
	analysis := &ProjectAnalysis{
		TotalIssues:      10,
		OpenIssues:       5,
		UnassignedIssues: 2,
		OldestOpenIssue: &IssueInfo{
			Number:    1,
			Title:     "Test Issue",
			CreatedAt: time.Now().Add(-7 * 24 * time.Hour), // 7 days ago
			Labels:    []string{"bug"},
		},
		LabelCounts: map[string]int{
			"bug":         3,
			"enhancement": 2,
		},
	}

	// Create a PlanExecutor to test generateSummary
	pe := &PlanExecutor{}
	summary := pe.generateSummary(analysis)

	// Check that summary contains expected information
	expectedParts := []string{
		"オープンIssue: 5件",
		"未アサイン: 2件",
		"#1",
		"bug",
		"enhancement",
	}

	for _, part := range expectedParts {
		if !strings.Contains(summary, part) {
			t.Errorf("Summary should contain %q, got:\n%s", part, summary)
		}
	}
}

func TestIssueInfo(t *testing.T) {
	info := &IssueInfo{
		Number:    42,
		Title:     "Test Issue",
		CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		Labels:    []string{"bug", "priority:high"},
	}

	if info.Number != 42 {
		t.Errorf("Number = %d, want 42", info.Number)
	}

	if info.Title != "Test Issue" {
		t.Errorf("Title = %q, want %q", info.Title, "Test Issue")
	}

	if len(info.Labels) != 2 {
		t.Errorf("Labels len = %d, want 2", len(info.Labels))
	}
}

func TestPlanIssue(t *testing.T) {
	planIssue := &PlanIssue{
		IssueNumber: 100,
		Title:       "[Plan] プロジェクト方針の提案",
		Body:        "テスト本文",
		CreatedAt:   time.Now(),
		URL:         "https://github.com/test/repo/issues/100",
	}

	if planIssue.IssueNumber != 100 {
		t.Errorf("IssueNumber = %d, want 100", planIssue.IssueNumber)
	}

	if !strings.Contains(planIssue.Title, "Plan") {
		t.Errorf("Title should contain 'Plan', got %q", planIssue.Title)
	}
}

func TestPlanModeConfig_CustomInterval(t *testing.T) {
	config := &PlanModeConfig{
		PollingInterval: 60 * time.Second,
	}

	if config.PollingInterval != 60*time.Second {
		t.Errorf("PollingInterval = %v, want 60s", config.PollingInterval)
	}
}
