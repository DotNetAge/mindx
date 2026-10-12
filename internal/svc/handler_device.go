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
	})
	if err != nil {
		d.logger.Warn("devgate: 设备接入闸初始化失败，闸已停用", "error", err.Error())
		return
	}
	d.devGate = mgr
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
func (d *Daemon) handleDeviceDeny(_ context.Context, params json.RawMessage) (any, error) {
	requestID, err := deviceRequestID(params)
	if err != nil {
		return nil, err
	}
	if d.devGate == nil {
		return nil, fmt.Errorf("设备接入功能未启用")
	}
	if err := d.devGate.Deny(requestID); err != nil {
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

// handleDeviceHandshake 处理 device.handshake：设备接入握手（强制闸唯一对外开口）。
func (d *Daemon) handleDeviceHandshake(ctx context.Context, params json.RawMessage) (any, error) {
	if d.devGate == nil {
		return nil, fmt.Errorf("设备接入功能未启用")
	}
	var p struct {
		DeviceID string `json:"device_id"`
		Name     string `json:"name"`
		Platform string `json:"platform"`
		Token    string `json:"token"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("参数解析失败")
	}
	clientID := gateway.ClientIDFromContext(ctx)
	if clientID == "" {
		return nil, fmt.Errorf("连接身份缺失，请重新连接后重试")
	}
	return d.devGate.Handshake(clientID, p.DeviceID, p.Name, p.Platform, p.Token), nil
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
