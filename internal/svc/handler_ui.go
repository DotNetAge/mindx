package svc

import (
	"context"
	"encoding/json"
	"fmt"
)

// ---------------------------------------------------------------------------
// ui.* RPC —— Agent-Driven UI 命令通道（daemon 侧受理端点）
//
// 数据流：Agent 执行 mindx ui open|open-link|run → CLI 经 RPC 调本组方法 →
// daemon 广播对应 JSON 事件（file_open / link_open / terminal_run）→ 各客户端
// 订阅执行（文件分发表 / 系统浏览器 / 终端插件）。
//
// 受理语义：RPC 返回即「已受理并广播」，不等客户端执行效果回执——效果发生在
// 用户屏幕上（用户可见的即时纠错）代替回执；事件命名对齐 tool_exec_end/loop_end
// 的 snake_case 惯例。参数闸在 CLI 侧（open 工作区路径闸、open-link 仅 http/https），
// daemon 作为广播中枢信任已过闸的参数，不重复校验。
// ---------------------------------------------------------------------------

// handleUIOpen 处理 ui.open：广播 file_open，客户端按文件类型分发打开对应 Detail。
func (d *Daemon) handleUIOpen(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.Path == "" {
		return nil, fmt.Errorf("缺少 path 参数")
	}
	d.broadcastUI("file_open", map[string]any{"path": p.Path})
	return map[string]any{"ok": true}, nil
}

// handleUIOpenLink 处理 ui.open_link：广播 link_open，客户端经系统浏览器打开。
func (d *Daemon) handleUIOpenLink(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.URL == "" {
		return nil, fmt.Errorf("缺少 url 参数")
	}
	d.broadcastUI("link_open", map[string]any{"url": p.URL})
	return map[string]any{"ok": true}, nil
}

// handleUIRun 处理 ui.run：广播 terminal_run，客户端打开终端执行命令。
// cwd 为 CLI 进程工作目录（Agent 工作区），客户端据此定位终端会话目录。
func (d *Daemon) handleUIRun(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		Command string `json:"command"`
		Cwd     string `json:"cwd"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.Command == "" {
		return nil, fmt.Errorf("缺少 command 参数")
	}
	d.broadcastUI("terminal_run", map[string]any{"command": p.Command, "cwd": p.Cwd})
	return map[string]any{"ok": true}, nil
}

// broadcastUI 统一广播 UI 命令事件：envelope 结构（type/title/data）对齐
// permission_request 旁路广播惯例（daemon.go WithPermissionSink），无会话归属。
func (d *Daemon) broadcastUI(eventType string, data map[string]any) {
	if d.gw == nil {
		return
	}
	d.gw.BroadcastNotification(eventType, map[string]any{
		"type": eventType,
		"data": data,
	})
}
