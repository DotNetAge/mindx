package tools

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestViewQueryListLabels 验证 list_labels 动作能正确调用 GET /api/view/labels。
func TestViewQueryListLabels(t *testing.T) {
	var capturedPath string
	var capturedMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedMethod = r.Method
		_, _ = io.WriteString(w, `{"code":0,"data":{"labels":[{"label":"Person","count":12,"description":"员工"}]}}`)
	}))
	defer srv.Close()

	tool := NewViewQueryTool(srv.URL)
	if tool == nil {
		t.Fatal("NewViewQueryTool 返回 nil")
	}

	result, err := tool.Execute(ctxWithLogger(), map[string]any{
		"action": "list_labels",
	})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	if capturedMethod != http.MethodGet {
		t.Errorf("期望 GET，实际 %s", capturedMethod)
	}
	if capturedPath != "/api/view/labels" {
		t.Errorf("期望 /api/view/labels，实际 %s", capturedPath)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("期望 map，实际 %T", result)
	}
	if m["code"].(float64) != 0 {
		t.Errorf("code 应为 0，实际 %v", m["code"])
	}
}

// TestViewQueryDescribe 验证 describe 动作的 query string 构造。
func TestViewQueryDescribe(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path + "?" + r.URL.RawQuery
		_, _ = io.WriteString(w, `{"code":0,"data":{"schema":{"label":"Person","columns":[{"name":"name","type":"string","required":true}]}}}`)
	}))
	defer srv.Close()

	tool := NewViewQueryTool(srv.URL).(*ViewQueryTool)
	_, err := tool.Execute(ctxWithLogger(), map[string]any{
		"action": "describe",
		"label":  "Person",
	})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	if !strings.Contains(capturedPath, "label=Person") {
		t.Errorf("query 应包含 label=Person，实际: %s", capturedPath)
	}
	if !strings.HasPrefix(capturedPath, "/api/view/schema?") {
		t.Errorf("路径错误: %s", capturedPath)
	}
}

// TestViewQueryQuery 验证 query 动作的请求体构造。
func TestViewQueryQuery(t *testing.T) {
	var capturedBody []byte
	var capturedMethod string
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		capturedBody, _ = io.ReadAll(r.Body)
		_, _ = io.WriteString(w, `{"code":0,"data":{"label":"Person","rows":[{"_id":"1","name":"Alice"}],"total":1,"page":1,"size":20}}`)
	}))
	defer srv.Close()

	tool := NewViewQueryTool(srv.URL).(*ViewQueryTool)
	_, err := tool.Execute(ctxWithLogger(), map[string]any{
		"action": "query",
		"label":  "Person",
		"page":   1,
		"size":   20,
		"where": map[string]any{
			"equals": map[string]any{"role": "Engineer"},
		},
		"order_by": []any{
			map[string]any{"field": "name", "desc": false},
		},
	})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	if capturedMethod != http.MethodPost {
		t.Errorf("期望 POST，实际 %s", capturedMethod)
	}
	if capturedPath != "/api/view" {
		t.Errorf("路径错误: %s", capturedPath)
	}

	var got viewQueryRequest
	if err := json.Unmarshal(capturedBody, &got); err != nil {
		t.Fatalf("解析请求体失败: %v", err)
	}
	if got.Label != "Person" {
		t.Errorf("Label 错误: %s", got.Label)
	}
	if got.Page != 1 {
		t.Errorf("Page 错误: %d", got.Page)
	}
	if got.Where == nil || got.Where.Equals["role"] != "Engineer" {
		t.Errorf("Where.Equals 错误: %+v", got.Where)
	}
	if len(got.OrderBy) != 1 || got.OrderBy[0].Field != "name" || got.OrderBy[0].Desc {
		t.Errorf("OrderBy 错误: %+v", got.OrderBy)
	}
}

