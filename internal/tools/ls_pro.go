package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/DotNetAge/goharness/events"
	"github.com/DotNetAge/goharness/sandbox"
	"github.com/DotNetAge/goharness/tools"
)

// LSPro 是增强版目录列表工具，融合知识库语义目录树和原生文件系统列表。
// 当知识库服务可用且有对应目录的索引数据时，返回语义化目录树（含文件摘要、分块位置）；
// 否则静默回退到原生文件系统目录列表。
//
// 设计意图：Agent 浏览目录时几乎总是先调用 LS，这是 LLM 的"肌肉记忆"。
// 通过增强 LS 而非要求 Agent 主动使用 QuickExplore，让 Agent 无需改变行为即可享受知识库的语义增强。
type LSPro struct {
	serverURL string   // 知识库服务地址
	whitelist []string // 允许列出的目录前缀（绕过项目边界检查）
}

// NewLSPro 创建增强版 LS 工具，查询知识库服务获取语义目录树。
func NewLSPro(serverURL string) tools.FuncTool {
	return &LSPro{serverURL: serverURL}
}

// AddWhiteList 添加允许列出的目录前缀（如用户偏好目录）。
// 当目标路径匹配任一白名单前缀时，Grant() 与 Execute() 跳过项目边界检查。
// 必须在工具注册到 ToolRegistry 之前调用，与 ReadPro 的行为一致。
func (t *LSPro) AddWhiteList(dirs ...string) *LSPro {
	t.whitelist = append(t.whitelist, dirs...)
	return t
}

// Grant implements tools.PermissionRequired。
//
// 安全决策（工作区边界、敏感文件拦截）统一由沙箱 CheckFile 负责，与内置 LS 工具保持一致：
//   - Allow → 放行
//   - Deny → 拒绝（敏感文件等硬性禁止，授权不可覆盖）
//   - AskUser（越界）→ 先放行工具白名单（AddWhiteList）与会话级白名单
//     （PermissionAllowSession 记忆）内的路径，其余越界目录触发授权流程
//     （返回 granted=false，运行时挂起思考循环等待用户回应）。
//
// 会话未注入沙箱时直接放行，由 Execute 阶段拒绝执行（配置错误，授权无意义）。
func (t *LSPro) Grant(ctx context.Context, params map[string]any) (bool, string) {
	raw, _ := tools.GetParam(params, "path")
	dirPath, _ := raw.(string)
	if dirPath == "" {
		return true, ""
	}

	tc := tools.GetToolContext(ctx)
	if tc == nil || tc.Session == nil {
		return true, ""
	}

	resolved, _ := tools.ResolveTargetPath(dirPath, tc.Session.ProjectDir(), tc.Session.SessionDir())
	if resolved == "" {
		return true, ""
	}

	sb := tc.Session.Sandbox()
	if sb == nil {
		return true, ""
	}
	dec := sb.CheckFile(resolved, tc.Session.ProjectDir())
	switch dec.Decision {
	case sandbox.DecisionAllow:
		return true, ""
	case sandbox.DecisionDeny:
		return false, dec.Reason
	default: // DecisionAskUser：先查工具白名单与会话白名单，未命中则触发授权
		for _, dir := range t.whitelist {
			if pathWithinScope(dir, resolved) {
				return true, ""
			}
		}
		if tc.SessionWhitelist != nil {
			for _, allowed := range tc.SessionWhitelist.Ls {
				if pathWithinScope(allowed, resolved) {
					return true, ""
				}
			}
		}
		return false, dec.Reason
	}
}

// pathWithinScope 判断 child 是否位于 parent 目录范围内（含 parent 本身）。
// 与 goharness/tools 的 pathWithinScope 语义一致：前缀匹配必须紧跟路径分隔符，
// 避免 /a/b 误匹配 /a/bc 的前缀歧义。mindx 侧无法直接复用未导出函数，故本地实现。
func pathWithinScope(parent, child string) bool {
	if parent == child {
		return true
	}
	return strings.HasPrefix(child, parent+string(filepath.Separator))
}

