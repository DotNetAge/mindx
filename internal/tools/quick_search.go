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
		Description: "Semantic search over the project's knowledge base — find code and docs by meaning, not by exact text. Use this FIRST before Grep when asking \"where is X?\", \"how does Y work?\", or \"find the Z module\".",
		Prompt: `Search the project's knowledge base by meaning. Think of it as "grep by meaning" — finds relevant code and documentation even when you don't know the exact keywords.

Use QuickSearch FIRST before Grep when:
- "where is authentication implemented?"
- "how does the payment flow work?"
- "find the config module"

Also consider QuickSearch before WebSearch — the answer might already be in the local knowledge base.`,
		IsReadOnly: true,
		Parameters: []tools.Parameter{
			{
				Name:        "query",
				Type:        "string",
				Description: "Natural language query — topic, question, or concept name.",
				Required:    true,
			},
			{
				Name:        "limit",
				Type:        "integer",
				Description: "Max results (1–20, default: 5).",
				Required:    false,
				Default:     float64(5),
			},
			{
				Name:        "entity_labels",
				Type:        "array",
				Description: "Filter entity node labels e.g. [\"Concept\",\"Term\"] to narrow result entity types.",
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

func (t *QuickSearch) Execute(ctx context.Context, params map[string]any) (any, error) {
	if t.indexer == nil {
		return nil, fmt.Errorf("QuickSearch: knowledge base indexer is not initialized")
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

	gq := query.NewGraphQuery(queryStr).(*query.GraphQuery)
	gq.SetTextQuery("") // Force vector search + entity enrichment, skip LLM text→Cypher path
	gq.SetLimit(limit)
	gq.SetDepth(1)

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

		// Pre-check: skip search if no indexed data for this region
		total, countErr := t.indexer.CountByRegion(ctx, projectDir)
		if countErr == nil && total == 0 {
			return map[string]any{
				"message": "No data in local knowledge base yet. Use Grep/Glob/Read or WebSearch instead.",
			}, nil
		}
	}

	hits, err := t.indexer.Search(ctx, gq)
	if err != nil {
		return nil, fmt.Errorf("QuickSearch failed: %w", err)
	}

	// Fallback: retry without region_id filter when region-filtered search yields nothing
	if len(hits) == 0 {
		gq2 := query.NewGraphQuery(queryStr).(*query.GraphQuery)
		gq2.SetTextQuery("")
		gq2.SetLimit(limit)
		gq2.SetDepth(1)
		hits2, err2 := t.indexer.Search(ctx, gq2)
		if err2 == nil && len(hits2) > 0 {
			hits = hits2
		}
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

	// Filter by entity_labels if specified (post-filter on hit.Entities)
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
	return formatQuickSearchResults(queryStr, hits), nil
}

// ── QuickSearch output formatting ─────────────────────────────────────────────────

func formatQuickSearchResults(query string, hits []core.Hit) string {
	var sb strings.Builder

	sb.WriteString("## Search Result\n\n")

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
			sb.WriteString("\n### Relevant Nodes\n\n")
			sb.WriteString(formatEntityTable(hit.Entities))
		}

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

	sb.WriteString("QuickSearch clue result.\n")
	return sb.String()
}
