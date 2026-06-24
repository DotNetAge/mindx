package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/DotNetAge/goharness/tools"
	"github.com/DotNetAge/gorag/v2/core"
	goragindexer "github.com/DotNetAge/gorag/v2/indexer"
	"github.com/DotNetAge/gorag/v2/query"
)

// LocalSearch searches the local knowledge graph for relevant chunks and their
// connected entity nodes. It returns concise "clues" (summary, position, tags,
// source path, and graph context). If the LLM needs the full content of a
// specific chunk or node, use LocalFetch instead.
//
// Two modes:
//   - semantic (default): vector similarity search + entity enrichment
//   - graph: multi-hop entity relationship traversal (depth=N)
//
// Max 3 results per query.
type LocalSearch struct {
	indexer *goragindexer.GraphIndexer
}

// NewLocalSearch creates a LocalSearch tool backed by the given GraphIndexer.
func NewLocalSearch(indexer *goragindexer.GraphIndexer) tools.FuncTool {
	return &LocalSearch{indexer: indexer}
}

func (t *LocalSearch) Info() *tools.ToolInfo {
	return &tools.ToolInfo{
		Name:        "LocalSearch",
		Description: "Search the local knowledge graph for relevant chunks and their connected entity nodes. Returns concise clues (summary, file path, line range, tags, entity properties). For full chunk/node details, use LocalFetch.",
		Prompt: `Search the local knowledge graph. Returns up to 3 key clues.

## Modes
- semantic (default): vector search + entity enrichment
- graph: multi-hop entity relationship traversal (depth=N)

## Output
Each result: [summary] - [file:path][POS:Lstart,end][ID:chunk_id][TAGS:tag1,tag2]
Then connected entity table (schema keys as columns).
Then entity relationship list.

## Usage
1. Start with semantic mode
2. For deeper relationship exploration, use graph mode
3. If more detail is needed on a specific result, use LocalFetch with the chunk_id`,
		IsReadOnly: true,
		Parameters: []tools.Parameter{
			{
				Name:        "query",
				Type:        "string",
				Description: "Search query. Can be topic, question, or entity/person name.",
				Required:    true,
			},
			{
				Name:        "mode",
				Type:        "string",
				Description: "\"semantic\" (default) or \"graph\" (multi-hop traversal).",
				Required:    false,
				Default:     "semantic",
				Enum:        []any{"semantic", "graph"},
			},
			{
				Name:        "limit",
				Type:        "integer",
				Description: "Max results (1–20, default: 5).",
				Required:    false,
				Default:     float64(5),
			},
			{
				Name:        "depth",
				Type:        "integer",
				Description: "Graph traversal depth (1–3, default: 1). Only for graph mode.",
				Required:    false,
				Default:     float64(1),
			},
			{
				Name:        "entity_types",
				Type:        "array",
				Description: "Filter entity types e.g. [\"Person\",\"Organization\"]. Only for graph mode.",
				Required:    false,
			},
			{
				Name:        "tags",
				Type:        "array",
				Description: "Filter results by tags. Only hits matching any of the specified tags will be returned.",
				Required:    false,
			},
		},
	}
}

func (t *LocalSearch) Execute(ctx context.Context, params map[string]any) (any, error) {
	if t.indexer == nil {
		return nil, fmt.Errorf("LocalSearch: knowledge base indexer is not initialized")
	}

	queryStr, err := tools.ValidateRequiredString(params, "query")
	if err != nil {
		return nil, err
	}
	if len(queryStr) < 2 {
		return nil, fmt.Errorf("query must be at least 2 characters")
	}

	mode := "semantic"
	if raw, ok := params["mode"]; ok {
		if s, ok := raw.(string); ok && (s == "semantic" || s == "graph") {
			mode = s
		}
	}

	limit := 5
	if raw, ok := params["limit"]; ok {
		if v, ok := tools.ToFloat64(raw); ok && v > 0 {
			limit = int(v)
			if limit > 20 {
				limit = 20
			}
		}
	}

	depth := 1
	if raw, ok := params["depth"]; ok && mode == "graph" {
		if v, ok := tools.ToFloat64(raw); ok && v > 0 {
			depth = int(v)
			if depth > 3 {
				depth = 3
			}
		}
	}

	var entityTypes []string
	if raw, ok := params["entity_types"].([]any); ok && mode == "graph" {
		for _, v := range raw {
			if s, ok := v.(string); ok {
				entityTypes = append(entityTypes, s)
			}
		}
	}

	gq := query.NewGraphQuery(queryStr).(*query.GraphQuery)
	gq.SetTextQuery("") // 防止走 LLM text→Cypher 路径，强制使用向量检索 + 图富化
	gq.SetLimit(limit)
	if mode == "graph" {
		gq.SetDepth(depth)
		if len(entityTypes) > 0 {
			gq.SetEdgeTypes(entityTypes)
		}
	} else {
		gq.SetDepth(1)
	}

	hits, err := t.indexer.Search(ctx, gq)
	if err != nil {
		return nil, fmt.Errorf("LocalSearch failed: %w", err)
	}

	// Filter by tags if specified (post-filter)
	if raw, ok := params["tags"].([]any); ok && len(raw) > 0 {
		var filterTags []string
		for _, v := range raw {
			if s, ok := v.(string); ok {
				filterTags = append(filterTags, s)
			}
		}
		if len(filterTags) > 0 {
			hits = filterHitsByTags(hits, filterTags)
		}
	}

	if len(hits) == 0 {
		return "", nil
	}
	return formatLocalSearchResults(queryStr, hits), nil
}

// ── Metadata helpers ──────────────────────────────────────────────────────────────