func (t *LSPro) Info() *tools.ToolInfo {
	return &tools.ToolInfo{
		Name:               "Ls",
		MaxResultSizeChars: 30000,
		Description:        "列出目录内容，包含文件元数据和内容摘要。在读取文件之前使用此工具了解项目结构。",
		Prompt: `列出目录内容以浏览文件系统结构。返回结果包含每个文件的内容摘要，无需读取即可了解文件用途。

## 何时使用
- 在读取文件之前探索目录内容
- 了解项目结构和文件组织
- 探索不熟悉的目录时，从根目录开始使用`,
		Tags:          []string{"file", "filesystem", "list", "directory"},
		SecurityLevel: events.LevelSafe,
		Parameters: []tools.Parameter{
			{Name: "path", Type: "string", Description: "要列出的目录路径。默认为当前目录。", Required: false},
			{Name: "recursive", Type: "boolean", Description: "是否递归列出子目录。默认 false。", Required: false},
			{Name: "show_hidden", Type: "boolean", Description: "是否包含隐藏文件。默认 false。", Required: false},
		},
	}
}

// ── 防重复查询缓存 ───────────────────────────────────────────────────

// exploreCache 记录已查询过的目录，避免重复请求知识库 API。
// 缓存命中时不返回结果内容，而是告知模型引用此前返回的结果。
var exploreCache sync.Map

