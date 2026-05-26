//go:build example

package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/viewport"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/DotNetAge/mindx/internal/client/component/choices"
	"github.com/DotNetAge/mindx/internal/client/component/conv"
	"github.com/DotNetAge/mindx/internal/client/component/input"
	"github.com/DotNetAge/mindx/internal/client/component/permission"
	"github.com/DotNetAge/mindx/internal/client/component/statusbar"
	"github.com/DotNetAge/mindx/internal/client/component/welcome"
	"github.com/DotNetAge/mindx/internal/client/data"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
	"github.com/DotNetAge/mindx/internal/client/style"
)

const (
	bottomInput = iota
	bottomChoices
	bottomPermission
)

var (
	borderStyle = lipgloss.NewStyle().
			Foreground(style.ThemeDim).
			Inline(true)

	sideBarStyle = lipgloss.NewStyle().
			Border(lipgloss.Border{Left: "│"}).
			BorderForeground(style.ThemeDim).
			PaddingLeft(1)
)

type model struct {
	width  int
	height int

	leftWidth  int
	rightWidth int
	mainVpHeight int
	sideVpHeight int

	convList conv.ConversationList
	mainVp   viewport.Model
	welcome  *welcome.WelcomePanel
	sideVp   viewport.Model
	fileList []fileChangeItem

	inputArea    *input.InputArea
	choicesPanel *choices.ChoicesPanel
	permBar      permission.PermissionBar
	statusBar    *statusbar.StatusBar

	bottomMode int
	scene      int
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
		m.statusBar.Update(clientmsg.WindowResizeMsg{Width: m.width})

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
		switch e.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.focusSide = !m.focusSide
			return m, nil
		case "1":
			m.scene = 1
			m.convList.Conversations = nil
			m.convList.Conversations = append(m.convList.Conversations, singleRoundExample())
			m.convList.MarkDirty()
			m.bottomMode = bottomInput
			m.mainVp.SetContent(m.buildMainContent())
			m.mainVp.GotoBottom()
			return m, nil
		case "2":
			m.scene = 2
			m.convList.Conversations = nil
			m.convList.Conversations = append(m.convList.Conversations, multiRoundExample())
			m.convList.MarkDirty()
			m.bottomMode = bottomInput
			m.mainVp.SetContent(m.buildMainContent())
			m.mainVp.GotoBottom()
			return m, nil
		case "3":
			m.scene = 3
			m.convList.Conversations = nil
			m.convList.Conversations = append(m.convList.Conversations, streamingExample())
			m.convList.MarkDirty()
			m.bottomMode = bottomInput
			m.mainVp.SetContent(m.buildMainContent())
			m.mainVp.GotoBottom()
			return m, nil
		case "4":
			m.scene = 4
			m.convList.Conversations = nil
			m.convList.Conversations = append(m.convList.Conversations, errorExample())
			m.convList.MarkDirty()
			m.bottomMode = bottomInput
			m.mainVp.SetContent(m.buildMainContent())
			m.mainVp.GotoBottom()
			return m, nil
		case "i":
			m.bottomMode = bottomInput
			return m, nil
		case "c":
			m.bottomMode = bottomChoices
			newPanel, _ := m.choicesPanel.Update(clientmsg.ShowChoicesMsg{
				Prompt:         "请选择要执行的操作（可多选）：",
				Options:        []string{"代码审查", "运行测试", "构建项目", "性能分析"},
				MultiSelect:    true,
				AllowTextInput: true,
			})
			m.choicesPanel = newPanel
			return m, nil
		case "p":
			m.bottomMode = bottomPermission
			newBar, _ := permission.UpdatePermissionBar(m.permBar, clientmsg.PermissionRequestMsg{
				ToolName:      "WriteFile",
				Reason:        "尝试写入配置文件 config.yaml",
				SecurityLevel: 2,
			})
			m.permBar = newBar
			return m, nil
		default:
			var cmd tea.Cmd
			switch m.bottomMode {
			case bottomInput:
				newInput, c := m.inputArea.Update(e)
				m.inputArea = newInput
				cmd = c
			case bottomChoices:
				newPanel, c := m.choicesPanel.Update(e)
				m.choicesPanel = newPanel
				cmd = c
			case bottomPermission:
				newBar, c := permission.UpdatePermissionBar(m.permBar, e)
				m.permBar = newBar
				cmd = c
			}
			var vpCmd tea.Cmd
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

	case tea.MouseWheelMsg:
		var vpCmd tea.Cmd
		if e.X >= m.leftWidth+1 {
			m.sideVp, vpCmd = m.sideVp.Update(e)
		} else {
			m.mainVp, vpCmd = m.mainVp.Update(e)
		}
		return m, vpCmd

	case clientmsg.UserSendMsg:
		return m, nil

	case clientmsg.ChoiceSelectedMsg:
		m.bottomMode = bottomInput
		return m, nil
	}

	newConv, convCmd := m.convList.Update(e)
	m.convList = newConv
	m.statusBar.Tick()
	if m.width > 0 {
		m.mainVp.SetContent(m.buildMainContent())
	}
	return m, convCmd
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
		m.statusBar.View(),
		bottomArea,
		hint,
	)

	v := tea.NewView(full)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m model) buildMainContent() string {
	view := m.convList.View()
	if view == "" {
		return style.DimStyle.Render("  (主视口 — 对话内容将在此显示)\n\n  按 1-4 切换场景")
	}
	return lipgloss.NewStyle().Width(m.leftWidth).Render(view)
}