func hitSummary(hit *core.Hit) string {
	if v, ok := hit.Metadata["summary"]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	if hit.Title != "" {
		return hit.Title
	}
	if v, ok := hit.Metadata["title"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func hitSourceFile(hit *core.Hit) string {
	if v, ok := hit.Metadata["source_file"]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	if hit.DocID != "" {
		return hit.DocID
	}
	if v, ok := hit.Metadata["doc_id"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func hitLineRange(hit *core.Hit) (int, int) {
	// vectorToHit 不填充 Hit.ChunkMeta，行号从 Metadata["chunk_meta"] 读取
	// start_pos/end_pos 同时存储在 vec.Metadata["chunk_meta"] 中（float64→int）
	if v, ok := hit.Metadata["chunk_meta"]; ok {
		if m, ok := v.(map[string]any); ok {
			sp, _ := m["start_pos"].(float64)
			ep, _ := m["end_pos"].(float64)
			return int(sp), int(ep)
		}
	}
	return 0, 0
}

func hitTags(hit *core.Hit) []string {
	v, ok := hit.Metadata["tags"]
	if !ok {
		return nil
	}
	switch tags := v.(type) {
	case []string:
		return tags
	case []any:
		result := make([]string, 0, len(tags))
		for _, t := range tags {
			if s, ok := t.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

// filterHitsByTags filters hits to only include those that have at least one
// matching tag from the specified list.
func filterHitsByTags(hits []core.Hit, tags []string) []core.Hit {
	if len(tags) == 0 {
		return hits
	}
	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[t] = true
	}
	filtered := make([]core.Hit, 0, len(hits))
	for _, h := range hits {
		ht := hitTags(&h)
		for _, t := range ht {
			if tagSet[t] {
				filtered = append(filtered, h)
				break
			}
		}
	}
	return filtered
}

// ── Format ────────────────────────────────────────────────────────────────────────

// formatLocalSearchResults produces concise, clue-oriented output.
// Max 3 results shown. Each result is a one-line chunk identifier
// followed by entity and relation context.
func formatLocalSearchResults(query string, hits []core.Hit) string {
	var sb strings.Builder

	sb.WriteString("## Search Result\n\n")

	// Cap at 3 results
	display := hits
	if len(display) > 3 {
		display = display[:3]
	}

	for _, hit := range display {
		// [summary] - [file:path][POS:Lstart,end][ID:chunk_id][TAGS:tag1,tag2]
		summary := hitSummary(&hit)
		file := hitSourceFile(&hit)
		startLine, endLine := hitLineRange(&hit)
		tags := hitTags(&hit)

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

		if len(tags) > 0 {
			sb.WriteString("[TAGS:")
			sb.WriteString(strings.Join(tags, ", "))
			sb.WriteString("]")
		}

		sb.WriteString("\n")

		// Relevant Nodes (entity table)
		if len(hit.Entities) > 0 {
			sb.WriteString("\n### Relevant Nodes\n\n")
			sb.WriteString(formatEntityTable(hit.Entities))
		}

		// Relations for this hit
		if len(hit.Relations) > 0 {
			sb.WriteString("\n### Relations\n")
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

	sb.WriteString("LocalSearch clue result. Use LocalFetch for full chunk/node details.\n")
	return sb.String()
}

// entityName looks up an entity's name by its ID in a list of nodes.
func entityName(entities []*core.Node, id string) string {
	for _, e := range entities {
		if e.ID == id {
			return e.Name
		}
	}
	return ""
}

// formatEntityTable renders entities as a markdown table.
// Columns: ID | Name | Type | <all unique property keys across entities>
func formatEntityTable(entities []*core.Node) string {
	if len(entities) == 0 {
		return ""
	}

	// Collect all property keys, sorted for deterministic output
	propKeys := make([]string, 0)
	seen := make(map[string]bool)
	for _, e := range entities {
		for k := range e.Properties {
			if !seen[k] {
				seen[k] = true
				propKeys = append(propKeys, k)
			}
		}
	}
	sort.Strings(propKeys)

	var tb strings.Builder
	// Header
	tb.WriteString("| ID | Name | Type")
	for _, k := range propKeys {
		tb.WriteString(fmt.Sprintf(" | %s", k))
	}
	tb.WriteString(" |\n")

	// Separator
	tb.WriteString("|---|------|------")
	for range propKeys {
		tb.WriteString("|------")
	}
	tb.WriteString("|\n")

	// Rows
	for _, e := range entities {
		label := "-"
		if len(e.Labels) > 0 {
			label = e.Labels[0]
		}
		eid := e.ID
		if eid == "" {
			eid = "-"
		}
		ename := e.Name
		if ename == "" {
			ename = "-"
		}
		tb.WriteString(fmt.Sprintf("| %s | %s | %s", eid, ename, label))
		for _, k := range propKeys {
			val := propertyValue(e.Properties[k])
			tb.WriteString(fmt.Sprintf(" | %s", val))
		}
		tb.WriteString(" |\n")
	}
	return tb.String()
}

// propertyValue formats a property value for table display.
func propertyValue(v any) string {
	if v == nil {
		return "—"
	}
	switch val := v.(type) {
	case string:
		if val == "" {
			return "—"
		}
		return val
	case float64:
		if val == 0 {
			return "—"
		}
		return fmt.Sprintf("%v", val)
	case bool:
		return fmt.Sprintf("%v", val)
	case []any:
		strs := make([]string, 0, len(val))
		for _, item := range val {
			strs = append(strs, fmt.Sprintf("%v", item))
		}
		if len(strs) == 0 {
			return "—"
		}
		return strings.Join(strs, ",")
	case []string:
		if len(val) == 0 {
			return "—"
		}
		return strings.Join(val, ",")
	default:
		return fmt.Sprintf("%v", val)
	}
}
