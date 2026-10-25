package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/devgate"
	"github.com/DotNetAge/mindx/internal/discovery"
)

// ---------------------------------------------------------------------------
// device.* RPC —— 设备接入闸与 Mac 间互连
//
// 数据流：外部 MindX 设备（另一台 Mac）连上本机 daemon 后调 device.handshake
// → devgate 登记接入请求并广播弹窗（devgate.pair_request）→ 用户在
// 「账户 - 设备」里批准 → 设备轮询握手领取长期 token（此后免配对重连）。
// RPC 注册层强制闸（handler_registry.go）：除白名单方法外，所有调用须先
// 完成 device.handshake 验证（本机令牌直通或已授权设备 token）。
// ---------------------------------------------------------------------------

// initDeviceGate 创建设备接入闸管理器（NewDaemon 阶段调用，失败降级为无闸）。
func (d *Daemon) initDeviceGate() {
	mgr, err := devgate.NewManager(devgate.Config{
		Store:      d.oauthCredStore, // 同一凭据存储（core.CredentialStore）
		LocalToken: d.loadOrCreateDaemonToken,
		Logger:     d.logger,
		Notify: func(method string, params any) {
			if d.gw != nil {
				d.gw.BroadcastNotification(method, params)
			}
		},
		// 吊销即断连：设备被解除/拉黑或总闸关闭时，强制关闭其在线连接
		// （连接级认证下，不断连则在线设备感知不到吊销）
		Disconnect: func(clientID string) {
			if d.gw != nil {
				d.gw.DisconnectClient(clientID)
			}
		},
	})
	if err != nil {
		d.logger.Warn("devgate: 设备接入闸初始化失败，闸已停用", "error", err.Error())
		return
	}
	d.devGate = mgr

	// 启动即把本机握手令牌落盘（loadOrCreate 懒生成会在「客户端永远读不到文件、
	// 因此永远发不出非空 token、daemon 因此永远不触发生成」间形成死锁——
	// 2026-10-23 事故实证：环境重置后 App 读空令牌被闸全拦）。闸装配成功即生成，
	// 保证 App/CLI 任何时刻启动都能读到有效令牌。
	if _, err := d.loadOrCreateDaemonToken(); err != nil {
		d.logger.Warn("devgate: 本机握手令牌生成失败，本机直通将不可用", "error", err.Error())
	}
}

// deviceGateAuthorized 连接是否已完成设备接入验证（RPC 闸与 user.message 闸共用）。
// 闸未装配（初始化失败）时放行，不阻断本机使用。
func (d *Daemon) deviceGateAuthorized(clientID string) bool {
	if d.devGate == nil {
		return true
	}
	return d.devGate.Authorized(clientID)
}

// handleDeviceStatus 处理 device.status：设备接入状态摘要（桌面端设备分组数据源）。
func (d *Daemon) handleDeviceStatus(_ context.Context, _ json.RawMessage) (any, error) {
	if d.devGate == nil {
		return map[string]any{"allowed": true, "devices": []any{}}, nil
	}
	return d.devGate.Status(), nil
}

