// Package devgate 设备接入闸：Mac 间互连（设备直连 daemon）的准入控制层。
//
// 职责（与手机链路 channel.Manager 的同意闸范式一致）：
//   - 「允许设备接入」总开关（持久化；关闭时非本机客户端一律拒绝）
//   - 设备接入请求的同意闸（桌面端弹窗批准/拒绝）
//   - 已授权设备表（批准时签发长期 token，重连凭 token 免配对）
//   - 已验证连接登记（clientID → deviceID），供 RPC 注册层强制闸查询
//
// 安全模型：局域网信任（与 mDNS 广播同级风险）。本机客户端凭 daemon
// 握手令牌（唯一落点 ~/.mindx/.token，0600，仅 daemon 写入）直通且不受
// 开关限制；外部设备必须经弹窗批准。批准后 token 按请求的 deviceID
// 下发给下一个握手的轮询者——批准即视同设备在场，此后连接均须凭 token。
package devgate

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
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
	keyBlocked = "devgate_blocked" // JSON map[deviceID]BlockedEntry
)

// 配对请求状态与时效（同 channel 同意闸语义）
const (
	reqPending  = "pending"
	reqApproved = "approved"
	reqDenied   = "denied"
	requestTTL  = 5 * time.Minute // 请求（含拒绝结果）的失效时长
)

// 设备令牌作用域（PAT 族分层授权）：
//   - ScopeLocal 本机全权：本机壳/CLI/桥凭 daemon_token 直通（等价持本机凭证）
//   - ScopeAsk   远程设备默认：对话/文件/市场等工作面可用，
//     管理面（设备管理、本机凭据下发、系统与全局配置变更）一律拒绝
const (
	ScopeLocal = "local"
	ScopeAsk   = "ask"
)

// 配对码（短码）时效：5 分钟一次性，连错 5 次即作废（防在线猜测）
const (
	pairCodeTTL         = 5 * time.Minute
	pairCodeMaxAttempts = 5
)

// pairCodeState 活跃配对码（纯内存态：daemon 重启即失效，重新生成即可）
type pairCodeState struct {
	code      string
	expiresAt time.Time
	attempts  int
}

// localDeviceID 本机直通客户端在会话表中的登记名（非接入设备，不出现在设备列表）
const localDeviceID = "local"

// Store 持久化最小接口（由 core.CredentialStore 满足，保持本包与其解耦）
type Store interface {
	Get(key string) (string, error)
	Set(key, value string) error
}

// 设备接入来源（TrustSource）：弹窗批准 / 配对码 / 同账号互信（设备列表标注用）
const (
	TrustPopup    = "popup"    // 主人弹窗批准（旧记录缺省）
	TrustPairCode = "paircode" // headless 配对码首连
	TrustAccount  = "account"  // 同账号互信（跨端连接第二步，免弹窗）
)

// Device 已授权接入的设备（token 长期有效，重连免配对）。
// Scope 为该设备令牌的作用域（PAT 分层授权），旧记录缺省归一为 ScopeAsk。
type Device struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Platform    string    `json:"platform"`
	Token       string    `json:"token"`
	Scope       string    `json:"scope"`
	TrustSource string    `json:"trust_source,omitempty"` // 接入来源（旧记录缺省 popup）
	ApprovedAt  time.Time `json:"approved_at"`
}

