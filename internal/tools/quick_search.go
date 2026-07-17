package tools

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DotNetAge/goharness/tools"
	"github.com/DotNetAge/gorag/v2/core"
	goragindexer "github.com/DotNetAge/gorag/v2/indexer"
	"github.com/DotNetAge/gorag/v2/query"
)

// QuickSearch performs semantic search over the local knowledge base.
// It finds relevant code and documentation by meaning, not by exact text match.
// Use this BEFORE Grep when searching for where or how something is implemented.
type QuickSearch struct {
	indexer *goragindexer.GraphIndexer
}

// NewQuickSearch creates a QuickSearch tool backed by the given GraphIndexer.
func NewQuickSearch(indexer *goragindexer.GraphIndexer) tools.FuncTool {
	return &QuickSearch{indexer: indexer}
}

func (t *QuickSearch) Info() *tools.ToolInfo {
	return &tools.ToolInfo{
		Name:        "QuickSearch",
		Description: "高效语义搜索 — 按含义查找本项目内的代码和文档速度远超Grep和WebSearch。",
		Prompt: `按含义搜索项目知识库。将其视为"按语义的 grep" — 即使不知道精确关键词也能找到相关代码和文档。

本地知识库可能已有答案，检索速度与精度远优于Grep与WebSearch, 在找不到相关结果才考虑回退Grep或WebSearch 使用。`,
		IsReadOnly: true,
		Parameters: []tools.Parameter{
			{
				Name:        "query",
				Type:        "string",
				Description: "自然语言查询 — 主题、问题或概念名称。",
				Required:    true,
			},
			{
				Name:        "limit",
				Type:        "integer",
				Description: "最大结果数（1-20，默认：5）。",
				Required:    false,
				Default:     float64(5),
			},
			{
				Name:        "entity_labels",
				Type:        "array",
				Description: "后过滤实体节点标签，例如 [\"Concept\",\"Term\"] 以缩小结果实体类型。",
				Required:    false,
			},
			{
				Name:        "tags",
				Type:        "array",
				Description: "按标签过滤结果。仅返回匹配指定标签的命中。",
				Required:    false,
			},
			{
				Name:        "projectDir",
				Type:        "string",
				Description: "将搜索限制在特定子目录。省略则搜索整个项目。",
				Required:    false,
			},
		},
	}
}

func (t *QuickSearch) Execute(ctx context.Context, params map[string]any) (any, error) {
	if t.indexer == nil {
		return nil, fmt.Errorf("QuickSearch：知识库索引器未初始化")
	}

	queryStr, err := tools.ValidateRequiredString(params, "query")
	if err != nil {
		return nil, err
	}
	if len(queryStr) < 2 {
		return nil, fmt.Errorf("查询必须至少 2 个字符")
	}

	limit := 5
	if raw, ok := getParam(params, "limit"); ok {
		if v, ok := tools.ToFloat64(raw); ok && v > 0 {
			limit = int(v)
			if limit > 20 {
				limit = 20
			}
		}
	}

	// Apply projectDir filter — default to current working directory
	projectDirRaw, _ := getParam(params, "projectDir")
	projectDir, _ := projectDirRaw.(string)
	if projectDir == "" {
		if cwd, err := os.Getwd(); err == nil {
			projectDir = cwd
		}
	}

	// Pre-check + 预计算 regionID 供 per-token 使用
	var regionID string
	if projectDir != "" {
		regionID = fmt.Sprintf("%x", sha256.Sum256([]byte(filepath.Clean(projectDir))))
		total, countErr := t.indexer.CountByRegion(ctx, projectDir)
		if countErr == nil && total == 0 {
			return map[string]any{
				"message": "本地知识库中暂时没有任何数据。请改用 Grep/Glob/Read 或 WebSearch。",
			}, nil
		}
	}

	// 将查询按空白符拆分为多个关键词，分别检索后合并去重。
	// LLM 倾向于输入空格分隔的关键词而非自然语句（如 "redis 迁移 配置"），
	// 多次查询比单次语义搜索能召回更全面的结果。
	tokens := splitQueryTokens(queryStr)

	var allHits []core.Hit
	seen := make(map[string]bool)

	for _, token := range tokens {
		gq := query.NewGraphQuery(token).(*query.GraphQuery)
		gq.SetTextQuery("") // Force vector search + entity enrichment, skip LLM text→Cypher path
		gq.SetLimit(limit)
		gq.SetDepth(1)

		if regionID != "" {
			gq.AddFilter("region_id", regionID)
		}

		hits, err := t.indexer.Search(ctx, gq)
		if err != nil {
			// 单个 token 查询失败，跳过
			continue
		}

		// Fallback: retry without region_id filter when region-filtered search yields nothing
		if len(hits) == 0 {
			gq2 := query.NewGraphQuery(token).(*query.GraphQuery)
			gq2.SetTextQuery("")
			gq2.SetLimit(limit)
			gq2.SetDepth(1)
			hits2, err2 := t.indexer.Search(ctx, gq2)
			if err2 == nil && len(hits2) > 0 {
				hits = hits2
			}
		}

		for _, h := range hits {
			if seen[h.ID] {
				continue
			}
			seen[h.ID] = true
			allHits = append(allHits, h)
		}
	}

	hits := allHits
	if len(hits) > limit {
		hits = hits[:limit]
	}

	// Filter by tags if specified (post-filter)
	if raw, ok := getParam(params, "tags"); ok {
		if arr, ok := raw.([]any); ok && len(arr) > 0 {
			var filterTags []string
			for _, v := range arr {
				if s, ok := v.(string); ok {
					filterTags = append(filterTags, s)
				}
			}
			if len(filterTags) > 0 {
				hits = filterHitsByTags(hits, filterTags)
			}
		}
	}

	// Filter by entity_labels if specified (post-filter on hit.Entities)
	if raw, ok := getParam(params, "entity_labels"); ok {
		if arr, ok := raw.([]any); ok && len(arr) > 0 {
			var filterLabels []string
			for _, v := range arr {
				if s, ok := v.(string); ok {
					filterLabels = append(filterLabels, s)
				}
			}
			if len(filterLabels) > 0 {
				hits = filterHitsByEntityLabels(hits, filterLabels)
			}
		}
	}

	if len(hits) == 0 {
		return "", nil
	}
	return formatQuickSearchResults(queryStr, hits), nil
}

