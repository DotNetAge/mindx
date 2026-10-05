package svc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/gorag/v2/native"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

// ---------------------------------------------------------------------------
// rag.seg.* JSON-RPC handlers — NativeRAG 条目语义索引（族 = Collection）
// 数据存储在 ~/.mindx/data/native.db，daemon 持有唯一写连接；
// CLI 与壳经 RPC 访问，禁止旁路直开库（bbolt 单写者）。
// ---------------------------------------------------------------------------

// handleRAGSegAdd 添加（或覆盖更新）一个条目。
// meta 的每个键值对会生成一条子键向量，任一维度命中即可召回该条目。
func (d *Daemon) handleRAGSegAdd(ctx context.Context, params json.RawMessage) (any, error) {
	var p rpc.SegAddParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Family == "" {
		return nil, fmt.Errorf("family is required")
	}
	if p.Value == "" {
		return nil, fmt.Errorf("value is required")
	}
	if d.segIndexer == nil {
		return nil, fmt.Errorf("rag seg index not initialized (no embedder configured)")
	}

	if err := d.segIndexer.Add(ctx, p.Family, p.Value, p.Meta); err != nil {
		return nil, fmt.Errorf("rag seg add failed: %w", err)
	}
	d.logger.Debug("rag.seg.add", "family", p.Family, "meta_keys", len(p.Meta))
	return map[string]any{
		"status": "ok",
		"family": p.Family,
	}, nil
}

// handleRAGSegSearch 语义检索：主值与全部子键联合检索，按条目聚合取最高分。
// key 支持 "族" 或 "族:子键"（定向子键检索）。
func (d *Daemon) handleRAGSegSearch(ctx context.Context, params json.RawMessage) (any, error) {
	var p rpc.SegSearchParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Key == "" {
		return nil, fmt.Errorf("key is required")
	}
	if p.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if d.segIndexer == nil {
		return nil, fmt.Errorf("rag seg index not initialized (no embedder configured)")
	}

	hits, err := d.segIndexer.Search(ctx, p.Key, p.Query, p.TopK)
	if err != nil {
		return nil, fmt.Errorf("rag seg search failed: %w", err)
	}
	if hits == nil {
		hits = []native.SegHit{}
	}
	return map[string]any{
		"key":   p.Key,
		"query": p.Query,
		"count": len(hits),
		"hits":  hits,
	}, nil
}

// handleRAGSegList 列出族内条目（仅主记录视图）。
func (d *Daemon) handleRAGSegList(ctx context.Context, params json.RawMessage) (any, error) {
	var p rpc.SegListParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Family == "" {
		return nil, fmt.Errorf("family is required")
	}
	if d.segIndexer == nil {
		return nil, fmt.Errorf("rag seg index not initialized (no embedder configured)")
	}

	hits, err := d.segIndexer.List(ctx, p.Family, p.Limit)
	if err != nil {
		return nil, fmt.Errorf("rag seg list failed: %w", err)
	}
	if hits == nil {
		hits = []native.SegHit{}
	}
	return map[string]any{
		"family": p.Family,
		"count":  len(hits),
		"hits":   hits,
	}, nil
}
