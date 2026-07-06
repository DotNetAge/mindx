package tools

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
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
// Three modes:
//   - semantic (default): vector similarity search + entity enrichment
//   - graph: multi-hop entity relationship traversal (depth=N)
//   - tree: browse the knowledge hierarchy (region/document tree)
//
// Max 3 results per query (semantic/graph modes only).
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
		Description: "Quickly explore the project's codebase by meaning rather than by filename. Use this BEFORE falling back to Grep/LS/Read — it understands code semantics and project structure. Can also check local knowledge before hitting WebSearch.",
		Prompt: `Search the project's knowledge base by meaning and structure, not by filename. Try this FIRST before using Grep/LS/Read — it saves time and finds things those tools miss.

Also consider using this before WebSearch when the user's question might be about their own codebase.

Three modes for different needs:

- **semantic** (default): The user asks "how does X work?", "find the Y module", "what is Z?". Returns relevant code/docs chunks with their entity context. Think of it as "grep by meaning".
- **graph**: The user asks "what depends on X?", "how are Y and Z connected?". Traverses entity relationships. Think of it as "a map of the codebase".
- **tree**: The user asks "show me the project layout", "what's in this directory?", "browse the knowledge". Returns a directory tree of the project with summaries for each file. Think of it as "ls -R with meaning".

Semantic and graph modes return up to 3 results. Use LocalFetch to read full content of any chunk or node.`,
		IsReadOnly: true,
		Parameters: []tools.Parameter{
			{
				Name:        "query",
				Type:        "string",
				Description: "Search query. Can be topic, question, or entity/person name. Not needed in tree mode.",
				Required:    true,
			},
			{
				Name:        "mode",
				Type:        "string",
				Description: "\"semantic\" (default), \"graph\" (multi-hop traversal), or \"tree\" (knowledge directory tree).",
				Required:    false,
				Default:     "semantic",
				Enum:        []any{"semantic", "graph", "tree"},
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
				Description: "Graph traversal depth (1–3, default: 1) or tree depth (1–5, default: 2). For graph and tree modes.",
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
			{
				Name:        "projectDir",
				Type:        "string",
				Description: "Limit search to a specific subdirectory. Omit to search the entire project.",
				Required:    false,
			},
		},
	}
}

