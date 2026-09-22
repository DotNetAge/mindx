package channel

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// bridge 中继 ⇄ 网关双向桥：把中继二进制透传帧与网关文本 JSON 消息互转，
// 手机经中继远程访问桌面控制台时如同直连网关的普通客户端。
type bridge struct {
	mgr  *Manager
	gw   *websocket.Conn
	once sync.Once
}

// startBridge 建立网关桥（收到 paired 后调用）；已建桥时幂等返回。
// 拨号失败不放弃：只要仍处于配对态，2 秒后重试。
func (m *Manager) startBridge() {
	m.mu.Lock()
	if m.br != nil {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	gw, err := m.cfg.GatewayDial()
	if err != nil {
		m.logger.Warn("channel: 网关桥建立失败，2 秒后重试", "error", err.Error())
		time.AfterFunc(2*time.Second, func() {
			m.mu.Lock()
			paired := m.paired
			m.mu.Unlock()
			if paired {
				m.startBridge()
			}
		})
		return
	}

	b := &bridge{mgr: m, gw: gw}
	m.mu.Lock()
	if m.br != nil { // 并发竞态兜底：已有桥则丢弃新建
		m.mu.Unlock()
		_ = gw.Close()
		return
	}
	m.br = b
	m.mu.Unlock()

	m.logger.Info("channel: 网关桥已建立")
	go b.pumpGatewayToRelay()
}

// toGateway 中继二进制帧 → 网关（文本 JSON 消息）。
// 写失败说明网关侧已断，触发双侧清理。
func (b *bridge) toGateway(data []byte) {
	_ = b.gw.SetWriteDeadline(time.Now().Add(writeWait))
	if err := b.gw.WriteMessage(websocket.TextMessage, data); err != nil {
		b.close("gateway_write_failed")
	}
}

// pumpGatewayToRelay 网关文本消息 → 中继（二进制透传帧）。
// 网关仅收发文本 JSON 帧；发送队列满时丢弃（与服务端慢端语义一致）。
func (b *bridge) pumpGatewayToRelay() {
	for {
		msgType, data, err := b.gw.ReadMessage()
		if err != nil {
			b.close("gateway_read_failed")
			return
		}
		if msgType != websocket.TextMessage || len(data) == 0 {
			continue
		}
		if c := b.mgr.currentConn(); c != nil {
			_ = c.sendBinary(data)
		}
	}
}

// teardownBridge 拆除当前桥（管理器侧入口，幂等）。
func (m *Manager) teardownBridge() {
	m.mu.Lock()
	b := m.br
	m.mu.Unlock()
	if b != nil {
		b.close("manager_teardown")
	}
}

// clearBridge 清除桥引用（仅当仍指向该桥）。
func (m *Manager) clearBridge(b *bridge) {
	m.mu.Lock()
	if m.br == b {
		m.br = nil
	}
	m.mu.Unlock()
}

// close 双侧清理：关闭网关连接并断开中继连接，由重连循环与
// 重配对事件（auth_cert → paired）自动恢复桥接。幂等。
func (b *bridge) close(reason string) {
	b.once.Do(func() {
		_ = b.gw.Close()
		b.mgr.logger.Info("channel: 网关桥已断开", "reason", reason)
		b.mgr.clearBridge(b)
		b.mgr.closeRelayConn(reason)
	})
}
