//go:build example

package main

import (
	"fmt"
	"os"
	"time"

	"charm.land/bubbles/v2/timer"
	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/mindx/internal/client/component/conv"
	"github.com/DotNetAge/mindx/internal/client/msg"
)

type model struct {
	convList conv.ConversationList
	scene    int
}

func (m model) Init() tea.Cmd {
	return m.convList.Init()
}

func (m model) Update(e tea.Msg) (tea.Model, tea.Cmd) {
	switch e := e.(type) {
	case tea.WindowSizeMsg:
		newList, _ := m.convList.Update(msg.WindowResizeMsg{Width: e.Width, Height: e.Height})
		m.convList = newList
		return m, nil

	case tea.KeyPressMsg:
		switch e.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "1":
			m.scene = 1
			m.convList.Conversations = nil
			m.convList.Conversations = append(m.convList.Conversations, singleRoundExample())
			m.convList.MarkDirty()
			return m, nil
		case "2":
			m.scene = 2
			m.convList.Conversations = nil
			m.convList.Conversations = append(m.convList.Conversations, multiRoundExample())
			m.convList.MarkDirty()
			return m, nil
		case "3":
			m.scene = 3
			m.convList.Conversations = nil
			m.convList.Conversations = append(m.convList.Conversations, streamingExample())
			m.convList.MarkDirty()
			return m, nil
		case "4":
			m.scene = 4
			m.convList.Conversations = nil
			m.convList.Conversations = append(m.convList.Conversations, errorExample())
			m.convList.MarkDirty()
			return m, nil
		}
		return m, nil

	case timer.TickMsg:
		newList, cmd := m.convList.Update(e)
		m.convList = newList
		return m, cmd
	}

	return m, nil
}

func (m model) View() tea.View {
	hint := "\n按 1 单轮T-A-O | 按 2 多轮T-A-O(推荐) | 按 3 实时流式 | 按 4 错误面板 | 按 q 退出\n"
	switch m.scene {
	case 1:
		hint = "\n📌 场景1: 单轮 T-A-O 循环（简单任务，一次思考+行动解决）\n" + hint
	case 2:
		hint = "\n📌 场景2: 多轮 T-A-O 循环（复杂任务，3轮思考→行动迭代）\n" + hint
	case 3:
		hint = "\n📌 场景3: 实时流式输出（模拟 Agent 正在工作）\n" + hint
	case 4:
		hint = "\n📌 场景4: Reactor 执行中断（Error 组件展示）\n" + hint
	default:
		hint = "\n👆 请选择一个场景查看 T-A-O 循环展示效果\n" + hint
	}
	return tea.NewView(
		m.convList.View() + hint,
	)
}

// ============================================================
// 场景1: 单轮 T-A-O 循环（简单任务）
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
					ResultText: `module github.com/DotNetAge/mindx

go 1.22`,
					Collapsed: true,
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
			{
				Role:    "assistant",
				Content: "根据 `go.mod` 文件显示，当前项目使用的是 **Go 1.22** 版本。",
			},
		},
	}

	return c
}

