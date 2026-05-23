package conv

import (
	lipgloss "charm.land/lipgloss/v2"
	"github.com/DotNetAge/mindx/internal/client/style"
)

type Blink struct {
	Symbol  string
	BlinkOn bool
}

func ViewBlink(m Blink, normalStyle lipgloss.Style) string {
	if m.BlinkOn {
		return style.WhiteStyle.Render(m.Symbol)
	}
	return normalStyle.Render(m.Symbol)
}
