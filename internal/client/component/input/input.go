package input

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/DotNetAge/mindx/internal/client/data"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/style"
)

type AgentSuggestion struct {
	Agents []data.AgentInfo
	Filter string
	SelIdx int
}

func (s *AgentSuggestion) filtered() []data.AgentInfo {
	if s.Filter == "" {
		return s.Agents
	}
	var out []data.AgentInfo
	for _, a := range s.Agents {
		if strings.Contains(strings.ToLower(a.Name), strings.ToLower(s.Filter)) {
			out = append(out, a)
		}
	}
	return out
}

func (s *AgentSuggestion) View(width int) string {
	list := s.filtered()
	if len(list) == 0 {
		return ""
	}
	var b strings.Builder
	for i, a := range list {
		line := fmt.Sprintf("@%s  %s", a.Name, a.Description)
		if i == s.SelIdx {
			b.WriteString(style.CyanStyle.Render("> " + line))
		} else {
			b.WriteString("  " + line)
		}
		b.WriteByte('\n')
	}
	return lipgloss.NewStyle().Width(width).Render(strings.TrimRight(b.String(), "\n"))
}

type CommandSuggestion struct {
	Commands []SlashCommand
	Filter   string
	SelIdx   int
}

type SlashCommand struct {
	Name        string
	Description string
}

func (s *CommandSuggestion) filtered() []SlashCommand {
	if s.Filter == "" {
		return s.Commands
	}
	var out []SlashCommand
	for _, c := range s.Commands {
		if strings.Contains(c.Name, s.Filter) {
			out = append(out, c)
		}
	}
	return out
}

func (s *CommandSuggestion) View(width int) string {
	list := s.filtered()
	if len(list) == 0 {
		return ""
	}
	var b strings.Builder
	for i, c := range list {
		line := fmt.Sprintf("/%s  %s", c.Name, c.Description)
		if i == s.SelIdx {
			b.WriteString(style.CyanStyle.Render("> " + line))
		} else {
			b.WriteString("  " + line)
		}
		b.WriteByte('\n')
	}
	return lipgloss.NewStyle().Width(width).Render(strings.TrimRight(b.String(), "\n"))
}

type InputArea struct {
	Width      int
	TextBuffer strings.Builder
	CursorPos  int
	Hidden     bool
	Agents     []data.AgentInfo
	Commands   []SlashCommand

	agentSuggest AgentSuggestion
	cmdSuggest   CommandSuggestion
}

func New() *InputArea {
	return &InputArea{}
}

func (i *InputArea) Update(msg any) (*InputArea, tea.Cmd) {
	switch m := msg.(type) {
	case clientmsg.WindowResizeMsg:
		i.Width = m.Width
	case tea.KeyPressMsg:
		if i.Hidden {
			return i, nil
		}
		return i.handleKey(m)
	}
	return i, nil
}

