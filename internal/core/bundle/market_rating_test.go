package bundle

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// capturedRequest 记录一次入站请求的关键信息（路径/方法/身份头/请求体）。
type capturedRequest struct {
	method string
	path   string
	auth   string
	client string
	body   string
}

// ratingFixture 假市场服务：按路由返回可控行为，记录全部入站请求（并发安全）。
type ratingFixture struct {
	mu       sync.Mutex
	requests []capturedRequest
	srv      *httptest.Server
}

// newRatingFixture 起假市场服务。路由覆盖懒登记（/clients）、评分读写、绑定、Me。
func newRatingFixture(t *testing.T, ratingStatus int, bindMigrated int64) *ratingFixture {
	t.Helper()
	f := &ratingFixture{}
	mux := http.NewServeMux()
	record := func(w http.ResponseWriter, r *http.Request, status int, resp string) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.requests = append(f.requests, capturedRequest{
			method: r.Method, path: r.URL.Path,
			auth: r.Header.Get("Authorization"), client: r.Header.Get("X-Client-Id"),
			body: string(body),
		})
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(resp))
	}
	mux.HandleFunc("/api/v1/clients", func(w http.ResponseWriter, r *http.Request) {
		record(w, r, http.StatusOK, `{}`)
	})
	mux.HandleFunc("/api/v1/clients/bind", func(w http.ResponseWriter, r *http.Request) {
		record(w, r, http.StatusOK,
			`{"bound":true,"migrated":`+itoa(bindMigrated)+`}`)
	})
	mux.HandleFunc("/api/v1/packages/agent/demo/rating", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			record(w, r, ratingStatus, `{"score":4}`)
			return
		}
		record(w, r, http.StatusOK, `{"rating_avg":4.5,"rating_count":2}`)
	})
	mux.HandleFunc("/api/v1/me", func(w http.ResponseWriter, r *http.Request) {
		record(w, r, http.StatusOK,
			`{"id":1,"provider":"github","login":"octo","nick_name":"Octo","points":0,"tos_accepted":true}`)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func itoa(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// reqs 返回入站请求快照。
func (f *ratingFixture) reqs() []capturedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]capturedRequest, len(f.requests))
	copy(out, f.requests)
	return out
}

// newRatingClient 构造指向假服务的市场客户端（cacheDir 由 NewMarketClient 哈希落子目录）。
func newRatingClient(t *testing.T, f *ratingFixture) *MarketClient {
	t.Helper()
	return NewMarketClient(f.srv.URL, t.TempDir())
}

func TestRateLazyRegistersAndCarriesIdentity(t *testing.T) {
	f := newRatingFixture(t, http.StatusOK, 0)
	m := newRatingClient(t, f)
	m.SetClientID("11111111-2222-4333-8444-555555555555")

	agg, err := m.Rate("agent", "demo", 5)
	if err != nil {
		t.Fatalf("Rate() error = %v", err)
	}
	if agg.Avg != 4.5 || agg.Count != 2 {
		t.Errorf("聚合 = %v, want avg 4.5 count 2", agg)
	}

	reqs := f.reqs()
	// 懒登记：首个身份动作先幂等注册，再评分
	if len(reqs) != 2 {
		t.Fatalf("入站请求数 = %d, want 2（登记 + 评分）", len(reqs))
	}
	if reqs[0].method != http.MethodPost || reqs[0].path != "/api/v1/clients" {
		t.Errorf("首个请求 = %s %s, want POST /api/v1/clients", reqs[0].method, reqs[0].path)
	}
	var regBody struct {
		ClientID string `json:"client_id"`
	}
	if err := json.Unmarshal([]byte(reqs[0].body), &regBody); err != nil {
		t.Fatalf("登记请求体解析失败：%v", err)
	}
	if regBody.ClientID != "11111111-2222-4333-8444-555555555555" {
		t.Errorf("登记 client_id = %q, want 设备身份", regBody.ClientID)
	}
	if reqs[1].path != "/api/v1/packages/agent/demo/rating" {
		t.Errorf("评分请求路径 = %q", reqs[1].path)
	}
	var rateBody struct {
		Score int64 `json:"score"`
	}
	if err := json.Unmarshal([]byte(reqs[1].body), &rateBody); err != nil || rateBody.Score != 5 {
		t.Errorf("评分请求体 = %q, err = %v, want score 5", reqs[1].body, err)
	}
	if reqs[1].client != "11111111-2222-4333-8444-555555555555" {
		t.Errorf("评分请求 X-Client-Id = %q, want 设备身份", reqs[1].client)
	}

	// 已登记后再次评分：不再重复登记
	if _, err := m.Rate("agent", "demo", 4); err != nil {
		t.Fatalf("第二次 Rate() error = %v", err)
	}
	if reqs := f.reqs(); len(reqs) != 3 {
		t.Errorf("第二次评分后入站请求数 = %d, want 3（无重复登记）", len(reqs))
	}
}

