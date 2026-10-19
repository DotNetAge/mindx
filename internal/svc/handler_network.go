package svc

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
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

	"github.com/DotNetAge/gort/pkg/gateway"
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

// networkAuthenticator 生成 WebSocket 握手鉴权钩子（initGateway 挂载）：
// 私网/环回来源直接放行（家庭网络与 mDNS 广播同级风险）；公网来源必须
// 凭以下之一放行，否则一律拒绝（陌生公网连接零准入）：
//  1. 对端认证令牌 = 本机握手令牌（本机经公网地址回连）
//  2. 对端认证令牌 = 已授权设备令牌（内网弹窗/短码配对过的设备跨网回连——
//     公网连入必须先在内网建立过信任）
//  3. ?pair_code= 有效配对码（headless 主机与云服务器的首连引导）
//
// 凭证传输（OWASP WebSocket 反模式修正）：Authorization: Bearer 头优先；
// URL ?token= 会泄漏进访问日志/代理日志/Referer，仅为旧客户端保留过渡
// 兼容，客户端全部升级后废除。配对码一次性短码（5 分钟时效、消费即废）
// 暂留 URL，泄漏窗口极小。
func (d *Daemon) networkAuthenticator() func(*http.Request) error {
	return func(r *http.Request) error {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		if isPrivateIPHost(host) {
			return nil
		}
		token := bearerToken(r)
		if token == "" {
			token = r.URL.Query().Get("token")
		}
		return d.authenticateCredentials(token, r.URL.Query().Get("pair_code"))
	}
}

// authenticateCredentials 公网准入凭证三分支校验（升级闸与首条消息认证共用）：
// 本机令牌 / 已授权设备令牌 / 一次性配对码，任一有效即放行。
func (d *Daemon) authenticateCredentials(token, pairCode string) error {
	if token != "" {
		if local, err := d.loadOrCreateDaemonToken(); err == nil && constantTimeEqual(token, local) {
			return nil
		}
		if d.devGate != nil && d.devGate.DeviceTokenExists(token) {
			return nil
		}
	}
	if pairCode != "" && d.devGate != nil && d.devGate.ValidPairCode(pairCode) {
		return nil
	}
	return fmt.Errorf("这台设备还没有建立信任，请先在局域网内连接一次或使用配对码")
}

// firstMessageAuthenticator 首条消息认证钩子（gort gateway 挂载）：
// 浏览器端（壳渲染进程）的 WebSocket 无法自定义升级请求头，公网回连的
// 凭证改由连接后的首条消息携带（device.auth）——这是升级闸的通道内等价物，
// 通过前连接处于待认证态（gort 保证认证前不进入常规消息分发，超时断连）。
// 私网/环回连接在升级闸已放行，不经过本钩子。
func (d *Daemon) firstMessageAuthenticator() func(string, []byte) error {
	return func(_ string, raw []byte) error {
		var req struct {
			Method string `json:"method"`
			Params struct {
				Token    string `json:"token"`
				PairCode string `json:"pair_code"`
			} `json:"params"`
		}
		if err := json.Unmarshal(raw, &req); err != nil || req.Method != "device.auth" {
			return fmt.Errorf("请先完成设备认证再使用其他功能")
		}
		return d.authenticateCredentials(req.Params.Token, req.Params.PairCode)
	}
}

// bearerToken 提取升级请求的 Authorization: Bearer 凭证（scheme 大小写不敏感）。
func bearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(auth) > len(prefix) && strings.EqualFold(auth[:len(prefix)], prefix) {
		return strings.TrimSpace(auth[len(prefix):])
	}
	return ""
}

// constantTimeEqual 常量时间比较（防时序侧信道逐字节猜测凭证）。
// 空值直接判不等：空凭证永不匹配任何本机凭证。
func constantTimeEqual(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// isPrivateIPHost 判断主机地址是否属于私网/环回/链路本地。
// IsPrivate 同时覆盖 IPv4 RFC1918 与 IPv6 ULA（fc00::/7）。
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
	defer func() { _ = resp.Body.Close() }()
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

// handleNetworkTokenGet 处理 network.token.get：下发本机握手 token
// （仅限本机直通连接调用——外接设备即使配对成功也不可得，防止换取
// 不可撤销的本机身份；外接设备跨网回连一律使用各自的设备令牌）。
func (d *Daemon) handleNetworkTokenGet(ctx context.Context, _ json.RawMessage) (any, error) {
	if d.devGate != nil {
		if clientID := gateway.ClientIDFromContext(ctx); !d.devGate.LocalVerified(clientID) {
			return nil, fmt.Errorf("这个凭据只提供给本机使用")
		}
	}
	token, err := d.loadOrCreateDaemonToken()
	if err != nil {
		return nil, err
	}
	return map[string]any{"token": token}, nil
}
