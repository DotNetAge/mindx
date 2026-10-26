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

// TestListExperiencesTreePassthrough 清单树字段透传：服务端卡片带 parent_id/position，
// 客户端解析后原样保留（前端按二者组树）。
func TestListExperiencesTreePassthrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"experiences":[
			{"id":1,"title":"根文章","skill_name":"demo","parent_id":null,"position":2},
			{"id":2,"title":"子文章","skill_name":"demo","parent_id":1,"position":1}]}`))
	}))
	defer srv.Close()

	m := NewMarketClient(srv.URL, t.TempDir())
	m.SetAuthToken("tok")
	items, err := m.ListExperiences("")
	if err != nil {
		t.Fatalf("ListExperiences 失败：%v", err)
	}
	if len(items) != 2 {
		t.Fatalf("条目数 = %d， want 2", len(items))
	}
	if items[0].ParentID != nil {
		t.Errorf("根文章 parent_id = %v， want nil", items[0].ParentID)
	}
	if items[0].Position != 2 {
		t.Errorf("根文章 position = %d， want 2", items[0].Position)
	}
	if items[1].ParentID == nil || *items[1].ParentID != 1 {
		t.Errorf("子文章 parent_id = %v， want 1", items[1].ParentID)
	}
	if items[1].Position != 1 {
		t.Errorf("子文章 position = %d， want 1", items[1].Position)
	}
}
