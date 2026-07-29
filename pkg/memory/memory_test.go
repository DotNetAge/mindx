package memory

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
	"time"

	goharnessmemory "github.com/DotNetAge/goharness/memory"
	goragcore "github.com/DotNetAge/gorag/v2/core"
	goragindexer "github.com/DotNetAge/gorag/v2/indexer"
	"github.com/DotNetAge/gorag/v2/logging"
	"github.com/DotNetAge/gorag/v2/store/vector/govector"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// mockEmbedder — 返回固定维度的单位向量
// ---------------------------------------------------------------------------

type mockEmbedder struct {
	dim int
}

// contentHash 根据文本内容生成确定性向量（相同文本产生相同向量，不同文本向量不同）。
func (m *mockEmbedder) embedText(text string) []float32 {
	h := sha256.Sum256([]byte(text))
	vals := make([]float32, m.dim)
	for i := 0; i < m.dim; i++ {
		// 将哈希的每个字节映射到 [-1, 1] 范围
		vals[i] = float32(h[i%32])/128.0 - 1.0
	}
	// 归一化
	var norm float32
	for _, v := range vals {
		norm += v * v
	}
	if norm > 0 {
		norm = 1.0 / sqrtFloat32(norm)
		for i := range vals {
			vals[i] *= norm
		}
	}
	return vals
}

func sqrtFloat32(x float32) float32 {
	// 简单牛顿法求平方根
	if x <= 0 {
		return 0
	}
	approx := x
	for i := 0; i < 10; i++ {
		approx = (approx + x/approx) / 2
	}
	return approx
}

func (m *mockEmbedder) CalcText(text string) (*goragcore.Vector, error) {
	return &goragcore.Vector{
		ID:      uuid.New().String(),
		ChunkID: uuid.New().String(),
		Values:  m.embedText(text),
	}, nil
}

func (m *mockEmbedder) CalcImage(data []byte) (*goragcore.Vector, error) {
	return &goragcore.Vector{
		ID:      uuid.New().String(),
		ChunkID: uuid.New().String(),
		Values:  m.embedText(string(data)),
	}, nil
}

func (m *mockEmbedder) Dim() int         { return m.dim }
func (m *mockEmbedder) Multimoding() bool { return false }

// ---------------------------------------------------------------------------
// newTestRAGMemory — 创建测试用的 RAGMemory（临时向量库 + mock embedder）
// ---------------------------------------------------------------------------

