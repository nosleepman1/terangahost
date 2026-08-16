package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type StepState int

const (
	StepPending StepState = iota
	StepRunning
	StepCompleted
	StepSkipped
	StepFailed
)

type StepItem struct {
	ID       string
	Title    string
	State    StepState
	Duration time.Duration
	Error    error
}

type Model struct {
	spinner  spinner.Model
	steps    []StepItem
	current  int
	finished bool
	quitting bool
}

func InitialModel(steps []StepItem) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ColorPrimary)

	return Model{
		spinner: s,
		steps:   steps,
		current: 0,
	}
}

func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return "Arrêt demandé.\n"
	}

	var out string
	for _, step := range m.steps {
		var icon string
		switch step.State {
		case StepCompleted:
			icon = lipgloss.NewStyle().Foreground(ColorSecondary).Render("✔")
		case StepSkipped:
			icon = lipgloss.NewStyle().Foreground(ColorWarning).Render("⚡")
		case StepFailed:
			icon = lipgloss.NewStyle().Foreground(ColorDanger).Render("✖")
		case StepRunning:
			icon = m.spinner.View()
		default:
			icon = lipgloss.NewStyle().Foreground(ColorMuted).Render("○")
		}

		out += fmt.Sprintf("  %s %s\n", icon, step.Title)
	}

	return out
}
