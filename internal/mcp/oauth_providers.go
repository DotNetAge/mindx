package mcp

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/DotNetAge/mindx/internal/core"
)

// 连接器自动授权的服务商注册表。
//
// 授权方式不写在代码里，而是从 settings 目录的 client-provider.yml 读（与技能 OAuth
// 共用同一份 oauth_providers 清单）。这里只做三件事：按 mcp.json 同级目录定位配置、
// 补一条内置兜底（配置缺失时 GitHub 仍可用）、把条目整理成界面可直接渲染的描述。
//
// 新增一家设备码服务商 = 在 yml 里加一条 kind: device_code 的条目，代码零改动。

// builtinDeviceProviders 是配置缺失时的兜底清单。只放公共客户端（公开 client_id +
// 设备码），绝不放 client_secret。
func builtinDeviceProviders() []*core.OAuthProviderConfig {
	return []*core.OAuthProviderConfig{
		{
			Name:          "github",
			Title:         "GitHub",
			Kind:          core.OAuthKindDeviceCode,
			DeviceURL:     "https://github.com/login/device/code",
			TokenURL:      "https://github.com/login/oauth/access_token",
			ClientID:      "Ov23likaZ2ZXJzRv8xgW",
			Scopes:        []string{"repo", "read:org"},
			UserInfoURL:   "https://api.github.com/user",
			UserNameField: "login",
			Interaction:   core.OAuthInteractionDeviceCode,
		},
	}
}

// OAuthProviderInfo 是给界面的授权方式描述：按钮文案与流程都由它驱动。
type OAuthProviderInfo struct {
	Name       string `json:"name"`
	Title      string `json:"title"`
	Kind       string `json:"kind"`
	Configured bool   `json:"configured"`
}

func (m *Manager) oauthProviders() *core.OAuthProviderRegistry {
	m.oauthRegMu.Lock()
	defer m.oauthRegMu.Unlock()
	if m.oauthReg != nil {
		return m.oauthReg
	}
	providers := builtinDeviceProviders()
	if m.configPath != "" {
		path := filepath.Join(filepath.Dir(m.configPath), "client-provider.yml")
		loaded, err := core.LoadClientProviders(path)
		if err != nil {
			m.log.Warn("OAuth 服务商配置读取失败，改用内置清单", "path", path, "err", err)
		} else if len(loaded) > 0 {
			// 文件条目覆盖内置兜底（同名以文件为准）
			providers = append(providers, loaded...)
		}
	}
	m.oauthReg = core.NewOAuthProviderRegistry(providers)
	return m.oauthReg
}

// SetOAuthProviders 注入注册表（测试用；生产走配置加载）。
func (m *Manager) SetOAuthProviders(providers []*core.OAuthProviderConfig) {
	m.oauthRegMu.Lock()
	m.oauthReg = core.NewOAuthProviderRegistry(providers)
	m.oauthRegMu.Unlock()
}

// OAuthProviders 列出可用于连接器授权的服务商（只返回设备码类：其余方式界面走手动引导）。
func (m *Manager) OAuthProviders() []OAuthProviderInfo {
	reg := m.oauthProviders()
	out := make([]OAuthProviderInfo, 0, len(reg.Names()))
	for _, name := range reg.Names() {
		p, ok := reg.Provider(name)
		if !ok || p.Kind != core.OAuthKindDeviceCode {
			continue
		}
		out = append(out, OAuthProviderInfo{
			Name:       p.Name,
			Title:      p.Title,
			Kind:       p.Kind,
			Configured: p.ClientID != "",
		})
	}
	return out
}

// deviceProvider 按名取设备码服务商；未注册 / 未配置 / 不是设备码类都给出可执行的原因。
func (m *Manager) deviceProvider(name string) (*core.OAuthProviderConfig, error) {
	reg := m.oauthProviders()
	p, ok := reg.Provider(name)
	if !ok {
		return nil, fmt.Errorf("暂不支持通过 %s 自动授权", name)
	}
	if p.Kind != core.OAuthKindDeviceCode {
		return nil, fmt.Errorf("%s 不是设备码授权方式，请按手动填写令牌的引导操作", p.Name)
	}
	if strings.TrimSpace(p.ClientID) == "" {
		return nil, fmt.Errorf("%s 的授权应用还没配置 client_id", p.Name)
	}
	if p.Title == "" {
		p.Title = p.Name
	}
	if p.UserNameField == "" {
		p.UserNameField = "login"
	}
	return p, nil
}

// providerClientID 允许用环境变量覆盖配置里的公开 client_id（换应用时不必改配置文件）。
func providerClientID(p *core.OAuthProviderConfig) string {
	key := "MINDX_OAUTH_" + strings.ToUpper(strings.ReplaceAll(p.Name, "-", "_")) + "_CLIENT_ID"
	if v := strings.TrimSpace(envLookup(key)); v != "" {
		return v
	}
	return p.ClientID
}
