package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/DotNetAge/goharness/tools"
)

// ReadPro 是增强版文件读取工具，集成知识库分片能力。
//
// 与 Read 的区别：ReadPro 增加显式 preview 参数。当 preview 打开时，
// 不直接读取全文，而是从知识库一次性返回全文的结构化语义摘要
// （分块树：标题+摘要+行号范围），提升大文件读取的精准度——
// LLM 先通过摘要定位关键分块，再用 offset/limit 精确读取。
//
// 分流逻辑（显式参数驱动，不做静默切换）：
//   - preview=true → 查询知识库返回分块树（对 30 行以内的文件无效：
//     全文都在读取阈值内，直接读全文更高效）
//     知识库不可用 / 无分块 / 查询超时 → 回退原 Read（行为与原 Read 完全一致）
//   - 大文件（超过原 Read 全量上限）+ 带 offset/limit → 按行范围精读（绕过全量限制）
//   - 其余情况 → 完全委托原 Read（文件不存在、目录、小文件、无 preview 等）
//
// 最佳实践：当文件超出读取阈值时，先用 preview 获取结构化语义摘要定位关键分块，
// 再用 offset/limit 精确读取，避免大文件全量进入上下文。
//
// 注意：知识库查询只在 preview 打开时发生；未打开 preview 的 ReadPro
// 与原 Read 的行为完全一致。ReadPro 只承担"知识库分块树预览"职责，
// 与图片读取链路无关（图片读取开关由调用方直接配置在内嵌的 goharness Read 上，
// 图片的消费由 goharness 层的 ImageHook 完成）。
type ReadPro struct {
	Read      *tools.Read // 内嵌原 Read（具名字段，避免方法提升暴露 Read 的图片相关 API）
	serverURL string      // 知识库服务地址
}

// NewReadPro 创建增强版 Read 工具。
func NewReadPro(serverURL string) tools.FuncTool {
	return &ReadPro{
		Read:      tools.NewReadTool(),
		serverURL: serverURL,
	}
}

// AddWhiteList 转发到内嵌 Read，保留项目外目录（如用户偏好目录）的读取白名单能力。
// 必须在工具注册到 ToolRegistry 之前调用，与原 Read 的行为一致。
func (p *ReadPro) AddWhiteList(dirs ...string) *ReadPro {
	p.Read.AddWhiteList(dirs...)
	return p
}

// Grant implements tools.PermissionRequired，转发到内嵌的原 Read。
// ReadPro 与 Read 共享完全相同的授权语义：越界路径（工作区外）触发授权流程，
// 工具白名单或会话级白名单（PermissionAllowSession）内的路径直接放行。
func (p *ReadPro) Grant(ctx context.Context, params map[string]any) (bool, string) {
	return p.Read.Grant(ctx, params)
}

// Info 返回 Read 工具的元信息。
// 在原 Read 元信息基础上增加显式 preview 参数（布尔开关）：
// 打开 preview 时一次性预览全文的结构化语义摘要（知识库分块树），
// 提升大文件读取的精准度。Prompt 与原 Read 保持一致。
func (p *ReadPro) Info() *tools.ToolInfo {
	base := p.Read.Info()
	cp := *base
	cp.Parameters = make([]tools.Parameter, 0, len(base.Parameters)+1)
	cp.Parameters = append(cp.Parameters, base.Parameters...)
	cp.Parameters = append(cp.Parameters, tools.Parameter{
		Name:        "preview",
		Type:        "boolean",
		Required:    false,
		Description: "为 true 时，不读取全文，而是从知识库返回该文件的结构化语义摘要（分块树：标题+摘要+行号范围），一次性预览全文结构，便于定位后再用 offset/limit 精确读取。仅对已索引到知识库的文件生效；知识库不可用或无分块时自动回退为普通读取。对 30 行以内的文件无效（全文都在读取阈值内，直接读取更高效）。默认 false。",
	})
	return &cp
}

