package tasklist

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// KeyMap defines the key bindings for the task list.
type KeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Enter  key.Binding
	Delete key.Binding
	Quit   key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "toggle status"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d", "delete"),
			key.WithHelp("d", "delete"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "esc"),
			key.WithHelp("q", "quit"),
		),
	}
}

// Model is the Bubble Tea model for the task list component.
type Model struct {
	tasks    []Task
	cursor   int
	service  TaskService
	keyMap   KeyMap
	width    int
	height   int
	focused  bool
}

// New creates a new task list model.
func New(service TaskService) Model {
	return Model{
		tasks:   service.List(),
		cursor:  0,
		service: service,
		keyMap:  DefaultKeyMap(),
		focused: true,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !m.focused {
			return m, nil
		}

		switch {
		case key.Matches(msg, m.keyMap.Up):
			if m.cursor > 0 {
				m.cursor--
			}

		case key.Matches(msg, m.keyMap.Down):
			if m.cursor < len(m.tasks)-1 {
				m.cursor++
			}

		case key.Matches(msg, m.keyMap.Enter):
			if len(m.tasks) > 0 {
				m.cycleStatus()
			}

		case key.Matches(msg, m.keyMap.Delete):
			if len(m.tasks) > 0 {
				m.deleteCurrentTask()
			}

		case key.Matches(msg, m.keyMap.Quit):
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	return Render(m.tasks, m.cursor, m.width)
}

// cycleStatus cycles through task statuses: pending -> in_progress -> done -> pending
func (m *Model) cycleStatus() {
	if m.cursor >= len(m.tasks) {
		return
	}

	task := m.tasks[m.cursor]
	var newStatus Status

	switch task.Status {
	case StatusPending:
		newStatus = StatusInProgress
	case StatusInProgress:
		newStatus = StatusDone
	case StatusDone:
		newStatus = StatusPending
	default:
		newStatus = StatusPending
	}

	if err := m.service.UpdateStatus(task.ID, newStatus); err == nil {
		m.tasks = m.service.List()
	}
}

// deleteCurrentTask deletes the currently selected task.
func (m *Model) deleteCurrentTask() {
	if m.cursor >= len(m.tasks) {
		return
	}

	task := m.tasks[m.cursor]
	if err := m.service.Delete(task.ID); err == nil {
		m.tasks = m.service.List()
		// Adjust cursor if needed
		if m.cursor >= len(m.tasks) && m.cursor > 0 {
			m.cursor--
		}
	}
}

// Refresh reloads tasks from the service.
func (m *Model) Refresh() {
	m.tasks = m.service.List()
}

// SetFocused sets whether the component is focused.
func (m *Model) SetFocused(focused bool) {
	m.focused = focused
}

// IsFocused returns whether the component is focused.
func (m Model) IsFocused() bool {
	return m.focused
}

// Tasks returns the current list of tasks.
func (m Model) Tasks() []Task {
	return m.tasks
}

// Cursor returns the current cursor position.
func (m Model) Cursor() int {
	return m.cursor
}
