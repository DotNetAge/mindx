package bundle

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestMoveExperience 挪挂载点封装：PATCH 方法/路径/身份头/请求体逐项断言。
func TestMoveExperience(t *testing.T) {
	var gotMethod, gotPath, gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotMethod, gotPath, gotAuth, gotBody = r.Method, r.URL.Path, r.Header.Get("Authorization"), string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":42,"parent_id":7}`))
	}))
	defer srv.Close()

	m := NewMarketClient(srv.URL, t.TempDir())
	m.SetAuthToken("tok")
	seven := int64(7)
	if err := m.MoveExperience(42, &seven, []int64{3, 7, 42}); err != nil {
		t.Fatalf("MoveExperience 失败：%v", err)
	}
	if gotMethod != http.MethodPatch {
		t.Errorf("方法 = %s， want PATCH", gotMethod)
	}
	if gotPath != "/api/v1/experiences/42/position" {
		t.Errorf("路径 = %s， want /api/v1/experiences/42/position", gotPath)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("身份头 = %q， want Bearer tok", gotAuth)
	}
	var body struct {
		ParentID *int64  `json:"parent_id"`
		Siblings []int64 `json:"siblings"`
	}
	if err := json.Unmarshal([]byte(gotBody), &body); err != nil {
		t.Fatalf("请求体不是合法 JSON：%v（%s）", err, gotBody)
	}
	if body.ParentID == nil || *body.ParentID != 7 {
		t.Errorf("parent_id = %v， want 7", body.ParentID)
	}
	if len(body.Siblings) != 3 || body.Siblings[0] != 3 || body.Siblings[1] != 7 || body.Siblings[2] != 42 {
		t.Errorf("siblings = %v， want [3 7 42]", body.Siblings)
	}

	// 挂回根层级：parent_id 显式 null（服务端 *int64 收 nil = 根层级）
	if err := m.MoveExperience(42, nil, []int64{42}); err != nil {
		t.Fatalf("MoveExperience(根层级) 失败：%v", err)
	}
	var body2 map[string]any
	if err := json.Unmarshal([]byte(gotBody), &body2); err != nil {
		t.Fatalf("请求体不是合法 JSON：%v", err)
	}
	if v, ok := body2["parent_id"]; !ok || v != nil {
		t.Errorf("parent_id = %v（存在=%v）， want 显式 null", v, ok)
	}
}

// TestListCommunityExperiences 统一回路清单映射：服务端经验卡片 → MarketArticle
// 归一形状（source/id/author/date 截取），树挂载字段留在服务端不透传客户端。
func TestListCommunityExperiences(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/experiences" {
			t.Errorf("路径 = %s， want /api/v1/experiences", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"experiences":[
			{"id":1,"title":"社区经验","summary":"摘要","skill_name":"demo",
			 "author_name":"张三","created_at":"2026-10-27T10:00:00Z","parent_id":null,"position":2}]}`))
	}))
	defer srv.Close()

	m := NewMarketClient(srv.URL, t.TempDir())
	m.SetAuthToken("tok")
	items, err := m.ListCommunityExperiences("")
	if err != nil {
		t.Fatalf("ListCommunityExperiences 失败：%v", err)
	}
	if len(items) != 1 {
		t.Fatalf("条目数 = %d， want 1", len(items))
	}
	a := items[0]
	if a.Source != "experience" || a.ID != 1 || a.Title != "社区经验" || a.SkillName != "demo" {
		t.Errorf("归一条目不符：%+v", a)
	}
	if a.AuthorName != "张三" {
		t.Errorf("作者 = %q， want 张三", a.AuthorName)
	}
	if a.Date != "2026-10-27" {
		t.Errorf("日期截取 = %q， want 2026-10-27", a.Date)
	}
}
