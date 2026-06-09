package choices

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/style"
	"github.com/DotNetAge/mindx/internal/i18n"
)

type choiceItem string

func (i choiceItem) FilterValue() string { return string(i) }

type choiceDelegate struct {
	multiSelect bool
	selected    map[int]bool
}

func (d choiceDelegate) Height() int                             { return 1 }
func (d choiceDelegate) Spacing() int                            { return 0 }
func (d choiceDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d choiceDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(choiceItem)
	if !ok {
		return
	}
	isCursor := index == m.Index()
	isChecked := d.selected != nil && d.selected[index]

	var line string
	switch {
	case d.multiSelect:
		check := "[ ] "
		if isChecked {
			check = "[✓] "
		}
		dot := "○"
		if isCursor {
			dot = "●"
		}
		line = dot + " " + check + string(item)
	default:
		dot := "○"
		if isCursor {
			dot = "●"
		}
		if isChecked {
			dot = "●"
		}
		line = dot + "  " + string(item)
	}

	if isCursor {
		fmt.Fprint(w, style.CyanStyle.Render(line))
	} else {
		fmt.Fprint(w, style.DimStyle.Render(line))
	}
}

type ChoicesPanel struct {
	Visible        bool
	Items          []string
	Prompt         string
	Cursor         int
	MultiSelect    bool
	AllowTextInput bool
	Selected       map[int]bool
	CustomText     string
	inputActive    bool
	list           list.Model
	width          int
}

func New() *ChoicesPanel {
	return &ChoicesPanel{}
}

func (p *ChoicesPanel) buildList() {
	items := make([]list.Item, len(p.Items))
	for i, opt := range p.Items {
		items[i] = choiceItem(opt)
	}

	w := p.width
	if w == 0 {
		w = 80
	}
	height := len(p.Items) + 2
	if p.AllowTextInput {
		height += 3
	}
	if height < 5 {
		height = 5
	}
	if height > 16 {
		height = 16
	}

	d := choiceDelegate{
		multiSelect: p.MultiSelect,
		selected:    p.Selected,
	}
	l := list.New(items, d, w, height)
	l.Title = p.Prompt
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)

	p.list = l
}

func (p *ChoicesPanel) rebuildView() {
	if !p.Visible {
		return
	}
	oldIdx := 0
	if p.list.Width() > 0 {
		oldIdx = p.list.Index()
	}
	p.buildList()
	if oldIdx < len(p.Items) {
		p.list.Select(oldIdx)
	}
}

func (p *ChoicesPanel) Update(msg any) (*ChoicesPanel, tea.Cmd) {
	switch msg := msg.(type) {
	case clientmsg.ShowChoicesMsg:
		p.Visible = true
		p.Items = msg.Options
		p.Prompt = msg.Prompt
		p.Cursor = 0
		p.MultiSelect = msg.MultiSelect
		p.AllowTextInput = msg.AllowTextInput
		p.CustomText = ""
		// Auto-activate text input when no options (free-text question).
		p.inputActive = len(msg.Options) == 0 && msg.AllowTextInput
		if p.MultiSelect || p.inputActive {
			if p.Selected == nil {
				p.Selected = make(map[int]bool)
			} else {
				clear(p.Selected)
			}
		}
		p.buildList()
		return p, nil

	case tea.WindowSizeMsg:
		p.width = msg.Width
		if p.Visible {
			p.rebuildView()
		}
		return p, nil

	case tea.KeyPressMsg:
		if !p.Visible {
			return p, nil
		}
		key := tea.Key(msg)

		if p.inputActive && p.AllowTextInput {
			return p.handleInputMode(key)
		}

		switch key.Code {
		case tea.KeyEnter:
			p.Visible = false
			// No items: just submit custom text (only reachable if text was active
			// and user tabbed away, or empty-options non-input mode fallback).
			if len(p.Items) == 0 {
				return p, func() tea.Msg {
					return clientmsg.ChoiceSelectedMsg{CustomText: p.CustomText}
				}
			}
			if p.MultiSelect {
				var indices []int
				for i, v := range p.Selected {
					if v {
						indices = append(indices, i)
					}
				}
				return p, func() tea.Msg {
					return clientmsg.ChoiceSelectedMsg{Indices: indices, CustomText: p.CustomText}
				}
			}
			p.Cursor = p.list.Index()
			return p, func() tea.Msg {
				return clientmsg.ChoiceSelectedMsg{Index: p.Cursor}
			}
		case tea.KeyEsc:
			p.inputActive = false
			p.Visible = false
			return p, func() tea.Msg {
				return clientmsg.ChoiceSelectedMsg{Index: -1}
			}
		case ' ':
			if p.MultiSelect {
				idx := p.list.Index()
				if p.Selected[idx] {
					delete(p.Selected, idx)
				} else {
					p.Selected[idx] = true
				}
				p.rebuildView()
				return p, nil
			}
		case tea.KeyTab:
			// Tab toggles text input when AllowTextInput (both single and multi select).
			if p.AllowTextInput {
				p.inputActive = !p.inputActive
				return p, nil
			}
		default:
			var cmd tea.Cmd
			p.list, cmd = p.list.Update(msg)
			p.Cursor = p.list.Index()
			return p, cmd
		}
	}
	return p, nil
}

func (p *ChoicesPanel) handleInputMode(key tea.Key) (*ChoicesPanel, tea.Cmd) {
	switch key.Code {
	case tea.KeyEnter:
		p.Visible = false
		var indices []int
		for i, v := range p.Selected {
			if v {
				indices = append(indices, i)
			}
		}
		return p, func() tea.Msg {
			return clientmsg.ChoiceSelectedMsg{Indices: indices, CustomText: p.CustomText}
		}
	case tea.KeyEsc:
		p.inputActive = false
		return p, nil
	case tea.KeyTab, tea.KeyUp:
		p.inputActive = false
		return p, nil
	case tea.KeyBackspace:
		if len(p.CustomText) > 0 {
			p.CustomText = p.CustomText[:len(p.CustomText)-1]
		}
	default:
		if isPrintable(key) {
			ch := key.Code
			if key.ShiftedCode != 0 {
				ch = key.ShiftedCode
			}
			p.CustomText += string(ch)
		}
	}
	return p, nil
}

func isPrintable(k tea.Key) bool {
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

func (p *ChoicesPanel) View() string {
	if !p.Visible {
		return ""
	}
	if len(p.Items) == 0 && !p.AllowTextInput {
		return ""
	}

	var b strings.Builder
	if len(p.Items) > 0 {
		listView := strings.TrimSpace(p.list.View())
		b.WriteString(listView)
	}

	if p.AllowTextInput {
		if len(p.Items) > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(viewCustomInput(p))
	}

	return b.String()
}

func viewCustomInput(p *ChoicesPanel) string {
	label := style.BoldWhite.Render("  " + i18n.T("choices.input.other") + ": ")
	if p.inputActive {
		input := style.CyanStyle.Render(p.CustomText) + style.DimStyle.Render("▌")
		hint := style.GrayStyle.Render("  (" + i18n.T("choices.hint.input.active") + ")")
		return label + input + "\n" + hint
	}
	input := style.DimStyle.Render(p.CustomText)
	if input == "" {
		input = style.DimStyle.Render("(" + i18n.T("choices.input.placeholder") + ")")
	}
	hint := style.GrayStyle.Render("  (" + i18n.T("choices.hint.select") + ")")
	return label + input + "\n" + hint
}