// ============================================================
// 场景2: 多轮 T-A-O 循环（复杂任务 - 核心展示）
// ============================================================
func multiRoundExample() conv.Conversation {
	now := time.Now()
	c := conv.NewConversation("s2", "architect", "帮我分析这个项目的整体架构和依赖关系")

	// ===== 第1轮 T-A: 分析项目结构 =====
	c.Rounds = append(c.Rounds, conv.ThoughtActionRound{
		Thought: conv.Thought{
			Content: "首先需要了解项目的整体结构。这是一个 CLI 工具项目，我应该先查看目录结构和 go.mod 来了解技术栈。\n" +
				"计划分步骤进行：\n" +
				"1. 查看根目录结构\n" +
				"2. 读取 go.mod 了解依赖\n" +
				"3. 分析核心模块的职责",
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
					ToolName:     "list_directory",
					Status:       conv.ActionStepDone,
					EstimatedTok: 200,
					Duration:     300 * time.Millisecond,
					ResultText: `cmd/
internal/
  client/     (终端 UI 层)
  core/       (业务逻辑层)
pkg/         (工具库)
go.mod
go.sum
README.md`,
					Collapsed: true,
				},
				{
					ToolName:     "read_file",
					Status:       conv.ActionStepDone,
					EstimatedTok: 600,
					Duration:     800 * time.Millisecond,
					ResultText: `module github.com/DotNetAge/mindx

go 1.22

require (
    charm.land/bubbletea/v2 v2.0.6
    charm.land/bubbles/v2  v2.1.0
    charm.land/lipgloss/v2 v2.0.3
    github.com/DotNetAge/goreact v0.5.0
    github.com/charmbracelet/x/ansi latest
)`,
					Collapsed: false,
				},
			},
			Completed:     true,
			SuccessCount:  2,
			FailedCount:   0,
			TotalTokens:   800,
			TotalDuration: 1100 * time.Millisecond,
		},
	})

	// ===== 第2轮 T-A: 深入分析依赖关系 =====
	c.Rounds = append(c.Rounds, conv.ThoughtActionRound{
		Thought: conv.Thought{
			Content: "从第1轮的结果来看，这是一个基于 Bubble Tea v2 的终端应用。关键发现：\n\n" +
				"**核心技术栈**：\n" +
				"- UI 框架：bubbletea v2 + lipgloss v2（终端渲染）\n" +
				"- 组件库：bubbles v2（viewport、spinner 等）\n" +
				"- AI 编排：goreact v0.5（Agent 框架）\n\n" +
				"接下来需要深入查看 internal/client 和 internal/core 的具体实现，了解架构分层。",
			TokensIn:  280,
			TokensOut: 150,
			Timestamp: now.Add(-3 * time.Minute),
		},
		Action: conv.Action{
			CurrentInfo: &conv.ActionInfo{
				ToolCount:            2,
				ToolNames:            []string{"grep_search", "read_file"},
				TotalPredictedTokens: 1200,
			},
			Steps: []conv.ActionStep{
				{
					ToolName:     "grep_search",
					Status:       conv.ActionStepDone,
					Params:       map[string]any{"pattern": "type.*struct", "path": "internal/client"},
					EstimatedTok: 400,
					Duration:     600 * time.Millisecond,
					ResultText: `internal/client/client.go:25:type rootModel struct {
internal/client/component/conv/conversation.go:10:type Conversation struct {
internal/client/component/conv/action.go:30:type Action struct {
internal/client/component/conv/thinking.go:13:type Thought struct {`,
					Collapsed: true,
				},
				{
					ToolName:     "read_file",
					Status:       conv.ActionStepDone,
					Params:       map[string]any{"file": "internal/core/app.go"},
					EstimatedTok: 800,
					Duration:     1 * time.Second,
					ResultText: `package core

type App struct {
    config    *MindxConfig
    agents    AgentRegistry
    models    ModelRegistry
    sessions  SessionStore
}

func (a *App) CurrentAgent() (*goreact.Agent, error) {...}
func (a *App) ResolveAgent(name string) error {...}`,
					Collapsed: false,
				},
			},
			Completed:     true,
			SuccessCount:  2,
			FailedCount:   0,
			TotalTokens:   1200,
			TotalDuration: 1600 * time.Millisecond,
		},
	})

	// ===== 第3轮 T-A: 总结并生成报告 =====
	c.Rounds = append(c.Rounds, conv.ThoughtActionRound{
		Thought: conv.Thought{
			Content: "经过前两轮的分析，现在对项目有了全面的了解。让我总结一下架构特点：\n\n" +
				"**架构模式**：Clean Architecture 分层\n" +
				"- client 层：纯 UI 逻辑（bubbletea）\n" +
				"- core 层：业务逻辑（Agent 管理、会话管理）\n" +
				"- pkg 层：可复用工具\n\n" +
				"**设计亮点**：\n" +
				"1. 使用 T-A-O 循环（Thought-Action-Output）展示 AI 推理过程\n" +
				"2. Viewport 组件支持大量内容的滚动浏览\n" +
				"3. 组件化设计，Thought/Action/Output 各司其职\n\n" +
				"现在可以生成完整的架构分析报告了。",
			TokensIn:  350,
			TokensOut: 180,
			Timestamp: now.Add(-1 * time.Minute),
		},
		Action: conv.Action{
			CurrentInfo: &conv.ActionInfo{
				ToolCount:            1,
				ToolNames:            []string{"final_answer"},
				TotalPredictedTokens: 500,
			},
			Steps: []conv.ActionStep{
				{
					ToolName:     "final_answer",
					Status:       conv.ActionStepDone,
					EstimatedTok: 500,
					Duration:     200 * time.Millisecond,
					ResultText:   "[Generating structured response...]",
					Collapsed:    true,
				},
			},
			Completed:     true,
			SuccessCount:  1,
			FailedCount:   0,
			TotalTokens:   500,
			TotalDuration: 200 * time.Millisecond,
		},
	})

	// ===== 最终 Output =====
	c.Output = conv.Output{
		Entries: []conv.OutputEntry{
			{
				Role: "assistant",
				Content: `## 🏗️ 项目架构分析报告

### 技术栈概览
| 层级 | 技术 | 用途 |
|------|------|------|
| **UI 层** | bubbletea v2 + lipgloss v2 | 终端界面渲染 |
| **组件** | bubbles v2 (viewport/spinner) | 可复用 UI 组件 |
| **AI 引擎** | goreact v0.5 | Agent 编排与推理 |

### 架构分层
` +
					"```" + `
mindx/
├── cmd/              # 入口点
├── internal/
│   ├── client/       # 终端 UI 层
│   │   └── component/
│   │       └── conv/ # 对话组件 (T-A-O 循环)
│   ├── core/         # 业务逻辑层
│   │   └── app.go    # 应用核心
│   └── data/         # 数据模型
└── pkg/              # 工具库
` + "```" + `

### 核心设计模式

#### ✨ T-A-O 循环（本项目特色）
每一轮对话都遵循 **Thought → Action → Output** 循环：
- **Thought**: AI 的思考过程（流式输出）
- **Action**: 工具调用（文件读取、搜索等）
- **Output**: 最终回答

这种设计让用户能**完整看到 AI 的推理链路**，而不是只看黑盒结果。

#### 🎯 关键组件
1. **Conversation**: 会话容器，包含多轮 Rounds
2. **ThoughtActionRound**: 单轮 T-A 配对
3. **Viewport**: 支持长内容滚动浏览

### 依赖关系图
` +
					"```" + `
bubbletea v2
    ├── lipgloss v2 (样式)
    └── bubbles v2
        └── viewport (滚动)

goreact v0.5
    └── Event Bus (事件驱动)
` + "```" + `

### 总结
这是一个**设计精良的 AI 终端应用**，采用：
- ✅ 清晰的分层架构
- ✅ 透明的 AI 推理过程
- ✅ 优雅的终端交互体验
- ✅ 组件化的可维护设计`,
			},
		},
	}

	return c
}

