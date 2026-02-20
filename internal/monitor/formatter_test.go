package monitor

import (
	"strings"
	"testing"
	"time"

	"github.com/ytnobody/MAXAM/internal/healthmgr"
)

func TestFormatStatusLine(t *testing.T) {
	tests := []struct {
		name     string
		status   Status
		expected string
	}{
		{
			name:     "empty status",
			status:   Status{},
			expected: "",
		},
		{
			name: "all healthy",
			status: Status{
				Agents: []AgentStatus{
					{Name: "agent1", Status: healthmgr.HealthStatusHealthy},
					{Name: "agent2", Status: healthmgr.HealthStatusHealthy},
				},
			},
			expected: "Agents: 2 healthy",
		},
		{
			name: "some unhealthy",
			status: Status{
				Agents: []AgentStatus{
					{Name: "agent1", Status: healthmgr.HealthStatusHealthy},
					{Name: "agent2", Status: healthmgr.HealthStatusUnresponsive},
					{Name: "agent3", Status: healthmgr.HealthStatusHealthy},
				},
			},
			expected: "Agents: 2/3 healthy",
		},
		{
			name: "with open issues",
			status: Status{
				Agents: []AgentStatus{
					{Name: "agent1", Status: healthmgr.HealthStatusHealthy},
				},
				OpenIssues: 5,
			},
			expected: "Agents: 1 healthy | Issues: 5 open",
		},
		{
			name: "only open issues",
			status: Status{
				OpenIssues: 3,
			},
			expected: "Issues: 3 open",
		},
		{
			name: "zero issues not shown",
			status: Status{
				Agents: []AgentStatus{
					{Name: "agent1", Status: healthmgr.HealthStatusHealthy},
				},
				OpenIssues: 0,
			},
			expected: "Agents: 1 healthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatStatusLine(tt.status)
			if result != tt.expected {
				t.Errorf("FormatStatusLine() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatAgentDetails(t *testing.T) {
	tests := []struct {
		name        string
		status      Status
		wantLines   []string
		wantNoAgent bool
	}{
		{
			name:        "no agents",
			status:      Status{},
			wantNoAgent: true,
		},
		{
			name: "healthy agents",
			status: Status{
				Agents: []AgentStatus{
					{Name: "mei", Status: healthmgr.HealthStatusHealthy},
					{Name: "yuki", Status: healthmgr.HealthStatusHealthy},
				},
			},
			wantLines: []string{"[OK] mei", "[OK] yuki"},
		},
		{
			name: "mixed status",
			status: Status{
				Agents: []AgentStatus{
					{Name: "agent1", Status: healthmgr.HealthStatusHealthy},
					{Name: "agent2", Status: healthmgr.HealthStatusUnresponsive, FailCount: 2},
					{Name: "agent3", Status: healthmgr.HealthStatusDead, FailCount: 5},
				},
			},
			wantLines: []string{
				"[OK] agent1",
				"[!!] agent2 (fails: 2)",
				"[XX] agent3 (fails: 5)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatAgentDetails(tt.status)

			if tt.wantNoAgent {
				if result != "No agents registered" {
					t.Errorf("FormatAgentDetails() = %q, want 'No agents registered'", result)
				}
				return
			}

			for _, line := range tt.wantLines {
				if !strings.Contains(result, line) {
					t.Errorf("FormatAgentDetails() missing line %q, got:\n%s", line, result)
				}
			}
		})
	}
}

func TestStatusIcon(t *testing.T) {
	tests := []struct {
		status   healthmgr.HealthStatus
		expected string
	}{
		{healthmgr.HealthStatusHealthy, "[OK]"},
		{healthmgr.HealthStatusUnresponsive, "[!!]"},
		{healthmgr.HealthStatusDead, "[XX]"},
		{healthmgr.HealthStatus("unknown"), "[??]"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			result := statusIcon(tt.status)
			if result != tt.expected {
				t.Errorf("statusIcon(%s) = %q, want %q", tt.status, result, tt.expected)
			}
		})
	}
}

func TestFormatStatusLine_WithLastUpdated(t *testing.T) {
	status := Status{
		Agents: []AgentStatus{
			{Name: "agent1", Status: healthmgr.HealthStatusHealthy, LastCheck: time.Now()},
		},
		OpenIssues:  2,
		LastUpdated: time.Now(),
	}

	result := FormatStatusLine(status)

	// Should contain basic info regardless of timestamps.
	if !strings.Contains(result, "Agents: 1 healthy") {
		t.Errorf("Expected agent count in result: %s", result)
	}

	if !strings.Contains(result, "Issues: 2 open") {
		t.Errorf("Expected issue count in result: %s", result)
	}
}
