package dialog

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/DotNetAge/mindx/internal/client/style"
)

type InputDialog struct {
	Visible    bool
	Title      string
	Placeholder string
	SubmitLabel string

	input textinput.Model
	width int
}

func NewInputDialog(title, placeholder string) *InputDialog {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.EchoMode = textinput.EchoNormal

	s := textinput.DefaultDarkStyles()
	s.Focused.Prompt = lipgloss.NewStyle().Foreground(style.ThemePurple)
	s.Focused.Text = style.WhiteStyle
	s.Focused.Placeholder = style.DimStyle
	ti.SetStyles(s)
	ti.Focus()

	return &InputDialog{
		Title:        title,
		Placeholder:  placeholder,
		SubmitLabel:  "enter submit",
		input:        ti,
	}
}

func (d *InputDialog) Update(msg any) (*InputDialog, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		d.width = m.Width
		if d.Visible {
			d.input.SetWidth(dialogWidth(d.width) - 6)
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
				return InputDialogResult{Cancelled: true}
			}
		case tea.KeyEnter:
			value := strings.TrimSpace(d.input.Value())
			d.Visible = false
			return d, func() tea.Msg {
				return InputDialogResult{
					Value:     value,
					Cancelled: false,
				}
			}
		default:
			newInput, inputCmd := d.input.Update(msg)
			d.input = newInput
			return d, inputCmd
		}

	case tea.PasteMsg:
		newInput, inputCmd := d.input.Update(msg)
		d.input = newInput
		return d, inputCmd
	}
	return d, nil
}

func (d *InputDialog) View() string {
	if !d.Visible {
		return ""
	}

	w := dialogWidth(d.width)
	innerW := w - 4

	titleLine := style.BoldWhite.Render("  " + d.Title)
	escHint := style.DimStyle.Render("esc")
	spacer := innerW - lipgloss.Width(titleLine) - lipgloss.Width(escHint)
	if spacer < 1 {
		spacer = 1
	}
	titleRow := lipgloss.JoinHorizontal(lipgloss.Left,
		titleLine,
		lipgloss.NewStyle().Width(spacer).Render(""),
		escHint,
	)

	inputView := d.input.View()
	footer := style.DimStyle.Render(" " + d.SubmitLabel)

	body := lipgloss.JoinVertical(lipgloss.Left,
		"", titleRow, "",
		inputView, "",
		footer,
	)

	return dialogBorder.Width(w).Render(body)
}

func (d *InputDialog) SetValue(v string) {
	d.input.SetValue(v)
}

type InputDialogResult struct {
	Value     string
	Cancelled bool
}
