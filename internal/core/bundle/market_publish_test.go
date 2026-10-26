package bundle

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
)

// publishFixture 假市场服务：覆盖发布/投递/收件箱/决策路由，记录入站请求关键信息。
type publishFixture struct {
	mu       sync.Mutex
	requests []publishCaptured
	srv      *httptest.Server
}

// publishCaptured 记录一次入站请求（路径/方法/身份/包校验头/请求体）。
type publishCaptured struct {
	method    string
	path      string
	auth      string
	pkgSha    string
	pkgVer    string
	bodyBytes []byte
}

// newPublishFixture 起假市场服务（路由按发布/投递契约返回固定响应）。
func newPublishFixture(t *testing.T) *publishFixture {
	t.Helper()
	f := &publishFixture{}
	mux := http.NewServeMux()
	record := func(w http.ResponseWriter, r *http.Request, resp string) {
		body, _ := io.ReadAll(r.Body)
		path := r.URL.Path
		if r.URL.RawQuery != "" {
			path += "?" + r.URL.RawQuery
		}
		f.mu.Lock()
		f.requests = append(f.requests, publishCaptured{
			method: r.Method, path: path,
			auth: r.Header.Get("Authorization"), pkgSha: r.Header.Get("X-Pkg-Sha256"),
			pkgVer: r.Header.Get("X-Pkg-Version"), bodyBytes: body,
		})
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(resp))
	}
	mux.HandleFunc("/api/v1/packages/skill/demo/versions", func(w http.ResponseWriter, r *http.Request) {
		record(w, r, `{"package_id":1,"version":"0.2.0","sha256":"abc","created":true}`)
	})
	mux.HandleFunc("/api/v1/packages/skill/derived-name/versions", func(w http.ResponseWriter, r *http.Request) {
		record(w, r, `{"package_id":2,"version":"0.1.0","sha256":"abc","created":true}`)
	})
	mux.HandleFunc("/api/v1/packages/skill/demo/submissions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			record(w, r, `{"submissions":[{"id":7,"from_user_id":"0a3f5b1e-6c2d-4a8b-9e1f-2b3c4d5e6f70","from_login":"dev-b","version":"0.3.0","sha256":"def","note":"修了个 bug","status":"pending","created_at":"2026-10-08T00:00:00Z"}]}`)
			return
		}
		record(w, r, `{"submission_id":7,"version":"0.3.0","sha256":"def","created":true}`)
	})
	mux.HandleFunc("/api/v1/packages/skill/demo/submissions/7/accept", func(w http.ResponseWriter, r *http.Request) {
		record(w, r, `{"submission_id":7,"pkg_id":1,"version":"0.3.0","sha256":"def","created":true}`)
	})
	mux.HandleFunc("/api/v1/packages/skill/demo/submissions/7/reject", func(w http.ResponseWriter, r *http.Request) {
		record(w, r, `{"submission_id":7,"status":"rejected"}`)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *publishFixture) reqs() []publishCaptured {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]publishCaptured, len(f.requests))
	copy(out, f.requests)
	return out
}

// mustBuildSkillPkg 构造一个含发布覆盖项的技能包字节（测试辅助）。
func mustBuildSkillPkg(t *testing.T, _ string, ov *PublishOverrides) []byte {
	t.Helper()
	root, _, skills := newTestStores(t)
	mustWriteSkill(t, filepath.Join(root, "skills"), "deploy-check", "部署检查经验", nil)
	if err := skills.ReloadGlobal(); err != nil {
		t.Fatalf("重载全局库失败: %v", err)
	}
	data, _, err := BuildSkillBytes(skills, "deploy-check", ov)
	if err != nil {
		t.Fatalf("BuildSkillBytes 失败: %v", err)
	}
	return data
}

