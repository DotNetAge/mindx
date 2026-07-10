package tools

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/DotNetAge/goharness/tools"
	"github.com/DotNetAge/gorag/v2/core"
	goragindexer "github.com/DotNetAge/gorag/v2/indexer"
)

// QuickExplore browses the project's knowledge directory tree — a semantic
// alternative to LS/Glob that shows file structure with per-file summaries.
// Use this FIRST when the user wants to explore project layout or browse a directory.
type QuickExplore struct {
	indexer *goragindexer.GraphIndexer
}

// NewQuickExplore creates a QuickExplore tool backed by the given GraphIndexer.
func NewQuickExplore(indexer *goragindexer.GraphIndexer) tools.FuncTool {
	return &QuickExplore{indexer: indexer}
}

func (t *QuickExplore) Info() *tools.ToolInfo {
	return &tools.ToolInfo{
		Name:        "QuickExplore",
		Description: "Browse the project's knowledge directory tree — see what files exist with semantic summaries. Use this FIRST before LS/Glob when you want to understand project layout or explore what's in a directory.",
		Prompt: `Browse the project's knowledge directory tree. Think of it as "ls -R with meaning" — returns a directory tree where each file includes a semantic summary of its contents.

Use QuickExplore FIRST before LS or Glob when:
- "show me the project layout"
- "what's in this directory?"
- "browse the project structure"

Unlike LS/Glob, QuickExplore includes per-file summaries so you can understand what each file does without reading it.

Fall back to LS/Glob when the knowledge base isn't indexed yet or you need raw file listing.`,
		IsReadOnly: true,
		Parameters: []tools.Parameter{
			{
				Name:        "projectDir",
				Type:        "string",
				Description: "Limit browsing to a specific subdirectory. Omit to browse the entire project.",
				Required:    false,
			},
			{
				Name:        "depth",
				Type:        "integer",
				Description: "Tree depth (1–5, default: 2).",
				Required:    false,
				Default:     float64(2),
			},
		},
	}
}

func (t *QuickExplore) Execute(ctx context.Context, params map[string]any) (any, error) {
	if t.indexer == nil {
		return nil, fmt.Errorf("QuickExplore: knowledge base indexer is not initialized")
	}

	var regionPath string
	if raw, ok := params["projectDir"].(string); ok && raw != "" {
		regionPath = raw
	}

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
		regionID = fmt.Sprintf("%x", sha256.Sum256([]byte(filepath.Clean(regionPath))))

		total, countErr := t.indexer.CountByRegion(ctx, regionPath)
		if countErr == nil && total == 0 {
			return map[string]any{
				"message": "No data in local knowledge base yet. Use LS/Glob to browse files instead.",
			}, nil
		}
	}

	root, err := t.indexer.Tree(ctx, regionID, depth)
	if err != nil {
		return nil, fmt.Errorf("QuickExplore tree failed: %w", err)
	}

	if root == nil {
		return "（空知识库）", nil
	}

	return formatTreeResult(root, ""), nil
}

// ── Tree formatting ───────────────────────────────────────────────────────────────

func formatTreeResult(node *core.ChunkNode, prefix string) string {
	if node == nil {
		return ""
	}

	var sb strings.Builder

	if node.Type == "root" {
		for i, child := range node.Children {
			isLast := i == len(node.Children)-1
			sb.WriteString(formatTreeBranch(child, prefix, isLast))
		}
		if sb.Len() == 0 {
			sb.WriteString("(empty)")
		}
		return sb.String()
	}

	sb.WriteString(formatTreeNodeLine(node))
	for i, child := range node.Children {
		isLast := i == len(node.Children)-1
		sb.WriteString(formatTreeBranch(child, prefix, isLast))
	}

	return sb.String()
}

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

func formatTreeBranch(node *core.ChunkNode, prefix string, isLast bool) string {
	var sb strings.Builder

	connector := "├── "
	if isLast {
		connector = "└── "
	}
	sb.WriteString(prefix)
	sb.WriteString(connector)

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
