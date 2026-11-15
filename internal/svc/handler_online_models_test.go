package svc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	goharnessconfig "github.com/DotNetAge/goharness/config"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

// seedSFPricing 预置/清理硅基流动价格页进程缓存，避免单测真连价格页。
func seedSFPricing(t *testing.T, table map[string]siliconFlowPrice, failed bool) {
	t.Helper()
	sfPricingMu.Lock()
	origTable, origAt, origFail := sfPricingTable, sfPricingFetchedAt, sfPricingFailedAt
	sfPricingMu.Unlock()
	t.Cleanup(func() {
		sfPricingMu.Lock()
		sfPricingTable, sfPricingFetchedAt, sfPricingFailedAt = origTable, origAt, origFail
		sfPricingMu.Unlock()
	})
	sfPricingMu.Lock()
	if failed {
		sfPricingTable, sfPricingFetchedAt = nil, time.Time{}
		sfPricingFailedAt = time.Now()
	} else {
		sfPricingTable, sfPricingFetchedAt = table, time.Now()
		sfPricingFailedAt = time.Time{}
	}
	sfPricingMu.Unlock()
}

// seedFXRate 预置/清理 USD→CNY 汇率进程缓存，避免单测真连汇率源。
func seedFXRate(t *testing.T, rate float64) {
	t.Helper()
	fxRateMu.Lock()
	origRate, origAt, origFail := fxRateUSDCNY, fxRateFetchedAt, fxRateFailedAt
	fxRateMu.Unlock()
	t.Cleanup(func() {
		fxRateMu.Lock()
		fxRateUSDCNY, fxRateFetchedAt, fxRateFailedAt = origRate, origAt, origFail
		fxRateMu.Unlock()
	})
	fxRateMu.Lock()
	fxRateUSDCNY, fxRateFetchedAt = rate, time.Now()
	fxRateFailedAt = time.Time{}
	fxRateMu.Unlock()
}

// setProviders 注入供应商配置（BaseURL 可指向 httptest 服务）。
func setProviders(t *testing.T, d *Daemon, providers ...*goharnessconfig.ProviderConfig) {
	t.Helper()
	d.app.SetProviderConfigs(providers)
}