// exploreCacheKey 生成缓存 key。
func exploreCacheKey(targetDir string) string {
	return "ls_pro_explore:" + targetDir
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

func (t *LSPro) Execute(ctx context.Context, params map[string]any) (any, error) {
	// 解析 path 参数
	var targetDir string
	if raw, ok := getParam(params, "path"); ok {
		if v, ok := raw.(string); ok && v != "" {
			targetDir = v
		}
	}
	if targetDir == "" {
		if tc := tools.GetToolContext(ctx); tc != nil && tc.Session != nil {
			targetDir = tc.Session.ProjectDir()
		}
	}
	if targetDir == "" {
		targetDir = "."
	}

	// 统一路径解析：绝对路径化 + ~ 展开 + 相对项目目录解析。
	// 修复前 "~/workspaces/sparrow" 原样传给知识库服务，数据库按字面量查询匹配不到数据，
	// 服务端返回空目录树，导致 Ls 误报"知识库未启用"并错误回退原生列表。
	var projectDir, sessionDir string
	if tc := tools.GetToolContext(ctx); tc != nil && tc.Session != nil {
		projectDir = tc.Session.ProjectDir()
		sessionDir = tc.Session.SessionDir()
	}
	if resolved, _ := tools.ResolveTargetPath(targetDir, projectDir, sessionDir); resolved != "" {
		targetDir = resolved
	}

	// 尝试查询知识库
	treeResult, err := t.tryKnowledgeBase(ctx, targetDir)
	if err == nil && treeResult != "" {
		// 知识库有数据，返回语义目录树
		return treeResult, nil
	}

	// 知识库不可用或无数据，回退到原生 LS
	return t.executeNativeLS(ctx, params, targetDir)
}

// tryKnowledgeBase 尝试从知识库获取语义目录树。
// 返回空字符串表示无数据或请求失败。
func (t *LSPro) tryKnowledgeBase(ctx context.Context, targetDir string) (string, error) {
	// 防重复查询缓存
	if isExploreCached(targetDir) {
		return "目录树自上次查询以来未发生变化。本对话中此前 Ls 的结果仍然有效，请引用此前的结果。", nil
	}

	// 调用 MindStore WebAPI /api/tree
	url := t.serverURL + "/api/tree"
	if targetDir != "" {
		url += "?dir=" + targetDir
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("构建请求失败: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// 服务未启动，返回空字符串触发回退
		return "", nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	var apiResp apiTreeResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if !apiResp.Success {
		return "", fmt.Errorf("知识库查询失败: %s", apiResp.Error)
	}

	if apiResp.Data == nil {
		return "", nil
	}

	// 检查是否为空目录树（无子节点、无文件块）
	if len(apiResp.Data.Children) == 0 && len(apiResp.Data.Chunks) == 0 {
		return "", nil
	}

	result := renderTreeResult(apiResp.Data, "", apiResp.Data.Path)
	markExploreCached(targetDir)
	return result, nil
}

// executeNativeLS 执行原生文件系统目录列表（回退逻辑）。
// 复制自 goharness/tools/ls.go 的 Execute 实现。
func (t *LSPro) executeNativeLS(ctx context.Context, params map[string]any, dirPath string) (any, error) {
	// 统一路径解析：绝对路径化 + ~ 展开 + 相对项目目录解析。
	// 修复前 "~/workspaces" 直接传给 os.Stat 会被当作字面量目录，导致"目录不存在"。
	var projectDir, sessionDir string
	tc := tools.GetToolContext(ctx)
	if tc != nil && tc.Session != nil {
		projectDir = tc.Session.ProjectDir()
		sessionDir = tc.Session.SessionDir()
	}
	resolvedPath, _ := tools.ResolveTargetPath(dirPath, projectDir, sessionDir)

	// 安全校验：工作区边界、敏感文件等策略统一由沙箱强制检查（含符号链接解析，防 TOCTOU）。
	// 透传工具白名单（AddWhiteList）+ 会话白名单：Grant 阶段白名单放行的路径
	// 在 Execute 阶段同样豁免目录边界；设备/敏感文件等硬性禁止不豁免。
	extra := make([]string, 0, len(t.whitelist)+2)
	extra = append(extra, t.whitelist...)
	if tc != nil && tc.SessionWhitelist != nil {
		extra = append(extra, tc.SessionWhitelist.Ls...)
	}
	if err := tools.EnforceSandboxFileWithWhitelist(ctx, resolvedPath, extra); err != nil {
		return nil, err
	}

	// Check if path exists
	info, err := os.Stat(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s", tools.GuideLsDirNotFound(resolvedPath))
		}
		return nil, fmt.Errorf("%s", tools.GuideLsStatFailed(resolvedPath, err))
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("%s", tools.GuideLsNotDirectory(resolvedPath))
	}

	// Get parameters
	recursive := false
	if rawRec, found := tools.GetParam(params, "recursive"); found {
		if rec, ok := rawRec.(bool); ok {
			recursive = rec
		}
	}

	showHidden := false
	if rawHidden, found := tools.GetParam(params, "show_hidden"); found {
		if hidden, ok := rawHidden.(bool); ok {
			showHidden = hidden
		}
	}

	// Read directory contents
	entries, err := os.ReadDir(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("%s", tools.GuideLsReadFailed(resolvedPath, err))
	}

	// Build result
	var items []map[string]any

	for _, entry := range entries {
		if len(items) >= maxLsItems {
			break
		}

		// Skip hidden files unless show_hidden is set
		if !showHidden && strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		finfo, _ := entry.Info()
		item := map[string]any{
			"name": entry.Name(),
			"type": func() string {
				if entry.IsDir() {
					return "directory"
				} else {
					return "file"
				}
			}(),
			"size":    finfo.Size(),
			"modTime": finfo.ModTime().Format("2006-01-02 15:04:05"),
			"mode":    finfo.Mode().String(),
		}

		// If recursive mode and entry is a directory, list its children
		if recursive && entry.IsDir() {
			subDir := filepath.Join(resolvedPath, entry.Name())
			subEntries, subErr := os.ReadDir(subDir)
			if subErr == nil {
				children := make([]map[string]any, 0)
				for _, subEntry := range subEntries {
					if !showHidden && strings.HasPrefix(subEntry.Name(), ".") {
						continue
					}
					subFinfo, _ := subEntry.Info()
					child := map[string]any{
						"name": subEntry.Name(),
						"type": func() string {
							if subEntry.IsDir() {
								return "directory"
							} else {
								return "file"
							}
						}(),
						"size":    subFinfo.Size(),
						"modTime": subFinfo.ModTime().Format("2006-01-02 15:04:05"),
					}
					children = append(children, child)
				}
				item["children"] = children
			}
		}

		items = append(items, item)
	}

	return map[string]any{
		"success":     true,
		"path":        resolvedPath,
		"total_items": len(items),
		"items":       items,
		"message":     fmt.Sprintf("在 '%s' 中列出了 %d 个项目（知识库未启用，使用原生目录列表）", resolvedPath, len(items)),
	}, nil
}

// maxLsItems 是目录列表返回的最大条目数。
// 超过此数量的目录会被截断，防止上下文爆炸。
const maxLsItems = 500

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
