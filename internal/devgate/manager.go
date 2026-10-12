// Package devgate 设备接入闸：Mac 间互连（设备直连 daemon）的准入控制层。
//
// 职责（与手机链路 channel.Manager 的同意闸范式一致）：
//   - 「允许设备接入」总开关（持久化；关闭时非本机客户端一律拒绝）
//   - 设备接入请求的同意闸（桌面端弹窗批准/拒绝）
//   - 已授权设备表（批准时签发长期 token，重连凭 token 免配对）
//   - 已验证连接登记（clientID → deviceID），供 RPC 注册层强制闸查询
//
// 安全模型：局域网信任（与 mDNS 广播同级风险）。本机客户端凭 daemon
// 握手令牌（~/.mindx/daemon_token，0600）直通且不受开关限制；外部设备
// 必须经弹窗批准。批准后 token 按请求的 deviceID 下发给下一个握手的
// 轮询者——批准即视同设备在场，此后连接均须凭 token。
package devgate

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/DotNetAge/mindx/pkg/logging"
)

// 持久化键（经 Store 存取，macOS 落钥匙串、其余落加密文件）
const (
	keyAllow   = "devgate_allow"   // "1"/"0"，无记录时默认开启（不破坏既有手机链路）
	keyDevices = "devgate_devices" // JSON map[deviceID]Device
)

// 配对请求状态与时效（同 channel 同意闸语义）
const (
	reqPending  = "pending"
	reqApproved = "approved"
	reqDenied   = "denied"
	requestTTL  = 5 * time.Minute // 请求（含拒绝结果）的失效时长
)

// localDeviceID 本机直通客户端在会话表中的登记名（非接入设备，不出现在设备列表）
const localDeviceID = "local"

// Store 持久化最小接口（由 core.CredentialStore 满足，保持本包与其解耦）
type Store interface {
	Get(key string) (string, error)
	Set(key, value string) error
}

// Device 已授权接入的设备（token 长期有效，重连免配对）
type Device struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Platform   string    `json:"platform"`
	Token      string    `json:"token"`
	ApprovedAt time.Time `json:"approved_at"`
}

// PairRequest 待审批接入请求（序列化给桌面端弹窗）
type PairRequest struct {
	RequestID string    `json:"request_id"`
	DeviceID  string    `json:"device_id"`
	Name      string    `json:"name"`
	Platform  string    `json:"platform"`
	CreatedAt time.Time `json:"created_at"`

	status    string    // pending | approved | denied（不序列化）
	decidedAt time.Time // 批准/拒绝时刻，用于 TTL 判定
}

// HandshakeResult device.handshake 响应体：
// paired=已完成接入验证；pending=等待桌面端批准（轮询重试）；
// denied=已被拒绝（TTL 内停止重试）；closed=设备接入未开启。
type HandshakeResult struct {
	Status    string `json:"status"`
	Token     string `json:"token,omitempty"`      // 批准后首次轮询下发
	RequestID string `json:"request_id,omitempty"` // pending 时供日志排查
}

// Config 管理器构造参数
type Config struct {
	// Store 持久化（必填）
	Store Store
	// LocalToken 本机握手令牌读取（必填；命中即本机直通）
	LocalToken func() (string, error)
	// Logger 日志（必填）
	Logger logging.Logger
	// Notify 经网关广播通知桌面端（接入请求弹窗依赖）
	Notify func(method string, params any)
}

// Manager 设备接入闸管理器：互斥锁保护状态，无后台循环
// （外部设备轮询 device.handshake 推进配对，无需常驻 goroutine）。
type Manager struct {
	cfg    Config
	logger logging.Logger

	mu       sync.Mutex
	allowed  bool
	devices  map[string]Device       // 已授权设备表（持久化）
	requests map[string]*PairRequest // 接入请求表
	sessions map[string]string       // 已验证连接：clientID → deviceID
}

// NewManager 创建管理器并加载持久化状态，依赖不满足时返回错误而非 panic。
func NewManager(cfg Config) (*Manager, error) {
	if cfg.Store == nil {
		return nil, fmt.Errorf("devgate: 持久化存储不能为空")
	}
	if cfg.LocalToken == nil {
		return nil, fmt.Errorf("devgate: 本机令牌读取器不能为空")
	}
	if cfg.Logger == nil {
		return nil, fmt.Errorf("devgate: 日志不能为空")
	}
	mgr := &Manager{
		cfg:      cfg,
		logger:   cfg.Logger,
		allowed:  true, // 默认开启：总闸只作紧急全断，准入主要靠弹窗批准
		devices:  map[string]Device{},
		requests: map[string]*PairRequest{},
		sessions: map[string]string{},
	}
	if v, err := cfg.Store.Get(keyAllow); err == nil {
		mgr.allowed = v == "1"
	}
	if v, err := cfg.Store.Get(keyDevices); err == nil && v != "" {
		var devices map[string]Device
		if err := json.Unmarshal([]byte(v), &devices); err == nil {
			mgr.devices = devices
		} else {
			mgr.logger.Warn("devgate: 已授权设备表损坏，忽略", "error", err.Error())
		}
	}
	return mgr, nil
}