func TestEnsureV1Suffix(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"  ", ""},
		{"https://api.siliconflow.cn", "https://api.siliconflow.cn/v1"},
		{"https://api.siliconflow.cn/", "https://api.siliconflow.cn/v1"},
		{" https://api.siliconflow.cn ", "https://api.siliconflow.cn/v1"},
		{"https://openrouter.ai/api", "https://openrouter.ai/api/v1"},
		{"https://openrouter.ai/api/v1", "https://openrouter.ai/api/v1"},
		{"https://mirror.example.com/api/v1/", "https://mirror.example.com/api/v1"},
	}
	for _, c := range cases {
		if got := ensureV1Suffix(c.in); got != c.want {
			t.Errorf("ensureV1Suffix(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParsePrice(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"0", 0}, {"0.0000015", 0.0000015}, {" 0.5 ", 0.5},
		{"", 0}, {"abc", 0}, {"-1", 0}, {"-0.1", 0},
	}
	for _, c := range cases {
		if got := parsePrice(c.in); got != c.want {
			t.Errorf("parsePrice(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseSiliconFlowPricingItems(t *testing.T) {
	// 常规（未转义）payload
	plain := []byte(`{"x":1,"pricingApiItems":[{"playgroundName":"M1","componentCode":"input-tokens","realTimePriceCnyUnit":"0.001","unitZhCnName":"K tokens"}],"after":2}`)
	items, ok := parseSiliconFlowPricingItems(plain)
	if !ok || len(items) != 1 || items[0].PlaygroundName != "M1" {
		t.Fatalf("plain parse: ok=%v items=%#v", ok, items)
	}

	// Next.js flight payload：引号呈 \" 转义形式
	escaped := []byte(`{"x\":1,\"pricingApiItems\":[{\"playgroundName\":\"M2\",\"componentCode\":\"output-tokens\",\"realTimePriceCnyUnit\":\"0.002\",\"unitZhCnName\":\"K tokens\"}]}`)
	items, ok = parseSiliconFlowPricingItems(escaped)
	if !ok || len(items) != 1 || items[0].ComponentCode != "output-tokens" {
		t.Fatalf("escaped parse: ok=%v items=%#v", ok, items)
	}

	// 错误分支
	if _, ok := parseSiliconFlowPricingItems([]byte(`{"other":1}`)); ok {
		t.Error("missing marker: expected ok=false")
	}
	if _, ok := parseSiliconFlowPricingItems([]byte(`"pricingApiItems":[{"playgroundName":"unclosed`)); ok {
		t.Error("truncated array: expected ok=false")
	}
	if _, ok := parseSiliconFlowPricingItems([]byte(`"pricingApiItems":[{"broken`)); ok {
		t.Error("invalid json: expected ok=false")
	}
	if _, ok := parseSiliconFlowPricingItems([]byte(`"pricingApiItems":[]`)); ok {
		t.Error("empty items: expected ok=false")
	}
}

func TestAggregateSiliconFlowPricing(t *testing.T) {
	items := []siliconFlowPricingItem{
		// 阶梯计费：input-tokens 两条，取第一条（起步价）
		{PlaygroundName: "Model A", ComponentCode: "input-tokens", RealTimePriceCnyUnit: "0.001", UnitZhCnName: "K tokens"},
		{PlaygroundName: "Model A", ComponentCode: "input-tokens", RealTimePriceCnyUnit: "0.002", UnitZhCnName: "K tokens"},
		{PlaygroundName: "Model A", ComponentCode: "output-tokens", RealTimePriceCnyUnit: "0.004", UnitZhCnName: "K tokens"},
		// 非 tokens 计价单位（音频按秒计费）不参与
		{PlaygroundName: "Model B", ComponentCode: "input-tokens", RealTimePriceCnyUnit: "9", UnitZhCnName: "秒"},
		// 无关计费项不参与
		{PlaygroundName: "Model A", ComponentCode: "cached-input-tokens", RealTimePriceCnyUnit: "0.0001", UnitZhCnName: "K tokens"},
		// 只有 output 的模型
		{PlaygroundName: "Model C", ComponentCode: "output-tokens", RealTimePriceCnyUnit: "0.003", UnitZhCnName: "K tokens"},
	}

	table := aggregateSiliconFlowPricing(items)

	a := table["model a"]
	if a.InPerM != 1.0 || a.OutPerM != 4.0 {
		t.Errorf("Model A = %+v, want InPerM=1 OutPerM=4（K→M ×1000，起步价优先）", a)
	}
	if _, has := table["model b"]; has {
		t.Error("Model B（按秒计费）不应出现在价格表")
	}
	c := table["model c"]
	if c.InPerM != 0 || c.OutPerM != 3.0 {
		t.Errorf("Model C = %+v, want InPerM=0 OutPerM=3", c)
	}
	// 键统一小写
	if _, has := table["Model A"]; has {
		t.Error("price table keys should be lowercased")
	}
}

func TestDashScopeTokenPrice(t *testing.T) {
	newRange := func(rangeName, in, out string) (g struct {
		RangeName string `json:"range_name"`
		Prices    []struct {
			Type  string `json:"type"`
			Price string `json:"price"`
		} `json:"prices"`
	}) {
		g.RangeName = rangeName
		g.Prices = []struct {
			Type  string `json:"type"`
			Price string `json:"price"`
		}{{"input_token", in}, {"output_token", out}}
		return g
	}

	var empty []struct {
		RangeName string `json:"range_name"`
		Prices    []struct {
			Type  string `json:"type"`
			Price string `json:"price"`
		} `json:"prices"`
	}
	if got := dashScopeTokenPrice(empty, "input_token"); got != 0 {
		t.Errorf("empty prices = %v, want 0", got)
	}

	// Default 分段优先于第一个分段
	prices := []struct {
		RangeName string `json:"range_name"`
		Prices    []struct {
			Type  string `json:"type"`
			Price string `json:"price"`
		} `json:"prices"`
	}{newRange("Tier1", "9", "9"), newRange("Default", "0.0003", "0.0006")}
	if got := dashScopeTokenPrice(prices, "input_token"); got != 0.0003 {
		t.Errorf("input_token = %v, want 0.0003（Default 优先）", got)
	}
	// 缺失计费项返回 0（图像按张计费等）
	if got := dashScopeTokenPrice(prices, "image_token"); got != 0 {
		t.Errorf("image_token = %v, want 0", got)
	}
}

func TestSiliconFlowPricingCacheBranches(t *testing.T) {
	// 命中缓存：1 小时内直接返回，不触发网络
	seedSFPricing(t, map[string]siliconFlowPrice{"m1": {InPerM: 1, OutPerM: 2}}, false)
	got := siliconFlowPricing()
	if got == nil || got["m1"].InPerM != 1 {
		t.Errorf("cache hit = %#v, want seeded table", got)
	}

	// 失败负缓存：5 分钟内返回 nil，不触发网络
	seedSFPricing(t, nil, true)
	if got := siliconFlowPricing(); got != nil {
		t.Errorf("negative cache = %#v, want nil", got)
	}
}

func TestResolveProviderConfig(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	if _, err := resolveProviderConfig(d, ""); err == nil {
		t.Error("empty name: expected error")
	}
	if _, err := resolveProviderConfig(d, "ghost"); err == nil {
		t.Error("unknown provider: expected error")
	}

	setProviders(t, d, &goharnessconfig.ProviderConfig{Name: "mine", BaseURL: "http://x", APIKey: "sk-1"})
	p, err := resolveProviderConfig(d, "mine")
	if err != nil || p.BaseURL != "http://x" {
		t.Errorf("resolve = %v, %v", p, err)
	}
}

// onlineModelsTestDaemon 带一个指向 httptest 的供应商（有 Key）与一个无 Key 供应商。
func onlineModelsTestDaemon(t *testing.T, handler http.HandlerFunc) (*Daemon, *httptest.Server) {
	t.Helper()
	d, cleanup := newTestDaemon(t)
	t.Cleanup(cleanup)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	setProviders(t, d,
		&goharnessconfig.ProviderConfig{Name: "remote", BaseURL: srv.URL, APIKey: "sk-test"},
		&goharnessconfig.ProviderConfig{Name: "nokey", BaseURL: srv.URL},
	)
	return d, srv
}

func TestHandleProviderFetchSiliconFlowModels(t *testing.T) {
	payload := `{"data":[{"id":"big-model","owned_by":"org"},{"id":"","owned_by":"x"},{"id":"FREE-1","owned_by":"org"}]}`

	// 价格缓存命中：FREE-1 命中免费价、big-model 命中付费价、未知模型无价
	seedSFPricing(t, map[string]siliconFlowPrice{
		"free-1":    {InPerM: 0, OutPerM: 0},
		"big-model": {InPerM: 2, OutPerM: 8},
	}, false)

	d, _ := onlineModelsTestDaemon(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("Authorization = %q", got)
		}
		if !strings.HasPrefix(r.URL.Path, "/v1/models") {
			t.Errorf("path = %q, want /v1/models 前缀（base_url 需补 /v1）", r.URL.Path)
		}
		_, _ = w.Write([]byte(payload))
	})

	result, err := d.handleProviderFetchSiliconFlowModels(context.Background(), mustParams(t, rpc.FetchSiliconFlowModelsParams{Provider: "remote"}))
	if err != nil {
		t.Fatalf("fetch error = %v", err)
	}
	m := result.(map[string]any)
	if m["total"].(int) != 2 {
		t.Fatalf("total = %v, want 2（空 id 过滤）", m["total"])
	}
	models := m["models"].([]rpc.OnlineModelInfo)
	if models[0].ID != "big-model" || models[0].CostPer1MIn != 2 || models[0].CostPer1MOut != 8 || models[0].Free {
		t.Errorf("big-model = %+v", models[0])
	}
	if models[1].ID != "FREE-1" || !models[1].Free {
		t.Errorf("FREE-1 = %+v, want Free=true", models[1])
	}
}

func TestHandleProviderFetchSiliconFlowModels_Errors(t *testing.T) {
	failSrv := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) }

	d, _ := onlineModelsTestDaemon(t, failSrv)

	if _, err := d.handleProviderFetchSiliconFlowModels(context.Background(), mustParams(t, rpc.FetchSiliconFlowModelsParams{})); err == nil {
		t.Error("empty provider: expected error")
	}
	if _, err := d.handleProviderFetchSiliconFlowModels(context.Background(), mustParams(t, rpc.FetchSiliconFlowModelsParams{Provider: "ghost"})); err == nil {
		t.Error("unknown provider: expected error")
	}
	if _, err := d.handleProviderFetchSiliconFlowModels(context.Background(), mustParams(t, rpc.FetchSiliconFlowModelsParams{Provider: "nokey"})); err == nil {
		t.Error("missing api key: expected error")
	}
	if _, err := d.handleProviderFetchSiliconFlowModels(context.Background(), mustParams(t, rpc.FetchSiliconFlowModelsParams{Provider: "remote"})); err == nil {
		t.Error("HTTP 500: expected error")
	}

	// 非法 JSON
	d, _ = onlineModelsTestDaemon(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("not-json")) })
	if _, err := d.handleProviderFetchSiliconFlowModels(context.Background(), mustParams(t, rpc.FetchSiliconFlowModelsParams{Provider: "remote"})); err == nil {
		t.Error("invalid json: expected error")
	}

	// 连接失败（服务已关闭）
	d2, cleanup := newTestDaemon(t)
	defer cleanup()
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	setProviders(t, d2, &goharnessconfig.ProviderConfig{Name: "dead", BaseURL: closed.URL, APIKey: "sk"})
	if _, err := d2.handleProviderFetchSiliconFlowModels(context.Background(), mustParams(t, rpc.FetchSiliconFlowModelsParams{Provider: "dead"})); err == nil {
		t.Error("connection refused: expected error")
	}
}

