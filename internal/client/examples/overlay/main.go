package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/DotNetAge/mindx/internal/client/component/conv"
	"github.com/DotNetAge/mindx/internal/client/component/dialog"
	"github.com/DotNetAge/mindx/internal/client/component/input"
	"github.com/DotNetAge/mindx/internal/client/component/welcome"
	"github.com/DotNetAge/mindx/internal/client/data"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/style"
)

const (
	bottomInput = iota
)

var (
	sideBarStyle = lipgloss.NewStyle().
			Border(lipgloss.Border{Left: "│"}).
			BorderForeground(style.ThemeDim).
			PaddingLeft(1)

	lastResult string
)

type model struct {
	width  int
	height int

	leftWidth    int
	rightWidth   int
	mainVpHeight int
	sideVpHeight int

	convList conv.ConversationList
	mainVp   viewport.Model
	welcome  *welcome.WelcomePanel
	sideVp   viewport.Model
	fileList []fileChangeItem

	inputArea     *input.InputArea
	selectDlg     *dialog.SelectDialog
	optionsDlg    *dialog.OptionsDialog
	activeOverlay int

	bottomMode int
	focusSide  bool
}

type fileChangeItem struct {
	Path   string
	Action string
	Time   time.Time
}

func (m model) Init() tea.Cmd {
	return m.convList.Init()
}

func (m model) Update(e tea.Msg) (tea.Model, tea.Cmd) {
	switch e := e.(type) {
	case tea.WindowSizeMsg:
		m.width = e.Width
		m.height = e.Height
		m.leftWidth = m.width*3/4 - 1
		if m.leftWidth < 40 {
			m.leftWidth = m.width - 30
		}
		m.rightWidth = m.width - m.leftWidth - 1
		if m.rightWidth < 20 {
			m.rightWidth = 20
		}

		bottomEstimate := 6
		m.mainVpHeight = m.height - bottomEstimate
		m.sideVpHeight = m.height - bottomEstimate
		if m.mainVpHeight < 3 {
			m.mainVpHeight = 3
		}
		if m.sideVpHeight < 3 {
			m.sideVpHeight = 3
		}

		m.inputArea.Update(clientmsg.WindowResizeMsg{Width: m.width, Height: e.Height})
		m.welcome.Update(clientmsg.WindowResizeMsg{Width: m.rightWidth})
		m.selectDlg.Update(e)
		m.optionsDlg.Update(e)

		newConv, _ := m.convList.Update(clientmsg.WindowResizeMsg{Width: m.leftWidth, Height: m.mainVpHeight})
		m.convList = newConv

		m.mainVp.SetWidth(m.leftWidth)
		m.mainVp.SetHeight(m.mainVpHeight)
		m.sideVp.SetWidth(m.rightWidth)
		m.sideVp.SetHeight(m.sideVpHeight)

		m.mainVp.SetContent(m.buildMainContent())
		m.sideVp.SetContent(m.buildSideContent())
		m.mainVp.GotoBottom()
		return m, nil

	case tea.KeyPressMsg:
		if m.activeOverlay != 0 {
			return m.handleOverlayKey(e)
		}
		return m.handleNormalKey(e)

	case tea.MouseWheelMsg:
		var vpCmd tea.Cmd
		if e.X >= m.leftWidth+1 {
			m.sideVp, vpCmd = m.sideVp.Update(e)
		} else {
			m.mainVp, vpCmd = m.mainVp.Update(e)
		}
		return m, vpCmd

	case dialog.SelectDialogResult:
		m.activeOverlay = 0
		if !e.Cancelled {
			lastResult = fmt.Sprintf("单选结果: index=%d custom=%q", e.Index, e.CustomText)
		} else {
			lastResult = "单选: 已取消"
		}
		return m, nil

	case dialog.OptionsDialogResult:
		m.activeOverlay = 0
		if !e.Cancelled {
			lastResult = fmt.Sprintf("多选结果: indices=%v custom=%q", e.Indices, e.CustomText)
		} else {
			lastResult = "多选: 已取消"
		}
		return m, nil
	}

	newConv, convCmd := m.convList.Update(e)
	m.convList = newConv
	if m.width > 0 {
		m.mainVp.SetContent(m.buildMainContent())
	}
	return m, convCmd
}

const (
	overlayNone = iota
	overlaySelect
	overlayOptions
)

