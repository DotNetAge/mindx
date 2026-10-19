package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/DotNetAge/mindx/internal/channel"
	"github.com/DotNetAge/mindx/internal/core"
)

// ---------------------------------------------------------------------------
// channel.* RPC —— 手机连接（局域网软准入 + AgentHub 中继集成）
//
// 数据流：手机经局域网网关调 channel.bootstrap → daemon 弹窗征求用户同意
// （notify 广播 channel.pair_request）→ 用户同意后 channel.approve →
// daemon 生成本地短码并广播 channel.pair_approved（桌面端全局弹窗展示）→
// 手机再次 bootstrap 拿到 ready(require_code) 弹出短码闸，用户在手机端
// 输入短码经 channel.verify 完成软准入（paired_devices 持久化，后续引导
// 直接返回 paired）。跨网段场景仍走中继短码（channel.status 展示，手动输入）。
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

// dialGatewayForBridge 建桥自拨网关：凭证走 Authorization Bearer 头
// （OWASP 反模式修正——token 不进 URL，避免泄漏进日志），不依赖 loopback
// 豁免。拨号成功后立即完成本机设备握手（设备接入强制闸要求）再交给桥使用。
func (d *Daemon) dialGatewayForBridge() (*websocket.Conn, error) {
	token, err := d.loadOrCreateDaemonToken()
	if err != nil {
		return nil, fmt.Errorf("读取握手 token 失败: %w", err)
	}
	u := fmt.Sprintf("ws://localhost%s%s", d.addr, d.wsPath)
	ws, _, err := websocket.DefaultDialer.Dial(u, http.Header{
		"Authorization": []string{"Bearer " + token},
	})
	if err != nil {
		return nil, err
	}
	if err := d.handshakeLocalConn(ws, token); err != nil {
		_ = ws.Close()
		return nil, fmt.Errorf("桥接入闸握手失败: %w", err)
	}
	return ws, nil
}

// handshakeLocalConn 桥连接的本机设备握手：经新连接同步发一次
// device.handshake（本机令牌直通）并等待响应，跳过途中可能收到的
// 广播通知帧。失败即关连接返回错误（调用方重试）。
func (d *Daemon) handshakeLocalConn(ws *websocket.Conn, token string) error {
	req, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "device.handshake",
		"params": map[string]any{
			"device_id": "local-bridge",
			"name":      "本机桥接",
			"platform":  "daemon",
			"token":     token,
		},
	})
	if err != nil {
		return err
	}
	_ = ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := ws.WriteMessage(websocket.TextMessage, req); err != nil {
		return err
	}
	_ = ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		_, raw, err := ws.ReadMessage()
		if err != nil {
			return err
		}
		var resp struct {
			ID    *int            `json:"id"`
			Error *map[string]any `json:"error"`
		}
		if json.Unmarshal(raw, &resp) != nil || resp.ID == nil || *resp.ID != 1 {
			continue // 广播通知等非本次请求响应帧，跳过
		}
		if resp.Error != nil {
			return fmt.Errorf("握手被拒绝")
		}
		return nil
	}
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
// 「允许设备接入」总闸关闭时直接拒绝（手机与 Mac 同受总闸约束）。
func (d *Daemon) handleChannelBootstrap(_ context.Context, params json.RawMessage) (any, error) {
	if d.chMgr == nil {
		return nil, fmt.Errorf("手机连接功能未启用")
	}
	if d.devGate != nil && !d.devGate.Allowed() {
		return nil, fmt.Errorf("设备接入未开启，请先在本机「账户 - 设备」中开启允许设备接入")
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

// handleChannelVerify 处理 channel.verify：手机端提交本地配对短码（软准入第二关）。
func (d *Daemon) handleChannelVerify(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		DeviceHint string `json:"device_hint"`
		Code       string `json:"code"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("参数解析失败")
	}
	if d.chMgr == nil {
		return nil, fmt.Errorf("手机连接功能未启用")
	}
	if err := d.chMgr.Verify(p.DeviceHint, p.Code); err != nil {
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
