package dialog

import (
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/DotNetAge/mindx/internal/client/component/choices"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/style"
)

type OptionsDialog struct {
	Visible  bool
	Title    string
	Prompt   string
	Options  []string
	panel    *choices.ChoicesPanel
	width    int
}

func NewOptionsDialog(title string) *OptionsDialog {
	return &OptionsDialog{
		Title: title,
		panel: choices.New(),
	}
}

func (d *OptionsDialog) SetOptions(prompt string, options []string) {
	d.Prompt = prompt
	d.Options = options
	d.panel.Update(clientmsg.ShowChoicesMsg{
		Prompt:         prompt,
		Options:        options,
		MultiSelect:    true,
		AllowTextInput: true,
	})
	d.Visible = true
}

func (d *OptionsDialog) Update(msg any) (*OptionsDialog, tea.Cmd) {
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
				return OptionsDialogResult{Cancelled: true}
			}
		default:
			newPanel, cmd := d.panel.Update(msg)
			d.panel = newPanel
			if !newPanel.Visible {
				d.Visible = false
				result := cmd()
				if sel, ok := result.(clientmsg.ChoiceSelectedMsg); ok {
					return d, func() tea.Msg {
						return OptionsDialogResult{
							Indices:    sel.Indices,
							CustomText: sel.CustomText,
							Cancelled:   len(sel.Indices) == 0 && sel.CustomText == "",
						}
					}
				}
			}
			return d, cmd
		}
	}
	return d, nil
}

func (d *OptionsDialog) View() string {
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

func (d *OptionsDialog) dialogWidth() int {
	w := d.width - 8
	if w < 40 {
		w = 40
	}
	if w > 56 {
		w = 56
	}
	return w
}

type OptionsDialogResult struct {
	Indices    []int
	CustomText string
	Cancelled   bool
}
