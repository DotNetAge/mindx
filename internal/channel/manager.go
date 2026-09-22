package channel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/DotNetAge/mindx/pkg/logging"
)

// 通道状态（channel.status 的 state 字段取值）
const (
	StateOffline    = "offline"    // 未配置或连接不可用
	StateConnecting = "connecting" // 正在连接中继
	StateWaiting    = "waiting"    // 已连接，等待手机配对/上线
	StatePaired     = "paired"     // 已与手机配对
	StateError      = "error"      // 服务端拒绝（如个人版配对冲突）
)

// 同意闸请求状态与时效
const (
	reqPending  = "pending"
	reqApproved = "approved"
	reqDenied   = "denied"
	requestTTL  = 5 * time.Minute  // 请求（含拒绝结果）的失效时长
	codeMaxAge  = 50 * time.Second // 引导下发的短码最大龄（低于服务端 60s TTL）
	codeRefresh = 55 * time.Second // 后台重拨刷新短码的龄阈值
)

// 证书存储键（经 CertStore 持久化，断线重连凭证书免配对）
const (
	keyCert  = "channel_device_cert"
	keyKey   = "channel_device_key"
	keyGroup = "channel_device_group"
)

// CertStore 证书存取最小接口（由 core.CredentialStore 满足，保持本包与其解耦）
type CertStore interface {
	Get(key string) (string, error)
	Set(key, value string) error
}

// PairRequest 同意闸配对请求（序列化给桌面端展示）
type PairRequest struct {
	RequestID  string    `json:"request_id"`
	DeviceHint string    `json:"device_hint,omitempty"`
	CreatedAt  time.Time `json:"created_at"`

	status    string    // pending | approved | denied（不序列化）
	decidedAt time.Time // 批准/拒绝时刻，用于 TTL 判定
}

// BootstrapResult channel.bootstrap 返回结果：
// pending=等待桌面端同意；ready=下发中继地址与短码；denied=已被拒绝。
type BootstrapResult struct {
	Status     string `json:"status"`
	ChannelURL string `json:"channel_url,omitempty"`
	Code       string `json:"code,omitempty"`
	ExpiresIn  int    `json:"expires_in,omitempty"`
}

// Config 管理器构造参数
type Config struct {
	// URL 中继服务 WebSocket 地址；为空表示未启用手机连接
	URL string
	// Certs 证书存取（必填）
	Certs CertStore
	// Logger 日志（必填）
	Logger logging.Logger
	// Notify 经网关广播通知桌面端（同意闸弹窗依赖）
	Notify func(method string, params any)
	// GatewayDial 建桥自拨网关（必填）
	GatewayDial func() (*websocket.Conn, error)
}

// Manager 中继连接管理器：负责连接与退避重连、短码刷新、证书持久化、
// 配对同意闸与网关桥接。单 goroutine 重连循环 + 互斥锁保护状态。
type Manager struct {
	cfg    Config
	logger logging.Logger

	mu         sync.Mutex
	url        string                  // 当前生效地址
	cn         *conn                   // 当前中继连接（nil=离线）
	state      string                  // 对外状态
	stateErr   string                  // 状态补充说明（如连接失败原因）
	code       string                  // 当前短码
	codeAt     time.Time               // 短码签发时刻
	estAt      time.Time               // 当前连接建立时刻
	certAuthed bool                    // 本次连接是否已用证书鉴权
	paired     bool                    // 是否已与手机配对
	groupID    string                  // 配对组标识
	requests   map[string]*PairRequest // 同意闸请求表
	br         *bridge                 // 网关桥（nil=未建立）

	urlChanged chan struct{}
	cancel     context.CancelFunc
}

// NewManager 创建管理器，依赖不满足时返回错误而非 panic。
func NewManager(cfg Config) (*Manager, error) {
	if cfg.Logger == nil {
		return nil, fmt.Errorf("channel: logger 不能为空")
	}
	if cfg.Certs == nil {
		return nil, fmt.Errorf("channel: 证书存取器不能为空")
	}
	if cfg.GatewayDial == nil {
		return nil, fmt.Errorf("channel: 网关拨号器不能为空")
	}
	return &Manager{
		cfg:        cfg,
		logger:     cfg.Logger,
		url:        cfg.URL,
		state:      StateOffline,
		requests:   map[string]*PairRequest{},
		urlChanged: make(chan struct{}, 1),
	}, nil
}

// Start 启动重连循环（后台 goroutine），ctx 取消后退出。
func (m *Manager) Start(ctx context.Context) {
	ctx, m.cancel = context.WithCancel(ctx)
	go m.runLoop(ctx)
}

// Stop 停止管理器：断开中继连接与网关桥。
func (m *Manager) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	m.teardownBridge()
	m.mu.Lock()
	c := m.cn
	m.cn = nil
	m.mu.Unlock()
	if c != nil {
		c.close()
	}
}

