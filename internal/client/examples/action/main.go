package main

import (
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/mindx/internal/client/component/conv"
	"github.com/DotNetAge/mindx/internal/client/msg"
)

type model struct {
	action conv.Action
	width  int
}

func (m model) Init() tea.Cmd {
	return tickCmd()
}

func (m model) Update(e tea.Msg) (tea.Model, tea.Cmd) {
	switch e := e.(type) {
	case tea.WindowSizeMsg:
		m.width = e.Width
		return m, nil
	case tea.KeyPressMsg:
		switch e.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "1":
			m.action = completedScenario()
			return m, nil
		case "2":
			m.action = executingScenario()
			return m, tickCmd()
		case "3":
			m.action = pendingScenario()
			return m, tickCmd()
		case "4":
			m.action = failedScenario()
			return m, nil
		case "t":
			for i := range m.action.Steps {
				if m.action.Steps[i].Status != conv.ActionStepExecuting {
					m.action.Steps[i].Collapsed = !m.action.Steps[i].Collapsed
				}
			}
			return m, nil
		}
		return m, nil
	case msg.TickMsg:
		newAction, _ := conv.UpdateAction(m.action, e)
		m.action = newAction
		return m, tickCmd()
	}
	return m, nil
}

func (m model) View() tea.View {
	return tea.NewView(
		conv.ViewAction(m.action, m.width) +
			"\n\n" +
			"按 1 全部完成 | 按 2 正在执行 | 按 3 等待执行 | 按 4 执行失败 | 按 t 折叠/展开 | 按 q 退出\n",
	)
}

func completedScenario() conv.Action {
	return conv.Action{
		CurrentInfo: &conv.ActionInfo{
			ToolCount:            3,
			ToolNames:            []string{"Bash", "Edit", "Read"},
			TotalPredictedTokens: 12500,
		},
		Steps: []conv.ActionStep{
			{
				ToolName:     "Bash",
				Status:       conv.ActionStepDone,
				EstimatedTok: 8430,
				Duration:     3 * time.Second,
				ResultText:   "00000000: 0909 0909 6361 7365 2067 6f72 6561 6374\n00000010: 636f 7265 2e54 6f6f 6c45 7865 6345 6e64\n00000020: 3a0a 0909 0909 0969 6620 6461 7461 2c20\n00000030: 7374 642e 4c6f 6e67 2e43 6f6e 7465 6e74\n00000040: 0909 0909 096d 7367 2e54 6f6f 6c52 6573",
				Collapsed:    true,
			},
			{
				ToolName:     "Edit",
				Status:       conv.ActionStepDone,
				EstimatedTok: 2100,
				Duration:     2*time.Second + 300*time.Millisecond,
				ResultText:   "已成功编辑文件 internal/client/client.go",
				Collapsed:    true,
			},
			{
				ToolName:     "Read",
				Status:       conv.ActionStepDone,
				EstimatedTok: 1970,
				Duration:     1*time.Second + 500*time.Millisecond,
				ResultText:   "package main\n\nimport (\n    \"fmt\"\n    \"os\"\n\n    tea \"charm.land/bubbletea/v2\"\n)",
				Collapsed:    true,
			},
		},
		Completed:    true,
		Elapsed:      8 * time.Second,
		StartTime:    time.Now().Add(-8 * time.Second),
		SuccessCount: 3,
		FailedCount:  0,
	}
}

func executingScenario() conv.Action {
	return conv.Action{
		CurrentInfo: &conv.ActionInfo{
			ToolCount:            3,
			ToolNames:            []string{"Bash", "Read", "Edit"},
			TotalPredictedTokens: 25200,
		},
		Steps: []conv.ActionStep{
			{
				ToolName:     "Bash",
				Status:       conv.ActionStepDone,
				EstimatedTok: 12400,
				Duration:     2 * time.Second,
				ResultText:   "found 42 matches\ninternal/client.go:285\ninternal/core/app.go:156\ninternal/pkg/util.go:89",
				Collapsed:    true,
			},
			{
				ToolName:     "Read",
				Status:       conv.ActionStepDone,
				EstimatedTok: 6300,
				Duration:     1*time.Second + 200*time.Millisecond,
				ResultText:   "type Config struct {\n    Host string\n    Port int\n}",
				Collapsed:    false,
			},
			{
				ToolName:     "Edit",
				Status:       conv.ActionStepExecuting,
				EstimatedTok: 6500,
				Params:       map[string]any{"file": "internal/client/client.go"},
				Collapsed:    true,
			},
		},
		Completed:    false,
		StartTime:    time.Now().Add(-4 * time.Second),
		Elapsed:      4 * time.Second,
		SuccessCount: 2,
		FailedCount:  0,
	}
}

func pendingScenario() conv.Action {
	return conv.Action{
		CurrentInfo: &conv.ActionInfo{
			ToolCount:            2,
			ToolNames:            []string{"Bash", "Read"},
			TotalPredictedTokens: 18000,
		},
		Steps:     nil,
		Completed: false,
		StartTime: time.Now().Add(-2 * time.Second),
		Elapsed:   2 * time.Second,
	}
}

func failedScenario() conv.Action {
	return conv.Action{
		CurrentInfo: &conv.ActionInfo{
			ToolCount:            3,
			ToolNames:            []string{"Bash", "Edit", "Read"},
			TotalPredictedTokens: 3600,
		},
		Steps: []conv.ActionStep{
			{
				ToolName:     "Bash",
				Status:       conv.ActionStepDone,
				EstimatedTok: 1200,
				Duration:     1 * time.Second,
				ResultText:   "build completed successfully",
				Collapsed:    true,
			},
			{
				ToolName:     "Edit",
				Status:       conv.ActionStepFailed,
				EstimatedTok: 1500,
				Duration:     500 * time.Millisecond,
				ResultText:   "Error editing file: permission denied\npath: internal/client/client.go\n建议使用 sudo 或检查文件权限",
				Collapsed:    false,
			},
			{
				ToolName:     "Read",
				Status:       conv.ActionStepDone,
				EstimatedTok: 900,
				Duration:     800 * time.Millisecond,
				ResultText:   "func main() {\n    fmt.Println(\"Hello\")\n}",
				Collapsed:    true,
			},
		},
		Completed:    true,
		Elapsed:      6 * time.Second,
		StartTime:    time.Now().Add(-6 * time.Second),
		SuccessCount: 2,
		FailedCount:  1,
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(800*time.Millisecond, func(t time.Time) tea.Msg {
		return msg.TickMsg{Time: t}
	})
}

func main() {
	m := model{
		action: completedScenario(),
		width:  80,
	}
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
