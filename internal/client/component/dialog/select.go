package dialog

import (
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/DotNetAge/mindx/internal/client/component/choices"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/style"
)

var (
	dialogBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(style.ThemePurple).
		Padding(0, 2)
)

type SelectDialog struct {
	Visible bool
	Title   string
	Prompt  string
	Options []string
	panel   *choices.ChoicesPanel
	width   int
}

func NewSelectDialog(title string) *SelectDialog {
	return &SelectDialog{
		Title: title,
		panel: choices.New(),
	}
}

func (d *SelectDialog) SetOptions(prompt string, options []string) {
	d.Prompt = prompt
	d.Options = options
	d.panel.Update(clientmsg.ShowChoicesMsg{
		Prompt:         prompt,
		Options:        options,
		MultiSelect:    false,
		AllowTextInput: true,
	})
	d.Visible = true
}

func (d *SelectDialog) Update(msg any) (*SelectDialog, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		d.width = m.Width
		if d.Visible {
			dialogW := min(d.width-8, 56)
			if dialogW < 40 {
				dialogW = 40
			}
			d.panel.Update(clientmsg.WindowResizeMsg{Width: dialogW - 6})
		}
		return d, nil

	case tea.KeyPressMsg:
		if !d.Visible {
			return d, nil
		}
		key := tea.Key(m)
		switch key.Code {
		case tea.KeyEsc:
			d.Visible = false
			return d, func() tea.Msg {
				return SelectDialogResult{Cancelled: true}
			}
		default:
			newPanel, cmd := d.panel.Update(msg)
			d.panel = newPanel
			if !newPanel.Visible {
				d.Visible = false
				result := cmd()
				if sel, ok := result.(clientmsg.ChoiceSelectedMsg); ok {
					return d, func() tea.Msg {
						return SelectDialogResult{
							Index:      sel.Index,
							CustomText: sel.CustomText,
							Cancelled:  sel.Index < 0,
						}
					}
				}
			}
			return d, cmd
		}
	}
	return d, nil
}

func (d *SelectDialog) View() string {
	if !d.Visible || len(d.Options) == 0 {
		return ""
	}

	titleLine := style.BoldWhite.Render("  " + d.Title)
	content := d.panel.View()
	footer := style.DimStyle.Render(" Enter 确认 │ Esc 取消")

	w := d.dialogWidth()
	innerW := w - 4
	if innerW < 10 {
		innerW = 10
	}

	body := lipgloss.JoinVertical(lipgloss.Left, "", titleLine, "",
		lipgloss.NewStyle().Width(innerW).Render(content), "",
		footer)

	return dialogBorder.Width(w).Render(body)
}

func (d *SelectDialog) dialogWidth() int {
	w := d.width - 8
	if w < 40 {
		w = 40
	}
	if w > 56 {
		w = 56
	}
	return w
}

type SelectDialogResult struct {
	Index      int
	CustomText string
	Cancelled  bool
}
