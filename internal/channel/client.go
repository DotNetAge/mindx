// Package channel 负责与 AgentHub 中继服务的连接、配对、同意闸与网关桥接。
// 协议字段与服务端 agent-hub（原 channels 项目）internal/wss/protocol.go 严格对齐：
// 文本帧为 JSON 控制消息，二进制帧为配对后的透传数据。
package channel

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// 服务端消息类型
const (
	msgHello       = "hello"        // 连接欢迎
	msgCode        = "code"         // 下发短码
	msgPaired      = "paired"       // 配对成功（重连场景不含证书字段）
	msgWaitingPeer = "waiting_peer" // 证书有效，等待对端上线
	msgPeerOffline = "peer_offline" // 对端已断开
	msgRevoked     = "revoked"      // 通道已被吊销
	msgError       = "error"        // 错误
)

// 客户端消息类型
const (
	msgSubmitCode = "submit_code" // 提交短码（桌面端不使用）
	msgAuthCert   = "auth_cert"   // 携带设备证书重连鉴权
)

// 服务端错误码（仅客户端需要识别的子集）
const (
	errPairExists  = "pair_exists"  // 个人版已存在有效配对
	errCertRevoked = "cert_revoked" // 证书已被吊销
)

// 保活与传输参数，与服务端默认配置（PongWait=60s）匹配
const (
	writeWait      = 10 * time.Second // 单次写超时
	pongWait       = 60 * time.Second // 读超时（依赖 pong/消息续期）
	pingPeriod     = 50 * time.Second // 客户端 ping 周期（须小于 pongWait）
	sendBufSize    = 256              // 发送队列长度（与服务端默认一致）
	maxMessageSize = int64(4 << 20)   // 单条消息上限 4MB（与服务端一致）
)

// clientMsg 客户端控制消息（JSON 文本帧）
type clientMsg struct {
	Type string `json:"type"`
	Code string `json:"code,omitempty"`
	Cert string `json:"cert,omitempty"`
}

// serverMsg 服务端控制消息（JSON 文本帧），字段按消息类型选用
type serverMsg struct {
	Type string `json:"type"`

	// hello
	ConnID string `json:"conn_id,omitempty"`
	Mode   string `json:"mode,omitempty"`

	// code
	Code      string `json:"code,omitempty"`
	ExpiresIn int    `json:"expires_in,omitempty"`

	// paired
	GroupID    string `json:"group_id,omitempty"`
	DeviceID   string `json:"device_id,omitempty"`
	DeviceCert string `json:"device_cert,omitempty"`
	DeviceKey  string `json:"device_key,omitempty"`

	// error / 通用
	ErrCode string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

// outMsg 发送队列元素：控制消息为文本帧，桥接数据为二进制帧
type outMsg struct {
	typ  int
	data []byte
}

// conn 与中继的一条 WebSocket 连接封装：发送队列 + 读写泵。
// 生命周期由 Manager 驱动：close 触发读循环退出，Manager 据此进入重连流程。
type conn struct {
	ws   *websocket.Conn
	send chan outMsg

	closed    chan struct{} // 读循环退出信号（close 触发）
	closeOnce sync.Once

	onText   func(data []byte) // 控制消息回调（Manager 分发）
	onBinary func(data []byte) // 透传数据回调（桥接转发）
	onClose  func()            // 连接断开回调（Manager 清理状态）
}

// newConn 建立连接封装并启动读写泵；三个回调由 Manager 注入。
func newConn(ws *websocket.Conn, onText, onBinary func([]byte), onClose func()) *conn {
	c := &conn{
		ws:       ws,
		send:     make(chan outMsg, sendBufSize),
		closed:   make(chan struct{}),
		onText:   onText,
		onBinary: onBinary,
		onClose:  onClose,
	}
	ws.SetReadLimit(maxMessageSize)
	go c.readPump()
	go c.writePump()
	return c
}

// enqueue 非阻塞入队；队列满或连接已关闭返回错误（慢速对端直接丢弃，与服务端语义一致）。
func (c *conn) enqueue(typ int, data []byte) error {
	select {
	case <-c.closed:
		return errors.New("连接已关闭")
	default:
	}
	select {
	case c.send <- outMsg{typ: typ, data: data}:
		return nil
	default:
		return errors.New("发送队列已满")
	}
}

// sendText 序列化并发送控制消息。
func (c *conn) sendText(m clientMsg) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return c.enqueue(websocket.TextMessage, data)
}

// sendBinary 发送透传二进制帧。
func (c *conn) sendBinary(data []byte) error {
	return c.enqueue(websocket.BinaryMessage, data)
}

// close 幂等关闭：仅关闭发送队列，写泵排空后下发关闭帧并断开底层连接，
// 读循环随底层连接关闭而退出并触发 onClose。
func (c *conn) close() {
	c.closeOnce.Do(func() {
		close(c.send)
	})
}

// closedCh 暴露读循环退出信号。
func (c *conn) closedCh() <-chan struct{} { return c.closed }

// readPump 读泵：文本帧回调 onText，二进制帧回调 onBinary，退出时统一收尾。
func (c *conn) readPump() {
	defer func() {
		close(c.closed)
		c.onClose()
	}()

	_ = c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		return c.ws.SetReadDeadline(time.Now().Add(pongWait))
	})
	// 显式回 pong 并续期读超时（与写泵并发安全）
	c.ws.SetPingHandler(func(appData string) error {
		_ = c.ws.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(writeWait))
		return c.ws.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		msgType, data, err := c.ws.ReadMessage()
		if err != nil {
			return
		}
		switch msgType {
		case websocket.TextMessage:
			c.onText(data)
		case websocket.BinaryMessage:
			c.onBinary(data)
		}
	}
}

// writePump 写泵：唯一写出口；定期 ping 保活；队列关闭后排空剩余消息、
// 下发关闭帧，退出时关闭底层连接以解除读泵阻塞。
func (c *conn) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.ws.Close()
	}()

	for {
		select {
		case m, ok := <-c.send:
			if !ok {
				// 队列已关闭且排空：下发关闭帧后退出
				_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
				_ = c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(m.typ, m.data); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
