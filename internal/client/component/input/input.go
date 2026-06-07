package input

import (
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/mindx/internal/client/data"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/style"
)

var isDarwin = runtime.GOOS == "darwin"

type InputArea struct {
	Width      int
	TextBuffer strings.Builder
	CursorPos  int
	Hidden     bool
	Executing  bool
	Agents     []data.AgentInfo
	Commands   []SlashCommand
	Models     []ModelItem
	Sessions   []SessionItem

	agentSuggest   AgentSuggestion
	cmdSuggest     CommandSuggestion
	modelSuggest   ModelSuggestion
	sessionSuggest SessionSuggestion

	history      []string
	historyIdx    int
	historyTmp    string
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
	case tea.PasteMsg:
		if i.Hidden {
			return i, nil
		}
		for _, r := range m.Content {
			i.insertAtCursor(r)
		}
		return i, nil
	}
	return i, nil
}

func (i *InputArea) handleKey(k tea.KeyPressMsg) (*InputArea, tea.Cmd) {
	key := tea.Key(k)

	switch {
	case key.Code == tea.KeyEnter || key.Code == '\n' || key.Code == '\r':
		if key.Mod.Contains(tea.ModAlt) {
			i.insertAtCursor('\n')
			return i, nil
		}
		if cmd := i.executeSuggestion(); cmd != nil {
			return i, cmd
		}
		text := strings.TrimSpace(i.TextBuffer.String())
		if text == "" {
			return i, nil
		}
		i.addToHistory(text)
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

	case i.isClearScreenShortcut(key):
		return i, func() tea.Msg { return clientmsg.ClearScreenMsg{} }

	case key.Code == tea.KeyTab:
		i.handleTab()
		return i, nil

	case key.Code == tea.KeyUp:
		if len(i.history) > 0 && i.historyIdx > 0 {
			if i.historyIdx == len(i.history) {
				i.historyTmp = i.TextBuffer.String()
			}
			i.historyIdx--
			i.setText(i.history[i.historyIdx])
		} else {
			i.navigateSuggestion(-1)
		}
		return i, nil

	case key.Code == tea.KeyDown:
		if i.historyIdx < len(i.history) {
			i.historyIdx++
			if i.historyIdx == len(i.history) {
				i.setText(i.historyTmp)
			} else {
				i.setText(i.history[i.historyIdx])
			}
		} else {
			i.navigateSuggestion(1)
		}
		return i, nil

	case key.Code == tea.KeyLeft:
		if i.CursorPos > 0 {
			i.CursorPos--
		}
		return i, nil

	case key.Code == tea.KeyRight:
		s := i.TextBuffer.String()
		if i.CursorPos < len([]rune(s)) {
			i.CursorPos++
		}
		return i, nil

	case i.isHomeShortcut(key):
		i.CursorPos = 0
		return i, nil

	case i.isEndShortcut(key):
		i.CursorPos = len([]rune(i.TextBuffer.String()))
		return i, nil

	case key.Code == tea.KeyDelete:
		i.deleteAfterCursor()
		return i, nil

	case key.Code == tea.KeyEsc:
		if i.Executing {
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
		i.historyIdx = len(i.history)
		return i, nil

	case key.Code == ' ' || key.ShiftedCode == ' ':
		i.insertAtCursor(' ')
		return i, nil

	case key.Code == tea.KeyBackspace:
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

	case i.isDeleteToStartOfLine(key):
		s := i.TextBuffer.String()
		runes := []rune(s)
		if i.CursorPos > 0 {
			i.TextBuffer.Reset()
			i.TextBuffer.WriteString(string(runes[i.CursorPos:]))
			i.CursorPos = 0
			i.updateSuggestion()
		}
		return i, nil

	case i.isDeleteToEndOfLine(key):
		s := i.TextBuffer.String()
		runes := []rune(s)
		if i.CursorPos < len(runes) {
			i.TextBuffer.Reset()
			i.TextBuffer.WriteString(string(runes[:i.CursorPos]))
			i.updateSuggestion()
		}
		return i, nil

	case i.isDeleteWord(key):
		i.deleteWordBeforeCursor()
		return i, nil

	default:
		if i.isPrintableKey(key) {
			if key.ShiftedCode != 0 {
				i.insertAtCursor(key.ShiftedCode)
			} else {
				i.insertAtCursor(key.Code)
			}
		}
		return i, nil
	}
}

func (i *InputArea) isPrintableKey(k tea.Key) bool {
	if k.Mod != 0 && !k.Mod.Contains(tea.ModShift) {
		return false
	}
	if k.Code >= tea.KeySpace && k.Code <= '~' {
		return true
	}
	if k.Code >= 0x80 && k.Code != tea.KeyExtended {
		return true
	}
	return false
}

func (i *InputArea) isClearScreenShortcut(k tea.Key) bool {
	if isDarwin {
		return k.Mod.Contains(tea.ModSuper) && (k.Code == 'l' || k.Code == 'L')
	}
	return k.Mod.Contains(tea.ModCtrl) && (k.Code == 'l' || k.Code == 'L')
}

func (i *InputArea) isHomeShortcut(k tea.Key) bool {
	if k.Code == tea.KeyHome {
		return true
	}
	if isDarwin {
		return k.Mod.Contains(tea.ModSuper) && (k.Code == tea.KeyLeft || k.Code == 'B' || k.Code == 'b')
	}
	return k.Mod.Contains(tea.ModCtrl) && (k.Code == 'a' || k.Code == 'A')
}

func (i *InputArea) isEndShortcut(k tea.Key) bool {
	if k.Code == tea.KeyEnd {
		return true
	}
	if isDarwin {
		return k.Mod.Contains(tea.ModSuper) && (k.Code == tea.KeyRight || k.Code == 'E' || k.Code == 'e')
	}
	return k.Mod.Contains(tea.ModCtrl) && (k.Code == 'e' || k.Code == 'E')
}

func (i *InputArea) isDeleteToStartOfLine(k tea.Key) bool {
	if isDarwin {
		return k.Mod.Contains(tea.ModSuper) && (k.Code == tea.KeyBackspace || k.Code == 'U' || k.Code == 'u')
	}
	return k.Mod.Contains(tea.ModCtrl) && (k.Code == 'u' || k.Code == 'U')
}

func (i *InputArea) isDeleteToEndOfLine(k tea.Key) bool {
	if isDarwin {
		return k.Mod.Contains(tea.ModSuper) && (k.Code == tea.KeyDelete || k.Code == 'K' || k.Code == 'k')
	}
	return k.Mod.Contains(tea.ModCtrl) && (k.Code == 'k' || k.Code == 'K')
}

func (i *InputArea) isDeleteWord(k tea.Key) bool {
	if isDarwin {
		return k.Mod.Contains(tea.ModAlt) && (k.Code == tea.KeyBackspace || k.Code == 'W' || k.Code == 'w')
	}
	return k.Mod.Contains(tea.ModCtrl) && (k.Code == 'w' || k.Code == 'W')
}

func (i *InputArea) insertAtCursor(ch rune) {
	s := i.TextBuffer.String()
	runes := []rune(s)
	pos := i.CursorPos
	var newRunes []rune
	newRunes = append(newRunes, runes[:pos]...)
	newRunes = append(newRunes, ch)
	newRunes = append(newRunes, runes[pos:]...)
	i.TextBuffer.Reset()
	i.TextBuffer.WriteString(string(newRunes))
	i.CursorPos++
	i.updateSuggestion()
}

func (i *InputArea) deleteAfterCursor() {
	s := i.TextBuffer.String()
	runes := []rune(s)
	if i.CursorPos < len(runes) {
		runes = append(runes[:i.CursorPos], runes[i.CursorPos+1:]...)
		i.TextBuffer.Reset()
		i.TextBuffer.WriteString(string(runes))
		i.updateSuggestion()
	}
}

func (i *InputArea) deleteWordBeforeCursor() {
	s := i.TextBuffer.String()
	runes := []rune(s)
	if i.CursorPos <= 0 {
		return
	}
	pos := i.CursorPos - 1
	for pos >= 0 && !isWordBoundary(runes[pos]) {
		pos--
	}
	for pos >= 0 && isWordBoundary(runes[pos]) {
		pos--
	}
	pos++
	runes = append(runes[:pos], runes[i.CursorPos:]...)
	i.TextBuffer.Reset()
	i.TextBuffer.WriteString(string(runes))
	i.CursorPos = pos
	i.updateSuggestion()
}

func isWordBoundary(r rune) bool {
	return r == ' ' || r == '\t' || r == '-' || r == '_'
}

func (i *InputArea) setText(s string) {
	i.TextBuffer.Reset()
	i.TextBuffer.WriteString(s)
	i.CursorPos = len([]rune(s))
	i.resetSuggestions()
}

func (i *InputArea) addToHistory(text string) {
	if text == "" {
		return
	}
	if len(i.history) == 0 || i.history[len(i.history)-1] != text {
		i.history = append(i.history, text)
	}
	i.historyIdx = len(i.history)
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
	runes := []rune(text)
	var buf strings.Builder
	buf.WriteString(div)
	buf.WriteByte('\n')
	buf.WriteString(prompt)
	if text == "" {
		buf.WriteString(style.GrayStyle.Render("发送消息或\"/\"选择指令..."))
	} else {
		pos := i.CursorPos
		if pos > len(runes) {
			pos = len(runes)
		}
		buf.WriteString(style.WhiteStyle.Render(string(runes[:pos])))
		buf.WriteString(cursor)
		buf.WriteString(style.WhiteStyle.Render(string(runes[pos:])))
	}
	if len(runes) == 0 {
		buf.WriteString(cursor)
	}
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
