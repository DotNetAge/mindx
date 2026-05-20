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
	Executing  bool // true while T-A-O loop is running; esc sends CancelMsg
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
		if cmd := i.executeSuggestion(); cmd != nil {
			return i, cmd
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

	case "ctrl+o":
		return i, func() tea.Msg {
			return clientmsg.CollapseToggleMsg{ActionIndex: -1}
		}

	case "tab":
		i.handleTab()
		return i, nil

	case "up":
		i.navigateSuggestion(-1)
		return i, nil

	case "down":
		i.navigateSuggestion(1)
		return i, nil

	case "esc":
		if i.Executing {
			// T-A-O loop is running: cancel execution
			return i, func() tea.Msg {
				return clientmsg.ExecutionCancelMsg{}
			}
		}
		if i.hasActiveSuggestion() {
			i.resetSuggestions()
		} else if i.TextBuffer.Len() > 0 {
			i.TextBuffer.Reset()
			i.CursorPos = 0
		}
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
	if len(i.modelSuggest.Items) > 0 {
		list := i.modelSuggest.filtered()
		if len(list) > 0 {
			sel := list[i.modelSuggest.SelIdx]
			i.TextBuffer.Reset()
			i.TextBuffer.WriteString("/model " + sel.Name + " ")
			i.CursorPos = i.TextBuffer.Len()
		}
	} else if len(i.sessionSuggest.Items) > 0 {
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
	} else if len(i.cmdSuggest.Items) > 0 {
		list := i.cmdSuggest.filtered()
		if len(list) > 0 {
			sel := list[i.cmdSuggest.SelIdx]
			i.TextBuffer.Reset()
			i.TextBuffer.WriteString("/" + sel.Name + " ")
			i.CursorPos = i.TextBuffer.Len()
		}
	} else if len(i.agentSuggest.Items) > 0 {
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
	if len(i.modelSuggest.Items) > 0 {
		list := i.modelSuggest.filtered()
		if len(list) > 0 {
			i.modelSuggest.SelIdx = (i.modelSuggest.SelIdx + dir + len(list)) % len(list)
		}
	} else if len(i.sessionSuggest.Items) > 0 {
		list := i.sessionSuggest.filtered()
		if len(list) > 0 {
			i.sessionSuggest.SelIdx = (i.sessionSuggest.SelIdx + dir + len(list)) % len(list)
		}
	} else if len(i.cmdSuggest.Items) > 0 {
		list := i.cmdSuggest.filtered()
		if len(list) > 0 {
			i.cmdSuggest.SelIdx = (i.cmdSuggest.SelIdx + dir + len(list)) % len(list)
		}
	} else if len(i.agentSuggest.Items) > 0 {
		list := i.agentSuggest.filtered()
		if len(list) > 0 {
			i.agentSuggest.SelIdx = (i.agentSuggest.SelIdx + dir + len(list)) % len(list)
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
	return len(i.modelSuggest.Items) > 0 ||
		len(i.sessionSuggest.Items) > 0 ||
		len(i.cmdSuggest.Items) > 0 ||
		len(i.agentSuggest.Items) > 0
}

// executeSuggestion returns a Cmd when Enter is pressed with an active suggestion panel.
// For command/model/session suggestions, it sends the appropriate command immediately.
// For agent suggestions, it returns nil (falls through to normal message send).
func (i *InputArea) executeSuggestion() tea.Cmd {
	if len(i.cmdSuggest.Items) > 0 {
		list := i.cmdSuggest.filtered()
		if len(list) > 0 && i.cmdSuggest.SelIdx < len(list) {
			sel := list[i.cmdSuggest.SelIdx]
			i.TextBuffer.Reset()
			i.CursorPos = 0
			i.resetSuggestions()
			return func() tea.Msg {
				return clientmsg.SlashCommandMsg{Name: sel.Name, Args: nil}
			}
		}
	}
	if len(i.modelSuggest.Items) > 0 {
		list := i.modelSuggest.filtered()
		if len(list) > 0 && i.modelSuggest.SelIdx < len(list) {
			sel := list[i.modelSuggest.SelIdx]
			i.TextBuffer.Reset()
			i.CursorPos = 0
			i.resetSuggestions()
			return func() tea.Msg {
				return clientmsg.SlashCommandMsg{Name: "model", Args: []string{sel.Name}}
			}
		}
	}
	if len(i.sessionSuggest.Items) > 0 {
		list := i.sessionSuggest.filtered()
		if len(list) > 0 && i.sessionSuggest.SelIdx < len(list) {
			sel := list[i.sessionSuggest.SelIdx]
			i.TextBuffer.Reset()
			i.CursorPos = 0
			i.resetSuggestions()
			arg := sel.ID
			if sel.IsSpecial {
				arg = sel.SpecialType
			}
			return func() tea.Msg {
				return clientmsg.SlashCommandMsg{Name: "chat", Args: []string{arg}}
			}
		}
	}
	return nil
}

func (i *InputArea) updateSuggestion() {
	text := i.TextBuffer.String()
	if strings.HasPrefix(text, "/model") {
		filter := ""
		if len(text) > 6 {
			filter = strings.TrimPrefix(text, "/model ")
		}
		i.modelSuggest = ModelSuggestion{Suggestion: Suggestion[ModelItem]{Items: i.Models, Filter: filter, SelIdx: 0}}
		i.cmdSuggest = CommandSuggestion{}
		i.agentSuggest = AgentSuggestion{}
		i.sessionSuggest = SessionSuggestion{}
	} else if strings.HasPrefix(text, "/chat ") {
		filter := strings.TrimPrefix(text, "/chat ")
		i.sessionSuggest = SessionSuggestion{Suggestion: Suggestion[SessionItem]{Items: i.Sessions, Filter: filter, SelIdx: 0}}
		i.cmdSuggest = CommandSuggestion{}
		i.agentSuggest = AgentSuggestion{}
		i.modelSuggest = ModelSuggestion{}
	} else if strings.HasPrefix(text, "/") {
		filter := strings.TrimPrefix(text, "/")
		i.cmdSuggest = CommandSuggestion{Suggestion: Suggestion[SlashCommand]{Items: i.Commands, Filter: filter, SelIdx: 0}}
		i.agentSuggest = AgentSuggestion{}
		i.modelSuggest = ModelSuggestion{}
		i.sessionSuggest = SessionSuggestion{}
	} else if strings.HasPrefix(text, "@") {
		filter := strings.TrimPrefix(text, "@")
		i.agentSuggest = AgentSuggestion{Suggestion: Suggestion[data.AgentInfo]{Items: i.Agents, Filter: filter, SelIdx: 0}}
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
	if text == "" {
		buf.WriteString(style.GrayStyle.Render("你的消息..."))
	} else {
		buf.WriteString(style.WhiteStyle.Render(text))
	}
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