func (m model) buildSideContent() string {
	var parts []string

	welcomeView := m.welcome.View()
	if welcomeView != "" {
		parts = append(parts, welcomeView)
	} else {
		parts = append(parts, style.DimStyle.Render("  Welcome Panel"))
	}

	sep := borderStyle.Render(strings.Repeat("─", max(m.rightWidth-4, 4)))
	parts = append(parts, sep)
	parts = append(parts, renderFileList(m.fileList, m.rightWidth))

	return strings.Join(parts, "\n")
}

func (m model) renderBottomArea() string {
	switch m.bottomMode {
	case bottomInput:
		return m.inputArea.View()
	case bottomChoices:
		return m.choicesPanel.View()
	case bottomPermission:
		return permission.ViewPermissionBar(m.permBar, m.width)
	default:
		return ""
	}
}

func (m model) renderHint() string {
	modeLabel := map[int]string{
		bottomInput:      "[Input]",
		bottomChoices:    "[Choices]",
		bottomPermission: "[Permission]",
	}[m.bottomMode]

	focusLabel := "主视口"
	if m.focusSide {
		focusLabel = "侧边栏"
	}

	hint := fmt.Sprintf(" %s | 焦点:%s | Tab切换焦点 | ↑↓/滚轮滚动 | i 输入 | c 选择 | p 权限 | 1-4 场景 | q 退出",
		style.CyanStyle.Render(modeLabel),
		style.YellowStyle.Render(focusLabel))
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

// ============================================================
// 场景数据
// ============================================================

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

func multiRoundExample() conv.Conversation {
	now := time.Now()
	c := conv.NewConversation("s2", "architect", "帮我分析这个项目的整体架构和依赖关系")

	c.Rounds = append(c.Rounds, conv.ThoughtActionRound{
		Thought: conv.Thought{
			Content:   "首先需要了解项目的整体结构。这是一个 CLI 工具项目，我应该先查看目录结构和 go.mod 来了解技术栈。",
			TokensIn:  120,
			TokensOut: 65,
			Timestamp: now.Add(-5 * time.Minute),
		},
		Action: conv.Action{
			CurrentInfo: &conv.ActionInfo{
				ToolCount:            2,
				ToolNames:            []string{"list_directory", "read_file"},
				TotalPredictedTokens: 800,
			},
			Steps: []conv.ActionStep{
				{
					ToolName: "list_directory", Status: conv.ActionStepDone,
					EstimatedTok: 200, Duration: 300 * time.Millisecond,
					ResultText: "cmd/\ninternal/\n  client/     (终端 UI 层)\n  core/       (业务逻辑层)\npkg/         (工具库)",
					Collapsed:  true,
				},
				{
					ToolName: "read_file", Status: conv.ActionStepDone,
					EstimatedTok: 600, Duration: 800 * time.Millisecond,
					ResultText: "module github.com/DotNetAge/mindx\ngo 1.22",
					Collapsed:  false,
				},
			},
			Completed: true, SuccessCount: 2, FailedCount: 0,
			TotalTokens: 800, TotalDuration: 1100 * time.Millisecond,
		},
	})

	c.Rounds = append(c.Rounds, conv.ThoughtActionRound{
		Thought: conv.Thought{
			Content:   "从第1轮的结果来看，这是一个基于 Bubble Tea v2 的终端应用。接下来需要深入分析核心模块。",
			TokensIn:  280,
			TokensOut: 150,
			Timestamp: now.Add(-3 * time.Minute),
		},
		Action: conv.Action{
			CurrentInfo: &conv.ActionInfo{
				ToolCount: 1, ToolNames: []string{"grep_search"}, TotalPredictedTokens: 500,
			},
			Steps: []conv.ActionStep{
				{
					ToolName: "grep_search", Status: conv.ActionStepDone,
					EstimatedTok: 400, Duration: 600 * time.Millisecond,
					ResultText: "client.go:25:type rootModel struct {\nconversation.go:10:type Conversation struct {",
					Collapsed:  true,
				},
			},
			Completed: true, SuccessCount: 1, FailedCount: 0,
			TotalTokens: 500, TotalDuration: 600 * time.Millisecond,
		},
	})

	c.Output = conv.Output{
		Entries: []conv.OutputEntry{
			{Role: "assistant", Content: "## 项目架构分析\n\n### 技术栈\n| 层级 | 技术 |\n|------|------|\n| UI | bubbletea v2 |\n| AI | goreact v0.5 |\n\n这是一个设计精良的 AI 终端应用。"},
		},
	}
	return c
}

func streamingExample() conv.Conversation {
	c := conv.NewConversation("s3", "coder", "帮我实现一个用户认证中间件")

	c.Rounds = append(c.Rounds, conv.ThoughtActionRound{
		Thought: conv.Thought{IsActive: true},
	})
	return c
}

func errorExample() conv.Conversation {
	now := time.Now()
	c := conv.NewConversation("s4", "researcher", "搜索最新的 Go release notes")

	c.Status = conv.StatusError

	c.Rounds = append(c.Rounds, conv.ThoughtActionRound{
		Thought: conv.Thought{
			Content:  "用户想了解 Go 的 release notes，我需要搜索并抓取官方页面。",
			TokensIn: 80, TokensOut: 40, Timestamp: now.Add(-8 * time.Second),
		},
		Action: conv.Action{
			CurrentInfo: &conv.ActionInfo{
				ToolCount: 2, ToolNames: []string{"WebSearch", "WebFetch"}, TotalPredictedTokens: 2000,
			},
			Steps: []conv.ActionStep{
				{ToolName: "WebSearch", Status: conv.ActionStepDone, EstimatedTok: 500, Duration: 1200 * time.Millisecond,
					ResultText: "Found 5 results for Go release notes...", Collapsed: true},
				{ToolName: "WebFetch", Status: conv.ActionStepFailed, EstimatedTok: 1500, Duration: 5 * time.Second,
					ResultText: "fetch failed: connection timed out", Collapsed: false},
			},
			Completed: true, SuccessCount: 1, FailedCount: 1, TotalTokens: 2000, TotalDuration: 6200 * time.Millisecond,
		},
	})

	c.Error = conv.ErrorMsg{Error: "act error: context canceled", Phase: "执行阶段", Time: now}
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
	ia.Models = []input.ModelItem{{Name: "claude-sonnet-4", Description: "Claude Sonnet 4"}}

	w := welcome.New()
	w.Data = data.WelcomeData{
		AppTitle:  "MindX CLI v2.0.0 Beta",
		AgentName: "architect",
		Workspace: "/home/user/project",
		SessionID: "sess-abc123",
		ModelName: "claude-sonnet-4-20250514",
	}

	list := conv.NewConversationList()
	list.Conversations = append(list.Conversations, multiRoundExample())

	files := []fileChangeItem{
		{Path: "internal/client/client.go", Action: "modified", Time: now().Add(-2 * time.Minute)},
		{Path: "internal/core/app.go", Action: "added", Time: now().Add(-5 * time.Minute)},
		{Path: "pkg/logging/zap.go", Action: "deleted", Time: now().Add(-10 * time.Minute)},
		{Path: "go.mod", Action: "modified", Time: now()},
		{Path: "main.go", Action: "added", Time: now().Add(-1 * time.Minute)},
	}

	sb := statusbar.New()
	sb.CurrentState = "空闲"
	sb.AgentName = "architect"
	sb.ModelName = "claude-sonnet-4"
	sb.SessionName = "sess-abc123"
	sb.TokensTotal = 12800
	sb.InputTokens = 8200
	sb.OutputTokens = 4600
	sb.SessionStart = now().Add(-3 * time.Minute)
	sb.Shortcuts = []data.Shortcut{
		{Key: "Tab", Description: "切换焦点"},
		{Key: "1-4", Description: "场景"},
		{Key: "q", Description: "退出"},
	}
	sb.ShowHints = true

	p := tea.NewProgram(model{
		inputArea:    ia,
		choicesPanel: choices.New(),
		permBar:      permission.NewPermissionBar("", "", 0),
		statusBar:    sb,
		convList:     list,
		mainVp:       viewport.New(),
		welcome:      w,
		sideVp:       viewport.New(),
		fileList:     files,
		scene:        2,
		bottomMode:   bottomInput,
	})

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func now() time.Time { return time.Now() }