// Execute 执行文件读取操作。
func (p *ReadPro) Execute(ctx context.Context, params map[string]any) (any, error) {
	raw, _ := tools.GetParam(params, "filePath")
	filePath, _ := raw.(string)
	if filePath == "" {
		// 缺参数：委托原 Read 返回错误引导
		return p.Read.Execute(ctx, params)
	}

	tc := tools.GetToolContext(ctx)
	var projectDir, sessionDir string
	if tc != nil && tc.Session != nil {
		projectDir = tc.Session.ProjectDir()
		sessionDir = tc.Session.SessionDir()
	}
	resolvedPath, _ := tools.ResolveTargetPath(filePath, projectDir, sessionDir)

	info, err := os.Stat(resolvedPath)
	// 文件不存在 / 是目录 → 原样委托原 Read
	if err != nil || info.IsDir() {
		return p.Read.Execute(ctx, params)
	}

	// 显式 preview 开关：打开时才查询知识库，返回全文的结构化语义摘要（分块树）。
	// 不做基于文件大小的静默切换——知识库查询只在 preview=true 时发生。
	if paramPreview(params) {
		// preview 只对"有意义大小"的文件生效：行数不超过 30 行的文件，
		// 全文都在读取阈值范围内，直接读取全文比查看分块摘要更高效，
		// 此时 preview 无意义，委托原 Read 读全文。
		if countLinesAtLeast(resolvedPath, 30) <= 30 {
			return p.Read.Execute(ctx, params)
		}
		if err := p.Read.CheckSafety(resolvedPath); err != nil {
			return nil, err
		}
		if result, ok := p.tryChunkTree(ctx, resolvedPath, info.Size()); ok {
			return result, nil
		}
		// 知识库不可用 / 无分块 / 查询超时 → 回退原 Read（行为与原 Read 一致）
		return p.Read.Execute(ctx, params)
	}

	// 大文件（超过原 Read 全量上限）+ 显式 offset/limit → 按行范围精读
	//（绕过全量限制，由显式范围参数触发，非静默切换）
	if hasOffsetOrLimit(params) && info.Size() > p.Read.Limits().MaxSizeBytes {
		if err := p.Read.CheckSafety(resolvedPath); err != nil {
			return nil, err
		}
		// 图片文件按行精读会把二进制内容当文本输出（乱码），
		// 委托原 Read 保持其行为（大图片返回 file_too_large 引导）。
		if isImagePath(resolvedPath) {
			return p.Read.Execute(ctx, params)
		}
		return p.readRange(resolvedPath, params)
	}

	// 默认：完全委托原 Read（行为 100% 一致）
	return p.Read.Execute(ctx, params)
}

// paramPreview 解析显式 preview 参数（兼容布尔、字符串与数字表示）。
func paramPreview(params map[string]any) bool {
	raw, ok := tools.GetParam(params, "preview")
	if !ok {
		return false
	}
	switch v := raw.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true") || v == "1"
	case float64:
		return v != 0
	}
	return false
}

// countLinesAtLeast 快速统计文件行数，超过 max 时立即中止扫描并返回 max+1。
// 用于 preview 的小文件判定：行数不超过 30 行的文件直接读全文即可，
// 不必为它们做无意义的知识库预览。文件读取失败返回 0（视为小文件）。
func countLinesAtLeast(path string, max int) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	// 扩大缓冲区上限，兼容超长行（如压缩后的单行 JSON）
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	count := 0
	for scanner.Scan() {
		count++
		if count > max {
			return count
		}
	}
	return count
}

// hasOffsetOrLimit 判断请求是否带范围参数。
func hasOffsetOrLimit(params map[string]any) bool {
	if _, found := tools.GetParam(params, "offset"); found {
		return true
	}
	if _, found := tools.GetParam(params, "limit"); found {
		return true
	}
	return false
}

// isImagePath 判断路径是否为 goharness Read 支持的图片格式。
// 与 goharness/tools 的 supportedImageExtensions 保持一致（png/jpg/jpeg/gif/bmp/webp）。
// 图片文件是二进制内容，不能按行范围精读。
func isImagePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".webp":
		return true
	default:
		return false
	}
}

