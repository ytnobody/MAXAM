package monitor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ytnobody/MAXAM/internal/healthmgr"
)

// FormatStatusLine formats the monitoring status for display in a status bar.
// Returns a compact string like: "Agents: 3/4 healthy | Issues: 5 open"
func FormatStatusLine(status Status) string {
	var parts []string

	// Format agent health summary.
	if len(status.Agents) > 0 {
		healthy := 0
		for _, agent := range status.Agents {
			if agent.Status == healthmgr.HealthStatusHealthy {
				healthy++
			}
		}

		total := len(status.Agents)
		if healthy == total {
			parts = append(parts, fmt.Sprintf("Agents: %d healthy", total))
		} else {
			parts = append(parts, fmt.Sprintf("Agents: %d/%d healthy", healthy, total))
		}
	}

	// Format open issues.
	if status.OpenIssues > 0 {
		parts = append(parts, fmt.Sprintf("Issues: %d open", status.OpenIssues))
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, " | ")
}

// FormatAgentDetails formats detailed agent status for display.
// Returns a multi-line string with each agent's status.
func FormatAgentDetails(status Status) string {
	if len(status.Agents) == 0 {
		return "No agents registered"
	}

	// Sort agents by name for consistent display.
	agents := make([]AgentStatus, len(status.Agents))
	copy(agents, status.Agents)
	sort.Slice(agents, func(i, j int) bool {
		return agents[i].Name < agents[j].Name
	})

	var lines []string
	for _, agent := range agents {
		icon := statusIcon(agent.Status)
		line := fmt.Sprintf("%s %s", icon, agent.Name)

		if agent.FailCount > 0 {
			line += fmt.Sprintf(" (fails: %d)", agent.FailCount)
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// statusIcon returns an emoji/icon for the given health status.
func statusIcon(status healthmgr.HealthStatus) string {
	switch status {
	case healthmgr.HealthStatusHealthy:
		return "[OK]"
	case healthmgr.HealthStatusUnresponsive:
		return "[!!]"
	case healthmgr.HealthStatusDead:
		return "[XX]"
	default:
		return "[??]"
	}
}
