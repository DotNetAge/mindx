package core

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/DotNetAge/goharness/logging"
)

// fakeMarketPage 是分页 API 最小响应（连通探测只要求 200 + 可解析）。
const fakeMarketPage = `{"total":0,"page":1,"packages":[]}`

// newSourceTestApp 构造最小可用 App（仅市场源链路所需字段）。
// 凭据存储显式用加密文件（而非 NewCredentialStore——darwin 下会写真实钥匙串，测试禁止污染）。
func newSourceTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	return &App{
		settings:    &Settings{Test: true, testDir: dir},
		mindxConfig: DefaultMindxConfig(dir),
		credStore:   &encryptedFileStore{path: filepath.Join(dir, "settings", ".credentials")},
		logger:      logging.NewNopLogger(),
	}
}

// sourceSrv 起一个假市场服务（探测端点返回可控行为）。
func sourceSrv(t *testing.T, status *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(int(status.Load()))
		_, _ = w.Write([]byte(fakeMarketPage))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestSetMarketSourceProbe 验证换源以「连通探测成功」为激活判据：
// 探测 200 才写入 MarketSourceID/MarketActiveBase 并落盘；失败原样报错且现有源保持不变。
func TestSetMarketSourceProbe(t *testing.T) {
	merchantStatus := &atomic.Int32{}
	merchantStatus.Store(http.StatusOK)
	merchant := sourceSrv(t, merchantStatus)

	// 引导源目录：default（空 base = 引导服务自身）+ merchant（指向 merchant srv）
	guideMux := http.NewServeMux()
	guideMux.HandleFunc("/api/v1/sources", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sources":[
			{"id":1,"name":"default","base_url":"","tier":"public"},
			{"id":2,"name":"merchant","base_url":"` + merchant.URL + `","tier":"merchant"}
		]}`))
	})
	guideMux.HandleFunc("/api/v1/packages", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fakeMarketPage))
	})
	guideSrv := httptest.NewServer(guideMux)
	t.Cleanup(guideSrv.Close)

	app := newSourceTestApp(t)
	app.mindxConfig.MarketAPIBase = guideSrv.URL

	// merchant 源探测 200 → 激活并落盘
	if err := app.SetMarketSource("2"); err != nil {
		t.Fatalf("探测 200 应激活成功：%v", err)
	}
	if app.mindxConfig.MarketSourceID != "2" {
		t.Errorf("MarketSourceID = %q, want 2", app.mindxConfig.MarketSourceID)
	}
	if app.mindxConfig.MarketActiveBase != merchant.URL {
		t.Errorf("MarketActiveBase = %q, want %q", app.mindxConfig.MarketActiveBase, merchant.URL)
	}

	// merchant 源探测 500 → 报错且现有源保持不变
	merchantStatus.Store(http.StatusInternalServerError)
	if err := app.SetMarketSource("2"); err == nil {
		t.Fatal("探测 500 应报错")
	}
	if app.mindxConfig.MarketActiveBase != merchant.URL {
		t.Errorf("探测失败后 MarketActiveBase 不应变化，got %q", app.mindxConfig.MarketActiveBase)
	}

	// 不存在的源 → 报错
	if err := app.SetMarketSource("99"); err == nil {
		t.Fatal("不存在的源应报错")
	}

	// 探测失败后切回默认源（引导服务 200）→ 激活回引导语义
	if err := app.SetMarketSource(""); err != nil {
		t.Fatalf("切回默认源应成功：%v", err)
	}
	if app.mindxConfig.MarketSourceID != "" {
		t.Errorf("默认源 source id 应为空，got %q", app.mindxConfig.MarketSourceID)
	}
}

// TestSetMarketPATKeychain 验证邀请码只进密钥存储：Get 回读一致，
// mindx.json 文件内容不含 market_pat 明文。
func TestSetMarketPATKeychain(t *testing.T) {
	app := newSourceTestApp(t)

	// 先落一份基线配置（模拟已有用户配置文件的场景）
	if err := app.mindxConfig.Save(); err != nil {
		t.Fatalf("基线落盘失败：%v", err)
	}

	if err := app.SetMarketPAT("mk_test_secret"); err != nil {
		t.Fatalf("SetMarketPAT 失败：%v", err)
	}
	if got := app.MarketPAT(); got != "mk_test_secret" {
		t.Errorf("MarketPAT() = %q, want 内存同步", got)
	}
	if got, err := app.credStore.Get("market:pat"); err != nil || got != "mk_test_secret" {
		t.Errorf("密钥存储回读 = %q, err = %v, want 一致", got, err)
	}

	raw, err := os.ReadFile(app.mindxConfig.filePath)
	if err != nil {
		t.Fatalf("读 mindx.json 失败：%v", err)
	}
	if strings.Contains(string(raw), "mk_test_secret") || strings.Contains(string(raw), "market_pat") {
		t.Error("mindx.json 不应包含邀请码明文或 market_pat 字段")
	}
}