// readRange 按行范围读取大文件，避免全量读入内存。
// offset 为起始行号（从 1 开始），limit 为最大行数（默认 500）。
func (p *ReadPro) readRange(resolvedPath string, params map[string]any) (any, error) {
	startLine := 1
	if rawOffset, found := tools.GetParam(params, "offset"); found {
		if offset, ok := tools.ToFloat64(rawOffset); ok && offset > 0 {
			startLine = int(offset)
		}
	}
	maxLines := 500
	if rawLimit, found := tools.GetParam(params, "limit"); found {
		if limit, ok := tools.ToFloat64(rawLimit); ok && limit > 0 {
			maxLines = int(limit)
		}
	}

	f, err := os.Open(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("%s", tools.GuideReadIO(resolvedPath, err))
	}
	defer f.Close()

	// 扩大 Scanner 缓冲区上限，兼容超长行（如压缩后的单行 JSON）
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var sb strings.Builder
	lineNum := 0
	linesRead := 0
	endLine := startLine + maxLines - 1
	hasMore := false
	for scanner.Scan() {
		lineNum++
		if lineNum < startLine {
			continue
		}
		if lineNum > endLine {
			hasMore = true
			break
		}
		sb.WriteString(fmt.Sprintf("%d\t%s\n", lineNum, scanner.Text()))
		linesRead++
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%s", tools.GuideReadIO(resolvedPath, err))
	}

	data := &tools.ReadData{
		Success:   true,
		Path:      resolvedPath,
		Content:   sb.String(),
		StartLine: startLine,
		LinesRead: linesRead,
		HasMore:   hasMore,
	}
	if hasMore {
		data.NextOffset = endLine + 1
		data.Suggestion = tools.SuggestionHasMoreLines
		data.Note = fmt.Sprintf("在偏移量 %d 处有更多内容。可继续使用 offset=%d 读取后续部分。", endLine+1, endLine+1)
	} else {
		data.Suggestion = tools.SuggestionReadComplete
	}
	return &tools.ReadResult{Data: data}, nil
}

// tryChunkTree 查询知识库获取文件分块树。
// 返回 false 表示知识库不可用或该文件无分块数据。
func (p *ReadPro) tryChunkTree(ctx context.Context, resolvedPath string, sizeBytes int64) (any, bool) {
	chunks, ok := p.fetchFileChunks(ctx, resolvedPath)
	if !ok || len(chunks) == 0 {
		return nil, false
	}

	nodes := p.buildChunkTree(chunks)
	content := p.renderChunkTree(nodes, resolvedPath, sizeBytes, len(chunks))

	return &tools.ReadResult{
		Data: &tools.ReadData{
			Success:    true,
			Path:       resolvedPath,
			SizeBytes:  sizeBytes,
			Content:    content,
			Suggestion: "chunk_tree_preview",
			Note:       "已返回知识库结构化语义摘要（分块树），未直接读取全文。请定位关键分块后用 offset/limit 精确读取。",
		},
	}, true
}

// ── 知识库分块查询 ───────────────────────────────────────────────

// chunksPageSize 是每页分块数量；chunksMaxPages 是最大翻页数（防御异常数据）。
const (
	chunksPageSize = 200
	chunksMaxPages = 20
)

// chunksAPIResponse 与 /api/chunks 的响应结构一致。
type chunksAPIResponse struct {
	Success bool        `json:"success"`
	Data    *chunksPage `json:"data"`
	Error   string      `json:"error,omitempty"`
}

// chunksPage 是分块列表页。
type chunksPage struct {
	Page  int         `json:"page"`
	Size  int         `json:"size"`
	Total int         `json:"total"`
	Items []chunkItem `json:"items"`
}