func TestHandleProviderFetchOpenRouterModels(t *testing.T) {
	// 免费模型 + 能力标识（tools / image modalities）
	payload := `{"data":[
		{"id":"free/a","name":"Free A","description":"d","context_length":8192,
		 "pricing":{"prompt":"0","completion":"0"},
		 "supported_parameters":["tools"],"architecture":{"input_modalities":["image"]}},
		{"id":"","name":"empty"},
		{"id":"paid/b","name":"Paid B","pricing":{"prompt":"0.000002","completion":"0.000006"},
		 "supported_parameters":[],"architecture":{"input_modalities":[]}}
	]}`
	seedFXRate(t, 7.0) // 付费模型换算需要汇率，走缓存命中避免真连汇率源

	d, _ := onlineModelsTestDaemon(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "sort=most-popular") {
			t.Errorf("query = %q, want sort=most-popular", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(payload))
	})

	result, err := d.handleProviderFetchOpenRouterModels(context.Background(), mustParams(t, rpc.FetchOpenRouterModelsParams{Provider: "remote"}))
	if err != nil {
		t.Fatalf("fetch error = %v", err)
	}
	models := result.(map[string]any)["models"].([]rpc.OnlineModelInfo)
	if len(models) != 2 {
		t.Fatalf("models = %d, want 2（空 id 过滤）", len(models))
	}
	free := models[0]
	if !free.Free || !free.FuncCalling || !free.Visioning || free.ContextLength != 8192 {
		t.Errorf("free model = %+v", free)
	}
	if free.CostPer1MIn != 0 || free.CostPer1MOut != 0 {
		t.Errorf("free model costs = %v/%v, want 0", free.CostPer1MIn, free.CostPer1MOut)
	}
	paid := models[1]
	if paid.Free || paid.CostPer1MIn != 0.000002*1e6*7.0 || paid.CostPer1MOut != 0.000006*1e6*7.0 {
		t.Errorf("paid model = %+v, want cost=14/42（USD/M × 汇率 7）", paid)
	}
}

