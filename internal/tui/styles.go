package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12")).
			MarginBottom(1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("8")).
			Padding(0, 1)

	cellStyle = lipgloss.NewStyle().
			Padding(0, 1)

	statusRunning = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")).
			Bold(true)

	statusStopped = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9"))

	statusOnline = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10"))

	statusOffline = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9"))

	statusBusy = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Bold(true)

	statusUnknown = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	configKey = lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")).
			Bold(true)

	configVal = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			MarginTop(1)

	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("8")).
			Foreground(lipgloss.Color("15"))
)
