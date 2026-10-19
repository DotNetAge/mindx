package svc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/DotNetAge/gochat/auth"
	"github.com/DotNetAge/mindx/internal/core"
)

// ---------------------------------------------------------------------------
// oauth.* RPC —— OAuth 提供商注册表查询 / 配置 / 授权流程（阶段四壳侧授权配合）
//
// 数据流：壳 oauth.authorize.start → daemon 起 goroutine 执行 gochat
// ConfigDrivenProvider（AuthenticateContext）→ InteractionHook 经 broadcastUI
// 把呈现事件推给壳（拉系统浏览器 / 展示码均由壳执行）→ 授权完成令牌经
// KeyringTokenStore 写 CredentialStore → 广播收尾事件。
//
// 凭据库覆盖序：CredentialStore 的 <name>:client_id / <name>:client_secret
// 非空则覆盖 yml 预置值——第二层通用平台模板（client_id 留空）由用户在
// 壳设置页注册应用后填入，凭据绝不落 yml（硬约束）。
// 存储键约定：client_id/client_secret 用 <name>:client_id 等派生键；
// 令牌默认 <name>:token（设置页手动授权），技能准备流程显式传
// <skill_name>:<env_name>（阶段五）。
// ---------------------------------------------------------------------------

// oauthProviderView 提供商列表条目（壳侧设置区渲染形状）
type oauthProviderView struct {
	Name        string   `json:"name"`
	Title       string   `json:"title"`
	Kind        string   `json:"kind"`
	Interaction string   `json:"interaction"`
	ClientID    string   `json:"client_id"`
	Configured  bool     `json:"configured"`
	Scopes      []string `json:"scopes,omitempty"`
}

// handleOAuthProviderList 处理 oauth.provider.list：全部提供商与配置态。
func (d *Daemon) handleOAuthProviderList(_ context.Context, _ json.RawMessage) (any, error) {
	registry := d.oauthRegistryOrEmpty()
	names := registry.Names()
	out := make([]oauthProviderView, 0, len(names))
	for _, name := range names {
		p, ok := registry.Provider(name)
		if !ok || p == nil {
			continue
		}
		clientID := effectiveClientID(p, d.oauthCredStore)
		out = append(out, oauthProviderView{
			Name:        p.Name,
			Title:       p.Title,
			Kind:        p.Kind,
			Interaction: p.Interaction,
			ClientID:    clientID,
			Configured:  strings.TrimSpace(clientID) != "",
			Scopes:      p.Scopes,
		})
	}
	return map[string]any{"providers": out}, nil
}

// handleOAuthConfigure 处理 oauth.provider.configure：第二层模板的 client_id
// 填入入口（写凭据库，绝不落 yml）。client_secret 仅机密客户端需要，可空。
func (d *Daemon) handleOAuthConfigure(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		Name         string `json:"name"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret,omitempty"`
	}
	if err := json.Unmarshal(params, &p); err != nil || strings.TrimSpace(p.Name) == "" {
		return nil, fmt.Errorf("缺少 name 参数")
	}
	if _, ok := d.oauthRegistryOrEmpty().Provider(p.Name); !ok {
		return nil, fmt.Errorf("未知的 OAuth 提供商 %q", p.Name)
	}
	if strings.TrimSpace(p.ClientID) == "" {
		return nil, fmt.Errorf("缺少 Client ID")
	}
	if err := d.oauthCredStore.Set(p.Name+":client_id", strings.TrimSpace(p.ClientID)); err != nil {
		return nil, fmt.Errorf("保存 Client ID 失败：%w", err)
	}
	if strings.TrimSpace(p.ClientSecret) != "" {
		if err := d.oauthCredStore.Set(p.Name+":client_secret", strings.TrimSpace(p.ClientSecret)); err != nil {
			return nil, fmt.Errorf("保存 Client Secret 失败：%w", err)
		}
	}
	return map[string]any{"ok": true}, nil
}

// handleOAuthAuthorizeStart 处理 oauth.authorize.start：异步启动授权流程。
// 未配置态同步报错（壳立即在触发行提示），流程事件经通知推给壳：
// oauth.present_auth_url / present_device_code / present_user_code /
// flow_completed / flow_failed / flow_cancelled（envelope 对齐 broadcastUI）。
func (d *Daemon) handleOAuthAuthorizeStart(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		Name     string `json:"name"`
		StoreKey string `json:"store_key,omitempty"`
	}
	if err := json.Unmarshal(params, &p); err != nil || strings.TrimSpace(p.Name) == "" {
		return nil, fmt.Errorf("缺少 name 参数")
	}
	registry := d.oauthRegistryOrEmpty()
	provider, ok := registry.Provider(p.Name)
	if !ok || provider == nil {
		return nil, fmt.Errorf("未知的 OAuth 提供商 %q", p.Name)
	}
	// 未配置态同步拦截：壳无需等通知即可在触发行提示「先填 Client ID」
	if strings.TrimSpace(effectiveClientID(provider, d.oauthCredStore)) == "" {
		return nil, fmt.Errorf("提供商 %q 尚未配置 Client ID，请先在设置的 OAuth 提供商页填入", p.Name)
	}
	storeKey := strings.TrimSpace(p.StoreKey)
	if storeKey == "" {
		storeKey = p.Name + ":token" // 设置页手动授权的默认落键
	}
	if !d.oauthFlows.start(p.Name, func(ctx context.Context) {
		d.runOAuthFlow(ctx, p.Name, storeKey)
	}) {
		return nil, fmt.Errorf("提供商 %q 已有进行中的授权流程，请先完成或取消", p.Name)
	}
	return map[string]any{"started": true, "store_key": storeKey}, nil
}

