// Package discovery 提供 daemon 的局域网服务发现能力（mDNS/Bonjour 广播）。
//
// iOS App 通过 NetServiceBrowser 搜索 "_mindx._tcp" 服务即可自动发现本机
// daemon 的 WebSocket 端口，实现同一 WiFi 下零配置直连。
// 设计依据：researching/mindx-remote-example/ios-remote-control-design.md 第 5.7 节。
package discovery

import (
	"fmt"
	"os"

	"github.com/grandcat/zeroconf"
)

// 服务类型与 TXT 版本号，与 iOS 端 MDNSDiscovery 的搜索约定保持一致。
const (
	ServiceType = "_mindx._tcp"
	Domain      = "local."
	TextVersion = "v=1"
)

// Broadcaster 管理 daemon 的 mDNS 服务广播生命周期（Register/Shutdown 成对）。
type Broadcaster struct {
	server *zeroconf.Server
}

// Start 注册 mDNS 服务广播并立即返回。
//
// instance 为空时自动取 "MindX@<主机名>"，便于局域网内多台 Mac 同时运行
// daemon 时在 iOS 端可辨识（zeroconf 会在实例名冲突时自动追加序号）。
// 注册失败返回 error，由调用方决定是否降级（广播失败不应阻断 daemon 启动）。
func Start(port int, instance string) (*Broadcaster, error) {
	if port <= 0 {
		return nil, fmt.Errorf("非法的监听端口: %d", port)
	}
	if instance == "" {
		hostname, err := os.Hostname()
		if err != nil || hostname == "" {
			hostname = "unknown-host"
		}
		instance = fmt.Sprintf("MindX@%s", hostname)
	}

	server, err := zeroconf.Register(
		instance,
		ServiceType,
		Domain,
		port,
		[]string{TextVersion},
		nil, // nil 表示在所有可用网络接口上广播
	)
	if err != nil {
		return nil, fmt.Errorf("注册 mDNS 广播失败: %w", err)
	}
	return &Broadcaster{server: server}, nil
}

// Shutdown 注销广播并释放底层资源，重复调用安全。
func (b *Broadcaster) Shutdown() {
	if b == nil || b.server == nil {
		return
	}
	b.server.Shutdown()
	b.server = nil
}