func (t *LocalSearch) Execute(ctx context.Context, params map[string]any) (any, error) {
	if t.indexer == nil {
		return nil, fmt.Errorf("LocalSearch: knowledge base indexer is not initialized")
	}

	// Detect mode early so tree mode can skip query validation
	mode := "semantic"
	if raw, ok := params["mode"]; ok {
		if s, ok := raw.(string); ok {
			switch s {
			case "semantic", "graph", "tree":
				mode = s
			}
		}
	}

	// ── Tree mode: return knowledge directory tree ──────────────────────
	if mode == "tree" {
		return t.execTree(ctx, params)
	}

	// ── Semantic / Graph mode: existing search logic ────────────────────
	queryStr, err := tools.ValidateRequiredString(params, "query")
	if err != nil {
		return nil, err
	}
	if len(queryStr) < 2 {
		return nil, fmt.Errorf("query must be at least 2 characters")
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

	// Apply projectDir filter — default to current working directory
	projectDir, _ := params["projectDir"].(string)
	if projectDir == "" {
		if cwd, err := os.Getwd(); err == nil {
			projectDir = cwd
		}
	}
	if projectDir != "" {
		regionID := fmt.Sprintf("%x", sha256.Sum256([]byte(projectDir)))
		gq.AddFilter("region_id", regionID)
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

// execTree handles the "tree" mode: returns a knowledge directory tree.
func (t *LocalSearch) execTree(ctx context.Context, params map[string]any) (any, error) {
	// Determine projectDir root
	var regionPath string
	if raw, ok := params["projectDir"].(string); ok && raw != "" {
		regionPath = raw
	}

	// Depth: default 2, max 5
	depth := 2
	if raw, ok := params["depth"]; ok {
		if v, ok := tools.ToFloat64(raw); ok && v > 0 {
			depth = int(v)
			if depth > 5 {
				depth = 5
			}
		}
	}

	var regionID string
	if regionPath != "" {
		regionID = fmt.Sprintf("%x", sha256.Sum256([]byte(regionPath)))
	}

	root, err := t.indexer.Tree(ctx, regionID, depth)
	if err != nil {
		return nil, fmt.Errorf("LocalSearch tree failed: %w", err)
	}

	if root == nil {
		return "知识库为空", nil
	}

	return formatTreeResult(root, ""), nil
}

// ── Tree formatting ───────────────────────────────────────────────────────────────

// formatTreeResult renders a ChunkNode tree into indented directory-tree text.
func formatTreeResult(node *core.ChunkNode, prefix string) string {
	if node == nil {
		return ""
	}

	var sb strings.Builder

	// Root node: just render children without a header line
	if node.Type == "root" {
		for i, child := range node.Children {
			isLast := i == len(node.Children)-1
			sb.WriteString(formatTreeBranch(child, prefix, isLast))
		}
		if sb.Len() == 0 {
			sb.WriteString("(空)")
		}
		return sb.String()
	}

	// Non-root: render this node then children
	sb.WriteString(formatTreeNodeLine(node))
	for i, child := range node.Children {
		isLast := i == len(node.Children)-1
		sb.WriteString(formatTreeBranch(child, prefix, isLast))
	}

	return sb.String()
}

// formatTreeNodeLine renders a single tree node as a indented line.
func formatTreeNodeLine(node *core.ChunkNode) string {
	var sb strings.Builder

	switch node.Type {
	case "region":
		sb.WriteString(node.Name)
		sb.WriteString("/")
		if node.Summary != "" {
			sb.WriteString("  — ")
			sb.WriteString(node.Summary)
		}
	case "document":
		sb.WriteString(node.Name)
		if len(node.ChunkIDs) > 0 {
			sb.WriteString(fmt.Sprintf("  [ID:%s]", strings.Join(node.ChunkIDs, ",")))
		}
		if node.Summary != "" {
			sb.WriteString("  ")
			sb.WriteString(node.Summary)
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

// formatTreeBranch renders a single child node with tree-drawing prefix.
func formatTreeBranch(node *core.ChunkNode, prefix string, isLast bool) string {
	var sb strings.Builder

	// Branch connector
	connector := "├── "
	if isLast {
		connector = "└── "
	}
	sb.WriteString(prefix)
	sb.WriteString(connector)

	// Content
	switch node.Type {
	case "region":
		sb.WriteString(node.Name)
		sb.WriteString("/")
		if node.Summary != "" {
			sb.WriteString("  — ")
			sb.WriteString(node.Summary)
		}
	case "document":
		sb.WriteString(node.Name)
		if len(node.ChunkIDs) > 0 {
			sb.WriteString(fmt.Sprintf("  [ID:%s]", strings.Join(node.ChunkIDs, ",")))
		}
		if node.Summary != "" {
			sb.WriteString("  ")
			sb.WriteString(node.Summary)
		}
	}
	sb.WriteString("\n")

	// Children
	childPrefix := prefix
	if isLast {
		childPrefix += "    "
	} else {
		childPrefix += "│   "
	}
	for i, child := range node.Children {
		childIsLast := i == len(node.Children)-1
		sb.WriteString(formatTreeBranch(child, childPrefix, childIsLast))
	}

	return sb.String()
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

// hitChunkType extracts the chunk type from a hit's metadata.
// Returns "root" for document-level summary chunks, "segment" for regular
// content chunks, or "" if unknown.
func hitChunkType(hit *core.Hit) string {
	if v, ok := hit.Metadata["chunk_type"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// hitParentID extracts the parent chunk ID from a hit's metadata.
// Returns "" when the chunk has no parent (root chunk) or when the
// parent relationship was not established.
func hitParentID(hit *core.Hit) string {
	if v, ok := hit.Metadata["parent_id"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
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
		// [summary] - [file:path][POS:Lstart,end][ID:chunk_id][TYPE:type][TAGS:tag1,tag2][PARENT:parent_id]
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

		// Show chunk type only when it's a document-level summary (root).
		// Regular segments are the default and not marked.
		if chunkType == "root" {
			sb.WriteString("[TYPE:document]")
		}

		if len(tags) > 0 {
			sb.WriteString("[TAGS:")
			sb.WriteString(strings.Join(tags, ", "))
			sb.WriteString("]")
		}

		// Show parent_id when available, enabling LLM to navigate hierarchy.
		// The LLM can use LocalFetch with the parent chunk ID to retrieve
		// the broader context this chunk belongs to.
		if parentID != "" {
			sb.WriteString("[PARENT:")
			sb.WriteString(parentID)
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