func (m model) handleOverlayKey(e tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.activeOverlay {
	case overlaySelect:
		newDlg, cmd := m.selectDlg.Update(e)
		m.selectDlg = newDlg
		return m, cmd
	case overlayOptions:
		newDlg, cmd := m.optionsDlg.Update(e)
		m.optionsDlg = newDlg
		return m, cmd
	}
	return m, nil
}

func (m model) handleNormalKey(e tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch e.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "tab":
		m.focusSide = !m.focusSide
		return m, nil
	case "s":
		m.activeOverlay = overlaySelect
		m.selectDlg.SetOptions(
			"请选择一个操作：",
			[]string{"代码审查", "运行测试", "构建项目", "性能分析", "部署发布", "生成文档"},
		)
		return m, nil
	case "m":
		m.activeOverlay = overlayOptions
		m.optionsDlg.SetOptions(
			"请选择要执行的操作（可多选）：",
			[]string{"代码审查", "运行测试", "构建项目", "性能分析", "部署发布", "生成文档"},
		)
		return m, nil
	default:
		var cmd tea.Cmd
		var vpCmd tea.Cmd
		switch m.bottomMode {
		case bottomInput:
			newInput, c := m.inputArea.Update(e)
			m.inputArea = newInput
			cmd = c
		}
		if m.focusSide {
			m.sideVp, vpCmd = m.sideVp.Update(e)
		} else {
			m.mainVp, vpCmd = m.mainVp.Update(e)
		}
		if vpCmd != nil {
			cmd = tea.Batch(cmd, vpCmd)
		}
		if cmd != nil {
			return m, cmd
		}
		return m, nil
	}
}

func (m model) View() tea.View {
	if m.width == 0 || m.rightWidth < 4 {
		return tea.NewView("初始化中...")
	}

	mainArea := m.mainVp.View()
	sideArea := sideBarStyle.Render(m.sideVp.View())
	bottomArea := m.renderBottomArea()
	hint := m.renderHint()

	layout := lipgloss.JoinHorizontal(
		lipgloss.Top,
		mainArea,
		sideArea,
	)

	full := lipgloss.JoinVertical(
		lipgloss.Left,
		layout,
		bottomArea,
		hint,
	)

	switch m.activeOverlay {
	case overlaySelect:
		modal := m.selectDlg.View()
		full = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
	case overlayOptions:
		modal := m.optionsDlg.View()
		full = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
	}

	v := tea.NewView(full)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m model) buildMainContent() string {
	view := m.convList.View()
	extra := ""
	if lastResult != "" {
		extra = "\n\n  " + style.GreenStyle.Render("✓ "+lastResult)
	}
	if view == "" {
		return style.DimStyle.Render("  (主视口 — 对话内容将在此显示)\n\n  按 s 打开 SelectDialog（单选）\n  按 m 打开 OptionsDialog（多选）") + extra
	}
	return lipgloss.NewStyle().Width(m.leftWidth).Render(view + extra)
}

func (m model) buildSideContent() string {
	var parts []string

	welcomeView := m.welcome.View()
	if welcomeView != "" {
		parts = append(parts, welcomeView)
	} else {
		parts = append(parts, style.DimStyle.Render("  Welcome Panel"))
	}

	sep := style.DimStyle.Render(strings.Repeat("─", max(m.rightWidth-4, 4)))
	parts = append(parts, sep)
	parts = append(parts, renderFileList(m.fileList, m.rightWidth))

	return strings.Join(parts, "\n")
}

func (m model) renderBottomArea() string {
	switch m.bottomMode {
	case bottomInput:
		return m.inputArea.View()
	default:
		return ""
	}
}

