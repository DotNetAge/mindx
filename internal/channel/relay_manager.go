package channel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/gorilla/websocket"
)

// relayLogger 上行通道日志入口（与 svc 层 logger 对齐）。
type relayLogger interface {
	Info(msg string, kv ...any)
	Warn(msg string, kv ...any)
}

// RelayConfig 账号中继上行注册通道的依赖装配。
type RelayConfig struct {
	// URL 中继 WebSocket 地址（云端 AgentHub）；为空表示未启用
	URL string
	// Logger 日志
	Logger relayLogger
	// GatewayDial 建桥自拨本机网关（凭证由调用方完成握手）；桥接重建时回调。
	GatewayDial func() (*websocket.Conn, error)
}

// RelayManager daemon 上行注册通道：登录账号后凭访问令牌外连 Hub，
// 登记为本账号下的出口端；手机经 Hub 反向寻址到本连接。
// 与旧配对组通道独立：不依赖短码、设备证书与局域网引导。
type RelayManager struct {
	cfg RelayConfig

	cancel context.CancelFunc
	mu     sync.Mutex
	cn     *upConn
	// wake 缓冲信号：Reconnect 唤醒可能在沉睡退避/空轮询的循环。
	wake chan struct{}
}

// NewRelayManager 组装上行注册通道管理器。
func NewRelayManager(cfg RelayConfig) *RelayManager {
	return &RelayManager{cfg: cfg, wake: make(chan struct{}, 1)}
}

// Start 启动后台重连循环；tokenProvider 每次拨号时读取当前登录令牌。
func (r *RelayManager) Start(ctx context.Context, tokenProvider func() string) {
	if r.cfg.URL == "" {
		return
	}
	ctx, r.cancel = context.WithCancel(ctx)
	go r.runLoop(ctx, tokenProvider)
}

// Stop 断开连接并退出循环。
func (r *RelayManager) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	r.mu.Lock()
	if c := r.cn; c != nil {
		c.close()
	}
	r.mu.Unlock()
}

// Reconnect 触发一次重连：登录态变化（拿到新令牌 / 登出清空）后调用。
// 当前连接断开后由 runLoop 按新令牌重新拨号；唤醒可能在沉睡的循环，未运行时忽略。
func (r *RelayManager) Reconnect() {
	r.mu.Lock()
	if c := r.cn; c != nil {
		c.close()
	}
	r.mu.Unlock()
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

// sleepOrWake 等待定时到期、ctx 取消或被 Reconnect 唤醒；返回 false 表示应退出循环。
func (r *RelayManager) sleepOrWake(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	case <-r.wake:
		return true
	}
}

// runLoop 重连主循环：连接失败指数退避；未登录稍后再试。
func (r *RelayManager) runLoop(ctx context.Context, tokenProvider func() string) {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		if tokenProvider() == "" {
			if !r.sleepOrWake(ctx, 10*time.Second) {
				return
			}
			continue
		}
		conn, err := dialRelay(r.cfg.URL, tokenProvider(), deviceIdentity(), r.cfg.Logger, r.cfg.GatewayDial)
		if err != nil {
			r.cfg.Logger.Warn("relay: 上行注册连接失败", "error", err.Error())
			if !r.sleepOrWake(ctx, backoff) {
				return
			}
			backoff = minDur(backoff*2, 30*time.Second)
			continue
		}
		backoff = time.Second
		r.cfg.Logger.Info("relay: 已接入 AgentHub 在线注册表")
		r.setConn(conn)
		<-conn.done
		r.setConn(nil)
		if ctx.Err() != nil {
			return
		}
		if !r.sleepOrWake(ctx, backoff) {
			return
		}
	}
}

// dialRelay 建立中继连接并发送 auth 首帧。
func dialRelay(url, token, device string, log relayLogger, gwDial func() (*websocket.Conn, error)) (*upConn, error) {
	if token == "" {
		return nil, fmt.Errorf("未登录市场账号，不上行中继")
	}
	ws, _, err := websocket.DefaultDialer.Dial(url, http.Header{
		"Authorization": []string{"Bearer " + token},
	})
	if err != nil {
		return nil, err
	}
	c := &upConn{ws: ws, send: make(chan outMsg, sendBufSize), done: make(chan struct{}), log: log, dialGateway: gwDial}
	authMsg := clientMsg{Type: msgAuth, Token: token, Role: RoleDaemon, Device: device}
	data, _ := json.Marshal(authMsg)
	_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
	if err := ws.WriteMessage(websocket.TextMessage, data); err != nil {
		_ = ws.Close()
		return nil, err
	}
	go c.readPump()
	go c.writePump()
	return c, nil
}