// Allowed 返回「允许设备接入」开关状态。
func (m *Manager) Allowed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.allowed
}

// SetAllowed 设置总开关并持久化。关闭时清除全部非本机连接的验证登记，
// 已接入设备立即失去调用权（重新开启后需凭 token 重新握手）。
func (m *Manager) SetAllowed(allowed bool) error {
	m.mu.Lock()
	m.allowed = allowed
	if !allowed {
		for clientID, deviceID := range m.sessions {
			if deviceID != localDeviceID {
				delete(m.sessions, clientID)
			}
		}
	}
	err := m.persistLocked()
	m.mu.Unlock()
	if err == nil {
		m.notifyDevicesChanged()
	}
	return err
}

// Handshake 处理设备接入握手（RPC 注册层唯一对外开口）：
// 本机令牌 → 直通；已授权 token → 免配对；待批准请求 → 轮询推进；
// 其余 → 登记请求并广播桌面端弹窗。
func (m *Manager) Handshake(clientID, deviceID, name, platform, token string) HandshakeResult {
	if clientID == "" {
		return HandshakeResult{Status: "denied"}
	}
	if deviceID == "" {
		deviceID = "unknown"
	}
	if name == "" {
		name = "未命名设备"
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	// 本机信任令牌：直通且不受开关限制（本机 GUI/CLI/桥不能被锁死）
	if token != "" && m.localTokenMatchesLocked(token) {
		m.sessions[clientID] = localDeviceID
		return HandshakeResult{Status: "paired"}
	}

	if !m.allowed {
		return HandshakeResult{Status: "closed"}
	}

	// 已授权设备：token 匹配直接放行（重连免配对）
	if token != "" {
		if dev, ok := m.devices[deviceID]; ok && dev.Token == token {
			m.sessions[clientID] = deviceID
			return HandshakeResult{Status: "paired"}
		}
	}

	// 既有请求轮询（对方批准后按 deviceID 领取 token 完成绑定）
	if req := m.latestRequestByDeviceLocked(deviceID); req != nil {
		switch req.status {
		case reqPending:
			return HandshakeResult{Status: "pending", RequestID: req.RequestID}
		case reqDenied:
			if time.Since(req.decidedAt) < requestTTL {
				return HandshakeResult{Status: "denied"}
			}
			delete(m.requests, req.RequestID) // 拒绝已过期，走新建流程
		case reqApproved:
			dev, ok := m.devices[deviceID]
			if !ok {
				return HandshakeResult{Status: "pending", RequestID: req.RequestID}
			}
			delete(m.requests, req.RequestID)
			m.sessions[clientID] = deviceID
			return HandshakeResult{Status: "paired", Token: dev.Token}
		}
	}

	// 新建待审批请求并广播桌面端弹窗
	req := &PairRequest{
		RequestID: newRequestID(),
		DeviceID:  deviceID,
		Name:      name,
		Platform:  platform,
		CreatedAt: time.Now(),
		status:    reqPending,
	}
	m.requests[req.RequestID] = req
	m.notify("devgate.pair_request", map[string]any{
		"type":  "devgate.pair_request",
		"title": "设备接入请求",
		"data": map[string]any{
			"request_id": req.RequestID,
			"device_id":  req.DeviceID,
			"name":       req.Name,
			"platform":   req.Platform,
			"created_at": req.CreatedAt,
		},
	})
	return HandshakeResult{Status: "pending", RequestID: req.RequestID}
}

// Approve 批准接入请求：为该设备签发长期 token 并持久化。
// 设备随后的握手轮询将领取 token（此后重连凭 token 免配对）。
func (m *Manager) Approve(requestID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	req, ok := m.requests[requestID]
	if !ok || req.status != reqPending {
		return fmt.Errorf("接入请求不存在或已处理")
	}
	req.status = reqApproved
	req.decidedAt = time.Now()
	m.devices[req.DeviceID] = Device{
		ID:         req.DeviceID,
		Name:       req.Name,
		Platform:   req.Platform,
		Token:      newToken(),
		ApprovedAt: time.Now(),
	}
	if err := m.persistLocked(); err != nil {
		return err
	}
	m.notifyDevicesChanged()
	return nil
}

// Deny 拒绝接入请求：TTL 内该设备重试握手将收到 denied。
func (m *Manager) Deny(requestID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	req, ok := m.requests[requestID]
	if !ok || req.status != reqPending {
		return fmt.Errorf("接入请求不存在或已处理")
	}
	req.status = reqDenied
	req.decidedAt = time.Now()
	return nil
}

// Revoke 撤销设备的接入授权：删除设备记录并清除其已验证连接的登记，
// 该设备的后续调用立即被强制闸拒绝。
func (m *Manager) Revoke(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.devices[deviceID]; !ok {
		return fmt.Errorf("该设备不在已授权列表中")
	}
	delete(m.devices, deviceID)
	for clientID, id := range m.sessions {
		if id == deviceID {
			delete(m.sessions, clientID)
		}
	}
	if err := m.persistLocked(); err != nil {
		return err
	}
	m.notifyDevicesChanged()
	return nil
}

// Authorized 查询连接是否已完成接入验证（RPC 强制闸与 user.message 闸共用）。
func (m *Manager) Authorized(clientID string) bool {
	if clientID == "" {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.sessions[clientID]
	return ok
}

// Status 返回设备接入状态摘要（device.status 响应体）。
// token 不下发（凭据只经握手通道发给对应设备）。
func (m *Manager) Status() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()

	resp := map[string]any{
		"allowed": m.allowed,
	}
	devices := make([]map[string]any, 0, len(m.devices))
	for _, dev := range m.devices {
		devices = append(devices, map[string]any{
			"id":          dev.ID,
			"name":        dev.Name,
			"platform":    dev.Platform,
			"approved_at": dev.ApprovedAt,
		})
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i]["id"].(string) < devices[j]["id"].(string) })
	resp["devices"] = devices

	// 待审批请求（同刻至多一个活跃流程，取最早创建的一个）
	var earliest *PairRequest
	for _, req := range m.requests {
		if req.status != reqPending {
			continue
		}
		if earliest == nil || req.CreatedAt.Before(earliest.CreatedAt) {
			earliest = req
		}
	}
	if earliest != nil {
		resp["pending_request"] = earliest
	}
	return resp
}

