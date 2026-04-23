package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFB86C")).
			Background(lipgloss.Color("#282A36")).
			Padding(0, 2)

	userStyle = lipgloss.NewStyle(). // user messages in purple
			Foreground(lipgloss.Color("#BD93F9"))

	assistantStyle = lipgloss.NewStyle(). // assistant messages in white
			Foreground(lipgloss.Color("#F8F8F2"))

	systemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4")).
			Italic(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555"))

	suggestionStyle = lipgloss.NewStyle(). // non-focus items: light gray
			Foreground(lipgloss.Color("#6272A4"))

	suggestionActiveStyle = lipgloss.NewStyle(). // focus item: light green bg, bright green text
				Foreground(lipgloss.Color("#50FA7B")).
				Background(lipgloss.Color("#2E5A45")).
				Bold(true)

	// Input area styles
	inputContainerStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#282A36")).
			Width(100).
			Padding(1, 1)

	tokenCounterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4"))

	inputHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4"))

	commandHighlightStyle = lipgloss.NewStyle(). // purple for /command and @path
				Foreground(lipgloss.Color("#BD93F9"))
)
