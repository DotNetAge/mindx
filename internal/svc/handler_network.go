package svc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/DotNetAge/mindx/internal/core"
)

// ---------------------------------------------------------------------------
// network.public_info / network.token.get —— 公网直连信息采集
//
// iOS App 在内网连接成功后调用这两个方法：
//   ① network.public_info 返回 daemon 当前出口公网 IP 与端口，App 缓存后
//      出门（蜂窝网络）可直接用该地址连回家里；
//   ② network.token.get 下发握手 token，公网直连时以 ?token= 携带。
//
// 安全模型：WebSocket 握手层统一鉴权（见 networkAuthenticator）——
// 私网来源放行（家庭网络与 mDNS 广播同级风险），公网来源必须携带
// 有效 token；因此 token.get 无需再做来源校验（无 token 的公网请求
// 根本无法建立连接）。
// ---------------------------------------------------------------------------

// publicIPProbes 公网出口 IP 检测源，依次兜底（任一成功即返回）。
var publicIPProbes = []string{
	"https://api.ipify.org",
	"https://ifconfig.me/ip",
	"https://icanhazip.com",
}

// daemonTokenPath 返回握手 token 文件路径（~/.mindx/daemon_token）。
func (d *Daemon) daemonTokenPath() string {
	base := d.dataDir
	if base == "" {
		base = core.DefaultUserPrefsDir()
	}
	return filepath.Join(base, "daemon_token")
}

// loadOrCreateDaemonToken 读取（无则生成）32 字节 hex 握手 token。
func (d *Daemon) loadOrCreateDaemonToken() (string, error) {
	path := d.daemonTokenPath()
	if data, err := os.ReadFile(path); err == nil {
		token := strings.TrimSpace(string(data))
		if token != "" {
			return token, nil
		}
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成握手 token 失败: %w", err)
	}
	token := hex.EncodeToString(buf)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("创建配置目录失败: %w", err)
	}
	if err := os.WriteFile(path, []byte(token), 0o600); err != nil {
		return "", fmt.Errorf("写入握手 token 失败: %w", err)
	}
	return token, nil
}

// networkAuthenticator 生成 WebSocket 握手鉴权钩子：
// 私网来源直接放行；公网来源必须携带与本地 token 一致的 ?token= 参数。
//
// TODO 公网直连可行性验证通过后，在 initGateway 中重新启用
// （gateway.WithAuthenticator(d.networkAuthenticator())），当前验证阶段停用。
//
//nolint:unused // 验证阶段临时停用，保留待启用
func (d *Daemon) networkAuthenticator() func(*http.Request) error {
	return func(r *http.Request) error {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		if isPrivateIPHost(host) {
			return nil
		}
		token, err := d.loadOrCreateDaemonToken()
		if err != nil {
			return fmt.Errorf("服务端 token 不可用: %w", err)
		}
		if r.URL.Query().Get("token") != token {
			return fmt.Errorf("缺少或错误的握手 token")
		}
		return nil
	}
}

// isPrivateIPHost 判断主机地址是否属于私网/环回/链路本地。
// IsPrivate 同时覆盖 IPv4 RFC1918 与 IPv6 ULA（fc00::/7）。
//
//nolint:unused // 随 networkAuthenticator 一并临时停用
func isPrivateIPHost(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

// detectPublicIP 依次探测公网出口 IP，任一源成功即返回（容错语义）。
func detectPublicIP() (string, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	var lastErr error
	for _, probe := range publicIPProbes {
		resp, err := client.Get(probe)
		if err != nil {
			lastErr = err
			continue
		}
		body, err := readBodyString(resp, 64)
		if err != nil {
			lastErr = err
			continue
		}
		ip := net.ParseIP(strings.TrimSpace(body))
		if ip == nil || ip.IsPrivate() || ip.IsLoopback() {
			lastErr = fmt.Errorf("%s 返回的不是公网 IP: %q", probe, body)
			continue
		}
		return ip.String(), nil
	}
	if lastErr != nil {
		return "", fmt.Errorf("所有公网 IP 检测源均不可达（最后错误: %v）", lastErr)
	}
	return "", fmt.Errorf("所有公网 IP 检测源均不可达")
}

// readBodyString 读取响应体并限制最大长度，防止异常响应撑爆内存。
func readBodyString(resp *http.Response, maxBytes int64) (string, error) {
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s 返回状态码 %d", resp.Request.URL, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// detectLANIP 返回本机首个非环回 IPv4（用于展示与诊断）。
func detectLANIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipNet.IP.To4()
			if ip4 == nil || ip4.IsLoopback() || ip4.IsLinkLocalUnicast() {
				continue
			}
			return ip4.String()
		}
	}
	return ""
}

// daemonPort 从监听地址（如 ":1314"）解析端口数字。
func (d *Daemon) daemonPort() int {
	port, err := strconv.Atoi(strings.TrimPrefix(d.addr, ":"))
	if err != nil {
		return 0
	}
	return port
}

// handleNetworkPublicInfo 处理 network.public_info：
// 返回 {public_ip, port, lan_ip}；公网 IP 检测失败不阻塞（空串 + 错误语义由调用方感知）。
func (d *Daemon) handleNetworkPublicInfo(_ context.Context, _ json.RawMessage) (any, error) {
	publicIP, err := detectPublicIP()
	if err != nil {
		return nil, fmt.Errorf("公网 IP 检测失败: %w", err)
	}
	return map[string]any{
		"public_ip": publicIP,
		"port":      d.daemonPort(),
		"lan_ip":    detectLANIP(),
	}, nil
}

// handleNetworkTokenGet 处理 network.token.get：下发握手 token（仅内网连接可达）。
func (d *Daemon) handleNetworkTokenGet(_ context.Context, _ json.RawMessage) (any, error) {
	token, err := d.loadOrCreateDaemonToken()
	if err != nil {
		return nil, err
	}
	return map[string]any{"token": token}, nil
}