func TestBuildSkillBytesOverrides(t *testing.T) {
	data := mustBuildSkillPkg(t, t.TempDir(), &PublishOverrides{
		Version:     "0.2.0",
		License:     "MIT",
		DerivedFrom: &DerivedRef{Kind: "skill", Name: "base-skill", Version: "0.1.0"},
	})

	// 内存解包读取清单（包字节直传服务端前，声明字段必须已在清单中）
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("解包失败: %v", err)
	}
	var raw []byte
	for _, f := range zr.File {
		if f.Name == zipPathManifest {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("打开清单失败: %v", err)
			}
			raw, _ = io.ReadAll(rc)
			_ = rc.Close()
			break
		}
	}
	if raw == nil {
		t.Fatal("包内缺少 manifest.json")
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("清单解析失败: %v", err)
	}
	if m.Version != "0.2.0" || m.License != "MIT" {
		t.Errorf("清单版本/许可 = %q/%q, want 0.2.0/MIT", m.Version, m.License)
	}
	if m.DerivedFrom == nil || m.DerivedFrom.Kind != "skill" || m.DerivedFrom.Name != "base-skill" || m.DerivedFrom.Version != "0.1.0" {
		t.Errorf("清单谱系 = %+v, want skill/base-skill@0.1.0", m.DerivedFrom)
	}
}

func TestPublishOverridesSelfRefRejected(t *testing.T) {
	m := &Manifest{Kind: KindSkill, Name: "demo"}
	ov := &PublishOverrides{Version: "0.1.0", DerivedFrom: &DerivedRef{Kind: "skill", Name: "demo"}}
	if err := ov.apply(m); err == nil {
		t.Fatal("指向自身的谱系声明应被拒绝")
	}
	// 上游 kind 枚举外（如 ghost）拒绝
	if err := (&PublishOverrides{DerivedFrom: &DerivedRef{Kind: "ghost", Name: "x"}}).apply(m); err == nil {
		t.Fatal("非法谱系 kind 应被拒绝")
	}
	// 缺上游包名拒绝
	if err := (&PublishOverrides{DerivedFrom: &DerivedRef{Kind: "agent"}}).apply(m); err == nil {
		t.Fatal("缺少上游包名的谱系声明应被拒绝")
	}
}

func TestMarketClientPublish(t *testing.T) {
	f := newPublishFixture(t)
	m := NewMarketClient(f.srv.URL, t.TempDir())

	// 无身份：直接报错不发请求
	if _, err := m.Publish("skill", "demo", []byte("pkg"), "0.2.0", ""); err == nil {
		t.Fatal("无身份发布应报错")
	}
	if reqs := f.reqs(); len(reqs) != 0 {
		t.Fatalf("无身份不应发出请求, got %d", len(reqs))
	}

	m.SetAuthToken("aut_secret")
	data := []byte("fake-mindpkg-bytes")
	res, err := m.Publish("skill", "demo", data, "0.2.0", "")
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if !res.Created || res.PackageID != 1 || res.Version != "0.2.0" {
		t.Errorf("发布响应 = %+v", res)
	}
	reqs := f.reqs()
	if len(reqs) != 1 {
		t.Fatalf("入站请求数 = %d, want 1", len(reqs))
	}
	r := reqs[0]
	if r.method != http.MethodPost || r.path != "/api/v1/packages/skill/demo/versions" {
		t.Errorf("发布请求 = %s %s", r.method, r.path)
	}
	if r.auth != "Bearer aut_secret" {
		t.Errorf("Authorization = %q, want 用户令牌", r.auth)
	}
	sum := sha256.Sum256(data)
	if r.pkgSha != hex.EncodeToString(sum[:]) {
		t.Errorf("X-Pkg-Sha256 = %q, want 包内容摘要", r.pkgSha)
	}
	if r.pkgVer != "0.2.0" {
		t.Errorf("X-Pkg-Version = %q, want 0.2.0", r.pkgVer)
	}
	if !bytes.Equal(r.bodyBytes, data) {
		t.Errorf("请求体不符：%d 字节", len(r.bodyBytes))
	}
}