// handleOAuthAuthorizeCancel 处理 oauth.authorize.cancel：取消进行中的流程
// （gochat AuthenticateContext 响应取消，流程自行广播 flow_cancelled）。
func (d *Daemon) handleOAuthAuthorizeCancel(_ context.Context, params json.RawMessage) (any, error) {
	var p struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(params, &p); err != nil || strings.TrimSpace(p.Name) == "" {
		return nil, fmt.Errorf("缺少 name 参数")
	}
	if !d.oauthFlows.stop(p.Name) {
		return nil, fmt.Errorf("提供商 %q 没有进行中的授权流程", p.Name)
	}
	return map[string]any{"cancelled": true}, nil
}

// runOAuthFlow 执行授权流程（阻塞）：gochat 泛化流程 + 壳广播 hook，
// 令牌落库后广播收尾事件。由 oauthFlowManager 的 goroutine 调用。
func (d *Daemon) runOAuthFlow(ctx context.Context, name, storeKey string) {
	defer func() {
		if r := recover(); r != nil && d.logger != nil {
			d.logger.Error("oauth flow panic", fmt.Errorf("%v", r), "provider", name)
			d.broadcastUI("oauth.flow_failed", map[string]any{"provider": name, "error": fmt.Sprintf("内部错误：%v", r)})
		}
	}()

	p, ok := d.oauthRegistryOrEmpty().Provider(name)
	if !ok || p == nil {
		d.finishFlowError(name, fmt.Errorf("未知的 OAuth 提供商 %q", name))
		return
	}
	cfg, err := effectiveProviderConfig(p, d.oauthCredStore)
	if err != nil {
		d.finishFlowError(name, err)
		return
	}
	provider, err := auth.NewConfigDrivenProvider(cfg, broadcastInteractionHook{d: d, provider: name, title: p.Title})
	if err != nil {
		d.finishFlowError(name, err)
		return
	}
	token, err := provider.AuthenticateContext(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			d.broadcastUI("oauth.flow_cancelled", map[string]any{"provider": name})
			return
		}
		d.finishFlowError(name, err)
		return
	}
	if err := auth.NewKeyringTokenStore(d.oauthCredStore, storeKey).Save(token); err != nil {
		d.finishFlowError(name, fmt.Errorf("令牌保存失败：%w", err))
		return
	}
	if d.logger != nil {
		d.logger.Info("oauth flow completed", "provider", name, "store_key", storeKey)
	}
	d.broadcastUI("oauth.flow_completed", map[string]any{"provider": name, "store_key": storeKey})
}

// finishFlowError 流程失败收尾：日志 + 广播 flow_failed
func (d *Daemon) finishFlowError(name string, err error) {
	if d.logger != nil {
		d.logger.Warn("oauth flow failed", "provider", name, "error", err)
	}
	d.broadcastUI("oauth.flow_failed", map[string]any{"provider": name, "error": err.Error()})
}

// oauthRegistryOrEmpty 空注册表兜底（测试直构 Daemon / 加载降级场景的 nil 安全）
func (d *Daemon) oauthRegistryOrEmpty() *core.OAuthProviderRegistry {
	if d.oauthRegistry == nil {
		return core.NewOAuthProviderRegistry(nil)
	}
	return d.oauthRegistry
}

// ── 生效配置解析 ──────────────────────────────────────────────────────────

// effectiveClientID 提供商生效 client_id：凭据库覆盖值优先，回退 yml 预置
func effectiveClientID(p *core.OAuthProviderConfig, store core.CredentialStore) string {
	if store != nil {
		if v, err := store.Get(p.Name + ":client_id"); err == nil && v != "" {
			return v
		}
	}
	return p.ClientID
}

