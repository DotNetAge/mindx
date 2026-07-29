package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/DotNetAge/goharness/tools"
)

// QuickExplore browses the project's knowledge directory tree — a semantic
// alternative to LS/Glob that shows file structure with per-file summaries.
// Use this FIRST when the user wants to explore project layout or browse a directory.
type QuickExplore struct {
	serverURL string
}

// NewQuickExplore creates a QuickExplore tool that queries MindStore WebAPI.
func NewQuickExplore(serverURL string) tools.FuncTool {
	return &QuickExplore{serverURL: serverURL}
}

func (t *QuickExplore) Info() *tools.ToolInfo {
	return &tools.ToolInfo{
		Name:        "QuickExplore",
		Description: "目录快览，快速浏览项目语义化目录树，无需读取文件就能对文件语义结构以及具体内容的精准定位都一览无遗。",
		Prompt: `目录快览，快速浏览项目语义化目录树，无需读取文件就能对摘要一览无遗。将其视为"带语义的 ls -R" — 返回目录树，每个文件附带内容的语义摘要。

与 LS/Glob 不同，QuickExplore 包含每个文件的摘要，无需读取即可了解文件用途。

要求知识库服务（mrag serve）已运行。`,
		IsReadOnly: true,
		Parameters: []tools.Parameter{
			{
				Name:        "targetDir",
				Type:        "string",
				Description: "目标目录，将浏览限制在特定子目录。省略则使用当前会话的项目目录。",
				Required:    false,
			},
		},
	}
}

// ── 防重复查询缓存 ───────────────────────────────────────────────────

// exploreCache 记录已查询过的目录，避免重复请求知识库 API。
// 缓存命中时不返回结果内容，而是告知模型引用此前返回的结果。
var exploreCache sync.Map

// exploreCacheKey 生成缓存 key。
func exploreCacheKey(targetDir string) string {
	return "explore:" + targetDir
}

// isExploreCached 检查指定目录是否已查询过。
func isExploreCached(targetDir string) bool {
	_, ok := exploreCache.Load(exploreCacheKey(targetDir))
	return ok
}

// markExploreCached 标记指定目录已查询过。
func markExploreCached(targetDir string) {
	exploreCache.Store(exploreCacheKey(targetDir), true)
}

func (t *QuickExplore) Execute(ctx context.Context, params map[string]any) (any, error) {
	var targetDir string
	if raw, ok := getParam(params, "targetDir"); ok {
		if v, ok := raw.(string); ok && v != "" {
			targetDir = v
		}
	}
	if targetDir == "" {
		if tc := tools.GetToolContext(ctx); tc != nil && tc.Session != nil {
			targetDir = tc.Session.ProjectDir()
		}
	}

	// 防重复查询缓存：相同参数已查询过则告知模型引用此前的结果
	if isExploreCached(targetDir) {
		return "目录树自上次查询以来未发生变化。本对话中此前 QuickExplore 的结果仍然有效，请引用此前的结果。", nil
	}

	// 调用 MindStore WebAPI /api/tree
	url := t.serverURL + "/api/tree"
	if targetDir != "" {
		url += "?dir=" + targetDir
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return map[string]any{
			"message": "知识库服务（mrag serve）未启动或无法连接，请先启动知识库服务。",
		}, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var apiResp apiTreeResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("知识库查询失败: %s", apiResp.Error)
	}

	if apiResp.Data == nil {
		return "（空知识库）", nil
	}

	// 检查是否为空目录树（无子节点、无文件块）
	if len(apiResp.Data.Children) == 0 && len(apiResp.Data.Chunks) == 0 {
		markExploreCached(targetDir)
		return "（该目录下没有索引数据）", nil
	}

	result := renderTreeResult(apiResp.Data, "", apiResp.Data.Path)
	markExploreCached(targetDir)
	return result, nil
}

// ── API 响应类型 ─────────────────────────────────────────────────────