// handleDeviceAllowSet 处理 device.allow.set：切换「允许设备接入」总开关。
func (d *Daemon) handleDeviceAllowSet(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		Allowed *bool `json:"allowed"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.Allowed == nil {
		return nil, fmt.Errorf("缺少 allowed 参数")
	}
	if d.devGate == nil {
		return nil, fmt.Errorf("设备接入功能未启用")
	}
	if err := d.devGate.SetAllowed(*p.Allowed); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "allowed": *p.Allowed}, nil
}

// handleDeviceApprove 处理 device.approve：批准设备接入请求。
func (d *Daemon) handleDeviceApprove(_ context.Context, params json.RawMessage) (any, error) {
	requestID, err := deviceRequestID(params)
	if err != nil {
		return nil, err
	}
	if d.devGate == nil {
		return nil, fmt.Errorf("设备接入功能未启用")
	}
	if err := d.devGate.Approve(requestID); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// handleDeviceDeny 处理 device.deny：拒绝设备接入请求。
// block=true 时同时拉黑该设备（「拒绝且不再询问」：后续握手静默拒绝，
// 直到主人在设备列表解除）。
func (d *Daemon) handleDeviceDeny(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		RequestID string `json:"request_id"`
		Block     bool   `json:"block"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.RequestID == "" {
		return nil, fmt.Errorf("缺少 request_id 参数")
	}
	if d.devGate == nil {
		return nil, fmt.Errorf("设备接入功能未启用")
	}
	var err error
	if p.Block {
		err = d.devGate.DenyAndBlock(p.RequestID)
	} else {
		err = d.devGate.Deny(p.RequestID)
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// handleDeviceUnblock 处理 device.unblock：解除设备拉黑，
// 该设备恢复「申请 → 弹窗审批」的正常流程。
func (d *Daemon) handleDeviceUnblock(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.DeviceID == "" {
		return nil, fmt.Errorf("缺少 device_id 参数")
	}
	if d.devGate == nil {
		return nil, fmt.Errorf("设备接入功能未启用")
	}
	if err := d.devGate.Unblock(p.DeviceID); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// handleDeviceRevoke 处理 device.revoke：撤销已授权设备的接入许可。
func (d *Daemon) handleDeviceRevoke(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.DeviceID == "" {
		return nil, fmt.Errorf("缺少 device_id 参数")
	}
	if d.devGate == nil {
		return nil, fmt.Errorf("设备接入功能未启用")
	}
	if err := d.devGate.Revoke(p.DeviceID); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// handleDevicePairCode 处理 device.pair.code：生成一次性配对码
// （仅本机连接可调——强制闸保证外部设备无法拿到本机批量的码）。
func (d *Daemon) handleDevicePairCode(_ context.Context, _ json.RawMessage) (any, error) {
	if d.devGate == nil {
		return nil, fmt.Errorf("设备接入功能未启用")
	}
	code, expiresAt, err := d.devGate.GeneratePairCode()
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"code":        code,
		"expires_at":  expiresAt.Format(time.RFC3339),
		"ttl_seconds": int(time.Until(expiresAt).Seconds()),
	}, nil
}

// handleDeviceHandshake 处理 device.handshake：设备接入握手（强制闸唯一对外开口）。
// account_trust 非空时走同账号互信路径（跨端连接第二步，DEVICE-TRUST.md 定案一）：
// 凭被连方自己的 AuthToken 向社区服务端核验对端凭据——同账号且凭据签发设备与
// 对端自报身份一致才放行互信，否则置空回落既有弹窗/配对码流程。
func (d *Daemon) handleDeviceHandshake(ctx context.Context, params json.RawMessage) (any, error) {
	if d.devGate == nil {
		return nil, fmt.Errorf("设备接入功能未启用")
	}
	var p struct {
		DeviceID     string                `json:"device_id"`
		Name         string                `json:"name"`
		Platform     string                `json:"platform"`
		Token        string                `json:"token"`
		PairCode     string                `json:"pair_code"`
		AccountTrust *devgate.AccountTrust `json:"account_trust"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("参数解析失败")
	}
	clientID := gateway.ClientIDFromContext(ctx)
	if clientID == "" {
		return nil, fmt.Errorf("连接身份缺失，请重新连接后重试")
	}
	return d.devGate.Handshake(clientID, p.DeviceID, p.Name, p.Platform, p.Token, p.PairCode, d.resolveAccountTrust(p.AccountTrust, p.DeviceID)), nil
}

// resolveAccountTrust 核验对端出示的同账号互信凭据，返回可放行的凭据或 nil（回落）：
//   - 本机未登录社区 → 无法核验，回落（同账号互信以双方都登录为前提）；
//   - 凭据核验失败（服务端不可达/凭据过期）→ 回落弹窗流程，绝不在核验不可用时放行；
//   - 非同账号 → 回落弹窗流程（互信只服务同账号）；
//   - 凭据签发设备与对端自报身份不一致 → 视为凭据挪用，回落弹窗流程。
func (d *Daemon) resolveAccountTrust(trust *devgate.AccountTrust, deviceID string) *devgate.AccountTrust {
	if trust == nil || trust.Nonce == "" || trust.ClientID == "" || deviceID == "" {
		return nil
	}
	if d.app == nil || d.app.MarketAuthToken() == "" {
		return nil
	}
	verdict, err := d.app.Market().DeviceTrustConfirm(trust.Nonce)
	if err != nil {
		d.logger.Warn("devgate: 同账号互信凭据核验失败，回落弹窗流程", "error", err.Error())
		return nil
	}
	if !verdict.SameAccount() {
		d.logger.Info("devgate: 互信凭据非同账号，回落弹窗流程")
		return nil
	}
	if verdict.IssuerClientID != trust.ClientID {
		d.logger.Warn("devgate: 互信凭据签发设备与对端自报身份不一致，回落弹窗流程")
		return nil
	}
	return trust
}

// handleDeviceTrustTicket 处理 device.trust.ticket：本机领取同账号互信凭据
// （跨端连接第二步）。仅本机直通连接可调——凭据绑定本机登记设备身份，
// 外接设备不得代领（防止凭连接权换取其他设备的互信凭据）。
func (d *Daemon) handleDeviceTrustTicket(ctx context.Context, _ json.RawMessage) (any, error) {
	if d.devGate == nil || !d.devGate.LocalVerified(gateway.ClientIDFromContext(ctx)) {
		return nil, fmt.Errorf("仅本机可领取互信凭据")
	}
	if d.app == nil || d.app.MarketAuthToken() == "" {
		return nil, fmt.Errorf("尚未登录社区，无法领取互信凭据")
	}
	ticket, err := d.app.Market().DeviceTrustStart()
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"nonce":      ticket.Nonce,
		"client_id":  ticket.ClientID,
		"user_id":    ticket.UserID,
		"expires_in": ticket.ExpiresIn,
	}, nil
}

// handleDeviceScan 处理 device.scan：局域网 mDNS 浏览，返回可连接的 MindX 主机
// （排除自身：本机局域网 IP 与本机实例名双重比对）。
func (d *Daemon) handleDeviceScan(_ context.Context, _ json.RawMessage) (any, error) {
	entries, err := discovery.Browse(3 * time.Second)
	if err != nil {
		return nil, err
	}
	lanIP := detectLANIP()
	selfInstance := fmt.Sprintf("MindX@%s", hostname())
	hosts := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		if (e.Host != "" && e.Host == lanIP) || e.Instance == selfInstance {
			continue
		}
		hosts = append(hosts, map[string]any{
			"instance": e.Instance,
			"host":     e.Host,
			"port":     e.Port,
		})
	}
	return map[string]any{"devices": hosts}, nil
}

// hostname 返回本机主机名（失败时回落 "unknown-host"，与 mDNS 广播命名一致）。
func hostname() string {
	name, err := os.Hostname()
	if err != nil || name == "" {
		return "unknown-host"
	}
	return name
}

// deviceRequestID 从 RPC 参数解析请求标识。
func deviceRequestID(params json.RawMessage) (string, error) {
	var p struct {
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.RequestID == "" {
		return "", fmt.Errorf("缺少 request_id 参数")
	}
	return p.RequestID, nil
}
