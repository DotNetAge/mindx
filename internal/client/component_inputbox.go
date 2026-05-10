package client

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// InputBox 底部输入栏。包含 textarea 编辑区 + 两个互斥的 suggestion 列表。
// Enter 发送消息，Alt+Enter 换行。
type InputBox struct {
	textarea       textarea.Model
	suggestAg      *AgentSuggestions
	suggestCmd     CommandSuggestions
	registry       *SlashCommandRegistry
	hidden         bool
	justCompleted  bool
	showAgSuggest  bool
	showCmdSuggest bool
}

func NewInputBox(registry *SlashCommandRegistry) InputBox {
	ta := textarea.New()
	ta.Placeholder = "输入消息... (Enter 发送, Alt+Enter 换行)"
	ta.Focus()
	ta.CharLimit = 0
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetKeys("alt+enter")
	ta.SetHeight(1)

	return InputBox{
		textarea:   ta,
		suggestAg:  NewAgentSuggestions(),
		suggestCmd: NewCommandSuggestions(registry),
		registry:   registry,
	}
}

func (b *InputBox) SetWidth(w int) {
	b.textarea.SetWidth(w)
	b.suggestAg.SetWidth(w)
	b.suggestCmd.SetWidth(w)
}

func (b *InputBox) SetAgents(agents []agentInfo) {
	b.suggestAg.SetAgents(agents)
}

func (b *InputBox) SetHidden(v bool) {
	b.hidden = v
	if v {
		b.textarea.Blur()
	} else {
		b.textarea.Focus()
	}
}

func (b *InputBox) InsertText(text string) {
	b.textarea.SetValue(text)
	b.suggestAg.Dismiss()
	b.suggestCmd.Dismiss()
	b.justCompleted = true // 标记刚从 suggestion 补全
}

func (b *InputBox) IsFocused() bool {
	return b.hidden == false && b.textarea.Focused()
}

func (b *InputBox) HasSuggestion() bool {
	return b.showAgSuggest || b.showCmdSuggest
}

func (b *InputBox) JustCompleted() bool {
	return b.justCompleted
}

func (b *InputBox) HandlePaste(msg tea.PasteMsg) (InputBox, tea.Cmd) {
	if b.hidden {
		return *b, nil
	}
	var cmd tea.Cmd
	b.textarea, cmd = b.textarea.Update(msg)
	b.updateSuggestions()
	return *b, cmd
}

func (b *InputBox) HandleKey(msg tea.KeyPressMsg) (InputBox, tea.Cmd) {
	if b.hidden {
		return *b, nil
	}

	switch msg.String() {
	case "ctrl+c":
		return *b, func() tea.Msg { return exitMsg{} }
	case "ctrl+l":
		b.textarea.Reset()
		return *b, func() tea.Msg { return clearScreenMsg{} }
	case "enter":
		text := b.textarea.Value()
		if strings.TrimSpace(text) == "" {
			return *b, nil
		}

		// 如果刚从 suggestion 补全，重置 suggestion 状态但继续正常处理
		if b.justCompleted {
			b.justCompleted = false
			b.showAgSuggest = false
			b.showCmdSuggest = false
		}

		b.textarea.Reset()
		b.suggestAg.Dismiss()
		b.suggestCmd.Dismiss()

		if strings.HasPrefix(text, "/") {
			parts := strings.Fields(text)
			cmdName := parts[0] // "/models"
			// 去掉前导 / 再查找（注册时不带 /）
			searchName := strings.TrimPrefix(cmdName, "/")
			cmd := b.registry.Find(searchName)
			if cmd != nil && cmd.Run != nil {
				var args string
				if len(parts) > 1 {
					args = strings.TrimPrefix(text, cmdName)
					args = strings.TrimSpace(args)
				}
				result := cmd.Run(args)
				if result.ClearChat {
					return *b, func() tea.Msg { return clearScreenMsg{} }
				}
				if result.Message == "EXIT" {
					return *b, func() tea.Msg { return exitMsg{} }
				}
				if result.Message != "" {
					return *b, func() tea.Msg { return localDisplayMsg{markdown: result.Message} }
				}
				return *b, nil
			}

			// 命令未找到，不发给 LLM，显示错误
			return *b, func() tea.Msg {
				return localDisplayMsg{markdown: fmt.Sprintf("❌ 未知命令: %s", cmdName)}
			}
		}

		return *b, func() tea.Msg {
			return sendMsg{text: text}
		}
	case "esc":
		b.suggestAg.Dismiss()
		b.suggestCmd.Dismiss()
		return *b, nil // Esc 只关闭 suggestion，绝不退出！
	case "up", "down":
		// Only let the suggestion list handle navigation if visible
		if b.HasSuggestion() {
			return *b, nil
		}
	}

	// 过滤掉 Esc 键，不让 textarea 处理（textarea 默认会用 Esc 退出）
	if msg.String() == "esc" {
		return *b, nil
	}

	var cmd tea.Cmd
	b.textarea, cmd = b.textarea.Update(msg)

	// Then update suggestions
	b.updateSuggestions()

	return *b, cmd
}

func (b *InputBox) updateSuggestions() {
	value := b.textarea.Value()

	// Check for @agent suggestion
	if b.suggestAg.Trigger(value) {
		b.suggestCmd.Dismiss()
		b.showAgSuggest = true
		b.showCmdSuggest = false
		return
	}

	// Check for /command suggestion
	if b.suggestCmd.Trigger(value) {
		b.suggestAg.Dismiss()
		b.showCmdSuggest = true
		b.showAgSuggest = false
		return
	}

	b.showAgSuggest = false
	b.showCmdSuggest = false
}

func (b *InputBox) UpdateSuggestions(msg tea.Msg) (InputBox, tea.Cmd) {
	if b.showAgSuggest {
		ag, cmd := b.suggestAg.Update(msg)
		b.suggestAg = ag
		return *b, cmd
	}
	if b.showCmdSuggest {
		cs, cmd := b.suggestCmd.Update(msg)
		b.suggestCmd = cs
		return *b, cmd
	}
	return *b, nil
}

func (b *InputBox) View() string {
	if b.hidden {
		return ""
	}

	return lipgloss.NewStyle().
		Width(b.textarea.Width()).
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderBottom(true).
		Render(b.textarea.View())
}

func (b *InputBox) SuggestionView() string {
	if b.showAgSuggest {
		return b.suggestAg.View()
	} else if b.showCmdSuggest {
		return b.suggestCmd.View()
	}
	return ""
}
