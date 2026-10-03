package svc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DotNetAge/mindx/internal/core"
)

// fakeCredStore 凭据库测试替身（对齐 encryptedFileStore 缺键 ("", nil) 语义）
type fakeCredStore struct {
	mu sync.Mutex
	m  map[string]string
}

func newFakeCredStore() *fakeCredStore {
	return &fakeCredStore{m: make(map[string]string)}
}

func (f *fakeCredStore) Get(key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.m[key], nil
}

func (f *fakeCredStore) Set(key, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.m[key] = value
	return nil
}

// TestEffectiveProviderConfig_StoreOverlay 凭据库覆盖值优先于 yml 预置；
// 两处皆空即未配置态报错；yml 路径 UseState 恒开。
func TestEffectiveProviderConfig_StoreOverlay(t *testing.T) {
	base := &core.OAuthProviderConfig{
		Name: "testp", Title: "Test", Kind: core.OAuthKindDeviceCode,
		TokenURL: "https://example.com/token", DeviceURL: "https://example.com/device",
		Interaction: core.OAuthInteractionDeviceCode,
	}
	store := newFakeCredStore()

	// 未配置态：client_id 空 → 报错
	if _, err := effectiveProviderConfig(base, store); err == nil {
		t.Fatal("expected unconfigured provider to error")
	}

	// 凭据库覆盖：client_id 写入后生效
	if err := store.Set("testp:client_id", "cid-from-store"); err != nil {
		t.Fatal(err)
	}
	cfg, err := effectiveProviderConfig(base, store)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ClientID != "cid-from-store" {
		t.Fatalf("client_id = %q, want store override", cfg.ClientID)
	}
	if !cfg.UseState {
		t.Fatal("yml registry path must force UseState (CSRF hard constraint)")
	}

	// 机密凭据：client_secret 同样走凭据库覆盖
	if err := store.Set("testp:client_secret", "sec"); err != nil {
		t.Fatal(err)
	}
	cfg, err = effectiveProviderConfig(base, store)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ClientSecret != "sec" {
		t.Fatalf("client_secret = %q, want store override", cfg.ClientSecret)
	}
}