// TestViewQueryInvalidAction 验证非法 action 报错。
func TestViewQueryInvalidAction(t *testing.T) {
	tool := NewViewQueryTool("http://127.0.0.1:1").(*ViewQueryTool)
	_, err := tool.Execute(ctxWithLogger(), map[string]any{
		"action": "drop",
	})
	if err == nil {
		t.Fatal("应返回错误")
	}
	if !strings.Contains(err.Error(), "下一步我应该") || !strings.Contains(err.Error(), "只支持 list_labels") {
		t.Errorf("错误信息不符: %v", err)
	}
}

// TestViewQueryMissingAction 验证缺失 action 报错。
func TestViewQueryMissingAction(t *testing.T) {
	tool := NewViewQueryTool("http://127.0.0.1:1").(*ViewQueryTool)
	_, err := tool.Execute(ctxWithLogger(), map[string]any{})
	if err == nil {
		t.Fatal("应返回错误")
	}
}

// TestNewViewQueryToolEmptyURL 验证空 URL 时返回 nil。
func TestNewViewQueryToolEmptyURL(t *testing.T) {
	if NewViewQueryTool("") != nil {
		t.Error("空 URL 应返回 nil")
	}
	if NewViewQueryTool("   ") != nil {
		t.Error("空白 URL 应返回 nil")
	}
}

// TestViewQueryInfo 验证 Info 元信息完整性。
func TestViewQueryInfo(t *testing.T) {
	tool := NewViewQueryTool("http://127.0.0.1:8484").(*ViewQueryTool)
	info := tool.Info()
	if info.Name != "ViewQuery" {
		t.Errorf("Name 错误: %s", info.Name)
	}
	if info.Description == "" {
		t.Error("Description 不应为空")
	}
	if info.Prompt == "" {
		t.Error("Prompt 不应为空")
	}
	// 三个动作的提示
	for _, kw := range []string{"list_labels", "describe", "query"} {
		if !strings.Contains(info.Prompt, kw) {
			t.Errorf("Prompt 应包含 %s", kw)
		}
	}
	// 必须有 action 参数
	var hasAction bool
	for _, p := range info.Parameters {
		if p.Name == "action" {
			hasAction = true
			if !p.Required {
				t.Error("action 必填")
			}
		}
	}
	if !hasAction {
		t.Error("缺少 action 参数")
	}
}

// TestViewQueryRangeWhere 验证 range 条件构造。
func TestViewQueryRangeWhere(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		_, _ = io.WriteString(w, `{"code":0,"data":{}}`)
	}))
	defer srv.Close()

	tool := NewViewQueryTool(srv.URL).(*ViewQueryTool)
	_, err := tool.Execute(ctxWithLogger(), map[string]any{
		"action": "query",
		"label":  "Trade",
		"where": map[string]any{
			"range": map[string]any{
				"amount": map[string]any{
					"gte": 100,
					"lte": 500,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}

	var got viewQueryRequest
	if err := json.Unmarshal(capturedBody, &got); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	rng, ok := got.Where.Range["amount"]
	if !ok {
		t.Fatal("Range.amount 缺失")
	}
	if rng.Gte.(float64) != 100 || rng.Lte.(float64) != 500 {
		t.Errorf("Range 错误: %+v", rng)
	}
}

// TestViewQueryInWhere 验证 in 条件构造。
func TestViewQueryInWhere(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		_, _ = io.WriteString(w, `{"code":0,"data":{}}`)
	}))
	defer srv.Close()

	tool := NewViewQueryTool(srv.URL).(*ViewQueryTool)
	_, err := tool.Execute(ctxWithLogger(), map[string]any{
		"action": "query",
		"label":  "Person",
		"where": map[string]any{
			"in": map[string]any{
				"role": []any{"Engineer", "Designer"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}

	var got viewQueryRequest
	if err := json.Unmarshal(capturedBody, &got); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	inList, ok := got.Where.In["role"]
	if !ok || len(inList) != 2 {
		t.Errorf("In.role 错误: %+v", got.Where.In)
	}
}
