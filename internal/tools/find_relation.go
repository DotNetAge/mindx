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

// FindRelation traverses entity relationships in the knowledge graph — find
// what depends on what, how things are connected, and multi-hop paths between concepts.
type FindRelation struct {
	indexer *goragindexer.GraphIndexer
}

// NewFindRelation creates a FindRelation tool backed by the given GraphIndexer.
func NewFindRelation(indexer *goragindexer.GraphIndexer) tools.FuncTool {
	return &FindRelation{indexer: indexer}
}

func (t *FindRelation) Info() *tools.ToolInfo {
	return &tools.ToolInfo{
		Name:        "FindRelation",
		Description: "Traverse entity relationships in the knowledge graph — find what depends on what, how things are connected, and explore multi-hop paths. Use this when asking \"what depends on X?\", \"how are Y and Z connected?\", or \"find related concepts\".",
		Prompt: `Traverse entity relationships in the project's knowledge graph. Think of it as "a map of the knowledge graph" — discovers connections, dependencies, and multi-hop paths between concepts.

Use FindRelation when:
- "what depends on the auth module?"
- "how are payment and invoicing connected?"
- "find all entities related to error handling"

Results show an entity table and connectivity view, then per-hit details.`,
		IsReadOnly: true,
		Parameters: []tools.Parameter{
			{
				Name:        "query",
				Type:        "string",
				Description: "Topic, question, or entity name to start graph traversal from.",
				Required:    true,
			},
			{
				Name:        "depth",
				Type:        "integer",
				Description: "Graph traversal depth (1–3, default: 1).",
				Required:    false,
				Default:     float64(1),
			},
			{
				Name:        "edge_types",
				Type:        "array",
				Description: "Filter traversal by edge types e.g. [\"CONTAINS\",\"RELATED_TO\"].",
				Required:    false,
			},
			{
				Name:        "entity_labels",
				Type:        "array",
				Description: "Post-filter entity node labels e.g. [\"Concept\",\"Term\"] to narrow result entity types.",
				Required:    false,
			},
			{
				Name:        "limit",
				Type:        "integer",
				Description: "Max results (1–20, default: 5).",
				Required:    false,
				Default:     float64(5),
			},
			{
				Name:        "projectDir",
				Type:        "string",
				Description: "Limit traversal to a specific subdirectory. Omit to search the entire project.",
				Required:    false,
			},
		},
	}
}

func (t *FindRelation) Execute(ctx context.Context, params map[string]any) (any, error) {
	if t.indexer == nil {
		return nil, fmt.Errorf("FindRelation: knowledge base indexer is not initialized")
	}

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
	if raw, ok := params["depth"]; ok {
		if v, ok := tools.ToFloat64(raw); ok && v > 0 {
			depth = int(v)
			if depth > 3 {
				depth = 3
			}
		}
	}

	var edgeTypes []string
	if raw, ok := params["edge_types"].([]any); ok {
		for _, v := range raw {
			if s, ok := v.(string); ok {
				edgeTypes = append(edgeTypes, s)
			}
		}
	}

	gq := query.NewGraphQuery(queryStr).(*query.GraphQuery)
	gq.SetTextQuery("") // Force vector search + entity enrichment, skip LLM text→Cypher path
	gq.SetLimit(limit)
	gq.SetDepth(depth)
	if len(edgeTypes) > 0 {
		gq.SetEdgeTypes(edgeTypes)
	}

	// Apply projectDir filter — default to current working directory
	projectDir, _ := params["projectDir"].(string)
	if projectDir == "" {
		if cwd, err := os.Getwd(); err == nil {
			projectDir = cwd
		}
	}
	if projectDir != "" {
		regionID := fmt.Sprintf("%x", sha256.Sum256([]byte(filepath.Clean(projectDir))))
		gq.AddFilter("region_id", regionID)

		total, countErr := t.indexer.CountByRegion(ctx, projectDir)
		if countErr == nil && total == 0 {
			return map[string]any{
				"message": "No data in local knowledge base yet. Use Grep/Glob/Read or WebSearch instead.",
			}, nil
		}
	}

	hits, err := t.indexer.Search(ctx, gq)
	if err != nil {
		return nil, fmt.Errorf("FindRelation failed: %w", err)
	}

	// Fallback: retry without region_id filter
	if len(hits) == 0 {
		gq2 := query.NewGraphQuery(queryStr).(*query.GraphQuery)
		gq2.SetTextQuery("")
		gq2.SetLimit(limit)
		gq2.SetDepth(depth)
		if len(edgeTypes) > 0 {
			gq2.SetEdgeTypes(edgeTypes)
		}
		hits2, err2 := t.indexer.Search(ctx, gq2)
		if err2 == nil && len(hits2) > 0 {
			hits = hits2
		}
	}

	// Filter by entity_labels if specified
	if raw, ok := params["entity_labels"].([]any); ok && len(raw) > 0 {
		var filterLabels []string
		for _, v := range raw {
			if s, ok := v.(string); ok {
				filterLabels = append(filterLabels, s)
			}
		}
		if len(filterLabels) > 0 {
			hits = filterHitsByEntityLabels(hits, filterLabels)
		}
	}

	if len(hits) == 0 {
		return "", nil
	}
	return formatFindRelationResults(queryStr, hits), nil
}