// ============================================================
// 场景3: 实时流式输出（模拟正在进行的对话）
// ============================================================
func streamingExample() conv.Conversation {
	c := conv.NewConversation("s3", "coder", "帮我实现一个用户认证中间件")

	c.Rounds = append(c.Rounds, conv.ThoughtActionRound{
		Thought: conv.Thought{
			IsActive: true,
		},
	})

	return c
}

// ============================================================
// 场景4: Reactor 执行中断（Error 组件展示）
// ============================================================
func errorExample() conv.Conversation {
	now := time.Now()
	c := conv.NewConversation("s4", "researcher", "帮我搜索最新的 Go 1.23 release notes 并总结关键特性")

	c.Status = conv.StatusError

	c.Rounds = append(c.Rounds, conv.ThoughtActionRound{
		Thought: conv.Thought{
			Content: "用户想了解 Go 1.23 的 release notes。我需要：\n\n" +
				"1. 使用 WebSearch 搜索 Go 1.23 release notes\n" +
				"2. 用 WebFetch 抓取官方页面内容\n" +
				"3. 总结关键新特性和 breaking changes",
			TokensIn:  80,
			TokensOut: 40,
			Timestamp: now.Add(-8 * time.Second),
		},
		Action: conv.Action{
			CurrentInfo: &conv.ActionInfo{
				ToolCount:            2,
				ToolNames:            []string{"WebSearch", "WebFetch"},
				TotalPredictedTokens: 2000,
			},
			Steps: []conv.ActionStep{
				{
					ToolName:     "WebSearch",
					Status:       conv.ActionStepDone,
					Params:       map[string]any{"query": "Go 1.23 release notes 2024"},
					EstimatedTok: 500,
					Duration:     1200 * time.Millisecond,
					ResultText:   "Found 5 results for Go 1.23 release notes...",
					Collapsed:    true,
				},
				{
					ToolName:     "WebFetch",
					Status:       conv.ActionStepFailed,
					Params:       map[string]any{"url": "https://go.dev/doc/go1.23"},
					EstimatedTok: 1500,
					Duration:     5 * time.Second,
					ResultText:   "fetch failed: connection timed out after 5s",
					Collapsed:    false,
				},
			},
			Completed:     true,
			SuccessCount:  1,
			FailedCount:   1,
			TotalTokens:   2000,
			TotalDuration: 6200 * time.Millisecond,
		},
	})

	c.Rounds = append(c.Rounds, conv.ThoughtActionRound{
		Thought: conv.Thought{
			Content: "WebFetch 超时了，但 WebSearch 返回了一些结果。让我尝试用另一个源获取信息...\n" +
				"可能的原因：\n" +
				"- 官方站点暂时不可达\n" +
				"- 网络连接不稳定\n\n" +
				"备选方案：使用 Read 工具查看本地缓存的文档。",
			TokensIn:  150,
			TokensOut: 75,
			Timestamp: now.Add(-2 * time.Second),
		},
		Action: conv.Action{
			CurrentInfo: &conv.ActionInfo{
				ToolCount:            1,
				ToolNames:            []string{"Read"},
				TotalPredictedTokens: 800,
			},
			Steps: []conv.ActionStep{
				{
					ToolName:     "Read",
					Status:       conv.ActionStepExecuting,
					Params:       map[string]any{"path": "/tmp/go123-release-notes.md"},
					EstimatedTok: 800,
					ProgressText: "reading file...",
				},
			},
			Elapsed: 3 * time.Second,
		},
	})

	c.Error = conv.ErrorMsg{
		Error: "act error: context canceled",
		Phase: "执行阶段",
		Time:  now,
	}

	return c
}

func main() {
	list := conv.NewConversationList()
	list.Conversations = append(list.Conversations, multiRoundExample())

	p := tea.NewProgram(model{
		convList: list,
		scene:    2,
	})
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