// AccountTrust 连接方出示的同账号互信凭据（跨端连接第二步，DEVICE-TRUST.md 定案一）：
// nonce 由社区服务端签发（绑定连接方登记设备身份与账号），握手层凭被连方自己的
// AuthToken 向服务端核验通过后才传到这里——同账号判定与设备身份核对都在
// 握手层完成，管理器只负责「已核实即免弹窗登记」。作用域仍为 ask（定案二）。
type AccountTrust struct {
	Nonce    string `json:"nonce"`
	ClientID string `json:"client_id"`
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

// BlockedEntry 被拉黑设备（「拒绝且不再询问」的持久记录）：
// 后续握手一律静默拒绝（不弹窗），直到主人在设备列表解除。
type BlockedEntry struct {
	DeviceID  string    `json:"device_id"`
	Name      string    `json:"name"`
	Platform  string    `json:"platform"`
	BlockedAt time.Time `json:"blocked_at"`
}

// HandshakeResult device.handshake 响应体：
// paired=已完成接入验证；pending=等待桌面端批准（轮询重试）；
// denied=已被拒绝（TTL 内停止重试）；closed=设备接入未开启。
type HandshakeResult struct {
	Status    string `json:"status"`
	Token     string `json:"token,omitempty"`      // 批准后首次轮询下发
	RequestID string `json:"request_id,omitempty"` // pending 时供日志排查
	Message   string `json:"message,omitempty"`    // 拒绝原因（人话，供连接方界面提示）
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
	// Disconnect 主动断开指定客户端的存活连接（可选）。
	// 吊销设备/关闭总闸时回调，保证吊销对在线设备即时生效（吊销即断连）；
	// 回调实现方必须可重入（内部会触发 Unbind），管理器在锁外调用。
	Disconnect func(clientID string)
}

// Manager 设备接入闸管理器：互斥锁保护状态，无后台循环
// （外部设备轮询 device.handshake 推进配对，无需常驻 goroutine）。
type Manager struct {
	cfg    Config
	logger logging.Logger

	mu       sync.Mutex
	allowed  bool
	devices  map[string]Device       // 已授权设备表（持久化）
	blocked  map[string]BlockedEntry // 拉黑名单（持久化；命中即静默拒绝）
	requests map[string]*PairRequest // 接入请求表
	sessions map[string]string       // 已验证连接：clientID → deviceID
	pairCode *pairCodeState          // 活跃配对码（单码：新码生成即替换旧码）
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
		blocked:  map[string]BlockedEntry{},
		requests: map[string]*PairRequest{},
		sessions: map[string]string{},
	}
	if v, err := cfg.Store.Get(keyAllow); err == nil {
		mgr.allowed = v == "1"
	}
	if v, err := cfg.Store.Get(keyDevices); err == nil && v != "" {
		var devices map[string]Device
		if err := json.Unmarshal([]byte(v), &devices); err == nil {
			// 旧持久化记录无作用域字段，归一为远程设备默认级（ask）
			for id, dev := range devices {
				if dev.Scope == "" {
					dev.Scope = ScopeAsk
					devices[id] = dev
				}
				if dev.TrustSource == "" {
					dev.TrustSource = TrustPopup
					devices[id] = dev
				}
			}
			mgr.devices = devices
		} else {
			mgr.logger.Warn("devgate: 已授权设备表损坏，忽略", "error", err.Error())
		}
	}
	if v, err := cfg.Store.Get(keyBlocked); err == nil && v != "" {
		var blocked map[string]BlockedEntry
		if err := json.Unmarshal([]byte(v), &blocked); err == nil {
			mgr.blocked = blocked
		} else {
			mgr.logger.Warn("devgate: 拉黑名单损坏，忽略", "error", err.Error())
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

// SetAllowed 设置总开关并持久化。关闭时清除全部非本机连接的验证登记并
// 主动断开其存活连接，已接入设备立即失去调用权（重新开启后需凭 token
// 重新握手）。
func (m *Manager) SetAllowed(allowed bool) error {
	var kicked []string
	m.mu.Lock()
	m.allowed = allowed
	if !allowed {
		for clientID, deviceID := range m.sessions {
			if deviceID != localDeviceID {
				delete(m.sessions, clientID)
				kicked = append(kicked, clientID)
			}
		}
	}
	err := m.persistLocked()
	m.mu.Unlock()
	for _, clientID := range kicked {
		m.disconnect(clientID)
	}
	if err == nil {
		m.notifyDevicesChanged()
	}
	return err
}

// Handshake 处理设备接入握手（RPC 注册层唯一对外开口）：
// 本机令牌 → 直通；已授权 token → 免配对；同账号互信凭据 → 免弹窗登记；
// 配对码 → 一次性消费视同批准；待批准请求 → 轮询推进；其余 → 登记请求并
// 广播桌面端弹窗。
func (m *Manager) Handshake(clientID, deviceID, name, platform, token, pairCode string, trust *AccountTrust) HandshakeResult {
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

	// 拉黑名单：命中即静默拒绝（不弹窗、不建请求，防反复骚扰）
	if _, hit := m.blocked[deviceID]; hit {
		return HandshakeResult{Status: "denied", Message: "对方已拒绝此设备的连接"}
	}

	// 已授权设备：token 匹配直接放行（重连免配对；常量时间比对防猜测）
	if token != "" {
		if dev, ok := m.devices[deviceID]; ok && subtle.ConstantTimeCompare([]byte(dev.Token), []byte(token)) == 1 {
			m.sessions[clientID] = deviceID
			return HandshakeResult{Status: "paired"}
		}
	}

	// 同账号互信（跨端连接第二步，定案一）：凭据已由握手层向社区服务端
	// 核验通过（同账号 + 设备身份一致），免弹窗直接登记签发长期令牌。
	// 作用域仍为 ask（定案二：远程设备永不开放本机级全权）。
	if trust != nil {
		dev := Device{
			ID:          deviceID,
			Name:        name,
			Platform:    platform,
			Token:       newToken(),
			Scope:       ScopeAsk,
			TrustSource: TrustAccount,
			ApprovedAt:  time.Now(),
		}
		m.devices[deviceID] = dev
		if err := m.persistLocked(); err != nil {
			return HandshakeResult{Status: "denied", Message: "保存设备记录失败"}
		}
		m.sessions[clientID] = deviceID
		m.notifyDevicesChanged()
		return HandshakeResult{Status: "paired", Token: dev.Token}
	}

	// 配对码路径（headless 主机与云服务器首连）：码有效即视同批准，
	// 一次性消费并按弹窗批准同款逻辑签发长期 token
	if pairCode != "" {
		dev, err := m.consumePairCodeLocked(pairCode, deviceID, name, platform)
		if err != nil {
			return HandshakeResult{Status: "denied", Message: err.Error()}
		}
		m.sessions[clientID] = deviceID
		return HandshakeResult{Status: "paired", Token: dev.Token}
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
		ID:          req.DeviceID,
		Name:        req.Name,
		Platform:    req.Platform,
		Token:       newToken(),
		Scope:       ScopeAsk,
		TrustSource: TrustPopup,
		ApprovedAt:  time.Now(),
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

// DenyAndBlock 拒绝并拉黑：请求置 denied，同时把该设备写入持久拉黑名单——
// 此后其握手一律静默拒绝（不弹窗），直到主人解除。
func (m *Manager) DenyAndBlock(requestID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	req, ok := m.requests[requestID]
	if !ok || req.status != reqPending {
		return fmt.Errorf("接入请求不存在或已处理")
	}
	req.status = reqDenied
	req.decidedAt = time.Now()
	m.blocked[req.DeviceID] = BlockedEntry{
		DeviceID:  req.DeviceID,
		Name:      req.Name,
		Platform:  req.Platform,
		BlockedAt: time.Now(),
	}
	if err := m.persistLocked(); err != nil {
		return err
	}
	m.notifyDevicesChanged()
	return nil
}

// Unblock 解除设备拉黑：该设备恢复「申请 → 弹窗审批」的正常流程。
func (m *Manager) Unblock(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.blocked[deviceID]; !ok {
		return fmt.Errorf("该设备不在已拒绝列表中")
	}
	delete(m.blocked, deviceID)
	if err := m.persistLocked(); err != nil {
		return err
	}
	m.notifyDevicesChanged()
	return nil
}

// Revoke 撤销设备的接入授权：删除设备记录并清除其已验证连接的登记，
// 该设备的后续调用立即被强制闸拒绝；同时主动断开其全部存活连接
// （吊销即断连——连接级认证下不断连则在线设备感知不到吊销）。
func (m *Manager) Revoke(deviceID string) error {
	var kicked []string
	m.mu.Lock()
	if _, ok := m.devices[deviceID]; !ok {
		m.mu.Unlock()
		return fmt.Errorf("该设备不在已授权列表中")
	}
	delete(m.devices, deviceID)
	for clientID, id := range m.sessions {
		if id == deviceID {
			delete(m.sessions, clientID)
			kicked = append(kicked, clientID)
		}
	}
	if err := m.persistLocked(); err != nil {
		m.mu.Unlock()
		return err
	}
	m.mu.Unlock()
	for _, clientID := range kicked {
		m.disconnect(clientID)
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

// LocalVerified 查询连接是否凭本机信任令牌直通（区别于外接设备）。
// 本机专属凭据（如 daemon_token）只下发给这类连接——外接设备一律使用
// 各自的设备令牌（可撤销、绑设备），防止配对成功的设备换取不可撤销的
// 本机直通身份。
func (m *Manager) LocalVerified(clientID string) bool {
	if clientID == "" {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[clientID] == localDeviceID
}

// ScopeOf 查询连接的作用域（RPC 注册层作用域闸用）：
// ScopeLocal=本机直通（全权）；ScopeAsk=远程设备（工作面）；""=未验证连接。
func (m *Manager) ScopeOf(clientID string) string {
	if clientID == "" {
		return ""
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	deviceID, ok := m.sessions[clientID]
	if !ok {
		return ""
	}
	if deviceID == localDeviceID {
		return ScopeLocal
	}
	if dev, ok := m.devices[deviceID]; ok {
		return dev.Scope
	}
	return ""
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
			"id":           dev.ID,
			"name":         dev.Name,
			"platform":     dev.Platform,
			"scope":        dev.Scope,
			"trust_source": dev.TrustSource,
			"approved_at":  dev.ApprovedAt,
		})
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i]["id"].(string) < devices[j]["id"].(string) })
	resp["devices"] = devices

	// 拉黑名单（「已拒绝」分组数据源；不含 token 等敏感字段）
	blockedList := make([]map[string]any, 0, len(m.blocked))
	for _, b := range m.blocked {
		blockedList = append(blockedList, map[string]any{
			"device_id":  b.DeviceID,
			"name":       b.Name,
			"platform":   b.Platform,
			"blocked_at": b.BlockedAt,
		})
	}
	sort.Slice(blockedList, func(i, j int) bool {
		return blockedList[i]["device_id"].(string) < blockedList[j]["device_id"].(string)
	})
	resp["blocked"] = blockedList

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

// ---------------------------------------------------------------------------
// 配对码（短码）：headless 主机与云服务器的首连入口
//
// 数据流：主机上 mindx pair → GeneratePairCode 打印 6 位码 → 连接方在
// 「连接主机」里填「地址 + 配对码」→ 握手层放行（networkAuthenticator 查
// ValidPairCode）→ device.handshake 携带 pair_code → 消费并视同批准签发
// 设备令牌（与弹窗批准同款），此后重连凭令牌免配对。
// ---------------------------------------------------------------------------

// GeneratePairCode 生成新的 6 位配对码（单活跃码：新码生成即替换旧码），
// 返回码与过期时刻。
func (m *Manager) GeneratePairCode() (string, time.Time, error) {
	code, err := newPairCode6()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("生成配对码失败: %w", err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pairCode = &pairCodeState{code: code, expiresAt: time.Now().Add(pairCodeTTL)}
	return code, m.pairCode.expiresAt, nil
}

// ValidPairCode 配对码是否当前有效（握手层鉴权钩子用）。
// 错误尝试与消费共用同一额度：连错 5 次码即作废，防在线猜测。
func (m *Manager) ValidPairCode(code string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pairCodeUsableLocked(code)
}

// DeviceTokenExists 查询 token 是否为某已授权设备的长期令牌
// （握手层鉴权钩子用：公网来源凭设备令牌放行）。
func (m *Manager) DeviceTokenExists(token string) bool {
	if token == "" {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, dev := range m.devices {
		if subtle.ConstantTimeCompare([]byte(dev.Token), []byte(token)) == 1 {
			return true
		}
	}
	return false
}

// pairCodeUsableLocked 配对码有效性判定（调用方持锁）：
// 不存在/已过期 → false；比对错误计尝试次数（超限作废）；正确 → true。
func (m *Manager) pairCodeUsableLocked(code string) bool {
	pc := m.pairCode
	if pc == nil {
		return false
	}
	if time.Now().After(pc.expiresAt) {
		m.pairCode = nil
		return false
	}
	if subtle.ConstantTimeCompare([]byte(pc.code), []byte(code)) != 1 {
		pc.attempts++
		if pc.attempts >= pairCodeMaxAttempts {
			m.pairCode = nil
		}
		return false
	}
	return true
}

// consumePairCodeLocked 校验并一次性消费配对码，视同批准为该设备签发
// 长期令牌并持久化（调用方持锁；错误文案面向连接方界面，说人话）。
func (m *Manager) consumePairCodeLocked(code, deviceID, name, platform string) (*Device, error) {
	if !m.pairCodeUsableLocked(code) {
		return nil, fmt.Errorf("配对码不正确或已过期，请在主机上重新生成后再试")
	}
	m.pairCode = nil // 一次性消费
	dev := Device{
		ID:          deviceID,
		Name:        name,
		Platform:    platform,
		Token:       newToken(),
		Scope:       ScopeAsk,
		TrustSource: TrustPairCode,
		ApprovedAt:  time.Now(),
	}
	m.devices[deviceID] = dev
	if err := m.persistLocked(); err != nil {
		return nil, fmt.Errorf("保存设备记录失败: %w", err)
	}
	m.notifyDevicesChanged()
	return &dev, nil
}

// newPairCode6 生成 6 位数字配对码（CSPRNG 取模：10^6 空间 × 5 分钟时效
// × 5 次尝试上限，在线猜测成功的概率可忽略）。
func newPairCode6() (string, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", binary.BigEndian.Uint64(buf[:])%1000000), nil
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

// localTokenMatchesLocked 本机令牌比对（常量时间；读取失败视同不匹配）。
func (m *Manager) localTokenMatchesLocked(token string) bool {
	lt, err := m.cfg.LocalToken()
	return err == nil && lt != "" && subtle.ConstantTimeCompare([]byte(lt), []byte(token)) == 1
}

// persistLocked 持久化开关、设备表与拉黑名单（调用方持锁）。
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
	blockedData, err := json.Marshal(m.blocked)
	if err != nil {
		return fmt.Errorf("序列化拉黑名单失败: %w", err)
	}
	if err := m.cfg.Store.Set(keyBlocked, string(blockedData)); err != nil {
		return fmt.Errorf("持久化拉黑名单失败: %w", err)
	}
	return nil
}

// notify 经网关广播通知桌面端（弹窗与列表刷新依赖）。
func (m *Manager) notify(method string, params any) {
	if m.cfg.Notify != nil {
		m.cfg.Notify(method, params)
	}
}

// disconnect 主动断开客户端连接（吊销即断连；锁外调用——回调链会经
// disconnectHandler 回到 Unbind 重入管理器锁）。
func (m *Manager) disconnect(clientID string) {
	if m.cfg.Disconnect != nil {
		m.cfg.Disconnect(clientID)
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