func (m model) renderHint() string {
	modeLabel := map[int]string{
		bottomInput: "[Input]",
	}[m.bottomMode]

	focusLabel := "主视口"
	if m.focusSide {
		focusLabel = "侧边栏"
	}

	overlayHint := ""
	switch m.activeOverlay {
	case overlaySelect:
		overlayHint = style.CyanStyle.Render(" │ [SelectDialog]")
	case overlayOptions:
		overlayHint = style.PurpleStyle.Render(" │ [OptionsDialog]")
	}

	hint := fmt.Sprintf(" %s | 焦点:%s%s | s 单选 | m 多选 | Tab切换焦点 | ↑↓滚动 | q 退出",
		style.CyanStyle.Render(modeLabel),
		style.YellowStyle.Render(focusLabel),
		overlayHint)
	return style.DimStyle.Render(hint)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func renderFileList(items []fileChangeItem, width int) string {
	if len(items) == 0 {
		return style.DimStyle.Render("  📂 文件变更列表（空）\n\n  接入 HistoryService 后显示")
	}

	header := style.BoldWhite.Render("📂 变更文件") + style.GrayStyle.Render(fmt.Sprintf(" (%d)", len(items)))
	dotWidth := width - 10
	if dotWidth < 1 {
		dotWidth = 1
	}
	sep := style.DimStyle.Render(strings.Repeat("·", dotWidth))

	var b strings.Builder
	b.WriteString(header)
	b.WriteByte('\n')
	b.WriteString(sep)
	b.WriteByte('\n')

	for _, item := range items {
		var actionIcon string
		switch item.Action {
		case "modified":
			actionIcon = style.YellowStyle.Render("~")
		case "added":
			actionIcon = style.GreenStyle.Render("+")
		case "deleted":
			actionIcon = style.RedStyle.Render("-")
		default:
			actionIcon = style.DimStyle.Render("·")
		}
		timeStr := item.Time.Format("15:04:05")
		line := fmt.Sprintf("  %s %s  %s", actionIcon, truncate(item.Path, width-16), style.DimStyle.Render(timeStr))
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}

func singleRoundExample() conv.Conversation {
	c := conv.NewConversation("s1", "assistant", "Go 的版本是多少？")

	c.Rounds = append(c.Rounds, conv.ThoughtActionRound{
		Thought: conv.Thought{
			Content:   "用户询问 Go 版本，需要读取 go.mod 文件获取信息。",
			TokensIn:  50,
			TokensOut: 20,
			Timestamp: time.Now().Add(-10 * time.Second),
		},
		Action: conv.Action{
			CurrentInfo: &conv.ActionInfo{
				ToolCount:            1,
				ToolNames:            []string{"read_file"},
				TotalPredictedTokens: 100,
			},
			Steps: []conv.ActionStep{
				{
					ToolName:     "read_file",
					Status:       conv.ActionStepDone,
					EstimatedTok: 100,
					Duration:     500 * time.Millisecond,
					ResultText:   "module github.com/DotNetAge/mindx\ngo 1.22",
					Collapsed:    true,
				},
			},
			Completed:     true,
			SuccessCount:  1,
			FailedCount:   0,
			TotalTokens:   100,
			TotalDuration: 500 * time.Millisecond,
		},
	})

	c.Output = conv.Output{
		Entries: []conv.OutputEntry{
			{Role: "assistant", Content: "根据 `go.mod` 文件显示，当前项目使用的是 **Go 1.22** 版本。"},
		},
	}
	return c
}

func main() {
	ia := input.New()
	ia.Width = 100
	ia.Commands = []input.SlashCommand{
		{Name: "help", Description: "显示帮助信息"},
		{Name: "clear", Description: "清除屏幕"},
		{Name: "chat", Description: "切换对话"},
		{Name: "model", Description: "切换模型"},
	}
	ia.Agents = []data.AgentInfo{{Name: "architect", Description: "架构师"}}
	ia.Models = []input.ModelItem{{Name: "claude-sonnet-4", Description: "Claude Sonet 4"}}

	w := welcome.New()
	w.Data = data.WelcomeData{
		AppTitle:  "MindX CLI v2.0.0 Beta",
		AgentName: "architect",
		Workspace: "/home/user/project",
		SessionID: "sess-abc123",
		ModelName: "claude-sonnet-4-20250514",
	}

	list := conv.NewConversationList()
	list.Conversations = append(list.Conversations, singleRoundExample())

	files := []fileChangeItem{
		{Path: "internal/client/client.go", Action: "modified", Time: now().Add(-2 * time.Minute)},
		{Path: "internal/core/app.go", Action: "added", Time: now().Add(-5 * time.Minute)},
		{Path: "pkg/logging/zap.go", Action: "deleted", Time: now().Add(-10 * time.Minute)},
		{Path: "go.mod", Action: "modified", Time: now()},
		{Path: "main.go", Action: "added", Time: now().Add(-1 * time.Minute)},
	}

	p := tea.NewProgram(model{
		inputArea:  ia,
		selectDlg:  dialog.NewSelectDialog("操作选择"),
		optionsDlg: dialog.NewOptionsDialog("批量操作"),
		convList:   list,
		mainVp:     viewport.New(),
		welcome:    w,
		sideVp:     viewport.New(),
		fileList:   files,
		bottomMode: bottomInput,
	})

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func now() time.Time { return time.Now() }
