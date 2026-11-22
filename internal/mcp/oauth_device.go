package mcp

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/DotNetAge/mindx/internal/core"
)

// 连接器自动授权（第一层通道）：能自动拿到的凭据，不让用户去复制粘贴。
//
// 流程本身不认识任何具体服务商：端点、client_id、scope、取账号名的字段全部来自
// client-provider.yml 的 kind: device_code 条目（见 oauth_providers.go）。这里只实现
// RFC 8628 的两步（申请短码 / 轮询换 token），因此新增一家服务商不需要改这份代码。
//
// 选设备码而不是授权码 + 本地回调，是因为它不需要回调地址、不需要起端口，桌面端与
// 容器里都能跑（容器只要能出网）。client_secret 一律不参与：设备码是公共客户端流程。

// DeviceFlowParams mcp.oauth.device.start 的入参。
type DeviceFlowParams struct {
	// OAuthProvider 授权服务商名（client-provider.yml 里的 name），如 github
	OAuthProvider string `json:"oauth_provider"`
	// Provider 是旧字段（历史界面传 provider），保留兼容；与新字段同时出现时以新字段为准
	Provider string   `json:"provider,omitempty"`
	Group    string   `json:"group"`   // 写进凭据库的组名
	Field    string   `json:"field"`   // 组内字段名（如 GITHUB_PERSONAL_ACCESS_TOKEN）
	Servers  []string `json:"servers"` // 要打上组标记的连接器
	Display  string   `json:"display"` // 面板展示名
	Scope    string   `json:"scope"`   // 覆盖配置里的 scopes（一般不用）
}

// providerName 取服务商名，兼容旧字段。
func (p DeviceFlowParams) providerName() string {
	if v := strings.TrimSpace(p.OAuthProvider); v != "" {
		return v
	}
	return strings.TrimSpace(p.Provider)
}

// DeviceFlowSession 是发起授权后留在 daemon 内存里的一次会话（不落盘）。
type DeviceFlowSession struct {
	mu            sync.Mutex // 保护 interval：轮询会并发改它
	oauthProvider string
	group         string
	field         string
	display       string
	servers       []string
	deviceCode    string
	interval      time.Duration
	expiresAt     time.Time
}

// getInterval 读取当前轮询间隔（slow_down 会被服务端调大）。
func (s *DeviceFlowSession) getInterval() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.interval
}

// setInterval 更新轮询间隔。
func (s *DeviceFlowSession) setInterval(d time.Duration) {
	s.mu.Lock()
	s.interval = d
	s.mu.Unlock()
}

type deviceFlowState struct {
	mu       sync.Mutex
	sessions map[string]*DeviceFlowSession
}

