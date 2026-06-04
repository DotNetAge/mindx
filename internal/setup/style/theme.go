package style

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	Border = lipgloss.NewStyle().Padding(1, 2)

	Title      = lipgloss.NewStyle().Bold(true)
	Dim        = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	Success    = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	Error      = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	Warning    = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	CodeInline = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
)

// GradientVersion is the version string used in the default gradient title.
// Set this at startup to reflect the actual build version.
var GradientVersion = "beta"

func GradientTitle(text string) string {
	if text == "" {
		text = "MindX v" + GradientVersion + " Setup"
	}

	runes := []rune(text)
	gradientColors := []color.Color{
		lipgloss.Color("#1799EA"),
		lipgloss.Color("#548BE1"),
		lipgloss.Color("#6985DC"),
		lipgloss.Color("#8D78CD"),
		lipgloss.Color("#9774C1"),
		lipgloss.Color("#A371B6"),
		lipgloss.Color("#AD6EAA"),
		lipgloss.Color("#B26CA4"),
		lipgloss.Color("#BC6899"),
		lipgloss.Color("#D0617F"),
	}

	blendedColors := lipgloss.Blend1D(len(runes), gradientColors...)
	var b strings.Builder
	b.Grow(len(runes) * 24)
	for i, ch := range runes {
		if i < len(blendedColors) {
			b.WriteString(
				lipgloss.NewStyle().
					Foreground(blendedColors[i]).
					Bold(true).
					Render(string(ch)),
			)
		} else {
			b.WriteRune(ch)
		}
	}
	return b.String()
}
