package style

import (
	lipgloss "charm.land/lipgloss/v2"
)

var (
	ThemeCyan      = lipgloss.Color("#4FC3F7")
	ThemeGreen     = lipgloss.Color("#4CAF50")
	ThemeRed       = lipgloss.Color("#CF6679")
	ThemeWhite     = lipgloss.Color("#E0E0E0")
	ThemeGray      = lipgloss.Color("#AAAAAA")
	ThemeDim       = lipgloss.Color("#666666")
	ThemeDark      = lipgloss.Color("#888888")
	ThemeBg        = lipgloss.Color("#1E1E2E")
	ThemePurple    = lipgloss.Color("#BB86FC")
	ThemeYellow    = lipgloss.Color("#FFD54F")
	ThemeDarkGray  = lipgloss.Color("#555555")

	CyanStyle   = lipgloss.NewStyle().Foreground(ThemeCyan)
	GreenStyle  = lipgloss.NewStyle().Foreground(ThemeGreen)
	RedStyle    = lipgloss.NewStyle().Foreground(ThemeRed)
	WhiteStyle  = lipgloss.NewStyle().Foreground(ThemeWhite)
	GrayStyle   = lipgloss.NewStyle().Foreground(ThemeGray)
	DimStyle    = lipgloss.NewStyle().Foreground(ThemeDim)
	DarkStyle   = lipgloss.NewStyle().Foreground(ThemeDark).Italic(true)
	BoldWhite   = lipgloss.NewStyle().Foreground(ThemeWhite).Bold(true)
	BoldCyan    = lipgloss.NewStyle().Foreground(ThemeCyan).Bold(true)
	PurpleStyle = lipgloss.NewStyle().Foreground(ThemePurple)
	YellowStyle = lipgloss.NewStyle().Foreground(ThemeYellow)
)

func Divider(text string) string {
	return lipgloss.NewStyle().Foreground(ThemeDim).Render(text)
}
