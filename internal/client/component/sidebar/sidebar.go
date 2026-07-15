package sidebar

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/DotNetAge/mindx/internal/client/component/welcome"
	"github.com/DotNetAge/mindx/internal/client/data"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/style"
	appcore "github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/i18n"
)

var (
	borderStyle = lipgloss.NewStyle().
			Foreground(style.ThemeDim).
			Inline(true)

	sideBarStyle = lipgloss.NewStyle().
			Border(lipgloss.Border{Left: "│"}).
			BorderForeground(style.ThemeDim).
			PaddingLeft(1)

	boldLabel = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("white"))

	greenAdd = lipgloss.NewStyle().Foreground(style.ThemeGreen)
	redDel   = lipgloss.NewStyle().Foreground(style.ThemeRed)
)

type Sidebar struct {
	width   int
	height  int
	vp      viewport.Model
	welcome *welcome.WelcomePanel

	PromptTokens     int
	CompletionTokens int
	CachedTokens     int
	TotalTokens      int
	ModelName        string

	FileChanges []data.FileChange

	// CostFunc model-specific cost breakdown. If nil, appcore.DefaultModelCost is used.
	// Returns (inputCost, outputCost, cachedCost) separately for detailed display.
	CostFunc func(modelName string, promptTokens, completionTokens, cachedTokens int) (inputCost, outputCost, cachedCost float64)
}

func New() *Sidebar {
	return &Sidebar{
		welcome: welcome.New(),
		vp:      viewport.New(),
	}
}

func (s *Sidebar) Update(msg any) (*Sidebar, tea.Cmd) {
	switch m := msg.(type) {
	case clientmsg.WindowResizeMsg:
		s.width = m.Width
		s.height = m.Height
		s.welcome.Update(m)
		s.vp.SetWidth(s.width)
		s.vp.SetHeight(s.height)
		s.vp.SetContent(s.buildContent())
		return s, nil
	}

	newVp, cmd := s.vp.Update(msg)
	s.vp = newVp
	if s.width > 0 {
		s.vp.SetContent(s.buildContent())
	}
	return s, cmd
}

func (s *Sidebar) View() string {
	content := s.vp.View()
	return sideBarStyle.Render(content)
}

func (s *Sidebar) SyncHeight(h int) {
	s.height = h
	s.vp.SetHeight(h)
}

func (s *Sidebar) SetWelcomeData(d data.WelcomeData) {
	s.welcome.Data = d
	if s.width > 0 {
		s.vp.SetContent(s.buildContent())
	}
}

// AddTokenUsage accumulates token counts from an LLM call into the sidebar's
// running total. Called on every ExecutionSummaryMsg so the fee detail section
// reflects the cumulative consumption across the entire session.
func (s *Sidebar) AddTokenUsage(promptTokens, completionTokens, cachedTokens, totalTokens int, modelName string) {
	s.PromptTokens += promptTokens
	s.CompletionTokens += completionTokens
	s.CachedTokens += cachedTokens
	// 计费口径：prompt + completion - cached；totalTokens 参数为原始 API total，仅作参考
	actualTokens := promptTokens + completionTokens - cachedTokens
	if actualTokens < 0 {
		actualTokens = 0
	}
	s.TotalTokens += actualTokens
	if modelName != "" {
		s.ModelName = modelName
	}
	if s.width > 0 {
		s.vp.SetContent(s.buildContent())
	}
}

// SetFileChanges replaces the current file changes list.
func (s *Sidebar) SetFileChanges(changes []data.FileChange) {
	s.FileChanges = changes
	if s.width > 0 {
		s.vp.SetContent(s.buildContent())
	}
}

func formatCost(cost float64) string {
	if cost < 0.005 {
		return "¥0"
	}
	if cost < 1 {
		return fmt.Sprintf("¥%.2f", cost)
	}
	return fmt.Sprintf("¥%.2f", cost)
}

func formatTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fm", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// contentWidth returns the usable content width inside the sidebar.
func (s *Sidebar) contentWidth() int {
	// PaddingLeft(1) + border(1) + some safety margin
	return max(s.width-4, 10)
}

func (s *Sidebar) buildContent() string {
	view := s.welcome.View()
	var parts []string

	if view != "" {
		parts = append(parts, view)
	} else {
		parts = append(parts, style.DimStyle.Render(i18n.T("client.ui.sidebar.welcome.fallback")))
	}

	sep := borderStyle.Render(strings.Repeat("─", max(s.width-4, 4)))
	parts = append(parts, sep)

	// Cost detail section
	if s.TotalTokens > 0 {
		var inputCost, outputCost, cachedCost, totalCost float64
		if s.CostFunc != nil {
			inputCost, outputCost, cachedCost = s.CostFunc(s.ModelName, s.PromptTokens, s.CompletionTokens, s.CachedTokens)
			totalCost = inputCost + outputCost + cachedCost
		} else {
			mc := appcore.DefaultModelCost()
			netInput := s.PromptTokens - s.CachedTokens
			if netInput < 0 {
				netInput = 0
			}
			inputCost = mc.CostPer1MIn / 1_000_000 * float64(netInput)
			outputCost = mc.CostPer1MOut / 1_000_000 * float64(s.CompletionTokens)
			cachedCost = 0
			totalCost = inputCost + outputCost
		}

		var costParts []string
		costParts = append(costParts, boldLabel.Render(i18n.T("client.ui.sidebar.cost.detail")))
		costParts = append(costParts, fmt.Sprintf(i18n.T("client.ui.sidebar.cost.input"), formatTokens(s.PromptTokens), formatCost(inputCost)))
		costParts = append(costParts, fmt.Sprintf(i18n.T("client.ui.sidebar.cost.output"), formatTokens(s.CompletionTokens), formatCost(outputCost)))
		if s.CachedTokens > 0 {
			costParts = append(costParts, fmt.Sprintf(i18n.T("client.ui.sidebar.cost.cached"), formatTokens(s.CachedTokens), formatCost(cachedCost)))
		}
		costParts = append(costParts, fmt.Sprintf(i18n.T("client.ui.sidebar.cost.total"), formatCost(totalCost)))

		padding := lipgloss.NewStyle().Padding(0, 1).Width(s.width)
		parts = append(parts, padding.Render(strings.Join(costParts, "\n")))
	}

	// File changes section
	if len(s.FileChanges) > 0 {
		cw := s.contentWidth()
		var fcParts []string

		// Blank line before new section
		fcParts = append(fcParts, "")
		fcParts = append(fcParts, boldLabel.Render(i18n.T("client.ui.sidebar.changes.title")))

		for _, c := range s.FileChanges {
			path := style.GrayStyle.Render(c.TruncatedPath())
			addStr := greenAdd.Render(fmt.Sprintf("+%d", c.Additions))
			delStr := redDel.Render(fmt.Sprintf("-%d", c.Deletions))
			counts := addStr + " " + delStr

			pathW := lipgloss.Width(path)
			countsW := lipgloss.Width(counts)
			pad := cw - pathW - countsW
			if pad < 1 {
				pad = 1
			}
			line := "  " + path + strings.Repeat(" ", pad) + counts
			fcParts = append(fcParts, line)
		}

		padding := lipgloss.NewStyle().Padding(0, 1).Width(s.width)
		parts = append(parts, padding.Render(strings.Join(fcParts, "\n")))
	}

	return strings.Join(parts, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
