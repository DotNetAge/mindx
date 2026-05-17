package statusbar

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/data"
	"github.com/DotNetAge/mindx/internal/client/style"
	lipgloss "charm.land/lipgloss/v2"
)

var (
	connectingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E5C07B"))
	sep             = fmt.Sprintf(" %s ", style.DimStyle.Render("│"))
)

type StatusBar struct {
	Width       int
	ConnState   data.ConnectionState
	SessionName string
	TokensIn    int
	TokensOut   int
	TokensTotal int
	SessionCost string
	AgentName   string
	ModelName   string
	ModeLabel   string
	ShowHints   bool
	Shortcuts   []data.Shortcut
}

func New() *StatusBar {
	return &StatusBar{}
}

func (s *StatusBar) Update(msg any) (*StatusBar, tea.Cmd) {
	switch m := msg.(type) {
	case clientmsg.WindowResizeMsg:
		s.Width = m.Width
	case clientmsg.AgentSwitchMsg:
		s.AgentName = m.AgentName
	case clientmsg.TranscriptToggleMsg:
		if s.ModeLabel == "" {
			s.ModeLabel = "Transcript"
		} else {
			s.ModeLabel = ""
		}
	}
	return s, nil
}

func (s *StatusBar) View() string {
	var connStr string
	switch s.ConnState {
	case data.Disconnected:
		connStr = style.RedStyle.Render("○ Disconnected")
	case data.Connecting:
		connStr = connectingStyle.Render("● Connecting")
	case data.Authenticated:
		connStr = style.GreenStyle.Render("● Authenticated")
	case data.Connected:
		connStr = style.GreenStyle.Render("● Connected")
	}

	line1 := connStr

	if s.SessionName != "" {
		line1 += sep + style.WhiteStyle.Render(s.SessionName)
	}

	tokStr := fmt.Sprintf("Tokens: %d in / %d out", s.TokensIn, s.TokensOut)
	line1 += sep + style.GrayStyle.Render(tokStr)

	if s.SessionCost != "" {
		line1 += sep + style.DimStyle.Render(s.SessionCost)
	}

	agentModelStr := fmt.Sprintf("%s (%s)", s.AgentName, s.ModelName)
	line1 += sep + style.WhiteStyle.Render(agentModelStr)

	if s.ModeLabel != "" {
		line1 += sep + style.DimStyle.Render(s.ModeLabel)
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
