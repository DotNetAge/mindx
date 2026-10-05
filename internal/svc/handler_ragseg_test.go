package svc

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"math"
	"path/filepath"
	"testing"

	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/native"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

// segMockEmbedder 确定性字符级词袋向量，用于在不加载 ONNX 模型的情况下
// 驱动 rag.seg.* handler 测试（与 gorag native 包测试的 mock 同思路）。
type segMockEmbedder struct{ dim int }

func newSegMockEmbedder() *segMockEmbedder { return &segMockEmbedder{dim: 64} }

func (m *segMockEmbedder) CalcText(text string) (*core.Vector, error) {
	v := make([]float32, m.dim)
	for _, r := range text {
		if r == ' ' {
			continue
		}
		sum := sha256.Sum256([]byte(string(r)))
		v[int(binary.BigEndian.Uint16(sum[:2]))%m.dim]++
	}
	norm := float32(0)
	for _, x := range v {
		norm += x * x
	}
	norm = float32(math.Sqrt(float64(norm)))
	if norm > 0 {
		for i := range v {
			v[i] /= norm
		}
	}
	return &core.Vector{Values: v}, nil
}

func (m *segMockEmbedder) CalcImage([]byte) (*core.Vector, error) {
	return nil, context.Canceled
}

func (m *segMockEmbedder) Dim() int          { return m.dim }
func (m *segMockEmbedder) Multimoding() bool { return false }

// newTestSegIndexer 为测试 daemon 挂载一个基于临时目录的 SegIndexer。
func newTestSegIndexer(t *testing.T, d *Daemon) {
	t.Helper()
	idx, err := native.NewSegIndexer(filepath.Join(t.TempDir(), "native.db"), newSegMockEmbedder())
	if err != nil {
		t.Fatalf("NewSegIndexer() error = %v", err)
	}
	d.segIndexer = idx
}

func mustSegAdd(t *testing.T, d *Daemon, family, value string, meta map[string]string) {
	t.Helper()
	params, _ := json.Marshal(rpc.SegAddParams{Family: family, Value: value, Meta: meta})
	if _, err := d.handleRAGSegAdd(context.Background(), params); err != nil {
		t.Fatalf("rag.seg.add(%s, %s) error = %v", family, value, err)
	}
}

// TestRAGSegAddSearchClosure 添加后语义检索应命中该条目并返回完整元数据。
func TestRAGSegAddSearchClosure(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()
	newTestSegIndexer(t, d)

	mustSegAdd(t, d, "skill", "websearch", map[string]string{
		"name":        "websearch",
		"title":       "网页搜索",
		"description": "搜索网络上内容",
	})

	params, _ := json.Marshal(rpc.SegSearchParams{Key: "skill", Query: "websearch", TopK: 3})
	result, err := d.handleRAGSegSearch(context.Background(), params)
	if err != nil {
		t.Fatalf("rag.seg.search error = %v", err)
	}
	wrapped, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("search 返回类型异常: %T", result)
	}
	if got := wrapped["count"].(int); got != 1 {
		t.Fatalf("期望命中 1 条，实际 %d", got)
	}
	hits := wrapped["hits"].([]native.SegHit)
	if hits[0].Value != "websearch" {
		t.Errorf("Value 期望 websearch，实际 %q", hits[0].Value)
	}
	if hits[0].Meta["title"] != "网页搜索" {
		t.Errorf("Meta 应含完整元数据: %v", hits[0].Meta)
	}
}

// TestRAGSegList 列出族内条目。
func TestRAGSegList(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()
	newTestSegIndexer(t, d)

	mustSegAdd(t, d, "tool", "lint", map[string]string{"title": "代码检查"})
	mustSegAdd(t, d, "tool", "fmt", map[string]string{"title": "格式化"})

	params, _ := json.Marshal(rpc.SegListParams{Family: "tool", Limit: 10})
	result, err := d.handleRAGSegList(context.Background(), params)
	if err != nil {
		t.Fatalf("rag.seg.list error = %v", err)
	}
	wrapped := result.(map[string]any)
	if got := wrapped["count"].(int); got != 2 {
		t.Fatalf("期望列出 2 条，实际 %d", got)
	}
}

// TestRAGSegNotInitialized 未初始化索引时应报错而非 panic。
// 注意：newTestDaemon 中 app.Embedder() 会因随包语义模型而成功初始化
// segIndexer，这里显式置 nil 模拟"未配置 embedder"的降级场景。
func TestRAGSegNotInitialized(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()
	d.segIndexer = nil

	addParams, _ := json.Marshal(rpc.SegAddParams{Family: "skill", Value: "x"})
	if _, err := d.handleRAGSegAdd(context.Background(), addParams); err == nil {
		t.Error("未初始化时 add 应报错")
	}
	searchParams, _ := json.Marshal(rpc.SegSearchParams{Key: "skill", Query: "x"})
	if _, err := d.handleRAGSegSearch(context.Background(), searchParams); err == nil {
		t.Error("未初始化时 search 应报错")
	}
	listParams, _ := json.Marshal(rpc.SegListParams{Family: "skill"})
	if _, err := d.handleRAGSegList(context.Background(), listParams); err == nil {
		t.Error("未初始化时 list 应报错")
	}
}

// TestRAGSegParamValidation 必填参数缺失应报错。
func TestRAGSegParamValidation(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()
	newTestSegIndexer(t, d)

	cases := []struct {
		name   string
		method string
		params any
	}{
		{"add 无 family", "add", rpc.SegAddParams{Value: "x"}},
		{"add 无 value", "add", rpc.SegAddParams{Family: "skill"}},
		{"search 无 key", "search", rpc.SegSearchParams{Query: "x"}},
		{"search 无 query", "search", rpc.SegSearchParams{Key: "skill"}},
		{"list 无 family", "list", rpc.SegListParams{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params, _ := json.Marshal(tc.params)
			var err error
			switch tc.method {
			case "add":
				_, err = d.handleRAGSegAdd(context.Background(), params)
			case "search":
				_, err = d.handleRAGSegSearch(context.Background(), params)
			case "list":
				_, err = d.handleRAGSegList(context.Background(), params)
			}
			if err == nil {
				t.Errorf("%s 缺参应报错", tc.method)
			}
		})
	}
}
