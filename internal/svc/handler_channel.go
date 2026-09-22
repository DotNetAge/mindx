package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/gorilla/websocket"

	"github.com/DotNetAge/mindx/internal/channel"
	"github.com/DotNetAge/mindx/internal/core"
)

// ---------------------------------------------------------------------------
// channel.* RPC —— 手机连接（AgentHub 中继集成）
//
// 数据流：手机经局域网网关调 channel.bootstrap → daemon 弹窗征求用户同意
// （notify 广播 channel.pair_request）→ 用户同意后 channel.approve →
// 手机再次 bootstrap 拿到中继地址与短码 → 双方连中继完成配对 → daemon
// 建立网关桥，手机经中继远程访问桌面控制台。
// ---------------------------------------------------------------------------

// initChannelManager 创建中继管理器（NewDaemon 阶段调用）。
func (d *Daemon) initChannelManager() {
	mgr, err := channel.NewManager(channel.Config{
		URL:    d.app.Config().ChannelURL,
		Certs:  core.NewCredentialStore(d.app.Settings().UserPreferences()),
		Logger: d.logger,
		Notify: func(method string, params any) {
			if d.gw != nil {
				d.gw.BroadcastNotification(method, params)
			}
		},
		GatewayDial: d.dialGatewayForBridge,
	})
	if err != nil {
		d.logger.Warn("channel: 中继管理器初始化失败，手机连接不可用", "error", err.Error())
		return
	}
	d.chMgr = mgr
}

// dialGatewayForBridge 建桥自拨网关：显式携带握手 token（不依赖 loopback 豁免）。
func (d *Daemon) dialGatewayForBridge() (*websocket.Conn, error) {
	token, err := d.loadOrCreateDaemonToken()
	if err != nil {
		return nil, fmt.Errorf("读取握手 token 失败: %w", err)
	}
	u := fmt.Sprintf("ws://localhost%s%s?token=%s", d.addr, d.wsPath, url.QueryEscape(token))
	ws, _, err := websocket.DefaultDialer.Dial(u, nil)
	return ws, err
}

// onChannelURLChanged 中继地址变更入口（user.config 保存后调用）。
func (d *Daemon) onChannelURLChanged(channelURL string) {
	if d.chMgr != nil {
		d.chMgr.OnURLChanged(channelURL)
	}
}

// handleChannelStatus 处理 channel.status：返回通道状态摘要（桌面端 3s 轮询）。
func (d *Daemon) handleChannelStatus(_ context.Context, _ json.RawMessage) (any, error) {
	if d.chMgr == nil {
		return map[string]any{
			"configured": false,
			"connected":  false,
			"state":      channel.StateOffline,
			"paired":     false,
		}, nil
	}
	return d.chMgr.Status(), nil
}

// handleChannelBootstrap 处理 channel.bootstrap：手机端引导配对（同意闸入口）。
func (d *Daemon) handleChannelBootstrap(_ context.Context, params json.RawMessage) (any, error) {
	if d.chMgr == nil {
		return nil, fmt.Errorf("手机连接功能未启用")
	}
	var p struct {
		DeviceHint string `json:"device_hint"`
	}
	_ = json.Unmarshal(params, &p)
	return d.chMgr.Bootstrap(p.DeviceHint), nil
}

// handleChannelApprove 处理 channel.approve：桌面端同意配对请求。
func (d *Daemon) handleChannelApprove(_ context.Context, params json.RawMessage) (any, error) {
	requestID, err := channelRequestID(params)
	if err != nil {
		return nil, err
	}
	if d.chMgr == nil {
		return nil, fmt.Errorf("手机连接功能未启用")
	}
	if err := d.chMgr.Approve(requestID); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// handleChannelDeny 处理 channel.deny：桌面端拒绝配对请求。
func (d *Daemon) handleChannelDeny(_ context.Context, params json.RawMessage) (any, error) {
	requestID, err := channelRequestID(params)
	if err != nil {
		return nil, err
	}
	if d.chMgr == nil {
		return nil, fmt.Errorf("手机连接功能未启用")
	}
	if err := d.chMgr.Deny(requestID); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// channelRequestID 从 RPC 参数解析请求标识。
func channelRequestID(params json.RawMessage) (string, error) {
	var p struct {
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.RequestID == "" {
		return "", fmt.Errorf("缺少 request_id 参数")
	}
	return p.RequestID, nil
}
