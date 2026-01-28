package tasklist

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Title style
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	// Status styles
	pendingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	inProgressStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFAA00")).
			Bold(true)

	doneStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00"))

	// Task item styles
	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#5A4FCF"))

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DDDDDD"))

	assigneeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#88AAFF")).
			Italic(true)

	// Container style
	containerStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Padding(0, 1)
)

// Render renders the task list view.
func Render(tasks []Task, cursor int, width int) string {
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("Task Board"))
	b.WriteString("\n\n")

	if len(tasks) == 0 {
		b.WriteString(normalStyle.Render("No tasks yet. Time to add some!"))
		b.WriteString("\n")
	} else {
		for i, task := range tasks {
			line := renderTaskLine(task, i == cursor)
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(renderHelp())

	content := b.String()
	if width > 0 {
		return containerStyle.Width(width - 4).Render(content)
	}
	return containerStyle.Render(content)
}

// renderTaskLine renders a single task line.
func renderTaskLine(task Task, selected bool) string {
	// Status indicator
	statusIcon := getStatusIcon(task.Status)
	statusStr := renderStatus(task.Status)

	// Task info
	assignee := ""
	if task.Assignee != "" {
		assignee = assigneeStyle.Render(fmt.Sprintf(" [%s]", task.Assignee))
	}

	line := fmt.Sprintf("%s %s%s - %s",
		statusIcon,
		task.Title,
		assignee,
		statusStr,
	)

	if selected {
		return selectedStyle.Render("> " + line)
	}
	return normalStyle.Render("  " + line)
}

// getStatusIcon returns an icon for the status.
func getStatusIcon(status Status) string {
	switch status {
	case StatusPending:
		return "○"
	case StatusInProgress:
		return "◐"
	case StatusDone:
		return "●"
	default:
		return "?"
	}
}

// renderStatus renders the status with appropriate styling.
func renderStatus(status Status) string {
	switch status {
	case StatusPending:
		return pendingStyle.Render(status.String())
	case StatusInProgress:
		return inProgressStyle.Render(status.String())
	case StatusDone:
		return doneStyle.Render(status.String())
	default:
		return status.String()
	}
}

// renderHelp renders the help text.
func renderHelp() string {
	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Render("↑/↓: navigate • enter: toggle status • d: delete • q: quit")
	return help
}