type apiTreeResponse struct {
	Success bool            `json:"success"`
	Data    *sourceTreeNode `json:"data,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// sourceTreeNode 与 mindstore internal/service/tree.go 的 SourceTreeNode 一致。
type sourceTreeNode struct {
	Name     string            `json:"name"`
	Path     string            `json:"path"`
	Size     int64             `json:"size"`
	IsDir    bool              `json:"is_dir"`
	Summary  string            `json:"summary"`
	Chunks   []sourceChunkNode `json:"chunks"`
	Children []*sourceTreeNode `json:"children"`
}

// sourceChunkNode 与 mindstore internal/service/tree.go 的 SourceChunkNode 一致。
type sourceChunkNode struct {
	Type      string            `json:"type"`
	Title     string            `json:"title"`
	Summary   string            `json:"summary"`
	StartLine int               `json:"start_line"`
	EndLine   int               `json:"end_line"`
	StartPos  int               `json:"start_pos"`
	EndPos    int               `json:"end_pos"`
	Children  []sourceChunkNode `json:"children"`
}

// ── 目录树渲染 ───────────────────────────────────────────────────────
// 以下渲染逻辑与 mrag tree 命令（cmd/tree.go）的输出格式一致。

func renderTreeResult(node *sourceTreeNode, prefix string, rootPath string) string {
	if node == nil {
		return ""
	}

	var sb strings.Builder

	sorted := sortTreeChildren(node.Children)
	nodeChunks := node.Chunks

	for i, child := range sorted {
		isLast := i == len(sorted)-1 && len(nodeChunks) == 0
		branch := "├── "
		connector := "│   "
		if isLast {
			branch = "└── "
			connector = "    "
		}

		if child.IsDir {
			line := fmt.Sprintf("%s%s%s", prefix, branch, child.Name)
			if child.Summary != "" {
				line = fmt.Sprintf("%s - %s", line, truncateString(child.Summary, 80))
			}
			sb.WriteString(line)
			sb.WriteString("\n")
			sb.WriteString(renderTreeResult(child, prefix+connector, rootPath))
		} else {
			sb.WriteString(renderFileNode(child, prefix, branch, connector, isLast))
		}
	}

	// 渲染当前层级下的文件节点（顶层文件）
	for i, chunk := range nodeChunks {
		isLast := i == len(nodeChunks)-1
		branch := "├── "
		connector := "│   "
		if isLast && len(node.Children) == 0 {
			branch = "└── "
			connector = "    "
		}
		sb.WriteString(renderChunkNodeLine(chunk, prefix+branch, prefix+connector))
	}

	return sb.String()
}

// renderFileNode 渲染文件节点及 Chunk 子树。
func renderFileNode(node *sourceTreeNode, prefix, branch, connector string, isLast bool) string {
	var sb strings.Builder
	if isLast {
		branch = "└── "
		connector = "    "
	}
	sb.WriteString(fmt.Sprintf("%s%s%s  [size:%s]\n", prefix, branch, filepath.Base(node.Path), formatBytes(node.Size)))

	childPrefix := prefix + connector
	for i, chunk := range node.Chunks {
		chunkBranch := "├── "
		chunkConn := "│   "
		if i == len(node.Chunks)-1 {
			chunkBranch = "└── "
			chunkConn = "    "
		}
		sb.WriteString(renderChunkNodeLine(chunk, childPrefix+chunkBranch, childPrefix+chunkConn))
	}
	return sb.String()
}

// renderChunkNodeLine 渲染单个 Chunk 节点。
func renderChunkNodeLine(node sourceChunkNode, branch, connector string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s%s\n", branch, formatChunkLine(node.Title, node.Summary, node.StartPos, node.EndPos)))
	sb.WriteString(renderChunkChildren(node, connector))
	return sb.String()
}

// renderChunkChildren 递归渲染 Chunk 子块。
func renderChunkChildren(node sourceChunkNode, prefix string) string {
	var sb strings.Builder
	for i, child := range node.Children {
		isLast := i == len(node.Children)-1
		branch := "├── "
		connector := "│   "
		if isLast {
			branch = "└── "
			connector = "    "
		}
		sb.WriteString(fmt.Sprintf("%s%s%s\n", prefix+branch, "", formatChunkLine(child.Title, child.Summary, child.StartPos, child.EndPos)))
		sb.WriteString(renderChunkChildren(child, prefix+connector))
	}
	return sb.String()
}

// formatChunkLine 格式化 Chunk 行：Title [- Summary] [Pstart-Pend]
func formatChunkLine(title, summary string, startPos, endPos int) string {
	pos := formatPos(startPos, endPos)
	title = strings.ReplaceAll(title, "\n", " ")
	summary = strings.ReplaceAll(summary, "\n", " ")
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return fmt.Sprintf("%s%s", title, pos)
	}
	return fmt.Sprintf("%s - %s%s", title, summary, pos)
}

// sortTreeChildren 对子节点排序：目录在前，文件在后，各自按名排序。
func sortTreeChildren(children []*sourceTreeNode) []*sourceTreeNode {
	sorted := make([]*sourceTreeNode, len(children))
	copy(sorted, children)
	for i := range len(sorted) {
		for j := i + 1; j < len(sorted); j++ {
			less := false
			if sorted[i].IsDir && !sorted[j].IsDir {
				less = true
			} else if !sorted[i].IsDir && sorted[j].IsDir {
				less = false
			} else {
				less = sorted[i].Name < sorted[j].Name
			}
			if !less {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return sorted
}

// formatPos 将字节偏移格式化为 [Pstart-Pend]，位置都为 0 时返回空字符串。
func formatPos(start, end int) string {
	if start == 0 && end == 0 {
		return ""
	}
	return fmt.Sprintf("  [P%d-P%d]", start, end)
}

// formatBytes 格式化字节大小
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// truncateString 截断字符串到最大长度，超出时追加省略号。
func truncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return s
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