func TestRateScoreBounds(t *testing.T) {
	f := newRatingFixture(t, http.StatusOK, 0)
	m := newRatingClient(t, f)
	for _, score := range []int64{0, -1, 6, 100} {
		if _, err := m.Rate("agent", "demo", score); err == nil {
			t.Errorf("score=%d 应报错", score)
		}
	}
	if reqs := f.reqs(); len(reqs) != 0 {
		t.Errorf("越界评分不应发出任何请求，got %d", len(reqs))
	}
}

func TestAttachIdentityAuthTokenOverPAT(t *testing.T) {
	f := newRatingFixture(t, http.StatusOK, 0)
	m := newRatingClient(t, f)
	m.SetPAT("pat_secret")
	m.SetAuthToken("aut_secret")

	// MyRating 走 GET，透传身份头
	score, err := m.MyRating("agent", "demo")
	if err != nil {
		t.Fatalf("MyRating() error = %v", err)
	}
	if score != 4 {
		t.Errorf("score = %d, want 4", score)
	}
	reqs := f.reqs()
	if len(reqs) != 1 {
		t.Fatalf("入站请求数 = %d, want 1", len(reqs))
	}
	if reqs[0].auth != "Bearer aut_secret" {
		t.Errorf("Authorization = %q, want authToken 优先于 pat", reqs[0].auth)
	}
}

func TestMyRatingNotFound(t *testing.T) {
	f := newRatingFixture(t, http.StatusNotFound, 0)
	m := newRatingClient(t, f)

	if _, err := m.MyRating("agent", "demo"); err != ErrRatingNotFound {
		t.Errorf("404 应映射 ErrRatingNotFound, got %v", err)
	}
}

func TestBindFlow(t *testing.T) {
	f := newRatingFixture(t, http.StatusOK, 3)
	m := newRatingClient(t, f)
	m.SetClientID("99999999-8888-4777-8666-555555555555")

	// 无 authToken：直接报错，不发请求
	if _, err := m.Bind(); err == nil {
		t.Fatal("无 authToken 的 Bind 应报错")
	}
	if reqs := f.reqs(); len(reqs) != 0 {
		t.Fatalf("无 authToken 不应发出请求, got %d", len(reqs))
	}

	m.SetAuthToken("aut_secret")
	migrated, err := m.Bind()
	if err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if migrated != 3 {
		t.Errorf("migrated = %d, want 3", migrated)
	}
	reqs := f.reqs()
	// 懒登记 + 绑定
	if len(reqs) != 2 {
		t.Fatalf("入站请求数 = %d, want 2", len(reqs))
	}
	if reqs[0].path != "/api/v1/clients" {
		t.Errorf("首个请求 = %s, want /api/v1/clients（懒登记）", reqs[0].path)
	}
	if reqs[1].path != "/api/v1/clients/bind" {
		t.Errorf("绑定请求 = %s, want /api/v1/clients/bind", reqs[1].path)
	}
	if reqs[1].auth != "Bearer aut_secret" {
		t.Errorf("绑定 Authorization = %q, want 用户令牌", reqs[1].auth)
	}
}

func TestMeWithoutToken(t *testing.T) {
	f := newRatingFixture(t, http.StatusOK, 0)
	m := newRatingClient(t, f)
	if _, err := m.Me(); err == nil {
		t.Fatal("无 authToken 的 Me 应报错")
	}
}

func TestMeAndBindValid(t *testing.T) {
	f := newRatingFixture(t, http.StatusOK, 0)
	m := newRatingClient(t, f)
	m.SetAuthToken("aut_secret")

	u, err := m.Me()
	if err != nil {
		t.Fatalf("Me() error = %v", err)
	}
	if u.Login != "octo" || u.Provider != "github" || u.ID != 1 {
		t.Errorf("用户信息 = %+v, want octo/github/1", u)
	}
}
