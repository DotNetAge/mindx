// Package rpc provides a thin JSON-RPC client for MindX daemon communication.
//
// It wraps the WebSocket-based gateway.Client and exposes typed methods for
// each daemon RPC method. This is the primary way CLI commands and external
// tools interact with the running daemon process.
package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/DotNetAge/gort/pkg/gateway"
)

const (
	// DefaultAddr is the default daemon WebSocket endpoint.
	DefaultAddr = "ws://localhost:1314/ws"

	// DefaultTimeout is the default RPC call timeout.
	DefaultTimeout = 30 * time.Second
)

// gwCaller is the minimal interface satisfied by *gateway.Client.
// Extracted for testability — typed RPC methods can be tested with a mock.
type gwCaller interface {
	Call(ctx context.Context, method string, params any) (json.RawMessage, error)
	Close() error
}

// Client is a thin JSON-RPC client for the MindX daemon.
type Client struct {
	gw        gwCaller
	closeOnce sync.Once
}

// Dial connects to the MindX daemon at the given WebSocket address.
// If addr is empty, DefaultAddr is used.
// 连接成功后自动完成本机设备握手（daemon 侧设备接入强制闸要求；
// 本机令牌直通，无弹窗）。握手失败视同连接失败（闸未装配的旧 daemon 除外）。
func Dial(addr string) (*Client, error) {
	if addr == "" {
		addr = DefaultAddr
	}
	c := gateway.NewClient(addr)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		return nil, fmt.Errorf("cannot connect to daemon at %s: %w\nIs the daemon running? (mindx start)", addr, err)
	}
	if err := HandshakeLocal(ctx, c); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("cannot handshake with daemon at %s: %w\nIs the daemon up to date? (mindx upgrade)", addr, err)
	}
	return &Client{gw: c}, nil
}

// LocalToken 读取本机 daemon 握手令牌：唯一落点 ~/.mindx/.token（AGENTS.md
// 数据落点军规：唯一写方是 daemon，CLI 只读不生成，缺失即失败；历史遗留的
// data/daemon_token 由 daemon 启动时单点迁移，CLI 不保留兼容分支）。
func LocalToken() (string, error) {
	path := filepath.Join(defaultUserPrefsDir(), ".token")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("读取本机握手令牌失败: %w（请确认本机 daemon 已启动）", err)
	}
	token := trimSpaceBytes(data)
	if token == "" {
		return "", fmt.Errorf("本机握手令牌为空: %s", path)
	}
	return token, nil
}

// HandshakeLocal 在已建立的连接上完成本机设备握手（设备接入强制闸要求）。
// 本机令牌直通（不受「允许设备接入」开关限制）；令牌缺失或校验失败返回错误。
func HandshakeLocal(ctx context.Context, c gwCaller) error {
	token, err := LocalToken()
	if err != nil {
		return err
	}
	raw, err := c.Call(ctx, "device.handshake", map[string]any{
		"device_id": "local-cli",
		"name":      "本机命令行",
		"platform":  "cli",
		"token":     token,
	})
	if err != nil {
		return fmt.Errorf("本机设备握手失败: %w", err)
	}
	var result struct {
		Status string `json:"status"`
	}
	if json.Unmarshal(raw, &result) != nil || (result.Status != "paired") {
		return fmt.Errorf("本机设备握手未通过（状态: %s）", result.Status)
	}
	return nil
}

// defaultUserPrefsDir 返回用户数据根目录（与 daemon 侧 core.DefaultUserPrefsDir
// 同语义；pkg/ 不得引用 internal/，故在此复刻最小实现）。
func defaultUserPrefsDir() string {
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "mindx")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".mindx")
	}
	return filepath.Join(home, ".mindx")
}

// trimSpaceBytes 去除首尾空白（避免引入 strings 只为一处调用）。
func trimSpaceBytes(b []byte) string {
	s := string(b)
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\n' || s[start] == '\r' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\n' || s[end-1] == '\r' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

// Close shuts down the client connection.
// Safe to call multiple times; subsequent calls are no-ops.
// This works around a race in gort's gateway.Client where readLoop may also
// call Close() after the main goroutine has already closed the done channel.
func (c *Client) Close() error {
	var err error
	c.closeOnce.Do(func() {
		err = c.gw.Close()
	})
	return err
}

// Call invokes a JSON-RPC method and returns the raw result.
func (c *Client) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	return c.gw.Call(ctx, method, params)
}

// CallWithTimeout calls a JSON-RPC method with a default timeout.
func (c *Client) CallWithTimeout(method string, params any) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()
	return c.Call(ctx, method, params)
}