// chunkItem 与服务端 core.Chunk 的 JSON 序列化字段保持一致。
type chunkItem struct {
	ID        string `json:"id"`
	ParentID  string `json:"parent_id,omitempty"`
	Title     string `json:"title"`
	Summary   string `json:"summary"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	StartPos  int    `json:"start_pos"`
	EndPos    int    `json:"end_pos"`
}

// fetchFileChunks 分页拉取指定文件的全部 Chunk。
func (p *ReadPro) fetchFileChunks(ctx context.Context, resolvedPath string) ([]chunkItem, bool) {
	var all []chunkItem
	for page := 1; page <= chunksMaxPages; page++ {
		u := fmt.Sprintf("%s/api/chunks?filter=%s&page=%d&size=%d",
			p.serverURL, url.QueryEscape(resolvedPath), page, chunksPageSize)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, false
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			// 服务未启动 → 回退
			return nil, false
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, false
		}
		var apiResp chunksAPIResponse
		if err := json.Unmarshal(body, &apiResp); err != nil || !apiResp.Success || apiResp.Data == nil {
			return nil, false
		}
		all = append(all, apiResp.Data.Items...)
		// 当前页不满一页或已达总数 → 拉全
		if len(apiResp.Data.Items) < chunksPageSize || page*apiResp.Data.Size >= apiResp.Data.Total {
			break
		}
	}
	return all, true
}

// ── 分块树构建与渲染 ─────────────────────────────────────────────

// buildChunkTree 将扁平分块列表按 ParentID 组织为分块树（顶层块按 StartPos 排序）。
func (p *ReadPro) buildChunkTree(chunks []chunkItem) []sourceChunkNode {
	childMap := make(map[string][]chunkItem)
	var parents []chunkItem
	for _, c := range chunks {
		if c.ParentID == "" {
			parents = append(parents, c)
		} else {
			childMap[c.ParentID] = append(childMap[c.ParentID], c)
		}
	}
	for pid := range childMap {
		sort.Slice(childMap[pid], func(i, j int) bool {
			return childMap[pid][i].StartPos < childMap[pid][j].StartPos
		})
	}
	sort.Slice(parents, func(i, j int) bool {
		return parents[i].StartPos < parents[j].StartPos
	})

	nodes := make([]sourceChunkNode, 0, len(parents))
	for _, pc := range parents {
		nodes = append(nodes, chunkToSourceNode(pc, childMap, make(map[string]bool)))
	}
	return nodes
}

// chunkToSourceNode 递归构建单个分块节点及其子块。
// visited 用于检测 ParentID 循环引用，防止异常数据导致无限递归。
func chunkToSourceNode(c chunkItem, childMap map[string][]chunkItem, visited map[string]bool) sourceChunkNode {
	node := sourceChunkNode{
		Title:     c.Title,
		Summary:   c.Summary,
		StartLine: c.StartLine,
		EndLine:   c.EndLine,
		StartPos:  c.StartPos,
		EndPos:    c.EndPos,
	}
	visited[c.ID] = true
	for _, child := range childMap[c.ID] {
		if visited[child.ID] {
			continue
		}
		node.Children = append(node.Children, chunkToSourceNode(child, childMap, visited))
	}
	return node
}

// renderChunkTree 渲染文件分块树（带行号范围，便于 LLM 用 offset/limit 定位）。
func (p *ReadPro) renderChunkTree(nodes []sourceChunkNode, resolvedPath string, sizeBytes int64, chunkCount int) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("文件 %s（%.1f KB，共 %d 个分块）未直接读取全文。知识库结构化语义摘要（分块树）如下，行号为源文件中的真实行号：\n\n",
		filepath.Base(resolvedPath), float64(sizeBytes)/1024, chunkCount))
	for i, node := range nodes {
		sb.WriteString(fmt.Sprintf("%d. ", i+1))
		sb.WriteString(readChunkLine(node, 0))
		sb.WriteString("\n")
	}
	sb.WriteString("\n定位到关键分块后，使用 Read 工具精确读取该段，例如：\n")
	if len(nodes) > 0 && nodes[0].StartLine > 0 {
		lines := nodes[0].EndLine - nodes[0].StartLine + 1
		if lines < 1 {
			lines = 1
		}
		sb.WriteString(fmt.Sprintf("  offset=%d limit=%d  → 读取第 1 个分块\n", nodes[0].StartLine, lines))
	}
	return sb.String()
}

// readChunkLine 渲染单行分块信息：行号范围 + 标题 + 摘要，子块递归缩进。
func readChunkLine(node sourceChunkNode, depth int) string {
	indent := strings.Repeat("  ", depth)
	lineRange := ""
	if node.StartLine > 0 && node.EndLine >= node.StartLine {
		lineRange = fmt.Sprintf(" [行 %d-%d]", node.StartLine, node.EndLine)
	}
	title := strings.ReplaceAll(node.Title, "\n", " ")
	summary := strings.TrimSpace(strings.ReplaceAll(node.Summary, "\n", " "))
	text := indent + title + lineRange
	if summary != "" {
		text += " - " + truncateString(summary, 100)
	}
	for _, child := range node.Children {
		text += "\n" + readChunkLine(child, depth+1)
	}
	return text
}