// splitQueryTokens 将查询拆分为多个关键词。
// 如果查询本身是自然语句（含空格但长度 > 50），视为完整查询不拆分。
// 否则按空白符拆分为多个关键词，过滤掉过短的词。
func splitQueryTokens(query string) []string {
	if len(query) > 50 {
		return []string{query}
	}
	parts := strings.Fields(query)
	if len(parts) <= 1 {
		return []string{query}
	}
	tokens := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) >= 2 {
			tokens = append(tokens, p)
		}
	}
	if len(tokens) == 0 {
		return []string{query}
	}
	return tokens
}

// ── QuickSearch output formatting ─────────────────────────────────────────────────

func formatQuickSearchResults(query string, hits []core.Hit) string {
	var sb strings.Builder

	sb.WriteString("## 搜索结果\n\n")

	display := hits
	if len(display) > 5 {
		display = display[:5]
	}

	for _, hit := range display {
		summary := hitSummary(&hit)
		file := hitSourceFile(&hit)
		startLine, endLine := hitLineRange(&hit)
		tags := hitTags(&hit)
		chunkType := hitChunkType(&hit)
		parentID := hitParentID(&hit)

		sb.WriteString("[")
		sb.WriteString(summary)
		sb.WriteString("] - ")

		sb.WriteString("[file:")
		sb.WriteString(file)
		sb.WriteString("]")

		if startLine > 0 || endLine > 0 {
			sb.WriteString(fmt.Sprintf("[POS:L%d,%d]", startLine+1, endLine+1))
		}

		sb.WriteString("[ID:")
		sb.WriteString(hit.ID)
		sb.WriteString("]")

		if chunkType == "root" {
			sb.WriteString("[TYPE:document]")
		}

		if len(tags) > 0 {
			sb.WriteString("[TAGS:")
			sb.WriteString(strings.Join(tags, ", "))
			sb.WriteString("]")
		}

		if parentID != "" {
			sb.WriteString("[PARENT:")
			sb.WriteString(parentID)
			sb.WriteString("]")
		}

		sb.WriteString("\n")

		if len(hit.Entities) > 0 {
			sb.WriteString("\n### 相关节点\n\n")
			sb.WriteString(formatEntityTable(hit.Entities))
		}

		if len(hit.Relations) > 0 {
			sb.WriteString("\n### 关系\n")
			for _, r := range hit.Relations {
				pred := r.Predicate
				if pred == "" {
					pred = r.Type
				}
				srcName := entityName(hit.Entities, r.Source)
				tgtName := entityName(hit.Entities, r.Target)
				if srcName == "" {
					srcName = r.Source
				}
				if tgtName == "" {
					tgtName = r.Target
				}
				sb.WriteString(fmt.Sprintf("%s --%s--> %s\n", srcName, pred, tgtName))
			}
		}

		sb.WriteString("\n---\n\n")
	}

	sb.WriteString("QuickSearch 结果。\n")
	return sb.String()
}
