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
)

var (
	sep = fmt.Sprintf(" %s ", style.DimStyle.Render("│"))
)

type StatusBar struct {
	Width           int
	CurrentState    string
	BlinkOn         bool
	TokensTotal     int
	SessionStart    time.Time
	SessionDuration time.Duration
	SessionName     string
	AgentName       string
	ModelName       string
	ModeLabel       string
	Shortcuts       []data.Shortcut
	ShowHints       bool
}

func New() *StatusBar {
	return &StatusBar{CurrentState: "空闲"}
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
	case clientmsg.ActionStartMsg:
		s.TokensTotal += m.EstimatedTok
	case clientmsg.ExecutionSummaryMsg:
		if m.TokensUsed > 0 {
			s.TokensTotal += m.TokensUsed
		}
	case clientmsg.FinalAnswerMsg:
		if s.SessionStart.IsZero() {
			s.SessionDuration = 0
		} else {
			s.SessionDuration = time.Since(s.SessionStart)
		}
	}
	return s, nil
}

func (s *StatusBar) Tick() {
	s.BlinkOn = !s.BlinkOn
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
	if state == "空闲" {
		return style.DimStyle.Render("● " + state)
	}
	dot := " "
	if blinkOn {
		dot = style.PurpleStyle.Render("●")
	}
	return fmt.Sprintf("%s %s", dot, style.PurpleStyle.Render(state))
}

func (s *StatusBar) View() string {
	var parts []string

	stateStr := stateStyle(s.CurrentState, s.BlinkOn)
	parts = append(parts, stateStr)

	tokStr := style.WhiteStyle.Render(fmt.Sprintf("Tokens: %s", formatTokens(s.TokensTotal)))
	parts = append(parts, tokStr)

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

	if s.ModeLabel != "" {
		parts = append(parts, style.DimStyle.Render(s.ModeLabel))
	}

	line1 := strings.Join(parts, sep)

	// Right-align contextual hint on the statusbar
	if s.Width > 0 {
		hint := "↑↓ 滚动"
		if s.CurrentState != "空闲" {
			hint = "esc 打断 • ↑↓ 滚动"
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