// OnURLChanged 中继地址变更（user.config 保存后调用）：
// 记录新地址并断开当前连接，重连循环按新地址重拨；清空则进入离线态。
func (m *Manager) OnURLChanged(url string) {
	m.mu.Lock()
	if url == m.url {
		m.mu.Unlock()
		return
	}
	m.url = url
	c := m.cn
	m.mu.Unlock()

	if c != nil {
		c.close()
	}
	select {
	case m.urlChanged <- struct{}{}:
	default:
	}
}

// Status 返回通道状态摘要（channel.status RPC 响应体）。
func (m *Manager) Status() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()

	resp := map[string]any{
		"configured": m.url != "",
		"url":        m.url,
		"connected":  m.cn != nil,
		"state":      m.state,
		"paired":     m.paired,
	}
	if m.stateErr != "" {
		resp["error"] = m.stateErr
	}
	if m.code != "" {
		resp["code"] = m.code
		left := 60 - int(time.Since(m.codeAt).Seconds())
		if left < 0 {
			left = 0
		}
		resp["code_expires_in"] = left
	}
	if m.groupID != "" {
		resp["group_id"] = m.groupID
	}
	// 待审批请求（同刻至多一个活跃引导流程，取最早创建的一个）
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

// Bootstrap 处理手机端引导配对请求（同意闸入口）：
// 未批准 → 登记请求并广播桌面端；已批准 → 下发中继地址与短码；
// 已拒绝（TTL 内）→ 返回 denied 让手机停止重试。
func (m *Manager) Bootstrap(deviceHint string) BootstrapResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 命中既有请求（手机端每 2s 重试引导，按设备提示匹配）
	if req := m.latestRequestByHintLocked(deviceHint); req != nil {
		switch req.status {
		case reqApproved:
			return m.readyResultLocked()
		case reqDenied:
			if time.Since(req.decidedAt) < requestTTL {
				return BootstrapResult{Status: "denied"}
			}
			delete(m.requests, req.RequestID) // 拒绝已过期，走新建流程
		default:
			return BootstrapResult{Status: "pending"}
		}
	}

	// 新建待审批请求并广播桌面端弹窗
	req := &PairRequest{
		RequestID:  newRequestID(),
		DeviceHint: deviceHint,
		CreatedAt:  time.Now(),
		status:     reqPending,
	}
	m.requests[req.RequestID] = req
	if m.cfg.Notify != nil {
		// 广播体走网关信封约定（type/title/data）：桌面端 handler 从 data 取业务字段
		m.cfg.Notify("channel.pair_request", map[string]any{
			"type":  "channel.pair_request",
			"title": "手机连接请求",
			"data": map[string]any{
				"request_id":  req.RequestID,
				"device_hint": req.DeviceHint,
				"created_at":  req.CreatedAt,
			},
		})
	}
	return BootstrapResult{Status: "pending"}
}

// Approve 批准配对请求：下次引导即下发短码。
func (m *Manager) Approve(requestID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	req, ok := m.requests[requestID]
	if !ok || req.status != reqPending {
		return fmt.Errorf("配对请求不存在或已处理")
	}
	req.status = reqApproved
	req.decidedAt = time.Now()
	return nil
}

// Deny 拒绝配对请求：TTL 内手机重试引导将收到 denied。
func (m *Manager) Deny(requestID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	req, ok := m.requests[requestID]
	if !ok || req.status != reqPending {
		return fmt.Errorf("配对请求不存在或已处理")
	}
	req.status = reqDenied
	req.decidedAt = time.Now()
	return nil
}

// ---------------------------------------------------------------------------
// 内部实现
// ---------------------------------------------------------------------------

// runLoop 重连主循环：未配置地址时等待变更信号；连接失败按 1s→30s 指数退避。
func (m *Manager) runLoop(ctx context.Context) {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		url := m.currentURL()
		if url == "" {
			m.setState(StateOffline, "")
			select {
			case <-ctx.Done():
				return
			case <-m.urlChanged:
			}
			continue
		}

		m.setState(StateConnecting, "")
		c, err := m.dial(url)
		if err != nil {
			m.logger.Warn("channel: 中继连接失败", "error", err.Error())
			m.setState(StateOffline, err.Error())
			if !sleepCtx(ctx, backoff) {
				return
			}
			backoff = minDur(backoff*2, 30*time.Second)
			continue
		}

		backoff = time.Second
		m.logger.Info("channel: 中继已连接", "url", url)
		m.serve(ctx, c)
		m.onConnDown()
		if ctx.Err() != nil {
			return
		}
		if !sleepCtx(ctx, backoff) {
			return
		}
	}
}