type upConn struct {
	ws   *websocket.Conn
	send chan outMsg
	done chan struct{}
	log  relayLogger
	// dialGateway 回调 svc 层自拨本机网关并完成握手，由配置注入。
	dialGateway func() (*websocket.Conn, error)

	bridgeMu sync.Mutex
	gw       *websocket.Conn
	gwDone   chan struct{}
}

func (c *upConn) close() {
	select {
	case <-c.done:
		return
	default:
		close(c.done)
	}
	_ = c.ws.Close()
	c.closeGateway("relay_closed")
}

// writePump 发送队列 → WebSocket；定时 ping 保活。连接异常即关闭。
func (c *upConn) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case msg := <-c.send:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(msg.typ, msg.data); err != nil {
				c.close()
				return
			}
		case <-ticker.C:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteControl(websocket.PingMessage, nil, time.Now()); err != nil {
				c.close()
				return
			}
		case <-c.done:
			return
		}
	}
}

func (c *upConn) closeGateway(reason string) {
	c.bridgeMu.Lock()
	gw := c.gw
	c.gw = nil
	done := c.gwDone
	c.bridgeMu.Unlock()
	if gw != nil {
		_ = gw.Close()
	}
	if done != nil {
		<-done
	}
	_ = reason
}

// sendBinary 网关回帧 → 中继二进制帧（发送队列满即丢弃，与慢端语义一致）。
func (c *upConn) sendBinary(data []byte) {
	select {
	case c.send <- outMsg{typ: websocket.BinaryMessage, data: data}:
	default:
	}
}

// readPump 中继入站帧：二进制业务帧 → 拨号本机网关桥；文本控制帧当前无需处理。
func (c *upConn) readPump() {
	defer c.close()
	c.ws.SetReadLimit(maxMessageSize)
	_ = c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		return c.ws.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		msgType, data, err := c.ws.ReadMessage()
		if err != nil {
			return
		}
		switch msgType {
		case websocket.BinaryMessage:
			c.relayToGateway(data)
		case websocket.TextMessage:
			c.handleControl(data)
		}
	}
}

// handleControl 服务端文本回执：鉴权/握手失败主动断开并记日志，不等读超时；
// 就绪与欢迎帧无需动作。
func (c *upConn) handleControl(data []byte) {
	var ack struct {
		Type    string `json:"type"`
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(data, &ack) != nil {
		return
	}
	switch ack.Type {
	case msgError:
		if c.log != nil {
			c.log.Warn("relay: 服务端拒绝连接", "error", ack.Error, "message", ack.Message)
		}
		c.close()
	case msgReady, msgHello:
		// 鉴权通过 / 连接欢迎，就绪态由 runLoop 观察连接存活即可
	}
}

// relayToGateway 确保网关桥存在后透传一帧。桥断后随下一帧重建。
func (c *upConn) relayToGateway(data []byte) {
	gw, err := c.gateway()
	if err != nil {
		return
	}
	_ = gw.SetWriteDeadline(time.Now().Add(writeWait))
	if err := gw.WriteMessage(websocket.TextMessage, data); err != nil {
		c.closeGateway("gateway_write_failed")
	}
}

// gateway 取得当前桥接；不存在则拨号本机网关并启动回帧泵。
func (c *upConn) gateway() (*websocket.Conn, error) {
	c.bridgeMu.Lock()
	gw := c.gw
	c.bridgeMu.Unlock()
	if gw != nil {
		return gw, nil
	}
	if c.dialGateway == nil {
		return nil, fmt.Errorf("relay: 网关拨号回调未配置")
	}

	newGw, err := c.dialGateway()
	if err != nil {
		return nil, err
	}
	done := make(chan struct{})
	c.bridgeMu.Lock()
	// 并发兜底：已有桥则丢弃新建
	if c.gw != nil {
		c.bridgeMu.Unlock()
		_ = newGw.Close()
		return c.gw, nil
	}
	c.gw = newGw
	c.gwDone = done
	c.bridgeMu.Unlock()

	go c.pumpGatewayToRelay(newGw, done)
	return newGw, nil
}

// pumpGatewayToRelay 网关文本消息 → 中继二进制回帧。
func (c *upConn) pumpGatewayToRelay(gw *websocket.Conn, done chan struct{}) {
	defer close(done)
	for {
		msgType, data, err := gw.ReadMessage()
		if err != nil {
			c.closeGateway("gateway_read_failed")
			return
		}
		if msgType != websocket.TextMessage || len(data) == 0 {
			continue
		}
		select {
		case <-c.done:
			return
		default:
		}
		c.sendBinary(data)
	}
}

func (r *RelayManager) setConn(c *upConn) {
	r.mu.Lock()
	r.cn = c
	r.mu.Unlock()
}

// deviceIdentity 本机设备标识：主机名（与设备列表上报口径一致）。
func deviceIdentity() string {
	name, _ := core.MarketDeviceMeta()
	return name
}