// Unbind 断连清理：连接关闭后其验证登记随之失效（重连需重新握手）。
func (m *Manager) Unbind(clientID string) {
	if clientID == "" {
		return
	}
	m.mu.Lock()
	delete(m.sessions, clientID)
	m.mu.Unlock()
}

// latestRequestByDeviceLocked 取该设备最近创建的一条请求（TTL 过期的顺带清理）。
func (m *Manager) latestRequestByDeviceLocked(deviceID string) *PairRequest {
	var latest *PairRequest
	for id, req := range m.requests {
		if req.DeviceID != deviceID {
			continue
		}
		if req.status != reqPending && time.Since(req.decidedAt) >= requestTTL {
			delete(m.requests, id)
			continue
		}
		if latest == nil || req.CreatedAt.After(latest.CreatedAt) {
			latest = req
		}
	}
	return latest
}

// localTokenMatchesLocked 本机令牌比对（读取失败视同不匹配）。
func (m *Manager) localTokenMatchesLocked(token string) bool {
	lt, err := m.cfg.LocalToken()
	return err == nil && lt != "" && token == lt
}

// persistLocked 持久化开关与设备表（调用方持锁）。
func (m *Manager) persistLocked() error {
	allow := "0"
	if m.allowed {
		allow = "1"
	}
	if err := m.cfg.Store.Set(keyAllow, allow); err != nil {
		return fmt.Errorf("持久化接入开关失败: %w", err)
	}
	data, err := json.Marshal(m.devices)
	if err != nil {
		return fmt.Errorf("序列化已授权设备表失败: %w", err)
	}
	if err := m.cfg.Store.Set(keyDevices, string(data)); err != nil {
		return fmt.Errorf("持久化已授权设备表失败: %w", err)
	}
	return nil
}

// notify 经网关广播通知桌面端（弹窗与列表刷新依赖）。
func (m *Manager) notify(method string, params any) {
	if m.cfg.Notify != nil {
		m.cfg.Notify(method, params)
	}
}

// notifyDevicesChanged 广播设备列表变更（桌面端刷新设备分组）。
func (m *Manager) notifyDevicesChanged() {
	m.notify("devgate.devices_changed", map[string]any{
		"type":  "devgate.devices_changed",
		"title": "设备列表已更新",
		"data":  map[string]any{},
	})
}

// newRequestID 生成请求标识（时间戳 + 随机后缀，同 channel 范式）。
func newRequestID() string {
	buf := make([]byte, 6)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("req-%d-%s", time.Now().UnixMilli(), hex.EncodeToString(buf))
}

// newToken 生成设备长期接入令牌（32 字节 CSPRNG hex）。
func newToken() string {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
