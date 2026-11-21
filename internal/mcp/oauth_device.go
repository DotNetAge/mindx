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
)

// 连接器自动授权（第一层通道）：能自动拿到的凭据，不让用户去复制粘贴。
//
// 目前只接 GitHub 的设备码流程：用户在自己的浏览器里输入一段短码并确认，
// daemon 轮询换回 token，直接写进凭据库并打组标记。选设备码是因为它不需要
// 回调地址、不需要本地起端口，桌面端与容器里都能跑（容器只要能出网）。
//
// client_secret 一律不参与：设备码是公共客户端流程，只需要 client_id。

// defaultGitHubClientID 是 MindX 注册的 OAuth App 的公开标识（不是密钥）。
const defaultGitHubClientID = "Ov23likaZ2ZXJzRv8xgW"

// GitHub 授权端点。做成变量是为了测试能指向 httptest，生产值不改。
var (
	githubDeviceCodeURL = "https://github.com/login/device/code"
	githubTokenURL      = "https://github.com/login/oauth/access_token"
	githubUserURL       = "https://api.github.com/user"
)

// DeviceFlowParams mcp.oauth.device.start 的入参。
type DeviceFlowParams struct {
	Provider string   `json:"provider"` // 目前只支持 github
	Group    string   `json:"group"`    // 写进凭据库的组名
	Field    string   `json:"field"`    // 组内字段名（如 GITHUB_PERSONAL_ACCESS_TOKEN）
	Servers  []string `json:"servers"`  // 要打上组标记的连接器
	Display  string   `json:"display"`  // 面板展示名
	Scope    string   `json:"scope"`
}

// DeviceFlowSession 是发起授权后留在 daemon 内存里的一次会话（不落盘）。
type DeviceFlowSession struct {
	provider   string
	group      string
	field      string
	display    string
	servers    []string
	deviceCode string
	interval   time.Duration
	expiresAt  time.Time
	slowDown   bool
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

func githubClientID() string {
	if v := strings.TrimSpace(envLookup("MINDX_GITHUB_CLIENT_ID")); v != "" {
		return v
	}
	return defaultGitHubClientID
}

// StartDeviceFlow 申请短码。session 只是一次轮询的句柄，不携带任何秘密。
func (m *Manager) StartDeviceFlow(p DeviceFlowParams) (*deviceStartResult, error) {
	if p.Provider == "" || p.Provider == "github" {
		return m.startGitHubDeviceFlow(p)
	}
	return nil, fmt.Errorf("暂不支持 %s 的自动授权", p.Provider)
}

func (m *Manager) startGitHubDeviceFlow(p DeviceFlowParams) (*deviceStartResult, error) {
	if p.Group == "" {
		p.Group = "github"
	}
	if p.Field == "" {
		p.Field = "GITHUB_PERSONAL_ACCESS_TOKEN"
	}
	if p.Display == "" {
		p.Display = "GitHub 授权"
	}
	if p.Scope == "" {
		// repo 覆盖仓库读写；read:org 覆盖组织信息。范围写死不给用户选，
		// 少一个要理解的选项，也避免申请到用不上的权限。
		p.Scope = "repo read:org"
	}

	form := url.Values{}
	form.Set("client_id", githubClientID())
	form.Set("scope", p.Scope)
	body, err := postForm(githubDeviceCodeURL, form)
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
			return nil, fmt.Errorf("这个 GitHub OAuth 应用还没开启设备码授权：请到 GitHub 设置里的该应用页面勾选 Enable Device Flow，再回来重试")
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
		provider:   "github",
		group:      p.Group,
		field:      p.Field,
		display:    p.Display,
		servers:    p.Servers,
		deviceCode: resp.DeviceCode,
		interval:   time.Duration(resp.Interval) * time.Second,
		expiresAt:  time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second),
	}
	m.deviceFlow.mu.Lock()
	if m.deviceFlow.sessions == nil {
		m.deviceFlow.sessions = map[string]*DeviceFlowSession{}
	}
	m.deviceFlow.sessions[session] = sess
	m.deviceFlow.mu.Unlock()

	return &deviceStartResult{
		Session:         session,
		UserCode:        resp.UserCode,
		VerificationURI: resp.VerificationURI,
		ExpiresIn:       resp.ExpiresIn,
		Interval:        int(sess.interval.Seconds()),
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

	form := url.Values{}
	form.Set("client_id", githubClientID())
	form.Set("device_code", sess.deviceCode)
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	body, err := postForm(githubTokenURL, form)
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
				sess.interval = time.Duration(resp.Interval) * time.Second
			} else {
				sess.interval += 5 * time.Second
			}
			return &devicePollResult{Status: "slow_down", Interval: int(sess.interval.Seconds())}, nil
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

	login := githubLogin(resp.AccessToken)
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
	for i := range servers {
		s := &servers[i]
		if !containsName(sess.servers, s.Name) {
			continue
		}
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
		enabled := s.Enabled
		if !enabled {
			s.Enabled = true
			enabled = true
		}
		if err := m.AddServer(context.Background(), *s); err != nil {
			return nil, err
		}
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

// githubLogin 取账号名用于提示「已授权给 @xxx」；失败不影响授权结果。
func githubLogin(token string) string {
	req, err := http.NewRequest(http.MethodGet, githubUserURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var u struct {
		Login string `json:"login"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&u)
	return u.Login
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