// dial 建立中继连接；本地存有证书时随即发送 auth_cert 免配对重连。
func (m *Manager) dial(url string) (*conn, error) {
	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}
	c := newConn(ws, m.handleServerText, m.handleRelayBinary, func() {})
	if cert := m.loadCert(); cert != "" {
		m.mu.Lock()
		m.certAuthed = true
		m.mu.Unlock()
		_ = c.sendText(clientMsg{Type: msgAuthCert, Cert: cert})
	}
	return c, nil
}

// serve 阻塞守护当前连接，直到连接断开、停机或地址变更；
// 期间周期巡检：未配对时短码过龄即重拨刷新（服务端短码 60s 一次性，零服务端改动）。
func (m *Manager) serve(ctx context.Context, c *conn) {
	m.mu.Lock()
	m.cn = c
	m.estAt = time.Now()
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		if m.cn == c {
			m.cn = nil
		}
		m.mu.Unlock()
	}()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			c.close()
			return
		case <-m.urlChanged:
			c.close()
			return
		case <-c.closedCh():
			return
		case <-ticker.C:
			m.pruneRequests()
			if m.shouldRefreshCode() {
				m.logger.Info("channel: 短码已过龄，重拨刷新")
				c.close()
				return
			}
		}
	}
}

// shouldRefreshCode 判断是否需要重拨刷新短码：仅未配对时执行，
// 覆盖两种情形——短码将过期（>55s）、证书鉴权态短码已被消费（拿不到可展示短码）。
func (m *Manager) shouldRefreshCode() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.paired {
		return false
	}
	if m.code != "" {
		return time.Since(m.codeAt) >= codeRefresh
	}
	return !m.certAuthed && time.Since(m.estAt) >= 10*time.Second
}

// onConnDown 连接断开后的统一清理：拆桥、清空短码与配对展示态（证书保留供重连）。
func (m *Manager) onConnDown() {
	m.teardownBridge()
	m.mu.Lock()
	m.code = ""
	m.codeAt = time.Time{}
	m.certAuthed = false
	m.paired = false
	m.state = StateConnecting
	m.stateErr = ""
	m.mu.Unlock()
}

// handleServerText 服务端控制消息分发（文本帧）。
func (m *Manager) handleServerText(data []byte) {
	var msg serverMsg
	if err := json.Unmarshal(data, &msg); err != nil || msg.Type == "" {
		m.logger.Warn("channel: 非法控制消息", "size", len(data))
		return
	}
	switch msg.Type {
	case msgHello:
		m.logger.Info("channel: 服务端欢迎", "mode", msg.Mode)
	case msgCode:
		m.mu.Lock()
		m.code = msg.Code
		m.codeAt = time.Now()
		m.setStateLocked(StateWaiting, "")
		m.mu.Unlock()
		m.logger.Info("channel: 收到短码", "code", msg.Code)
	case msgPaired:
		m.onPaired(&msg)
	case msgWaitingPeer:
		m.mu.Lock()
		m.paired = false
		m.setStateLocked(StateWaiting, "")
		m.mu.Unlock()
		m.logger.Info("channel: 证书有效，等待手机上线")
	case msgPeerOffline:
		m.teardownBridge()
		m.mu.Lock()
		m.paired = false
		m.setStateLocked(StateWaiting, "")
		m.mu.Unlock()
		m.logger.Info("channel: 手机已断开")
	case msgRevoked:
		m.onRevoked("通道已吊销：" + msg.Message)
	case msgError:
		m.onServerError(&msg)
	}
}

// onPaired 配对成功：记录身份、持久化证书（首次配对随消息下发）、建立网关桥。
// 重连场景 paired 消息不含证书字段，本地已有证书无需重写。
func (m *Manager) onPaired(msg *serverMsg) {
	m.mu.Lock()
	m.paired = true
	m.groupID = msg.GroupID
	m.setStateLocked(StatePaired, "")
	m.requests = map[string]*PairRequest{} // 配对完成，清空同意闸请求
	m.mu.Unlock()

	if msg.DeviceCert != "" && msg.DeviceKey != "" {
		m.saveCerts(msg.DeviceCert, msg.DeviceKey, msg.GroupID)
	}
	m.logger.Info("channel: 已与手机配对", "group_id", msg.GroupID)
	m.startBridge()
}

// onServerError 服务端错误处理。
func (m *Manager) onServerError(msg *serverMsg) {
	switch msg.ErrCode {
	case errCertRevoked:
		// 证书失效：清空本地证书与配对态，重连后走全新短码配对（重新引导）
		m.onRevoked("证书已吊销：" + msg.Message)
	case errPairExists:
		// 个人版已有活跃配对（如旧手机仍在线）：断开后退避重试
		m.setState(StateError, "个人版已存在配对通道："+msg.Message)
		m.mu.Lock()
		c := m.cn
		m.mu.Unlock()
		if c != nil {
			c.close()
		}
	default:
		m.logger.Warn("channel: 服务端错误", "code", msg.ErrCode, "message", msg.Message)
	}
}

