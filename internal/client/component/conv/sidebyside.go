package conv

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/DotNetAge/mindx/internal/client/style"
	"github.com/DotNetAge/mindx/internal/i18n"
)

type diffLine struct {
	kind byte
	text string
}

var (
	colLine   = lipgloss.NewStyle().Foreground(style.ThemeDim)
	colHdrOld = lipgloss.NewStyle().Foreground(style.ThemeRed).Bold(true)
	colHdrNew = lipgloss.NewStyle().Foreground(style.ThemeGreen).Bold(true)
	numStyle  = lipgloss.NewStyle().Foreground(style.ThemeDarkGray)
)

func ViewSideBySideDiff(diff string, width int) string {
	if diff == "" || width < 40 {
		return ""
	}
	lines := parseDiff(diff)
	if len(lines) == 0 {
		return ""
	}

	colW := (width - 3) / 2
	if colW < 15 {
		colW = 15
	}
	nw := 3 // line number width

	var b strings.Builder
	var oldNum, newNum int
	inHunk := false
	started := false

	for _, dl := range lines {
		switch dl.kind {
		case '@':
			fmt.Sscanf(dl.text, "@@ -%d,%*d +%d,%*d", &oldNum, &newNum)
			inHunk = true

			if started {
				b.WriteString(colLine.Render(strings.Repeat("─", width)))
				b.WriteByte('\n')
			}
			started = true

			// Top border + column headers
			top := colLine.Render("╭─ ") + colHdrOld.Render(i18n.T("diff.old")) +
				colLine.Render(" ──── ") + colHdrNew.Render(i18n.T("diff.new")) +
				colLine.Render(" ─╮")
			b.WriteString(top)
			b.WriteByte('\n')

		case ' ':
			left := renderSide(dl.text, colW, nw, oldNum, style.DimStyle)
			right := renderSide(dl.text, colW, nw, newNum, style.DimStyle)
			b.WriteString(left + " " + right + "\n")
			if inHunk {
				oldNum++
				newNum++
			}

		case '-':
			left := renderSide(dl.text, colW, nw, oldNum, style.RedStyle)
			right := renderEmpty(colW, nw)
			b.WriteString(left + " " + right + "\n")
			if inHunk {
				oldNum++
			}

		case '+':
			left := renderEmpty(colW, nw)
			right := renderSide(dl.text, colW, nw, newNum, style.GreenStyle)
			b.WriteString(left + " " + right + "\n")
			if inHunk {
				newNum++
			}
		}
	}

	if started {
		b.WriteString(colLine.Render(strings.Repeat("─", width)))
	}

	result := b.String()
	lc := strings.Count(result, "\n")
	const maxLines = 40
	if lc > maxLines {
		parts := strings.SplitN(result, "\n", maxLines+1)
		result = strings.Join(parts[:maxLines], "\n")
		extra := lc - maxLines
		result += fmt.Sprintf("\n%s", numStyle.Render(fmt.Sprintf("  ⋯ %d more lines", extra)))
	}
	return result
}

func parseDiff(diff string) []diffLine {
	raw := strings.Split(diff, "\n")
	var out []diffLine
	for _, line := range raw {
		if len(line) == 0 {
			continue
		}
		switch line[0] {
		case '@':
			out = append(out, diffLine{'@', line})
		case ' ', '-', '+':
			content := ""
			if len(line) > 1 {
				content = line[1:]
			}
			out = append(out, diffLine{line[0], content})
		}
	}
	return out
}

func renderSide(content string, colW, nw, lineNum int, s lipgloss.Style) string {
	colBody := colW - nw - 4 // "│ " + num + " " + content
	if colBody < 0 {
		colBody = 0
	}
	display := content
	if len(display) > colBody {
		display = display[:colBody]
	}
	num := ""
	if lineNum > 0 {
		num = numStyle.Render(fmt.Sprintf("%*d", nw, lineNum))
	} else {
		num = numStyle.Render(strings.Repeat(" ", nw))
	}
	body := s.Render(fmt.Sprintf(" %-*s", colBody, display))
	return colLine.Render("│") + num + body
}

func renderEmpty(colW, nw int) string {
	colBody := colW - nw - 4
	if colBody < 0 {
		colBody = 0
	}
	num := numStyle.Render(strings.Repeat(" ", nw))
	body := fmt.Sprintf(" %-*s", colBody, "")
	return colLine.Render("│") + num + style.DimStyle.Render(body)
}

func ViewDiffWithFile(diff, filePath string, adds, dels int, width int) string {
	var b strings.Builder
	b.WriteString(style.GrayStyle.Render(fmt.Sprintf("📄 %s  +%d -%d", filePath, adds, dels)))
	b.WriteByte('\n')
	dv := ViewSideBySideDiff(diff, width)
	if dv != "" {
		b.WriteString(dv)
	}
	return b.String()
}
