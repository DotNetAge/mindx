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
	"net"
	"os"
	"os/exec"
	"strings"
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
	host := LocalHostName()
	if instance == "" {
		instance = fmt.Sprintf("MindX@%s", host)
	}

	// 显式指定 HostName 与 IP —— 这是「广播出去却连不上」的修复点。
	//
	// zeroconf.Register 在 HostName 留空时用 os.Hostname() 拼成
	// "<uname 名>.local."。macOS 上这不是 Bonjour 主机名
	// （Bonjour 名由 `scutil --get LocalHostName` 给出），于是 SRV 记录
	// 指向一个不存在的主机名：客户端能搜到服务、拿到端口，却解析不出地址，
	// 表现为「局域网发现了设备但连不上」。
	//
	// 正确做法是两件事一起做：
	//   1. HostName 用系统真正的 Bonjour 名；
	//   2. 显式带上本机非回环 IP，保证 A/AAAA 记录有确切内容
	//      （RegisterProxy 不做自动探测，ips 为空则广播里没有地址记录）。
	server, err := zeroconf.RegisterProxy(
		instance,
		ServiceType,
		Domain,
		port,
		host,
		localIPStrings(),
		[]string{TextVersion},
		nil, // nil = 在所有可用组播接口上广播
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

// LocalHostName 返回本机的 Bonjour 主机名（不含 ".local" 后缀）。
//
// macOS 上 `os.Hostname()` 与 Bonjour 主机名是两套东西：
//
//	os.Hostname()            → "iMac"        （uname 名，不可解析）
//	scutil --get LocalHostName → "RaydeiMac"  （Bonjour 名，有 A 记录）
//
// 广播必须用后者，否则客户端拿到的地址解析不出来。非 macOS 平台
// 没有这个区分，直接用 os.Hostname()。
func LocalHostName() string {
	if name := bonjourHostName(); name != "" {
		return name
	}
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return "unknown-host"
	}
	// os.Hostname 偶尔带 ".local" 后缀，去掉以免拼成 "x.local.local"
	return strings.TrimSuffix(hostname, "."+strings.TrimSuffix(Domain, "."))
}

// bonjourHostName 读 macOS 的 Bonjour 主机名；非 macOS 或读取失败返回空串。
func bonjourHostName() string {
	if !isDarwin {
		return ""
	}
	out, err := exec.Command("scutil", "--get", "LocalHostName").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// localIPStrings 收集本机所有非回环、非链路本地的 IP（字符串形式）。
//
// 广而全比精确更重要：客户端可能走 Wi-Fi、有线或 USB 共享网络，
// 只广播一张网卡的地址会让另一种接入方式的用户连不上。
func localIPStrings() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var ips []string
	seen := map[string]bool{}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipnet.IP
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
				continue
			}
			text := ip.String()
			if seen[text] {
				continue
			}
			seen[text] = true
			ips = append(ips, text)
		}
	}
	return ips
}