// TestOAuthFlowManager 并发登记语义：同名流程互斥、stop 取消并移除登记。
func TestOAuthFlowManager(t *testing.T) {
	m := newOAuthFlowManager()
	started := make(chan struct{})
	entered := make(chan context.Context, 1)

	if !m.start("p", func(ctx context.Context) {
		close(started)
		entered <- ctx
		<-ctx.Done() // 阻塞至取消
	}) {
		t.Fatal("first start should succeed")
	}
	<-started
	if m.start("p", func(context.Context) {}) {
		t.Fatal("duplicate start must be rejected while running")
	}
	if !m.running("p") {
		t.Fatal("flow should be running")
	}
	if !m.stop("p") {
		t.Fatal("stop should succeed for running flow")
	}
	ctx := <-entered
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("flow ctx not cancelled after stop")
	}
	// 登记移除可能稍晚于 cancel（goroutine defer），短暂等待后复查
	deadline := time.Now().Add(time.Second)
	for m.running("p") {
		if time.Now().After(deadline) {
			t.Fatal("flow entry not removed after stop")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if m.stop("p") {
		t.Fatal("stop on absent flow must fail")
	}
}

// TestOAuthAuthorizeStart_Guards 启动闸：未知提供商与未配置态同步报错。
func TestOAuthAuthorizeStart_Guards(t *testing.T) {
	store := newFakeCredStore()
	d := &Daemon{
		oauthRegistry: core.NewOAuthProviderRegistry([]*core.OAuthProviderConfig{
			{
				Name: "testdev", Title: "Test Device", Kind: core.OAuthKindDeviceCode,
				TokenURL: "https://example.com/token", DeviceURL: "https://example.com/device",
				Interaction: core.OAuthInteractionDeviceCode,
			},
		}),
		oauthCredStore: store,
		oauthFlows:     newOAuthFlowManager(),
	}

	params := func(s string) json.RawMessage { return json.RawMessage(s) }

	if _, err := d.handleOAuthAuthorizeStart(context.Background(), params(`{"name":"nope"}`)); err == nil {
		t.Fatal("unknown provider must error")
	}
	if _, err := d.handleOAuthAuthorizeStart(context.Background(), params(`{"name":"testdev"}`)); err == nil {
		t.Fatal("unconfigured provider must error synchronously")
	}
	// 未配置拦截不得留下流程登记
	if d.oauthFlows.running("testdev") {
		t.Fatal("no flow should be registered for rejected start")
	}
}

// TestOAuthAuthorizeCancelE2E 设备码流程端到端取消：
// httptest 模拟恒 pending 的端点 → 流程进入轮询 → cancel → 流程退出并清登记。
func TestOAuthAuthorizeCancelE2E(t *testing.T) {
	var tokenHits atomic.Int64
	firstTokenHit := make(chan struct{}, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/device", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"device_code":"dc","user_code":"ABCD-1234","verification_uri":"https://example.com/activate","expires_in":600,"interval":1}`))
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		if tokenHits.Add(1) == 1 {
			select {
			case firstTokenHit <- struct{}{}:
			default:
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	store := newFakeCredStore()
	if err := store.Set("testdev:client_id", "cid"); err != nil {
		t.Fatal(err)
	}
	d := &Daemon{
		oauthRegistry: core.NewOAuthProviderRegistry([]*core.OAuthProviderConfig{
			{
				Name: "testdev", Title: "Test Device", Kind: core.OAuthKindDeviceCode,
				TokenURL: srv.URL + "/token", DeviceURL: srv.URL + "/device",
				Interaction: core.OAuthInteractionDeviceCode,
			},
		}),
		oauthCredStore: store,
		oauthFlows:     newOAuthFlowManager(),
	}

	res, err := d.handleOAuthAuthorizeStart(context.Background(), json.RawMessage(`{"name":"testdev"}`))
	if err != nil {
		t.Fatal(err)
	}
	if started, _ := res.(map[string]any)["started"].(bool); !started {
		t.Fatalf("unexpected start result: %v", res)
	}

	// 等流程进入轮询（token 端点首次命中）
	select {
	case <-firstTokenHit:
	case <-time.After(5 * time.Second):
		t.Fatal("flow never reached token polling")
	}

	if _, err := d.handleOAuthAuthorizeCancel(context.Background(), json.RawMessage(`{"name":"testdev"}`)); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for d.oauthFlows.running("testdev") {
		if time.Now().After(deadline) {
			t.Fatal("flow not cleaned up after cancel")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestOAuthConfigure 第二层模板 client_id 填入：写凭据库 + 列表配置态翻转。
func TestOAuthConfigure(t *testing.T) {
	store := newFakeCredStore()
	d := &Daemon{
		oauthRegistry: core.NewOAuthProviderRegistry([]*core.OAuthProviderConfig{
			{
				Name: "github", Title: "GitHub", Kind: core.OAuthKindAuthorizationCode,
				AuthorizeURL: "https://github.com/login/oauth/authorize", TokenURL: "https://github.com/login/oauth/access_token",
				Interaction: core.OAuthInteractionCallback,
			},
		}),
		oauthCredStore: store,
		oauthFlows:     newOAuthFlowManager(),
	}

	if _, err := d.handleOAuthConfigure(context.Background(), json.RawMessage(`{"name":"github","client_id":"  "}`)); err == nil {
		t.Fatal("empty client_id must error")
	}
	if _, err := d.handleOAuthConfigure(context.Background(), json.RawMessage(`{"name":"nope","client_id":"x"}`)); err == nil {
		t.Fatal("unknown provider must error")
	}
	if _, err := d.handleOAuthConfigure(context.Background(), json.RawMessage(`{"name":"github","client_id":" iv1 ","client_secret":"s1"}`)); err != nil {
		t.Fatal(err)
	}
	// 写入应去空白
	if v, _ := store.Get("github:client_id"); v != "iv1" {
		t.Fatalf("client_id = %q, want trimmed value", v)
	}

	res, err := d.handleOAuthProviderList(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	providers, _ := res.(map[string]any)["providers"].([]oauthProviderView)
	if len(providers) != 1 || !providers[0].Configured || providers[0].ClientID != "iv1" {
		t.Fatalf("unexpected provider list: %+v", providers)
	}
}