func TestMarketClientPublishRename(t *testing.T) {
	// 发布名覆盖（§4.2 衍生改名发布）：URL 与请求头按新名，清单 name 由打包覆盖
	f := newPublishFixture(t)
	m := NewMarketClient(f.srv.URL, t.TempDir())
	m.SetAuthToken("aut_secret")
	if _, err := m.Publish("skill", "local-name", []byte("pkg"), "0.1.0", "derived-name"); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	reqs := f.reqs()
	if len(reqs) != 1 {
		t.Fatalf("入站请求数 = %d, want 1", len(reqs))
	}
	if reqs[0].path != "/api/v1/packages/skill/derived-name/versions" {
		t.Fatalf("发布路径 = %s, want /api/v1/packages/skill/derived-name/versions", reqs[0].path)
	}
	if reqs[0].pkgVer != "0.1.0" {
		t.Fatalf("X-Pkg-Version = %q, want 0.1.0", reqs[0].pkgVer)
	}
}

func TestMarketClientSubmit(t *testing.T) {
	f := newPublishFixture(t)
	m := NewMarketClient(f.srv.URL, t.TempDir())
	m.SetAuthToken("aut_secret")

	data := []byte("fake-evolved-pkg")
	res, err := m.Submit("skill", "demo", "修了个 bug", "0.3.0", data)
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if res.SubmissionID != 7 || res.Version != "0.3.0" || !res.Created {
		t.Errorf("投递响应 = %+v", res)
	}
	reqs := f.reqs()
	if len(reqs) != 1 {
		t.Fatalf("入站请求数 = %d, want 1", len(reqs))
	}
	r := reqs[0]
	if r.path != "/api/v1/packages/skill/demo/submissions?note=%E4%BF%AE%E4%BA%86%E4%B8%AA+bug" {
		t.Errorf("投递请求路径 = %q, want note 进 query", r.path)
	}
	if r.pkgSha == "" || r.pkgVer != "0.3.0" {
		t.Errorf("投递校验头 = sha %q ver %q", r.pkgSha, r.pkgVer)
	}
}

func TestMarketClientSubmissionsAndDecide(t *testing.T) {
	f := newPublishFixture(t)
	m := NewMarketClient(f.srv.URL, t.TempDir())
	m.SetAuthToken("aut_secret")

	subs, err := m.Submissions("skill", "demo")
	if err != nil {
		t.Fatalf("Submissions() error = %v", err)
	}
	if len(subs) != 1 || subs[0].ID != 7 || subs[0].FromLogin != "dev-b" || subs[0].Status != "pending" {
		t.Fatalf("收件箱 = %+v", subs)
	}

	res, err := m.DecideSubmission("skill", "demo", 7, true)
	if err != nil {
		t.Fatalf("DecideSubmission(accept) error = %v", err)
	}
	if res.SubmissionID != 7 || res.PkgID != 1 || res.Version != "0.3.0" || !res.Created {
		t.Errorf("采纳响应 = %+v", res)
	}
	res, err = m.DecideSubmission("skill", "demo", 7, false)
	if err != nil {
		t.Fatalf("DecideSubmission(reject) error = %v", err)
	}
	if res.Status != "rejected" {
		t.Errorf("拒绝响应 = %+v", res)
	}
	reqs := f.reqs()
	if len(reqs) != 3 {
		t.Fatalf("入站请求数 = %d, want 3（收件箱+采纳+拒绝）", len(reqs))
	}
	if reqs[1].path != "/api/v1/packages/skill/demo/submissions/7/accept" {
		t.Errorf("采纳请求路径 = %q", reqs[1].path)
	}
	if reqs[2].path != "/api/v1/packages/skill/demo/submissions/7/reject" {
		t.Errorf("拒绝请求路径 = %q", reqs[2].path)
	}
}
