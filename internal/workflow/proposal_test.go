package workflow

import (
	"strings"
	"testing"

	"github.com/ytnobody/MAXAM/internal/github"
)

func TestBuildAnalysisPrompt(t *testing.T) {
	pw := &ProposalWorkflow{}

	title := "Add dark mode support"
	body := "We need dark mode for better user experience"

	prompt := pw.buildAnalysisPrompt(title, body)

	// Check that title and body are included
	if !strings.Contains(prompt, title) {
		t.Error("prompt should contain issue title")
	}
	if !strings.Contains(prompt, body) {
		t.Error("prompt should contain issue body")
	}

	// Check analysis sections are defined
	expectedSections := []string{
		"要件の明確化",
		"技術的な実現可能性",
		"影響範囲",
		"リスク",
		"実装方針",
	}

	for _, section := range expectedSections {
		if !strings.Contains(prompt, section) {
			t.Errorf("prompt should contain section %q", section)
		}
	}
}

func TestBuildProposal(t *testing.T) {
	pw := &ProposalWorkflow{}

	title := "Add dark mode support"
	analysis := "### 要件サマリ\n- ダークモード対応\n\n### 推奨実装方針\n- CSS変数を使用"

	proposal := pw.buildProposal(title, analysis)

	// Check structure
	if !strings.Contains(proposal, "## 提案: "+title) {
		t.Error("proposal should have title header")
	}
	if !strings.Contains(proposal, analysis) {
		t.Error("proposal should contain analysis")
	}
	if !strings.Contains(proposal, "ProposalWorkflow") {
		t.Error("proposal should have attribution footer")
	}
}

func TestHandleLabeledEvent_IgnoresNonLabeledAction(t *testing.T) {
	pw := &ProposalWorkflow{}

	event := &github.IssuesEvent{
		Action: "opened", // Not "labeled"
	}
	event.Issue.Number = 1
	event.Issue.Labels = []struct {
		Name string `json:"name"`
	}{{Name: LabelProposal}}

	// This should return nil without doing anything
	// (since Action != "labeled")
	err := pw.HandleLabeledEvent(nil, event)
	if err != nil {
		t.Errorf("expected nil error for non-labeled action, got %v", err)
	}
}

func TestHandleLabeledEvent_IgnoresNonProposalLabel(t *testing.T) {
	pw := &ProposalWorkflow{}

	event := &github.IssuesEvent{
		Action: "labeled",
	}
	event.Issue.Number = 1
	event.Issue.Labels = []struct {
		Name string `json:"name"`
	}{{Name: "bug"}} // Not "proposal"

	// This should return nil without doing anything
	err := pw.HandleLabeledEvent(nil, event)
	if err != nil {
		t.Errorf("expected nil error for non-proposal label, got %v", err)
	}
}

func TestLabelConstants(t *testing.T) {
	// Ensure label constants are defined correctly
	if LabelProposal != "proposal" {
		t.Errorf("LabelProposal = %q, want %q", LabelProposal, "proposal")
	}
	if LabelProposalReady != "proposal-ready" {
		t.Errorf("LabelProposalReady = %q, want %q", LabelProposalReady, "proposal-ready")
	}
}