func TestHandleProviderFetchOpenRouterModels_UpstreamError(t *testing.T) {
	// 上游 500 + error.message JSON → upstreamErrorText 提取 message
	d, _ := onlineModelsTestDaemon(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":{"message":"upstream exploded"}}`))
	})
	_, err := d.handleProviderFetchOpenRouterModels(context.Background(), mustParams(t, rpc.FetchOpenRouterModelsParams{Provider: "remote"}))
	if err == nil || err.Error() != "upstream exploded" {
		t.Errorf("err = %v, want upstream message", err)
	}
}

func TestHandleProviderFetchDashScopeModels(t *testing.T) {
	payload := `{"success":true,"output":{"models":[{
		"model":"qwen-turbo","name":"Qwen Turbo","description":"fast",
		"features":["function-calling"],
		"prices":[{"range_name":"Default","prices":[{"type":"input_token","price":"0.0003"},{"type":"output_token","price":"0.0006"}]}],
		"inference_metadata":{"request_modality":["Image","Text"]},
		"model_info":{"context_window":1000000}
	},{"model":"","name":"empty"}]}}`

	d, _ := onlineModelsTestDaemon(t, func(w http.ResponseWriter, r *http.Request) {
		// 端点复用 base_url 的 host，路径固定 /api/v1/models
		if r.URL.Path != "/api/v1/models" {
			t.Errorf("path = %q, want /api/v1/models", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("Authorization = %q", got)
		}
		w.Write([]byte(payload))
	})

	result, err := d.handleProviderFetchDashScopeModels(context.Background(), mustParams(t, rpc.FetchDashScopeModelsParams{Provider: "remote"}))
	if err != nil {
		t.Fatalf("fetch error = %v", err)
	}
	models := result.(map[string]any)["models"].([]rpc.OnlineModelInfo)
	if len(models) != 1 {
		t.Fatalf("models = %d, want 1", len(models))
	}
	m := models[0]
	if m.ID != "qwen-turbo" || !m.FuncCalling || !m.Visioning || m.ContextLength != 1000000 {
		t.Errorf("model = %+v", m)
	}
	if m.Free || m.CostPer1MIn != 0.0003 || m.CostPer1MOut != 0.0006 {
		t.Errorf("pricing = %+v", m)
	}
}

func TestHandleProviderFetchDashScopeModels_Errors(t *testing.T) {
	// 业务失败：success=false
	d, _ := onlineModelsTestDaemon(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"success":false,"output":{}}`))
	})
	if _, err := d.handleProviderFetchDashScopeModels(context.Background(), mustParams(t, rpc.FetchDashScopeModelsParams{Provider: "remote"})); err == nil {
		t.Error("success=false: expected error")
	}

	// 无 Key
	if _, err := d.handleProviderFetchDashScopeModels(context.Background(), mustParams(t, rpc.FetchDashScopeModelsParams{Provider: "nokey"})); err == nil {
		t.Error("missing api key: expected error")
	}

	// 非法 JSON
	d, _ = onlineModelsTestDaemon(t, func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("{broken")) })
	if _, err := d.handleProviderFetchDashScopeModels(context.Background(), mustParams(t, rpc.FetchDashScopeModelsParams{Provider: "remote"})); err == nil {
		t.Error("invalid json: expected error")
	}
}

func TestHandleProviderFetchBigModelModels(t *testing.T) {
	payload := `{"success":true,"data":[{"id":"glm-4","owned_by":"zhipu"},{"id":"","owned_by":"x"}]}`

	d, _ := onlineModelsTestDaemon(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("path = %q, want /models（自定义镜像 host 原样拼接）", r.URL.Path)
		}
		w.Write([]byte(payload))
	})

	result, err := d.handleProviderFetchBigModelModels(context.Background(), mustParams(t, rpc.FetchBigModelModelsParams{Provider: "remote"}))
	if err != nil {
		t.Fatalf("fetch error = %v", err)
	}
	models := result.(map[string]any)["models"].([]rpc.OnlineModelInfo)
	if len(models) != 1 || models[0].ID != "glm-4" || models[0].OwnedBy != "zhipu" {
		t.Errorf("models = %+v", models)
	}

	// success=false → 业务失败
	d, _ = onlineModelsTestDaemon(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"success":false,"data":[]}`))
	})
	if _, err := d.handleProviderFetchBigModelModels(context.Background(), mustParams(t, rpc.FetchBigModelModelsParams{Provider: "remote"})); err == nil {
		t.Error("success=false: expected error")
	}

	// HTTP 500
	d, _ = onlineModelsTestDaemon(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) })
	if _, err := d.handleProviderFetchBigModelModels(context.Background(), mustParams(t, rpc.FetchBigModelModelsParams{Provider: "remote"})); err == nil {
		t.Error("HTTP 500: expected error")
	}
}

