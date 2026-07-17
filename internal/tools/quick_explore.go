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
		Description: "目录快览，快速浏览项目的语义化目录树，无需读取文件就能对摘要一览无遗。",
		Prompt: `目录快览，快速浏览项目的语义化目录树，无需读取文件就能对摘要一览无遗。将其视为"带语义的 ls -R" — 返回目录树，每个文件附带内容的语义摘要。

与 LS/Glob 不同，QuickExplore 包含每个文件的摘要，无需读取即可了解文件用途。`,
		IsReadOnly: true,
		Parameters: []tools.Parameter{
			{
				Name:        "projectDir",
				Type:        "string",
				Description: "将浏览限制在特定子目录。省略则浏览整个项目。",
				Required:    false,
			},
			{
				Name:        "depth",
				Type:        "integer",
				Description: "树深度（1-5，默认：2）。",
				Required:    false,
				Default:     float64(2),
			},
		},
	}
}

func (t *QuickExplore) Execute(ctx context.Context, params map[string]any) (any, error) {
	if t.indexer == nil {
		return nil, fmt.Errorf("QuickExplore：知识库索引器未初始化")
	}

	var regionPath string
	if raw, ok := getParam(params, "projectDir"); ok {
		if v, ok := raw.(string); ok && v != "" {
			regionPath = v
		}
	}

	depth := 2
	if raw, ok := getParam(params, "depth"); ok {
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
				"message": "本地知识库中暂时没有任何数据。请改用 LS/Glob 浏览文件。",
			}, nil
		}
	}

	root, err := t.indexer.Tree(ctx, regionID, depth)
	if err != nil {
		return nil, fmt.Errorf("QuickExplore 目录树查询失败：%w", err)
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
			sb.WriteString("（空）")
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