// deviceStartResult 返回给界面：用户只需看到短码和地址。
type deviceStartResult struct {
	Session         string `json:"session"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type devicePollResult struct {
	Status   string         `json:"status"` // ok | pending | slow_down | expired | error
	Interval int            `json:"interval,omitempty"`
	Login    string         `json:"login,omitempty"`
	Error    string         `json:"error,omitempty"`
	Detail   string         `json:"detail,omitempty"`
	Applied  *deviceApplied `json:"applied,omitempty"`
}

type deviceApplied struct {
	Group   string   `json:"group"`
	Servers []string `json:"servers"`
	Enabled []string `json:"enabled"`
}

// StartDeviceFlow 申请短码。session 只是一次轮询的句柄，不携带任何秘密。
func (m *Manager) StartDeviceFlow(p DeviceFlowParams) (*deviceStartResult, error) {
	name := p.providerName()
	if name == "" {
		return nil, fmt.Errorf("缺少授权服务商（oauth_provider）")
	}
	provider, err := m.deviceProvider(name)
	if err != nil {
		return nil, err
	}
	if p.Group == "" {
		p.Group = provider.Name
	}
	if p.Field == "" {
		p.Field = defaultTokenField(provider.Name)
	}
	if p.Display == "" {
		p.Display = provider.Title + " 授权"
	}
	scope := strings.TrimSpace(p.Scope)
	if scope == "" {
		scope = strings.Join(provider.Scopes, " ")
	}

	form := url.Values{}
	form.Set("client_id", providerClientID(provider))
	if scope != "" {
		form.Set("scope", scope)
	}
	body, err := postForm(provider.DeviceURL, form)
	if err != nil {
		return nil, err
	}

	var resp struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
		Error           string `json:"error"`
		ErrorDesc       string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("授权服务返回看不懂的内容：%w", err)
	}
	if resp.Error != "" {
		if resp.Error == "device_flow_disabled" {
			return nil, fmt.Errorf("这个 %s 授权应用还没开启设备码授权：请到 %s 的应用设置里打开设备码（Device Flow），再回来重试",
				provider.Title, provider.Title)
		}
		return nil, fmt.Errorf("%s：%s", resp.Error, resp.ErrorDesc)
	}
	if resp.DeviceCode == "" {
		return nil, fmt.Errorf("授权服务没有返回设备码")
	}
	if resp.Interval <= 0 {
		resp.Interval = 5
	}

	session := randomToken(16)
	sess := &DeviceFlowSession{
		oauthProvider: provider.Name,
		group:         p.Group,
		field:         p.Field,
		display:       p.Display,
		servers:       p.Servers,
		deviceCode:    resp.DeviceCode,
		interval:      time.Duration(resp.Interval) * time.Second,
		expiresAt:     time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second),
	}
	m.deviceFlow.mu.Lock()
	if m.deviceFlow.sessions == nil {
		m.deviceFlow.sessions = map[string]*DeviceFlowSession{}
	}
	// 顺手清掉超时没人轮询的会话：它们不会再被使用，留着只占内存。
	now := time.Now()
	for id, old := range m.deviceFlow.sessions {
		if now.After(old.expiresAt) {
			delete(m.deviceFlow.sessions, id)
		}
	}
	m.deviceFlow.sessions[session] = sess
	m.deviceFlow.mu.Unlock()

	return &deviceStartResult{
		Session:         session,
		UserCode:        resp.UserCode,
		VerificationURI: resp.VerificationURI,
		ExpiresIn:       resp.ExpiresIn,
		Interval:        int(sess.getInterval().Seconds()),
	}, nil
}

// PollDeviceFlow 轮询一次。成功后把 token 写进凭据库并让同组连接器生效。
func (m *Manager) PollDeviceFlow(session string) (*devicePollResult, error) {
	m.deviceFlow.mu.Lock()
	sess := m.deviceFlow.sessions[session]
	m.deviceFlow.mu.Unlock()
	if sess == nil {
		return nil, fmt.Errorf("这次授权已经结束，请重新发起")
	}
	if time.Now().After(sess.expiresAt) {
		m.forgetDeviceFlow(session)
		return &devicePollResult{Status: "expired", Error: "短码已过期"}, nil
	}
	provider, err := m.deviceProvider(sess.oauthProvider)
	if err != nil {
		m.forgetDeviceFlow(session)
		return nil, err
	}

	form := url.Values{}
	form.Set("client_id", providerClientID(provider))
	form.Set("device_code", sess.deviceCode)
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	body, err := postForm(provider.TokenURL, form)
	if err != nil {
		return nil, err
	}

	var resp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
		Interval    int    `json:"interval"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("授权服务返回看不懂的内容：%w", err)
	}

	if resp.Error != "" {
		switch resp.Error {
		case "authorization_pending":
			return &devicePollResult{Status: "pending"}, nil
		case "slow_down":
			if resp.Interval > 0 {
				sess.setInterval(time.Duration(resp.Interval) * time.Second)
			} else {
				sess.setInterval(sess.getInterval() + 5*time.Second)
			}
			return &devicePollResult{Status: "slow_down", Interval: int(sess.getInterval().Seconds())}, nil
		case "expired_token":
			m.forgetDeviceFlow(session)
			return &devicePollResult{Status: "expired", Error: "短码已过期，请重新发起"}, nil
		case "access_denied":
			m.forgetDeviceFlow(session)
			return &devicePollResult{Status: "error", Error: "你取消了授权"}, nil
		default:
			m.forgetDeviceFlow(session)
			return &devicePollResult{Status: "error", Error: resp.Error + "：" + resp.ErrorDesc}, nil
		}
	}
	if resp.AccessToken == "" {
		return &devicePollResult{Status: "pending"}, nil
	}

	login := providerLogin(provider, resp.AccessToken)
	applied, err := m.applyDeviceToken(sess, resp.AccessToken)
	if err != nil {
		return nil, err
	}
	m.forgetDeviceFlow(session)
	return &devicePollResult{Status: "ok", Login: login, Applied: applied}, nil
}