func (i *InputArea) handleKey(k tea.KeyPressMsg) (*InputArea, tea.Cmd) {
	switch k.String() {
	case "enter", "ctrl+j":
		if alt := strings.Contains(k.String(), "alt"); alt {
			i.TextBuffer.WriteByte('\n')
			i.CursorPos++
			return i, nil
		}
		text := strings.TrimSpace(i.TextBuffer.String())
		if text == "" {
			return i, nil
		}
		i.TextBuffer.Reset()
		i.CursorPos = 0
		i.cmdSuggest = CommandSuggestion{}
		i.agentSuggest = AgentSuggestion{}
		if strings.HasPrefix(text, "/") {
			parts := strings.Fields(text)
			return i, func() tea.Msg {
				return clientmsg.SlashCommandMsg{Name: strings.TrimPrefix(parts[0], "/"), Args: parts[1:]}
			}
		}
		return i, func() tea.Msg {
			return clientmsg.UserSendMsg{Text: text}
		}

	case "ctrl+c":
		return i, func() tea.Msg { return clientmsg.ExitMsg{} }

	case "ctrl+l":
		return i, func() tea.Msg { return clientmsg.ClearScreenMsg{} }

	case "tab":
		if i.cmdSuggest.Filter != "" {
			list := i.cmdSuggest.filtered()
			if len(list) > 0 {
				sel := list[i.cmdSuggest.SelIdx]
				i.TextBuffer.Reset()
				i.TextBuffer.WriteString("/" + sel.Name + " ")
				i.CursorPos = i.TextBuffer.Len()
			}
		} else if i.agentSuggest.Filter != "" {
			list := i.agentSuggest.filtered()
			if len(list) > 0 {
				sel := list[i.agentSuggest.SelIdx]
				i.TextBuffer.Reset()
				i.TextBuffer.WriteString("@" + sel.Name + " ")
				i.CursorPos = i.TextBuffer.Len()
			}
		}
		i.cmdSuggest = CommandSuggestion{}
		i.agentSuggest = AgentSuggestion{}
		return i, nil

	case "up":
		if i.cmdSuggest.Filter != "" {
			if i.cmdSuggest.SelIdx > 0 {
				i.cmdSuggest.SelIdx--
			}
		} else if i.agentSuggest.Filter != "" {
			if i.agentSuggest.SelIdx > 0 {
				i.agentSuggest.SelIdx--
			}
		}
		return i, nil

	case "down":
		if i.cmdSuggest.Filter != "" {
			list := i.cmdSuggest.filtered()
			if i.cmdSuggest.SelIdx < len(list)-1 {
				i.cmdSuggest.SelIdx++
			}
		} else if i.agentSuggest.Filter != "" {
			list := i.agentSuggest.filtered()
			if i.agentSuggest.SelIdx < len(list)-1 {
				i.agentSuggest.SelIdx++
			}
		}
		return i, nil

	case "backspace":
		s := i.TextBuffer.String()
		if i.CursorPos > 0 && len(s) > 0 {
			pos := i.CursorPos
			runes := []rune(s)
			runes = append(runes[:pos-1], runes[pos:]...)
			i.TextBuffer.Reset()
			i.TextBuffer.WriteString(string(runes))
			i.CursorPos--
		}
		i.updateSuggestion()
		return i, nil

	default:
		if !strings.HasPrefix(k.String(), "ctrl+") {
			s := i.TextBuffer.String()
			runes := []rune(s)
			pos := i.CursorPos
			var newRunes []rune
			newRunes = append(newRunes, runes[:pos]...)
			newRunes = append(newRunes, []rune(k.String())...)
			newRunes = append(newRunes, runes[pos:]...)
			i.TextBuffer.Reset()
			i.TextBuffer.WriteString(string(newRunes))
			i.CursorPos++
			i.updateSuggestion()
		}
		return i, nil
	}
}

func (i *InputArea) updateSuggestion() {
	text := i.TextBuffer.String()
	if strings.HasPrefix(text, "/") {
		filter := strings.TrimPrefix(text, "/")
		i.cmdSuggest = CommandSuggestion{Commands: i.Commands, Filter: filter, SelIdx: 0}
		i.agentSuggest = AgentSuggestion{}
	} else if strings.HasPrefix(text, "@") {
		filter := strings.TrimPrefix(text, "@")
		i.agentSuggest = AgentSuggestion{Agents: i.Agents, Filter: filter, SelIdx: 0}
		i.cmdSuggest = CommandSuggestion{}
	} else {
		i.cmdSuggest = CommandSuggestion{}
		i.agentSuggest = AgentSuggestion{}
	}
}

func (i *InputArea) View() string {
	if i.Hidden {
		return ""
	}
	div := style.Divider(strings.Repeat("─", i.Width))
	prompt := style.CyanStyle.Render("❯ ")
	cursor := style.BoldWhite.Render("█")
	text := i.TextBuffer.String()
	var buf strings.Builder
	buf.WriteString(div)
	buf.WriteByte('\n')
	buf.WriteString(prompt)
	displayText := text
	if displayText == "" {
		displayText = style.DimStyle.Render("你的消息...")
		buf.WriteString(displayText)
	} else {
		buf.WriteString(style.WhiteStyle.Render(displayText))
	}
	buf.WriteString(cursor)
	buf.WriteByte('\n')
	buf.WriteString(div)

	suggestView := i.cmdSuggest.View(i.Width)
	if suggestView == "" {
		suggestView = i.agentSuggest.View(i.Width)
	}
	if suggestView != "" {
		buf.WriteByte('\n')
		buf.WriteString(suggestView)
	}
	return buf.String()
}
