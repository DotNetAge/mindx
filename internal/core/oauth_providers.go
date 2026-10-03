package core

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// OAuth 客户端提供商授权方式（与 gochat auth.ProviderKind 对齐）
const (
	OAuthKindAuthorizationCode = "authorization_code"
	OAuthKindDeviceCode        = "device_code"
	OAuthKindUserCode          = "user_code"
)

// OAuth 客户端提供商交互方式（由 kind 派生，冗余写出供壳侧零推导）
const (
	OAuthInteractionCallback   = "callback"
	OAuthInteractionDeviceCode = "device_code"
	OAuthInteractionUserCode   = "user_code"
)

// OAuthProviderConfig OAuth 客户端提供商配置——与 runtime/settings/client-provider.yml
// 的 oauth_providers 条目字段一比一对应。
type OAuthProviderConfig struct {
	// Name 提供商唯一键，技能 frontmatter 的 requires.oauth.provider 引用此名
	Name string `yaml:"name"`
	// Title 展示名（授权预告 UI 用）
	Title string `yaml:"title"`
	// Kind 授权方式：authorization_code | device_code | user_code
	Kind string `yaml:"kind"`
	// AuthorizeURL 授权端点（authorization_code / user_code 必填）
	AuthorizeURL string `yaml:"authorize_url,omitempty"`
	// TokenURL 令牌端点（三类通用必填）
	TokenURL string `yaml:"token_url,omitempty"`
	// DeviceURL 设备码端点（device_code 必填）
	DeviceURL string `yaml:"device_url,omitempty"`
	// ClientID 客户端 ID；空 = 未配置态（第二层模板由用户注册后经设置填入）
	ClientID string `yaml:"client_id,omitempty"`
	// ClientSecret 机密客户端凭据——yml 恒不承载（硬约束），加载遇到非空即报错；
	// 用户侧凭据仅经 CredentialStore 存放
	ClientSecret string `yaml:"client_secret,omitempty"`
	// Scopes 授权范围
	Scopes []string `yaml:"scopes,omitempty"`
	// ExtraParams 附加授权参数（如 Google 的 access_type=offline）
	ExtraParams map[string]string `yaml:"extra_params,omitempty"`
	// UsePKCE 公共客户端恒 true（S256）
	UsePKCE bool `yaml:"uses_pkce,omitempty"`
	// ListenAddr 授权码流本地回调监听地址（127.0.0.1:0 = 回环随机端口）
	ListenAddr string `yaml:"listen_addr,omitempty"`
	// Interaction 交互方式（callback | device_code | user_code），须与 kind 派生值一致
	Interaction string `yaml:"interaction,omitempty"`
}

// DeriveInteraction 由 kind 推导交互方式
func (c *OAuthProviderConfig) DeriveInteraction() string {
	switch c.Kind {
	case OAuthKindAuthorizationCode:
		return OAuthInteractionCallback
	case OAuthKindDeviceCode:
		return OAuthInteractionDeviceCode
	case OAuthKindUserCode:
		return OAuthInteractionUserCode
	default:
		return ""
	}
}

// Validate 校验配置完整性与一致性：
// name 非空、kind 枚举、必填端点与 kind 匹配、interaction 派生一致、
// client_secret 必须为空（yml 恒不承载机密凭据）。
func (c *OAuthProviderConfig) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("oauth provider config: name is required")
	}
	switch c.Kind {
	case OAuthKindAuthorizationCode:
		if c.AuthorizeURL == "" {
			return fmt.Errorf("oauth provider %q: authorize_url is required for kind %s", c.Name, c.Kind)
		}
	case OAuthKindDeviceCode:
		if c.DeviceURL == "" {
			return fmt.Errorf("oauth provider %q: device_url is required for kind %s", c.Name, c.Kind)
		}
	case OAuthKindUserCode:
		if c.AuthorizeURL == "" {
			return fmt.Errorf("oauth provider %q: authorize_url is required for kind %s", c.Name, c.Kind)
		}
	default:
		return fmt.Errorf("oauth provider %q: unknown kind %q", c.Name, c.Kind)
	}
	if c.TokenURL == "" {
		return fmt.Errorf("oauth provider %q: token_url is required", c.Name)
	}
	if c.Interaction != "" && c.Interaction != c.DeriveInteraction() {
		return fmt.Errorf("oauth provider %q: interaction %q mismatches kind %q (expect %q)",
			c.Name, c.Interaction, c.Kind, c.DeriveInteraction())
	}
	if c.ClientSecret != "" {
		return fmt.Errorf("oauth provider %q: client_secret must never be preset in yml (store via CredentialStore instead)", c.Name)
	}
	return nil
}

// ClientProvidersConfig client-provider.yml 顶层结构
type ClientProvidersConfig struct {
	OauthProviders []*OAuthProviderConfig `yaml:"oauth_providers"`
}

// LoadClientProviders 从指定路径加载 OAuth 客户端提供商注册表并逐条校验。
// 文件不存在时返回 (nil, nil)——对齐 LoadProvidersFile 先例（首次启动无配置属正常）。
func LoadClientProviders(path string) ([]*OAuthProviderConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read client provider file: %w", err)
	}

	var cfg ClientProvidersConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse client provider file: %w", err)
	}

	seen := make(map[string]bool, len(cfg.OauthProviders))
	for _, p := range cfg.OauthProviders {
		if p == nil {
			return nil, fmt.Errorf("client provider config: nil entry")
		}
		if err := p.Validate(); err != nil {
			return nil, err
		}
		if seen[p.Name] {
			return nil, fmt.Errorf("client provider config: duplicate name %q", p.Name)
		}
		seen[p.Name] = true
	}
	return cfg.OauthProviders, nil
}

// OAuthProviderRegistry OAuth 客户端提供商注册表——按名查询与配置态判定。
type OAuthProviderRegistry struct {
	byName map[string]*OAuthProviderConfig
	order  []string
}

// NewOAuthProviderRegistry 构建注册表（同名条目以最后一条为准，加载校验已保证唯一）
func NewOAuthProviderRegistry(providers []*OAuthProviderConfig) *OAuthProviderRegistry {
	r := &OAuthProviderRegistry{byName: make(map[string]*OAuthProviderConfig, len(providers))}
	for _, p := range providers {
		if p == nil {
			continue
		}
		if _, exists := r.byName[p.Name]; !exists {
			r.order = append(r.order, p.Name)
		}
		r.byName[p.Name] = p
	}
	return r
}

// Provider 按名查询提供商配置
func (r *OAuthProviderRegistry) Provider(name string) (*OAuthProviderConfig, bool) {
	p, ok := r.byName[name]
	return p, ok
}

// Configured 判定提供商是否已配置（client_id 非空）；未注册视为未配置
func (r *OAuthProviderRegistry) Configured(name string) bool {
	p, ok := r.byName[name]
	return ok && p.ClientID != ""
}

// Names 返回注册表内全部提供商名（按 yml 声明顺序）
func (r *OAuthProviderRegistry) Names() []string {
	return append([]string(nil), r.order...)
}