func TestHandleProviderFetchTencentModels(t *testing.T) {
	payload := `{"data":[
		{"id":"keep-1","name":"Keep One","status":"online"},
		{"id":"gone","name":"Pre Offline","status":"pre-offline"},
		{"id":"keep-2","name":"No Status","status":""},
		{"id":"","name":"Empty","status":"online"}
	]}`

	d, _ := onlineModelsTestDaemon(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/models") {
			t.Errorf("path = %q, want /v1/models 前缀", r.URL.Path)
		}
		w.Write([]byte(payload))
	})

	result, err := d.handleProviderFetchTencentModels(context.Background(), mustParams(t, rpc.FetchTencentModelsParams{Provider: "remote"}))
	if err != nil {
		t.Fatalf("fetch error = %v", err)
	}
	models := result.(map[string]any)["models"].([]rpc.OnlineModelInfo)
	if len(models) != 2 || models[0].ID != "keep-1" || models[0].Title != "Keep One" || models[1].ID != "keep-2" {
		t.Errorf("models = %+v, want 仅 online/空 status 保留", models)
	}

	// HTTP 500 → 界面人话错误（不透传上游正文）
	d, _ = onlineModelsTestDaemon(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(502) })
	_, err = d.handleProviderFetchTencentModels(context.Background(), mustParams(t, rpc.FetchTencentModelsParams{Provider: "remote"}))
	if err == nil || !strings.Contains(err.Error(), "腾讯云 TokenHub 暂时不可用") {
		t.Errorf("err = %v, want 人话提示", err)
	}

	// 非法 JSON
	d, _ = onlineModelsTestDaemon(t, func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("nope")) })
	if _, err := d.handleProviderFetchTencentModels(context.Background(), mustParams(t, rpc.FetchTencentModelsParams{Provider: "remote"})); err == nil {
		t.Error("invalid json: expected error")
	}
}