// applyDeviceToken 落库并让连接器生效，与手动填 Key 走同一条凭据通道。
func (m *Manager) applyDeviceToken(sess *DeviceFlowSession, token string) (*deviceApplied, error) {
	if m.credStore == nil {
		return nil, fmt.Errorf("凭据库不可用")
	}
	key := groupCredentialKey(sess.group, sess.field)
	if err := m.credStore.Set(key, token); err != nil {
		return nil, err
	}
	if err := m.credStore.SetMeta(key, map[string]string{
		"display": sess.display,
		"label":   "授权令牌",
	}); err != nil {
		m.log.Warn("凭据元信息写入失败", "group", sess.group, "err", err)
	}

	servers, err := LoadServers(m.configPath)
	if err != nil {
		return nil, err
	}
	applied := &deviceApplied{Group: sess.group, Servers: []string{}}
	// 逐个生效，中途失败要把已经改过的条目恢复原样：授权是「要么都生效、
	// 要么都不生效」的动作，留一半条目指向不存在的凭据组会让连接器直接坏掉。
	var rollback []ServerConfig
	restore := func() {
		for i := len(rollback) - 1; i >= 0; i-- {
			prev := rollback[i]
			if rbErr := m.AddServer(context.Background(), prev); rbErr != nil {
				m.log.Warn("授权回滚失败，连接器配置可能只改了一半", "server", prev.Name, "err", rbErr)
			}
		}
	}
	for i := range servers {
		s := &servers[i]
		if !containsName(sess.servers, s.Name) {
			continue
		}
		before := *s
		s.CredentialGroup = sess.group
		if s.Env != nil {
			if _, ok := s.Env[sess.field]; ok {
				s.Env[sess.field] = ""
			}
		}
		if s.Headers != nil {
			if _, ok := s.Headers[sess.field]; ok {
				s.Headers[sess.field] = ""
			}
		}
		// 授权完成后服务端默认可用；原先关闭的条目要显式打开，否则加了凭据也不会生效。
		if !s.Enabled {
			s.Enabled = true
		}
		if err := m.AddServer(context.Background(), *s); err != nil {
			restore()
			return nil, err
		}
		rollback = append(rollback, before)
		m.pool.Disconnect(s.Name)
		applied.Servers = append(applied.Servers, s.Name)
		if s.Enabled {
			applied.Enabled = append(applied.Enabled, s.Name)
		}
	}
	return applied, nil
}

func (m *Manager) forgetDeviceFlow(session string) {
	m.deviceFlow.mu.Lock()
	delete(m.deviceFlow.sessions, session)
	m.deviceFlow.mu.Unlock()
}

// providerLogin 取账号名用于提示「已授权给 @xxx」；配置里没有 userinfo 端点或请求失败都不影响授权结果。
func providerLogin(p *core.OAuthProviderConfig, token string) string {
	if p.UserInfoURL == "" {
		return ""
	}
	req, err := http.NewRequest(http.MethodGet, p.UserInfoURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var decoded map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return ""
	}
	field := p.UserNameField
	if field == "" {
		field = "login"
	}
	if v, ok := decoded[field].(string); ok {
		return v
	}
	return ""
}

func postForm(endpoint string, form url.Values) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader([]byte(form.Encode())))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("连不上授权服务：%w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	return body, nil
}

func containsName(list []string, name string) bool {
	for _, v := range list {
		if v == name {
			return true
		}
	}
	return false
}

// defaultTokenField 是连接器没声明字段名时的兜底键名。
func defaultTokenField(provider string) string {
	if provider == "github" {
		return "GITHUB_PERSONAL_ACCESS_TOKEN"
	}
	return strings.ToUpper(strings.ReplaceAll(provider, "-", "_")) + "_ACCESS_TOKEN"
}

// randomToken 生成会话句柄（只需不可猜，不承载任何秘密）。
func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("sess-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func envLookup(key string) string {
	return os.Getenv(key)
}
