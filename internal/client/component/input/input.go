package input

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/mindx/internal/client/data"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/style"
)

type InputArea struct {
	Width      int
	TextBuffer strings.Builder
	CursorPos  int
	Hidden     bool
	Agents     []data.AgentInfo
	Commands   []SlashCommand
	Models     []ModelItem
	Sessions   []SessionItem

	agentSuggest   AgentSuggestion
	cmdSuggest     CommandSuggestion
	modelSuggest   ModelSuggestion
	sessionSuggest SessionSuggestion
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
		if strings.Contains(k.String(), "alt") {
			i.TextBuffer.WriteByte('\n')
			i.CursorPos++
			i.updateSuggestion()
			return i, nil
		}
		if i.hasActiveSuggestion() {
			i.handleTab()
			return i, nil
		}
		text := strings.TrimSpace(i.TextBuffer.String())
		if text == "" {
			return i, nil
		}
		i.TextBuffer.Reset()
		i.CursorPos = 0
		i.resetSuggestions()
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
		i.handleTab()
		return i, nil

	case "up":
		i.navigateSuggestion(-1)
		return i, nil

	case "down":
		i.navigateSuggestion(1)
		return i, nil

	case "space":
		s := i.TextBuffer.String()
		runes := []rune(s)
		pos := i.CursorPos
		var newRunes []rune
		newRunes = append(newRunes, runes[:pos]...)
		newRunes = append(newRunes, ' ')
		newRunes = append(newRunes, runes[pos:]...)
		i.TextBuffer.Reset()
		i.TextBuffer.WriteString(string(newRunes))
		i.CursorPos++
		i.updateSuggestion()
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

func (i *InputArea) handleTab() {
	if len(i.modelSuggest.Models) > 0 {
		list := i.modelSuggest.filtered()
		if len(list) > 0 {
			sel := list[i.modelSuggest.SelIdx]
			i.TextBuffer.Reset()
			i.TextBuffer.WriteString("/model " + sel.Name + " ")
			i.CursorPos = i.TextBuffer.Len()
		}
	} else if len(i.sessionSuggest.Sessions) > 0 {
		list := i.sessionSuggest.filtered()
		if len(list) > 0 {
			sel := list[i.sessionSuggest.SelIdx]
			i.TextBuffer.Reset()
			if sel.IsSpecial {
				i.TextBuffer.WriteString("/chat " + sel.SpecialType + " ")
			} else {
				i.TextBuffer.WriteString("/chat " + sel.ID + " ")
			}
			i.CursorPos = i.TextBuffer.Len()
		}
	} else if len(i.cmdSuggest.Commands) > 0 {
		list := i.cmdSuggest.filtered()
		if len(list) > 0 {
			sel := list[i.cmdSuggest.SelIdx]
			i.TextBuffer.Reset()
			i.TextBuffer.WriteString("/" + sel.Name + " ")
			i.CursorPos = i.TextBuffer.Len()
		}
	} else if len(i.agentSuggest.Agents) > 0 {
		list := i.agentSuggest.filtered()
		if len(list) > 0 {
			sel := list[i.agentSuggest.SelIdx]
			i.TextBuffer.Reset()
			i.TextBuffer.WriteString("@" + sel.Name + " ")
			i.CursorPos = i.TextBuffer.Len()
		}
	}
	i.resetSuggestions()
}

func (i *InputArea) navigateSuggestion(dir int) {
	if len(i.modelSuggest.Models) > 0 {
		list := i.modelSuggest.filtered()
		newIdx := i.modelSuggest.SelIdx + dir
		if newIdx >= 0 && newIdx < len(list) {
			i.modelSuggest.SelIdx = newIdx
		}
	} else if len(i.sessionSuggest.Sessions) > 0 {
		list := i.sessionSuggest.filtered()
		newIdx := i.sessionSuggest.SelIdx + dir
		if newIdx >= 0 && newIdx < len(list) {
			i.sessionSuggest.SelIdx = newIdx
		}
	} else if len(i.cmdSuggest.Commands) > 0 {
		list := i.cmdSuggest.filtered()
		newIdx := i.cmdSuggest.SelIdx + dir
		if newIdx >= 0 && newIdx < len(list) {
			i.cmdSuggest.SelIdx = newIdx
		}
	} else if len(i.agentSuggest.Agents) > 0 {
		list := i.agentSuggest.filtered()
		newIdx := i.agentSuggest.SelIdx + dir
		if newIdx >= 0 && newIdx < len(list) {
			i.agentSuggest.SelIdx = newIdx
		}
	}
}

func (i *InputArea) resetSuggestions() {
	i.cmdSuggest = CommandSuggestion{}
	i.agentSuggest = AgentSuggestion{}
	i.modelSuggest = ModelSuggestion{}
	i.sessionSuggest = SessionSuggestion{}
}

func (i *InputArea) hasActiveSuggestion() bool {
	return len(i.modelSuggest.Models) > 0 ||
		len(i.sessionSuggest.Sessions) > 0 ||
		len(i.cmdSuggest.Commands) > 0 ||
		len(i.agentSuggest.Agents) > 0
}

func (i *InputArea) updateSuggestion() {
	text := i.TextBuffer.String()
	if strings.HasPrefix(text, "/model") {
		filter := ""
		if len(text) > 6 {
			filter = strings.TrimPrefix(text, "/model ")
		}
		i.modelSuggest = ModelSuggestion{Models: i.Models, Filter: filter, SelIdx: 0}
		i.cmdSuggest = CommandSuggestion{}
		i.agentSuggest = AgentSuggestion{}
		i.sessionSuggest = SessionSuggestion{}
	} else if strings.HasPrefix(text, "/chat ") {
		filter := strings.TrimPrefix(text, "/chat ")
		i.sessionSuggest = SessionSuggestion{Sessions: i.Sessions, Filter: filter, SelIdx: 0}
		i.cmdSuggest = CommandSuggestion{}
		i.agentSuggest = AgentSuggestion{}
		i.modelSuggest = ModelSuggestion{}
	} else if strings.HasPrefix(text, "/") {
		filter := strings.TrimPrefix(text, "/")
		i.cmdSuggest = CommandSuggestion{Commands: i.Commands, Filter: filter, SelIdx: 0}
		i.agentSuggest = AgentSuggestion{}
		i.modelSuggest = ModelSuggestion{}
		i.sessionSuggest = SessionSuggestion{}
	} else if strings.HasPrefix(text, "@") {
		filter := strings.TrimPrefix(text, "@")
		i.agentSuggest = AgentSuggestion{Agents: i.Agents, Filter: filter, SelIdx: 0}
		i.cmdSuggest = CommandSuggestion{}
		i.modelSuggest = ModelSuggestion{}
		i.sessionSuggest = SessionSuggestion{}
	} else {
		i.resetSuggestions()
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
	buf.WriteString(style.WhiteStyle.Render(text))
	buf.WriteString(cursor)
	buf.WriteByte('\n')
	buf.WriteString(div)

	suggestView := i.modelSuggest.View(i.Width)
	if suggestView == "" {
		suggestView = i.sessionSuggest.View(i.Width)
	}
	if suggestView == "" {
		suggestView = i.cmdSuggest.View(i.Width)
	}
	if suggestView == "" {
		suggestView = i.agentSuggest.View(i.Width)
	}
	if suggestView != "" {
		buf.WriteByte('\n')
		buf.WriteString(suggestView)
	}
	return buf.String()
}