// onRevoked 吊销处理：拆桥、清空本地证书与配对态；连接随后被服务端关闭，
// 重连后重新下发短码，手机需重新走同意闸引导。
func (m *Manager) onRevoked(reason string) {
	m.logger.Warn("channel: " + reason)
	m.teardownBridge()
	m.clearCerts()
	m.mu.Lock()
	m.paired = false
	m.groupID = ""
	m.mu.Unlock()
}

// handleRelayBinary 中继二进制帧 → 网关（桥接转发；未建桥时丢弃）。
func (m *Manager) handleRelayBinary(data []byte) {
	m.mu.Lock()
	b := m.br
	m.mu.Unlock()
	if b != nil {
		b.toGateway(data)
	}
}

// readyResultLocked 构造就绪结果：携带当前短码；短码过龄时先重拨再返回 pending，
// 保证手机拿到的短码至少还有约 10 秒有效期。
func (m *Manager) readyResultLocked() BootstrapResult {
	if m.code == "" {
		return BootstrapResult{Status: "pending"} // 中继暂无可用短码，手机稍后重试
	}
	left := 60 - int(time.Since(m.codeAt).Seconds())
	if time.Since(m.codeAt) >= codeMaxAge {
		if c := m.cn; c != nil {
			c.close() // 触发重拨取新码
		}
		return BootstrapResult{Status: "pending"}
	}
	return BootstrapResult{
		Status:     "ready",
		ChannelURL: m.url,
		Code:       m.code,
		ExpiresIn:  left,
	}
}

// latestRequestByHintLocked 按设备提示取最近一次请求（无匹配返回 nil）。
func (m *Manager) latestRequestByHintLocked(deviceHint string) *PairRequest {
	var latest *PairRequest
	for _, req := range m.requests {
		if req.DeviceHint != deviceHint {
			continue
		}
		if latest == nil || req.CreatedAt.After(latest.CreatedAt) {
			latest = req
		}
	}
	return latest
}

// pruneRequests 清理过期请求：pending 超时失效（桌面端弹窗随之消失）、denied 过 TTL 删除。
func (m *Manager) pruneRequests() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for id, req := range m.requests {
		if req.status == reqPending && now.Sub(req.CreatedAt) >= requestTTL {
			delete(m.requests, id)
			continue
		}
		if req.status == reqDenied && now.Sub(req.decidedAt) >= requestTTL {
			delete(m.requests, id)
		}
	}
}

// setState 状态写入（加锁包装）。
func (m *Manager) setState(state, stateErr string) {
	m.mu.Lock()
	m.setStateLocked(state, stateErr)
	m.mu.Unlock()
}

// setStateLocked 状态写入（调用方须持锁）。
func (m *Manager) setStateLocked(state, stateErr string) {
	m.state = state
	m.stateErr = stateErr
}

// currentURL 读取当前地址。
func (m *Manager) currentURL() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.url
}

// currentConn 读取当前连接快照。
func (m *Manager) currentConn() *conn {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cn
}

// closeRelayConn 主动断开当前中继连接（幂等）。
func (m *Manager) closeRelayConn(reason string) {
	if c := m.currentConn(); c != nil {
		m.logger.Info("channel: 断开中继连接", "reason", reason)
		c.close()
	}
}

// saveCerts 持久化设备证书、私钥与配对组。
func (m *Manager) saveCerts(certPEM, keyPEM, groupID string) {
	for _, kv := range [][2]string{
		{keyCert, certPEM},
		{keyKey, keyPEM},
		{keyGroup, groupID},
	} {
		if err := m.cfg.Certs.Set(kv[0], kv[1]); err != nil {
			m.logger.Warn("channel: 证书持久化失败", "key", kv[0], "error", err.Error())
		}
	}
}

// loadCert 读取本地设备证书（无则返回空串）。
func (m *Manager) loadCert() string {
	cert, err := m.cfg.Certs.Get(keyCert)
	if err != nil || cert == "" {
		return ""
	}
	return cert
}

// clearCerts 清空本地证书与配对组（吊销/重装恢复路径）。
func (m *Manager) clearCerts() {
	for _, key := range []string{keyCert, keyKey, keyGroup} {
		if err := m.cfg.Certs.Set(key, ""); err != nil {
			m.logger.Warn("channel: 证书清理失败", "key", key, "error", err.Error())
		}
	}
}

// newRequestID 生成 16 位 hex 随机请求标识。
func newRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b) // crypto/rand.Read 不会返回错误
	return hex.EncodeToString(b)
}

// sleepCtx 可中断休眠；ctx 取消返回 false。
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// minDur 取两者较小时长。
func minDur(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
