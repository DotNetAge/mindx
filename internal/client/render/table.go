package render

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/DotNetAge/mindx/internal/client/style"
)

type Table struct {
	Headers []string
	Rows    [][]string
	Width   int
}

func NewTable(headers []string, width int) *Table {
	return &Table{
		Headers: headers,
		Width:   width,
	}
}

func (t *Table) AddRow(row []string) {
	t.Rows = append(t.Rows, row)
}

func (t *Table) Render() string {
	if len(t.Headers) == 0 && len(t.Rows) == 0 {
		return ""
	}

	colCount := len(t.Headers)
	if colCount == 0 && len(t.Rows) > 0 {
		colCount = len(t.Rows[0])
	}

	colWidths := t.calculateColumnWidths(colCount)

	var b strings.Builder

	b.WriteString(t.renderRow(t.Headers, colWidths, true))
	b.WriteString(t.renderSeparator(colWidths))

	for _, row := range t.Rows {
		b.WriteString(t.renderRow(row, colWidths, false))
	}

	return b.String()
}

func (t *Table) calculateColumnWidths(colCount int) []int {
	widths := make([]int, colCount)

	for i, header := range t.Headers {
		if len(header) > widths[i] {
			widths[i] = len(header)
		}
	}

	for _, row := range t.Rows {
		for i, cell := range row {
			if i < colCount && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	totalWidth := 0
	for _, w := range widths {
		totalWidth += w + 3
	}
	totalWidth--

	if totalWidth > t.Width && t.Width > 0 {
		scale := float64(t.Width) / float64(totalWidth)
		for i := range widths {
			widths[i] = int(float64(widths[i]+3) * scale)
		}
	}

	return widths
}

func (t *Table) renderRow(cells []string, widths []int, isHeader bool) string {
	var b strings.Builder
	styleFunc := style.WhiteStyle
	if isHeader {
		styleFunc = style.BoldWhite
	}

	for i, cell := range cells {
		if i < len(widths) {
			padded := fmt.Sprintf(" %-*s ", widths[i], cell)
			b.WriteString(styleFunc.Render(padded))
			if i < len(cells)-1 {
				b.WriteString(style.DimStyle.Render("│"))
			}
		}
	}
	b.WriteByte('\n')
	return b.String()
}

func (t *Table) renderSeparator(widths []int) string {
	var b strings.Builder
	for _, w := range widths {
		b.WriteString(style.DimStyle.Render("┼" + strings.Repeat("─", w+2)))
	}
	b.WriteString(style.DimStyle.Render("┼\n"))
	return b.String()
}

func RenderTable(headers []string, rows [][]string, width int) string {
	t := NewTable(headers, width)
	for _, row := range rows {
		t.AddRow(row)
	}
	return t.Render()
}

type TableCell struct {
	Content string
	Align   lipgloss.Position
	Style   lipgloss.Style
}

func NewCell(content string) TableCell {
	return TableCell{
		Content: content,
		Align:   lipgloss.Left,
		Style:   style.WhiteStyle,
	}
}

func (c TableCell) WithStyle(s lipgloss.Style) TableCell {
	c.Style = s
	return c
}

func (c TableCell) WithAlign(a lipgloss.Position) TableCell {
	c.Align = a
	return c
}