func newTestRAGMemory(t *testing.T) (*RAGMemory, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	emb := &mockEmbedder{dim: 64}
	noopLogger := logging.DefaultNoopLogger()

	vecDir := filepath.Join(tmpDir, "vectors")
	if err := os.MkdirAll(vecDir, 0755); err != nil {
		t.Fatalf("mkdir vecDir: %v", err)
	}

	vs, err := govector.NewStore(
		govector.WithCollection("test_mem"),
		govector.WithDimension(emb.Dim()),
		govector.WithDBPath(filepath.Join(vecDir, "test.db")),
		govector.WithHNSW(true),
	)
	if err != nil {
		t.Fatalf("govector.NewStore: %v", err)
	}

	semIdx, err := goragindexer.NewSemanticIndexer(vs, emb,
		goragindexer.WithSemanticLogger(noopLogger),
	)
	if err != nil {
		t.Fatalf("NewSemanticIndexer: %v", err)
	}

	mem := NewRAGMemory(semIdx, WithEmbedder(emb))
	mem.logger = noopLogger // 防止 RetrieveLatest 等使用 logger 时 panic

	cleanup := func() {
		if closer, ok := semIdx.(goragindexer.IndexerCloser); ok {
			_ = closer.Close(context.Background())
		}
	}

	return mem, cleanup
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestRAGMemory_StoreAndRetrieve(t *testing.T) {
	mem, cleanup := newTestRAGMemory(t)
	defer cleanup()

	ctx := context.Background()

	// Store 一条记忆
	chunk := goharnessmemory.MemoryChunk{
		Content:   "今天天气很好",
		Summary:   "天气",
		AgentName: "test-agent",
		SessionID: "sess_001",
		Timestamp: time.Now(),
	}
	id, err := mem.Store(ctx, chunk)
	if err != nil {
		t.Fatalf("Store error = %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}

	// Retrieve 检索（内容匹配查询向量，mock embedder 固定返回全1向量，应能命中）
	results, err := mem.Retrieve(ctx, "天气")
	if err != nil {
		t.Fatalf("Retrieve error = %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}
	// storeMemoryChunk 拼接了 Summary + "\n" + Content
	expected := "天气\n今天天气很好"
	if results[0].Content != expected {
		t.Errorf("expected content %q, got %q", expected, results[0].Content)
	}
}

func TestRAGMemory_StoreAndCount(t *testing.T) {
	mem, cleanup := newTestRAGMemory(t)
	defer cleanup()

	ctx := context.Background()

	// Store 3 条记忆
	chunks := []goharnessmemory.MemoryChunk{
		{Content: "第一条记忆", Summary: "一", AgentName: "agent-a", Timestamp: time.Now()},
		{Content: "第二条记忆", Summary: "二", AgentName: "agent-a", Timestamp: time.Now()},
		{Content: "第三条记忆", Summary: "三", AgentName: "agent-b", Timestamp: time.Now()},
	}
	for _, c := range chunks {
		_, err := mem.Store(ctx, c)
		if err != nil {
			t.Fatalf("Store error = %v", err)
		}
	}

	// 通过 Semantic Indexer 的 Count 验证
	admin, ok := mem.Semantic().(goragindexer.IndexerAdmin)
	if !ok {
		t.Fatal("semantic indexer does not implement IndexerAdmin")
	}
	count, err := admin.Count(ctx)
	if err != nil {
		t.Fatalf("Count error = %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 chunks, got %d", count)
	}
}

func TestRAGMemory_Delete(t *testing.T) {
	mem, cleanup := newTestRAGMemory(t)
	defer cleanup()

	ctx := context.Background()

	// Store 一条记忆
	id, err := mem.Store(ctx, goharnessmemory.MemoryChunk{
		Content:   "待删除的记忆",
		Summary:   "删除测试",
		AgentName: "test-agent",
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("Store error = %v", err)
	}

	// 验证存在
	admin, ok := mem.Semantic().(goragindexer.IndexerAdmin)
	if !ok {
		t.Fatal("semantic indexer does not implement IndexerAdmin")
	}
	count, _ := admin.Count(ctx)
	if count != 1 {
		t.Fatalf("expected 1 chunk before delete, got %d", count)
	}

	// 删除
	if err := mem.Delete(ctx, id); err != nil {
		t.Fatalf("Delete error = %v", err)
	}

	// 验证已删除
	count, _ = admin.Count(ctx)
	if count != 0 {
		t.Errorf("expected 0 chunks after delete, got %d", count)
	}
}

func TestRAGMemory_Delete_EmptyID(t *testing.T) {
	mem, cleanup := newTestRAGMemory(t)
	defer cleanup()

	err := mem.Delete(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty id")
	}
}

func TestRAGMemory_RetrieveLatest(t *testing.T) {
	mem, cleanup := newTestRAGMemory(t)
	defer cleanup()

	ctx := context.Background()

	now := time.Now()

	// Store 多条记忆，时间戳不同
	for i := 0; i < 5; i++ {
		ts := now.Add(-time.Duration(5-i) * time.Hour) // 第5条最新
		chunk := goharnessmemory.MemoryChunk{
			Content:   "记忆内容",
			Summary:   "记忆",
			AgentName: "agent-a",
			SessionID: "sess_001",
			Tags:      []string{"tag_a"},
			Timestamp: ts,
		}
		_, err := mem.Store(ctx, chunk)
		if err != nil {
			t.Fatalf("Store error = %v", err)
		}
	}

	// RetrieveLatest 取 3 条
	results, err := mem.RetrieveLatest(ctx, "agent-a", "", 3)
	if err != nil {
		t.Fatalf("RetrieveLatest error = %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// 验证按时间倒序（最新的在前）
	for i := 1; i < len(results); i++ {
		if results[i].Timestamp.After(results[i-1].Timestamp) {
			t.Errorf("results not in descending order at index %d", i)
		}
	}
}

func TestRAGMemory_RetrieveLatest_FilterByAgent(t *testing.T) {
	mem, cleanup := newTestRAGMemory(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()

	// Store 不同 agent 的记忆
	_, _ = mem.Store(ctx, goharnessmemory.MemoryChunk{
		Content: "agent-b 的记忆", Summary: "b",
		AgentName: "agent-b", Timestamp: now,
	})
	_, _ = mem.Store(ctx, goharnessmemory.MemoryChunk{
		Content: "agent-a 的记忆 1", Summary: "a1",
		AgentName: "agent-a", Timestamp: now,
	})
	_, _ = mem.Store(ctx, goharnessmemory.MemoryChunk{
		Content: "agent-a 的记忆 2", Summary: "a2",
		AgentName: "agent-a", Timestamp: now,
	})

	// 只取 agent-a 的
	results, err := mem.RetrieveLatest(ctx, "agent-a", "", 10)
	if err != nil {
		t.Fatalf("RetrieveLatest error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results for agent-a, got %d", len(results))
	}
	for _, r := range results {
		if r.AgentName != "agent-a" {
			t.Errorf("expected agent-a, got %s", r.AgentName)
		}
	}
}

func TestRAGMemory_RetrieveBySession(t *testing.T) {
	mem, cleanup := newTestRAGMemory(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()

	// Store 不同 session 的记忆
	_, _ = mem.Store(ctx, goharnessmemory.MemoryChunk{
		Content: "session 1 的记忆", Summary: "s1",
		AgentName: "agent-a", SessionID: "sess_001", Timestamp: now,
	})
	_, _ = mem.Store(ctx, goharnessmemory.MemoryChunk{
		Content: "session 2 的记忆", Summary: "s2",
		AgentName: "agent-a", SessionID: "sess_002", Timestamp: now,
	})
	_, _ = mem.Store(ctx, goharnessmemory.MemoryChunk{
		Content: "session 1 的另一条", Summary: "s1b",
		AgentName: "agent-a", SessionID: "sess_001", Timestamp: now,
	})

	results, err := mem.RetrieveBySession(ctx, "sess_001", 10)
	if err != nil {
		t.Fatalf("RetrieveBySession error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results for sess_001, got %d", len(results))
	}
	for _, r := range results {
		if r.SessionID != "sess_001" {
			t.Errorf("expected sess_001, got %s", r.SessionID)
		}
	}
}

func TestRAGMemory_StoreMemoryChunks(t *testing.T) {
	mem, cleanup := newTestRAGMemory(t)
	defer cleanup()

	ctx := context.Background()

	chunks := []goharnessmemory.MemoryChunk{
		{Content: "批量记忆 1", Summary: "b1", AgentName: "agent-a", Timestamp: time.Now()},
		{Content: "批量记忆 2", Summary: "b2", AgentName: "agent-a", Timestamp: time.Now()},
	}

	if err := mem.StoreMemoryChunks(ctx, chunks); err != nil {
		t.Fatalf("Storegoharnessmemory.MemoryChunks error = %v", err)
	}

	admin, ok := mem.Semantic().(goragindexer.IndexerAdmin)
	if !ok {
		t.Fatal("semantic indexer does not implement IndexerAdmin")
	}
	count, _ := admin.Count(ctx)
	if count != 2 {
		t.Errorf("expected 2 chunks, got %d", count)
	}
}

func TestRAGMemory_EmptyChunks(t *testing.T) {
	mem, cleanup := newTestRAGMemory(t)
	defer cleanup()

	// 空切片不应报错
	if err := mem.StoreMemoryChunks(context.Background(), nil); err != nil {
		t.Errorf("nil chunks should not error, got: %v", err)
	}
	if err := mem.StoreMemoryChunks(context.Background(), []goharnessmemory.MemoryChunk{}); err != nil {
		t.Errorf("empty chunks should not error, got: %v", err)
	}
}