// ── FindRelation output formatting ────────────────────────────────────────────────

func formatFindRelationResults(query string, hits []core.Hit) string {
	var sb strings.Builder

	sb.WriteString("## Graph Traversal\n\n")

	// Collect all unique entities & relations
	entityMap := make(map[string]*core.Node)
	var allRelations []*core.Edge
	for _, hit := range hits {
		for _, e := range hit.Entities {
			entityMap[e.ID] = e
		}
		allRelations = append(allRelations, hit.Relations...)
	}

	// Consolidated entity table
	entities := make([]*core.Node, 0, len(entityMap))
	for _, e := range entityMap {
		entities = append(entities, e)
	}
	sb.WriteString(fmt.Sprintf("### Entities Found (%d total)\n\n", len(entities)))
	if len(entities) > 0 {
		sb.WriteString(formatEntityTable(entities))
	} else {
		sb.WriteString("(no entities)\n")
	}
	sb.WriteString("\n")

	// Consolidated relations (connectivity view)
	if len(allRelations) > 0 {
		sb.WriteString(fmt.Sprintf("### Connections (%d edges)\n\n", len(allRelations)))
		seen := make(map[string]bool)
		for _, r := range allRelations {
			pred := r.Predicate
			if pred == "" {
				pred = r.Type
			}
			srcName := entityName(entities, r.Source)
			tgtName := entityName(entities, r.Target)
			if srcName == "" {
				srcName = r.Source
			}
			if tgtName == "" {
				tgtName = r.Target
			}
			key := srcName + pred + tgtName
			if seen[key] {
				continue
			}
			seen[key] = true
			sb.WriteString(fmt.Sprintf("- **%s** --%s--> **%s**\n", srcName, pred, tgtName))
		}
		sb.WriteString("\n")
	}

	// Per-hit details
	sb.WriteString("---\n\n### Per-Hit Details\n\n")
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
			sb.WriteString("\n  Nodes: ")
			names := make([]string, 0, len(hit.Entities))
			for _, e := range hit.Entities {
				label := "-"
				if len(e.Labels) > 0 {
					label = e.Labels[0]
				}
				names = append(names, fmt.Sprintf("%s (%s)", e.Name, label))
			}
			sb.WriteString(strings.Join(names, ", "))
			sb.WriteString("\n")
		}

		sb.WriteString("\n---\n\n")
	}

	sb.WriteString("FindRelation result.\n")
	return sb.String()
}
