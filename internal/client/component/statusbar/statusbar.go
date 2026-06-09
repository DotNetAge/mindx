package statusbar

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/DotNetAge/mindx/internal/client/data"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/style"
	"github.com/DotNetAge/mindx/internal/i18n"
)

var (
	sep = fmt.Sprintf(" %s ", style.DimStyle.Render("│"))
)

type StatusBar struct {
	Width           int
	CurrentState    string
	BlinkOn         bool
	TokensTotal     int
	InputTokens     int
	OutputTokens    int
	CachedTokens    int
	SessionStart    time.Time
	SessionDuration time.Duration
	SessionName     string
	AgentName       string
	ModelName       string
	Provider        string
	ModeLabel       string
	Shortcuts       []data.Shortcut
	ShowHints       bool
	DaemonStatus    clientmsg.DaemonConnStatus

	// CostFn overrides the default pricing lookup for cost calculation.
	// If nil, data.GetPricing/data.CalculateCost is used as fallback.
	CostFn func(modelName string, inputTokens, outputTokens, cachedTokens int) float64
}

func New() *StatusBar {
	return &StatusBar{CurrentState: i18n.T("client.status.idle")}
}

func (s *StatusBar) Update(msg any) (*StatusBar, tea.Cmd) {
	switch m := msg.(type) {
	case clientmsg.WindowResizeMsg:
		s.Width = m.Width
	case clientmsg.AgentSwitchMsg:
		s.AgentName = m.AgentName
	case clientmsg.SessionLoadedMsg:
		s.AgentName = m.AgentName
		s.SessionName = m.SessionID
	case clientmsg.ExecutionSummaryMsg:
		s.InputTokens += m.TokensUsed.InputTokens
		s.OutputTokens += m.TokensUsed.OutputTokens
		s.CachedTokens += m.TokensUsed.CachedTokens
		s.TokensTotal += m.TokensUsed.TotalTokens
	case clientmsg.FinalAnswerMsg:
		if s.SessionStart.IsZero() {
			s.SessionDuration = 0
		} else {
			s.SessionDuration = time.Since(s.SessionStart)
		}
	case clientmsg.DaemonStatusMsg:
		s.DaemonStatus = m.Status
	}
	return s, nil
}

func (s *StatusBar) Tick() {
	s.BlinkOn = !s.BlinkOn
}

func (s *StatusBar) Cost() float64 {
	if s.CostFn != nil {
		return s.CostFn(s.ModelName, s.InputTokens, s.OutputTokens, s.CachedTokens)
	}
	return data.CalculateCost(data.GetPricing(s.ModelName), s.InputTokens, s.OutputTokens, s.CachedTokens)
}

func formatCost(cost float64) string {
	if cost < 0.005 {
		return "¥0"
	}
	if cost < 1 {
		return fmt.Sprintf("¥%.2f", cost)
	}
	return fmt.Sprintf("¥%.2f", cost)
}

func formatTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fm", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func stateStyle(state string, blinkOn bool) string {
	if state == i18n.T("client.status.idle") {
		return style.DimStyle.Render("● " + state)
	}
	dot := " "
	if blinkOn {
		dot = style.PurpleStyle.Render("●")
	}
	return fmt.Sprintf("%s %s", dot, style.PurpleStyle.Render(state))
}

func daemonIndicator(status clientmsg.DaemonConnStatus) string {
	switch status {
	case clientmsg.DaemonConnected:
		return style.GreenStyle.Render("● Daemon")
	case clientmsg.DaemonDisconnected:
		return style.DimStyle.Render("○ Daemon")
	default:
		return ""
	}
}

func (s *StatusBar) View() string {
	var parts []string

	stateStr := stateStyle(s.CurrentState, s.BlinkOn)
	parts = append(parts, stateStr)

	tokStr := style.WhiteStyle.Render(fmt.Sprintf("Tokens: %s", formatTokens(s.TokensTotal)))
	parts = append(parts, tokStr)

	cost := s.Cost()
	costStr := style.YellowStyle.Render(formatCost(cost))
	parts = append(parts, costStr)

	if !s.SessionStart.IsZero() {
		d := s.SessionDuration
		if d == 0 {
			d = time.Since(s.SessionStart)
		}
		parts = append(parts, style.GrayStyle.Render(formatDuration(d)))
	}

	if s.AgentName != "" {
		parts = append(parts, style.WhiteStyle.Render(s.AgentName))
	}
	if s.ModelName != "" {
		parts = append(parts, style.GrayStyle.Render(s.ModelName))
	}
	if s.Provider != "" {
		parts = append(parts, style.DimStyle.Render(s.Provider))
	}

	daemonStr := daemonIndicator(s.DaemonStatus)
	if daemonStr != "" {
		parts = append(parts, daemonStr)
	}

	if s.ModeLabel != "" {
		parts = append(parts, style.DimStyle.Render(s.ModeLabel))
	}

	line1 := strings.Join(parts, sep)

	if s.Width > 0 {
		hint := i18n.T("client.status.hint.scroll")
		if s.CurrentState != i18n.T("client.status.idle") {
			hint = i18n.T("client.status.hint.interrupt")
		}
		hintRendered := style.GrayStyle.Render(hint)
		l1w := lipgloss.Width(line1)
		hw := lipgloss.Width(hintRendered)
		if l1w+hw+2 <= s.Width {
			line1 += strings.Repeat(" ", s.Width-l1w-hw) + hintRendered
		}
	}

	if s.ShowHints && len(s.Shortcuts) > 0 {
		var hintParts []string
		for _, sc := range s.Shortcuts {
			hintParts = append(hintParts, style.GrayStyle.Render(fmt.Sprintf("%s: %s", sc.Key, sc.Description)))
		}
		line2 := strings.Join(hintParts, "  ")
		return lipgloss.JoinVertical(lipgloss.Top, line1, line2)
	}

	return line1
}
