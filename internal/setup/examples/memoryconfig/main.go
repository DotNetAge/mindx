package main

import (
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/DotNetAge/mindx/internal/setup/component/memoryconfig"
	setupmsg "github.com/DotNetAge/mindx/internal/setup/msg"
)

type model struct {
	memoryConfig *memoryconfig.Model
	scene       int
	simulating  bool
	progress    float64
}

type tickMsg time.Time

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		var cmd tea.Cmd
		m.memoryConfig, cmd = m.memoryConfig.Update(msg)
		if m.simulating {
			return m, tea.Batch(cmd, tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) }))
		}
		return m, cmd

	case tickMsg:
		if !m.simulating {
			return m, nil
		}
		m.progress += 1.5 + (m.progress * 0.05)
		if m.progress >= 100 {
			m.progress = 100
			m.simulating = false
			fakeDone := setupmsg.DownloadProgressMsg{
				Done:   true,
				Status: "模型下载完成",
			}
			var cmd tea.Cmd
			m.memoryConfig, cmd = m.memoryConfig.Update(fakeDone)
			return m, cmd
		}
		totalMB := 23.5
		downloadedMB := m.progress / 100 * totalMB
		fakeProgress := setupmsg.DownloadProgressMsg{
			Current: int64(downloadedMB * 1024 * 1024),
			Total:   int64(totalMB * 1024 * 1024),
			Status:  fmt.Sprintf("下载中  %.1f / %.1f MB", downloadedMB, totalMB),
		}
		var cmd tea.Cmd
		m.memoryConfig, cmd = m.memoryConfig.Update(fakeProgress)
		return m, cmd

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.simulating {
				m.simulating = false
			}
			return m, tea.Quit
		case "1":
			m.scene = 1
			m.simulating = false
			m.progress = 0
			m.memoryConfig = memoryconfig.New("/tmp/workspace", false)
			return m, nil
		case "2":
			m.scene = 2
			m.simulating = true
			m.progress = 0
			m.memoryConfig = memoryconfig.New("/tmp/workspace", false)
			var cmd tea.Cmd
			m.memoryConfig, cmd = m.memoryConfig.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			return m, tea.Batch(cmd, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) }))
		case "3":
			m.scene = 3
			m.simulating = false
			m.progress = 0
			m.memoryConfig = memoryconfig.New("/tmp/workspace", true)
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.memoryConfig, cmd = m.memoryConfig.Update(msg)
	return m, cmd
}

func (m model) View() tea.View {
	hint := "\n按 1 未下载状态 | 按 2 模拟下载进度 | 按 3 已下载完成 | 按 q 退出"
	switch m.scene {
	case 1:
		hint = "\n📌 场景: Embedder 模型未下载（可选择是否下载）" + hint
	case 2:
		hint = "\n📌 场景: 模拟下载中...（自动推进进度条）" + hint
	case 3:
		hint = "\n📌 场景: Embedder 模型已就绪" + hint
	default:
		hint = "\n👆 选择一个场景查看记忆体配置页面" + hint
	}
	return tea.NewView(m.memoryConfig.View() + hint)
}

func main() {
	m := model{
		memoryConfig: memoryconfig.New("/tmp/workspace", false),
		scene:       0,
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
