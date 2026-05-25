package dialog

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/DotNetAge/mindx/internal/client/style"
)

type ListDialog struct {
	Visible     bool
	Title       string
	Items       []string
	SearchLabel string

	search textinput.Model
	cursor int
	filter string
	width  int
}

func NewListDialog(title string) *ListDialog {
	ti := textinput.New()
	ti.Placeholder = "Search"

	s := textinput.DefaultDarkStyles()
	s.Focused.Prompt = lipgloss.NewStyle().Foreground(style.ThemePurple)
	s.Focused.Text = style.WhiteStyle
	ti.SetStyles(s)
	ti.Focus()

	return &ListDialog{
		Title:       title,
		SearchLabel: "Search",
		search:      ti,
	}
}

func (d *ListDialog) SetItems(items []string) {
	d.Items = items
	d.cursor = 0
	d.filter = ""
	d.search.SetValue("")
	d.Visible = true
}

func (d *ListDialog) filteredItems() []string {
	if d.filter == "" {
		return d.Items
	}
	var result []string
	for _, item := range d.Items {
		if strings.Contains(strings.ToLower(item), strings.ToLower(d.filter)) {
			result = append(result, item)
		}
	}
	return result
}

func (d *ListDialog) Update(msg any) (*ListDialog, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		d.width = m.Width
		if d.Visible {
			d.search.SetWidth(dialogWidth(d.width) - 6)
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
				return ListDialogResult{Cancelled: true}
			}
		case tea.KeyEnter:
			filtered := d.filteredItems()
			if len(filtered) > 0 && d.cursor < len(filtered) {
				selected := filtered[d.cursor]
				d.Visible = false
				return d, func() tea.Msg {
					return ListDialogResult{
						Index:     d.findOriginalIndex(selected),
						Value:     selected,
						Cancelled: false,
					}
				}
			}
			return d, nil
		case tea.KeyUp:
			filtered := d.filteredItems()
			if len(filtered) > 0 {
				d.cursor--
				if d.cursor < 0 {
					d.cursor = len(filtered) - 1
				}
			}
			return d, nil
		case tea.KeyDown:
			filtered := d.filteredItems()
			if len(filtered) > 0 {
				d.cursor++
				if d.cursor >= len(filtered) {
					d.cursor = 0
				}
			}
			return d, nil
		default:
			newSearch, searchCmd := d.search.Update(msg)
			d.search = newSearch
			d.filter = d.search.Value()
			d.cursor = 0
			return d, searchCmd
		}

	case tea.PasteMsg:
		if !d.Visible {
			return d, nil
		}
		newSearch, searchCmd := d.search.Update(msg)
		d.search = newSearch
		d.filter = d.search.Value()
		d.cursor = 0
		return d, searchCmd
	}
	return d, nil
}

func (d *ListDialog) findOriginalIndex(value string) int {
	for i, item := range d.Items {
		if item == value {
			return i
		}
	}
	return -1
}

func (d *ListDialog) View() string {
	if !d.Visible || len(d.Items) == 0 {
		return ""
	}

	w := dialogWidth(d.width)
	innerW := w - 4
	if innerW < 10 {
		innerW = 10
	}

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

	searchView := d.search.View()

	filtered := d.filteredItems()
	listContent := d.renderList(filtered)

	body := lipgloss.JoinVertical(lipgloss.Left,
		"", titleRow, "",
		searchView, "",
		listContent,
	)

	return dialogBorder.Width(w).Render(body)
}

func (d *ListDialog) renderList(items []string) string {
	if len(items) == 0 {
		return style.DimStyle.Render("  (无匹配项)")
	}

	var lines []string
	for i, item := range items {
		var cursor string
		var label string
		if i == d.cursor {
			cursor = style.PurpleStyle.Render("●")
			label = style.WhiteStyle.Render(item)
		} else {
			cursor = " "
			label = style.GrayStyle.Render(item)
		}
		line := fmt.Sprintf("  %s %s", cursor, label)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

type ListDialogResult struct {
	Index     int
	Value     string
	Cancelled bool
}

func dialogWidth(termWidth int) int {
	w := termWidth - 8
	if w < 40 {
		w = 40
	}
	if w > 56 {
		w = 56
	}
	return w
}