// effectiveProviderConfig 注册表条目 + 凭据库覆盖 → gochat ProviderConfig。
// UseState 恒开（yml 注册表路径防 CSRF 硬约束）；client_id 空即未配置态报错。
func effectiveProviderConfig(p *core.OAuthProviderConfig, store core.CredentialStore) (auth.ProviderConfig, error) {
	clientID := strings.TrimSpace(effectiveClientID(p, store))
	if clientID == "" {
		return auth.ProviderConfig{}, fmt.Errorf("提供商 %q 尚未配置 Client ID，请先在设置的 OAuth 提供商页填入", p.Name)
	}
	var secret string
	if store != nil {
		if v, err := store.Get(p.Name + ":client_secret"); err == nil {
			secret = v
		}
	}
	cfg := auth.ProviderConfig{
		Name:         p.Name,
		Title:        p.Title,
		Kind:         auth.ProviderKind(p.Kind),
		AuthorizeURL: p.AuthorizeURL,
		TokenURL:     p.TokenURL,
		DeviceURL:    p.DeviceURL,
		ClientID:     clientID,
		ClientSecret: secret,
		Scopes:       append([]string(nil), p.Scopes...),
		ExtraParams:  p.ExtraParams,
		UsePKCE:      p.UsePKCE,
		UseState:     true,
		ListenAddr:   p.ListenAddr,
		Interaction:  p.Interaction,
	}
	if err := cfg.Validate(); err != nil {
		return auth.ProviderConfig{}, err
	}
	return cfg, nil
}

// ── 壳广播 hook 与流程登记 ────────────────────────────────────────────────

// broadcastInteractionHook InteractionHook 的壳广播实现：呈现事件经 gateway
// 通知推给壳（拉系统浏览器 / 展示码 / 等待态均由壳执行），CLI 打印实现之外
// 的 GUI 接缝。广播目标为全部客户端，mindx-work 常驻场景即用户端。
type broadcastInteractionHook struct {
	d        *Daemon
	provider string
	title    string
}

// presentOrLog 呈现事件的统一出口：有客户端连接时经 broadcastUI 推壳；
// 无客户端（CLI 直连 daemon / 无壳场景）广播等于静默丢弃，降级为日志输出，
// 保证 daemon 前台与日志中可见授权链接与用户码。
func (h broadcastInteractionHook) presentOrLog(event string, fields map[string]any) {
	if h.d.gw != nil && h.d.gw.ClientCount() > 0 {
		h.d.broadcastUI(event, fields)
		return
	}
	if h.d.logger == nil {
		return
	}
	args := make([]any, 0, len(fields)*2+2)
	args = append(args, "event", event)
	for k, v := range fields {
		args = append(args, k, v)
	}
	h.d.logger.Info("oauth 呈现事件（无客户端连接，降级日志）", args...)
}

// PresentAuthURL 授权码流：呈现授权链接（监听已就绪，壳拉浏览器即可命中回调）
func (h broadcastInteractionHook) PresentAuthURL(authorizeURL string) {
	h.presentOrLog("oauth.present_auth_url", map[string]any{
		"provider": h.provider, "title": h.title, "url": authorizeURL,
	})
}

// PresentDeviceCode 设备码流：呈现用户码与验证网址（expiresIn 秒后过期）
func (h broadcastInteractionHook) PresentDeviceCode(userCode, verifyURL string, expiresIn int) {
	h.presentOrLog("oauth.present_device_code", map[string]any{
		"provider": h.provider, "title": h.title,
		"user_code": userCode, "verify_url": verifyURL, "expires_in": expiresIn,
	})
}

// PresentUserCode 用户码两步协议：呈现用户码与验证网址
func (h broadcastInteractionHook) PresentUserCode(userCode, verifyURL string) {
	h.presentOrLog("oauth.present_user_code", map[string]any{
		"provider": h.provider, "title": h.title,
		"user_code": userCode, "verify_url": verifyURL,
	})
}

// oauthFlowManager 进行中授权流程登记：每 provider 至多一条，防并发重复授权。
type oauthFlowManager struct {
	mu     sync.Mutex
	cancel map[string]context.CancelFunc
}

func newOAuthFlowManager() *oauthFlowManager {
	return &oauthFlowManager{cancel: make(map[string]context.CancelFunc)}
}

// start 登记并异步启动流程；同名流程进行中返回 false（不启动）
func (m *oauthFlowManager) start(name string, fn func(ctx context.Context)) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.cancel[name]; exists {
		return false
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel[name] = cancel
	go func() {
		defer func() {
			cancel()
			m.mu.Lock()
			delete(m.cancel, name)
			m.mu.Unlock()
		}()
		fn(ctx)
	}()
	return true
}

// stop 取消流程；流程不存在返回 false。ctx 取消后流程 goroutine 自行
// 广播 flow_cancelled 并从登记表移除。
func (m *oauthFlowManager) stop(name string) bool {
	m.mu.Lock()
	cancel, ok := m.cancel[name]
	m.mu.Unlock()
	if !ok {
		return false
	}
	cancel()
	return true
}

// running 查询流程是否进行中
func (m *oauthFlowManager) running(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.cancel[name]
	return ok
}