func TestHandleProviderFetchKimiModels(t *testing.T) {
	payload := `{"data":[
		{"id":"moonshot-v1","owned_by":"moonshot","context_length":131072,"supports_image_in":true},
		{"id":"","owned_by":"x"}
	]}`

	d, _ := onlineModelsTestDaemon(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/models") {
			t.Errorf("path = %q, want /v1/models 前缀", r.URL.Path)
		}
		w.Write([]byte(payload))
	})

	result, err := d.handleProviderFetchKimiModels(context.Background(), mustParams(t, rpc.FetchKimiModelsParams{Provider: "remote"}))
	if err != nil {
		t.Fatalf("fetch error = %v", err)
	}
	models := result.(map[string]any)["models"].([]rpc.OnlineModelInfo)
	if len(models) != 1 {
		t.Fatalf("models = %d, want 1", len(models))
	}
	m := models[0]
	if m.ID != "moonshot-v1" || m.OwnedBy != "moonshot" || m.ContextLength != 131072 || !m.Visioning {
		t.Errorf("model = %+v", m)
	}

	// HTTP 500 → upstreamErrorText
	d, _ = onlineModelsTestDaemon(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(503)
		w.Write([]byte(`{"error":{"message":"kimi down"}}`))
	})
	_, err = d.handleProviderFetchKimiModels(context.Background(), mustParams(t, rpc.FetchKimiModelsParams{Provider: "remote"}))
	if err == nil || err.Error() != "kimi down" {
		t.Errorf("err = %v, want kimi down", err)
	}
}

func TestHandleProviderFetchMiniMaxModels(t *testing.T) {
	payload := `{"data":[{"id":"abab6.5","display_name":"Abab 6.5"},{"id":"","display_name":"x"}]}`

	d, _ := onlineModelsTestDaemon(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/models") {
			t.Errorf("path = %q, want /v1/models 前缀", r.URL.Path)
		}
		w.Write([]byte(payload))
	})

	result, err := d.handleProviderFetchMiniMaxModels(context.Background(), mustParams(t, rpc.FetchMiniMaxModelsParams{Provider: "remote"}))
	if err != nil {
		t.Fatalf("fetch error = %v", err)
	}
	models := result.(map[string]any)["models"].([]rpc.OnlineModelInfo)
	if len(models) != 1 || models[0].ID != "abab6.5" || models[0].Title != "Abab 6.5" {
		t.Errorf("models = %+v", models)
	}

	// 无 Key / HTTP 500
	if _, err := d.handleProviderFetchMiniMaxModels(context.Background(), mustParams(t, rpc.FetchMiniMaxModelsParams{Provider: "nokey"})); err == nil {
		t.Error("missing api key: expected error")
	}
	d, _ = onlineModelsTestDaemon(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) })
	if _, err := d.handleProviderFetchMiniMaxModels(context.Background(), mustParams(t, rpc.FetchMiniMaxModelsParams{Provider: "remote"})); err == nil {
		t.Error("HTTP 500: expected error")
	}
}
