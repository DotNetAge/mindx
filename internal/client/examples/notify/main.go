package main

import (
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/DotNetAge/mindx/internal/client/component/notify"
	"github.com/DotNetAge/mindx/internal/client/data"
	clientmsg "github.com/DotNetAge/mindx/internal/client/msg"
)

type model struct {
	notifBar *notify.NotificationBar
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(e tea.Msg) (tea.Model, tea.Cmd) {
	switch e := e.(type) {
	case tea.WindowSizeMsg:
		m.notifBar.Update(clientmsg.WindowResizeMsg{Width: e.Width, Height: e.Height})
		return m, nil
	case tea.KeyPressMsg:
		switch e.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "i":
			return m, m.notifBar.Add(data.Notification{
				Message:  "这是一个信息通知",
				Level:    data.NotifInfo,
				Duration: 10 * time.Second,
			})
		case "s":
			return m, m.notifBar.Add(data.Notification{
				Message:  "操作成功完成！",
				Level:    data.NotifSuccess,
				Duration: 10 * time.Second,
			})
		case "e":
			return m, m.notifBar.Add(data.Notification{
				Message:  "发生了一个错误：连接超时",
				Level:    data.NotifError,
				Duration: 10 * time.Second,
			})
		case "w":
			return m, m.notifBar.Add(data.Notification{
				Message:  "警告：磁盘空间不足",
				Level:    data.NotifWarning,
				Duration: 10 * time.Second,
			})
		case "c":
			m.notifBar.Notifications = nil
			return m, nil
		}
		return m, nil
	case clientmsg.NotifTimeoutMsg:
		m.notifBar.Update(e)
		return m, nil
	}
	return m, nil
}

func (m model) View() tea.View {
	return tea.NewView(
		m.notifBar.View() +
			"\n\n按 i 信息 | s 成功 | e 错误 | w 警告 | c 清除 | q 退出\n",
	)
}

func main() {
	nb := notify.New()
	nb.Width = 80

	nb.Add(data.Notification{
		Message:  "欢迎使用 NotificationBar 演示（本通知 30 秒后消失）",
		Level:    data.NotifInfo,
		Duration: 30 * time.Second,
	})

	p := tea.NewProgram(model{notifBar: nb})
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
