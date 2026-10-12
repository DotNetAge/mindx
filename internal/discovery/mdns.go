// Package discovery 提供 daemon 的局域网服务发现能力（mDNS/Bonjour 广播与浏览）。
//
// iOS App 通过 NetServiceBrowser 搜索 "_mindx._tcp" 服务即可自动发现本机
// daemon 的 WebSocket 端口，实现同一 WiFi 下零配置直连。
// 设计依据：researching/mindx-remote-example/ios-remote-control-design.md 第 5.7 节。
// Browse 支持桌面端设备列表发现局域网内的其他 MindX 主机（Mac 间互连）。
package discovery

import (
	"context"
	"fmt"
	"os"
	"time"

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

// ServiceEntry 局域网内发现的一个 MindX 主机。
type ServiceEntry struct {
	Instance string // 服务实例名（如 "MindX@Ray-MacBook-Pro"）
	Host     string // 主机 IPv4（空 = 未解析到地址）
	Port     int    // daemon 监听端口
}

// Browse 局域网内搜索 MindX 主机（设备列表「发现的设备」数据源）。
// timeout 到期即返回已收集条目（mDNS 是尽力而为协议，短超时漏报属正常，
// 由调用方提示用户重试）；一条未发现且无错误时返回空切片。
func Browse(timeout time.Duration) ([]ServiceEntry, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("创建 mDNS 浏览器失败: %w", err)
	}
	entriesCh := make(chan *zeroconf.ServiceEntry, 64)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := resolver.Browse(ctx, ServiceType, Domain, entriesCh); err != nil {
		return nil, fmt.Errorf("启动 mDNS 浏览失败: %w", err)
	}

	// 去重（同名实例可能多次通告）并等超时收集
	seen := map[string]bool{}
	entries := make([]ServiceEntry, 0, 8)
	for {
		select {
		case entry := <-entriesCh:
			if entry == nil || seen[entry.Instance] {
				continue
			}
			seen[entry.Instance] = true
			host := ""
			if len(entry.AddrIPv4) > 0 {
				host = entry.AddrIPv4[0].String()
			}
			entries = append(entries, ServiceEntry{
				Instance: entry.Instance,
				Host:     host,
				Port:     entry.Port,
			})
		case <-ctx.Done():
			return entries, nil
		}
	}
}
