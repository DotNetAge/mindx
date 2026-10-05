package rpc

import "encoding/json"

// SegAddParams are the params for rag.seg.add.
// 同族同值重复添加即覆盖更新（主键 = 族:sha256(值)）。
type SegAddParams struct {
	Family string            `json:"family"`         // 族名（= Collection 名，不可含冒号）
	Value  string            `json:"value"`          // 条目主值
	Meta   map[string]string `json:"meta,omitempty"` // 元数据：每个键值对会生成一条子键向量
}

// SegSearchParams are the params for rag.seg.search.
// Key 支持 "skill"（主值 + 全部子键联合检索）或 "skill:title"（定向子键检索）。
type SegSearchParams struct {
	Key   string `json:"key"`
	Query string `json:"query"`
	TopK  int    `json:"top_k,omitempty"` // <=0 时按 1 处理
}

// SegListParams are the params for rag.seg.list.
type SegListParams struct {
	Family string `json:"family"`
	Limit  int    `json:"limit,omitempty"` // <=0 时按 100 处理
}

func (c *Client) SegAdd(family, value string, meta map[string]string) (json.RawMessage, error) {
	return c.CallWithTimeout("rag.seg.add", SegAddParams{Family: family, Value: value, Meta: meta})
}

func (c *Client) SegSearch(key, query string, topK int) (json.RawMessage, error) {
	return c.CallWithTimeout("rag.seg.search", SegSearchParams{Key: key, Query: query, TopK: topK})
}

func (c *Client) SegList(family string, limit int) (json.RawMessage, error) {
	return c.CallWithTimeout("rag.seg.list", SegListParams{Family: family, Limit: limit})
}
