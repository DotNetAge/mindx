package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/goharness/tools"
)

// writeLargeFile 写入超过 MaxSizeBytes（256KB）的大文件，返回其路径。
func writeLargeFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "big.log")
	var sb strings.Builder
	// 每行 100 字符，共 4000 行 → ~400KB > 256KB
	for i := 0; i < 4000; i++ {
		sb.WriteString(fmt.Sprintf("line-%05d-%s\n", i, strings.Repeat("x", 88)))
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("写入大文件失败: %v", err)
	}
	return path
}

// TestReadPro_Info 验证工具元信息：增加显式 preview 参数，Prompt 与原 Read 保持一致。
func TestReadPro_Info(t *testing.T) {
	readPro := NewReadPro("http://localhost:1318")
	original := tools.NewReadTool()
	info := readPro.Info()

	if info.Name != "Read" {
		t.Errorf("期望 Name='Read'，实际='%s'", info.Name)
	}
	if info.Prompt != original.Info().Prompt {
		t.Error("Prompt 不应被改动（与原 Read 保持一致）")
	}
	if strings.Contains(info.Prompt, "大文件自动预览") {
		t.Error("Prompt 不应包含扩展说明（与原 Read 保持一致）")
	}
	// 参数应在原 Read 基础上增加 preview 开关
	paramNames := make(map[string]bool)
	paramDescs := make(map[string]string)
	for _, p := range info.Parameters {
		paramNames[p.Name] = true
		paramDescs[p.Name] = p.Description
	}
	for _, name := range []string{"filePath", "offset", "limit", "preview"} {
		if !paramNames[name] {
			t.Errorf("缺少参数: %s", name)
		}
	}
	if !strings.Contains(paramDescs["preview"], "知识库") {
		t.Error("preview 参数说明应提及知识库语义摘要")
	}
}

// TestReadPro_SmallFile_DelegatesToRead 小文件完全委托原 Read（行为一致）。
func TestReadPro_SmallFile_DelegatesToRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "small.txt")
	content := "hello\nworld\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("创建小文件失败: %v", err)
	}

	// 知识库地址故意不可用，小文件也不应查询知识库
	readPro := NewReadPro("http://localhost:19999")
	ctx := testCtxWithDir(t, dir)

	result, err := readPro.Execute(ctx, map[string]any{"filePath": path})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}

	rr, ok := result.(*tools.ReadResult)
	if !ok {
		t.Fatalf("期望返回 *tools.ReadResult，实际=%T", result)
	}
	if !rr.Data.Success {
		t.Fatalf("期望 success=true，实际 note=%q", rr.Data.Note)
	}
	if !strings.Contains(rr.Data.Content, "hello") || !strings.Contains(rr.Data.Content, "world") {
		t.Errorf("小文件应返回完整内容，实际=%q", truncateString(rr.Data.Content, 200))
	}
}

// TestReadPro_MissingPath_DelegatesError 缺 filePath 参数时委托原 Read 返回错误引导。
func TestReadPro_MissingPath_DelegatesError(t *testing.T) {
	readPro := NewReadPro("http://localhost:19999")
	ctx := testCtxWithDir(t, t.TempDir())

	_, err := readPro.Execute(ctx, map[string]any{})
	if err == nil {
		t.Fatal("缺少 filePath 应返回错误")
	}
	if !strings.Contains(err.Error(), "filePath") {
		t.Errorf("错误信息应提及 filePath，实际=%q", err.Error())
	}
}

// TestReadPro_LargeFile_WithRange 大文件 + offset/limit → 按行范围读取，不读全量。
func TestReadPro_LargeFile_WithRange(t *testing.T) {
	path := writeLargeFile(t)
	dir := filepath.Dir(path)

	// 知识库地址不可用；但带范围参数时不应查询知识库
	readPro := NewReadPro("http://localhost:19999")
	ctx := testCtxWithDir(t, dir)

	result, err := readPro.Execute(ctx, map[string]any{"filePath": path, "offset": 101, "limit": 3})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}

	rr, ok := result.(*tools.ReadResult)
	if !ok {
		t.Fatalf("期望返回 *tools.ReadResult，实际=%T", result)
	}
	if !rr.Data.Success {
		t.Fatalf("期望 success=true，实际=%+v", rr.Data)
	}
	if rr.Data.StartLine != 101 {
		t.Errorf("期望 StartLine=101，实际=%d", rr.Data.StartLine)
	}
	if rr.Data.LinesRead != 3 {
		t.Errorf("期望 LinesRead=3，实际=%d", rr.Data.LinesRead)
	}
	// 内容应包含第 101-103 行的行号前缀
	for _, want := range []string{"101\t", "102\t", "103\t"} {
		if !strings.Contains(rr.Data.Content, want) {
			t.Errorf("内容应包含 %q，实际=%q", want, truncateString(rr.Data.Content, 200))
		}
	}
	if !rr.Data.HasMore {
		t.Error("读取 [101,103] 后文件仍有更多，期望 HasMore=true（4000 行远大于 103）")
	}
}

// TestReadPro_LargeFile_ChunkTree 大文件 + preview=true + 知识库有分块 → 返回分块树。
func TestReadPro_LargeFile_ChunkTree(t *testing.T) {
	path := writeLargeFile(t)
	dir := filepath.Dir(path)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chunks" {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("filter"); got != path {
			t.Errorf("filter 参数=%q，期望=%q", got, path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"success":true,"data":{"page":1,"size":200,"total":3,"items":[
			{"id":"c1","parent_id":"","title":"开头说明","summary":"日志格式简介","start_line":1,"end_line":2000,"start_pos":0,"end_pos":200000},
			{"id":"c2","parent_id":"","title":"业务实现","summary":"核心逻辑所在","start_line":2001,"end_line":3500,"start_pos":200001,"end_pos":350000},
			{"id":"c2-1","parent_id":"c2","title":"子块A","summary":"子块细节","start_line":2100,"end_line":2200,"start_pos":210001,"end_pos":220000}
		]}}`)
	}))
	defer server.Close()

	readPro := NewReadPro(server.URL)
	ctx := testCtxWithDir(t, dir)

	result, err := readPro.Execute(ctx, map[string]any{"filePath": path, "preview": true})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}

	rr, ok := result.(*tools.ReadResult)
	if !ok {
		t.Fatalf("期望返回 *tools.ReadResult，实际=%T", result)
	}
	if !rr.Data.Success {
		t.Fatalf("期望 success=true，实际=%+v", rr.Data)
	}
	if rr.Data.Suggestion != "chunk_tree_preview" {
		t.Errorf("期望 Suggestion=chunk_tree_preview，实际=%q", rr.Data.Suggestion)
	}
	if rr.Data.SizeBytes == 0 {
		t.Error("期望返回文件大小")
	}
	// 分块树应包含标题、行号范围与子块缩进
	content := rr.Data.Content
	for _, want := range []string{"开头说明", "[行 1-2000]", "业务实现", "[行 2001-3500]", "子块A"} {
		if !strings.Contains(content, want) {
			t.Errorf("分块树应包含 %q，实际=%q", want, truncateString(content, 400))
		}
	}
	// 应给出 offset/limit 精读示例
	if !strings.Contains(content, "offset=") || !strings.Contains(content, "limit=") {
		t.Errorf("分块树应包含 offset/limit 精读示例，实际=%q", truncateString(content, 400))
	}
}

// writeMediumFile 写入约 48KB 的中型文件（>24KB，远小于原 Read 的 256KB 全量上限）。
func writeMediumFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "medium.go")
	var sb strings.Builder
	// 每行 40 字节，共 1200 行 → ~48KB
	for i := 0; i < 1200; i++ {
		sb.WriteString(fmt.Sprintf("// 第 %d 行 - %s\n", i, strings.Repeat("中", 11)))
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("写入中型文件失败: %v", err)
	}
	return path
}

// TestReadPro_MediumFile_ChunkTree 中型文件 + preview=true → 返回分块树。
// preview 开关与文件大小无关：只要显式打开，就查询知识库返回结构化语义摘要。
func TestReadPro_MediumFile_ChunkTree(t *testing.T) {
	path := writeMediumFile(t)
	dir := filepath.Dir(path)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chunks" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"success":true,"data":{"page":1,"size":200,"total":2,"items":[
			{"id":"m1","parent_id":"","title":"类型定义","summary":"主要类型","start_line":1,"end_line":600,"start_pos":0,"end_pos":24000},
			{"id":"m2","parent_id":"","title":"核心逻辑","summary":"处理流程","start_line":601,"end_line":1200,"start_pos":24001,"end_pos":48000}
		]}}`)
	}))
	defer server.Close()

	readPro := NewReadPro(server.URL)
	ctx := testCtxWithDir(t, dir)

	result, err := readPro.Execute(ctx, map[string]any{"filePath": path, "preview": true})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}

	rr, ok := result.(*tools.ReadResult)
	if !ok {
		t.Fatalf("期望返回 *tools.ReadResult，实际=%T", result)
	}
	if rr.Data.Suggestion != "chunk_tree_preview" {
		t.Errorf("preview=true 应返回分块树预览（Suggestion=chunk_tree_preview），实际=%q", rr.Data.Suggestion)
	}
	if !strings.Contains(rr.Data.Content, "类型定义") || !strings.Contains(rr.Data.Content, "核心逻辑") {
		t.Errorf("分块树应包含两个分块标题，实际=%q", truncateString(rr.Data.Content, 400))
	}
}

// TestReadPro_MediumFile_KBUnavailable 中型文件 + preview=true + 知识库不可用 → 回退原 Read。
// 文件小于原 Read 的 256KB 上限，降级后由原 Read 全量读取（与 Read 行为一致）。
func TestReadPro_MediumFile_KBUnavailable(t *testing.T) {
	path := writeMediumFile(t)
	dir := filepath.Dir(path)

	readPro := NewReadPro("http://localhost:19999")
	ctx := testCtxWithDir(t, dir)

	result, err := readPro.Execute(ctx, map[string]any{"filePath": path, "preview": true})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}

	rr, ok := result.(*tools.ReadResult)
	if !ok {
		t.Fatalf("期望返回 *tools.ReadResult，实际=%T", result)
	}
	if !rr.Data.Success {
		t.Fatalf("知识库不可用时 preview 应降级为原 Read 全量读取，实际 success=false note=%q", rr.Data.Note)
	}
	if !strings.Contains(rr.Data.Content, "第 1 行") {
		t.Errorf("降级模式应读取到文件内容，实际=%q", truncateString(rr.Data.Content, 200))
	}
}

// emptyChunksServer 返回知识库服务可用、但该文件未被索引（空分块）的 mock 服务。
func emptyChunksServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"success":true,"data":{"page":1,"size":200,"total":0,"items":[]}}`)
	}))
}

// TestReadPro_NotIndexed_MediumFile 知识库可用但该文件未被索引（空分块）+ preview=true → 回退原 Read 全量读取。
func TestReadPro_NotIndexed_MediumFile(t *testing.T) {
	path := writeMediumFile(t) // 48KB：<256KB 原 Read 全量上限
	dir := filepath.Dir(path)

	server := emptyChunksServer()
	defer server.Close()

	readPro := NewReadPro(server.URL)
	ctx := testCtxWithDir(t, dir)

	result, err := readPro.Execute(ctx, map[string]any{"filePath": path, "preview": true})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}

	rr, ok := result.(*tools.ReadResult)
	if !ok {
		t.Fatalf("期望返回 *tools.ReadResult，实际=%T", result)
	}
	if !rr.Data.Success {
		t.Fatalf("文件未被索引时应回退原 Read 全量读取，实际 success=false note=%q", rr.Data.Note)
	}
	if !strings.Contains(rr.Data.Content, "第 1 行") {
		t.Errorf("回退模式应读取到文件内容，实际=%q", truncateString(rr.Data.Content, 200))
	}
}

// TestReadPro_NotIndexed_LargeFile 知识库可用但该文件未被索引 + preview=true → 回退原 Read 的"文件太大"引导。
func TestReadPro_NotIndexed_LargeFile(t *testing.T) {
	path := writeLargeFile(t) // 400KB：超过原 Read 的 256KB 全量上限
	dir := filepath.Dir(path)

	server := emptyChunksServer()
	defer server.Close()

	readPro := NewReadPro(server.URL)
	ctx := testCtxWithDir(t, dir)

	result, err := readPro.Execute(ctx, map[string]any{"filePath": path, "preview": true})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}

	rr, ok := result.(*tools.ReadResult)
	if !ok {
		t.Fatalf("期望返回 *tools.ReadResult，实际=%T", result)
	}
	if rr.Data.Suggestion != tools.SuggestionFileTooLarge {
		t.Errorf("文件未被索引时大文件应回退原 Read 的 %q 引导，实际=%q", tools.SuggestionFileTooLarge, rr.Data.Suggestion)
	}
}

// TestReadPro_LargeFile_OutsideProject 大文件位于项目目录外 → Grant 触发授权流程。
// 授权语义与原 Read 一致（PermissionRequired）：越界路径不再在 Execute 侧硬拦截，
// 而是由 Grant 返回 granted=false 挂起思考循环等待用户授权；授权后
// （PermissionAllowSession 写入会话白名单）Execute 放行并可真正读取。
func TestReadPro_LargeFile_OutsideProject(t *testing.T) {
	path := writeLargeFile(t) // 400KB，位于独立临时目录
	projectDir := t.TempDir() // 项目目录指向另一处，文件在边界之外

	readPro, ok := NewReadPro("http://localhost:19999").(*ReadPro)
	if !ok {
		t.Fatalf("期望返回 *ReadPro，实际=%T", readPro)
	}
	ctx := testCtxWithDir(t, projectDir)

	// 越界路径 → Grant 返回 false（触发授权流程）
	granted, reason := readPro.Grant(ctx, map[string]any{"filePath": path, "offset": 1, "limit": 10})
	if granted {
		t.Fatal("项目外文件应触发授权流程（granted=false）")
	}
	// 沙箱拒绝文案格式："该路径位于工作区 %q 之外"
	if !strings.Contains(reason, "位于工作区") || !strings.Contains(reason, "之外") {
		t.Errorf("授权原因应说明越界，实际=%q", reason)
	}

	// 会话级白名单记忆（PermissionAllowSession 写入的条目）→ Grant 放行，Execute 可执行
	ctx = testCtxWithDir(t, projectDir)
	tc := tools.GetToolContext(ctx)
	tc.SessionWhitelist = &session.SessionWhitelist{Read: []string{filepath.Dir(path)}}

	if granted, _ = readPro.Grant(ctx, map[string]any{"filePath": path}); !granted {
		t.Fatal("会话白名单内的路径应放行（granted=true）")
	}
	result, err := readPro.Execute(ctx, map[string]any{"filePath": path, "offset": 1, "limit": 5})
	if err != nil {
		t.Fatalf("会话白名单内大文件应允许读取: %v", err)
	}
	rr, ok := result.(*tools.ReadResult)
	if !ok {
		t.Fatalf("期望返回 *tools.ReadResult，实际=%T", result)
	}
	if !rr.Data.Success {
		t.Fatalf("期望 success=true，实际=%+v", rr.Data)
	}
}

// TestReadPro_LargeFile_Whitelisted 大文件位于项目外但加入白名单 → 允许读取（与原 Read 一致）。
func TestReadPro_LargeFile_Whitelisted(t *testing.T) {
	path := writeLargeFile(t)
	projectDir := t.TempDir() // 项目目录在另一处

	readPro, ok := NewReadPro("http://localhost:19999").(*ReadPro)
	if !ok {
		t.Fatalf("期望返回 *ReadPro，实际=%T", readPro)
	}
	readPro.AddWhiteList(filepath.Dir(path))
	ctx := testCtxWithDir(t, projectDir)

	result, err := readPro.Execute(ctx, map[string]any{"filePath": path, "offset": 1, "limit": 5})
	if err != nil {
		t.Fatalf("白名单内大文件应允许读取: %v", err)
	}
	rr, ok := result.(*tools.ReadResult)
	if !ok {
		t.Fatalf("期望返回 *tools.ReadResult，实际=%T", result)
	}
	if !rr.Data.Success {
		t.Fatalf("期望 success=true，实际=%+v", rr.Data)
	}
}

// TestReadPro_LargeFile_KBUnavailable_Fallback 大文件 + preview=true + 知识库不可用 → 回退原 Read。
func TestReadPro_LargeFile_KBUnavailable_Fallback(t *testing.T) {
	path := writeLargeFile(t)
	dir := filepath.Dir(path)

	// 不可用的知识库地址（端口不会监听）
	readPro := NewReadPro("http://localhost:19999")
	ctx := testCtxWithDir(t, dir)

	result, err := readPro.Execute(ctx, map[string]any{"filePath": path, "preview": true})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}

	rr, ok := result.(*tools.ReadResult)
	if !ok {
		t.Fatalf("期望返回 *tools.ReadResult，实际=%T", result)
	}
	// 原 Read 对大文件的引导：file_too_large
	if rr.Data.Suggestion != tools.SuggestionFileTooLarge {
		t.Errorf("回退模式应返回 %q 引导，实际=%q", tools.SuggestionFileTooLarge, rr.Data.Suggestion)
	}
}

// TestReadPro_LargeFile_Pagination 知识库分页拉取：mock 返回两页，应拉全所有分块。
func TestReadPro_LargeFile_Pagination(t *testing.T) {
	path := writeLargeFile(t)
	dir := filepath.Dir(path)

	// 第一页 200 条 + 第二页 50 条 = 250 个分块
	page1 := buildChunksJSON(0, 200, 250)
	page2 := buildChunksJSON(200, 50, 250)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		page := r.URL.Query().Get("page")
		if page == "2" {
			fmt.Fprint(w, page2)
		} else {
			fmt.Fprint(w, page1)
		}
	}))
	defer server.Close()

	readPro := NewReadPro(server.URL)
	ctx := testCtxWithDir(t, dir)

	result, err := readPro.Execute(ctx, map[string]any{"filePath": path, "preview": true})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}

	rr, ok := result.(*tools.ReadResult)
	if !ok {
		t.Fatalf("期望返回 *tools.ReadResult，实际=%T", result)
	}
	if !strings.Contains(rr.Data.Content, "共 250 个分块") {
		t.Errorf("分块树应声明 250 个分块，实际=%q", truncateString(rr.Data.Content, 400))
	}
	if !strings.Contains(rr.Data.Content, "chunk-249") {
		t.Errorf("应包含第 250 个分块（分页拉全），实际=%q", truncateString(rr.Data.Content, 400))
	}
}

// buildChunksJSON 构造 /api/chunks 响应体（id 从 chunk-%d 起始）。
func buildChunksJSON(start, count, total int) string {
	var sb strings.Builder
	sb.WriteString(`{"success":true,"data":{"page":1,"size":200,"total":` + fmt.Sprint(total) + `,"items":[`)
	for i := 0; i < count; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		id := start + i
		sb.WriteString(fmt.Sprintf(`{"id":"chunk-%d","parent_id":"","title":"chunk-%d","summary":"分块%d摘要","start_line":%d,"end_line":%d,"start_pos":%d,"end_pos":%d}`,
			id, id, id, id*100+1, id*100+100, id*1000, id*1000+999))
	}
	sb.WriteString(`]}}`)
	return sb.String()
}

// kbIndexed 探测真实知识库中该文件是否已有分块数据。
// 兼容两种服务响应：正常态 data.items（page/size/total/items）与
// 服务未初始化态 data.chunks（svc==nil 时空数组）。两者都无分块时视为未索引。
func kbIndexed(t *testing.T, target string) bool {
	t.Helper()
	// 带超时的客户端：服务异常挂起时快速失败（按未索引处理 → 跳过测试），避免阻塞整个测试进程
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:1318/api/chunks?filter=%s&page=1&size=1", url.PathEscape(target)))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false
	}
	var probe struct {
		Success bool `json:"success"`
		Data    struct {
			Total  int               `json:"total"`
			Items  []json.RawMessage `json:"items"`
			Chunks []json.RawMessage `json:"chunks"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &probe) != nil || !probe.Success {
		return false
	}
	return probe.Data.Total > 0 || len(probe.Data.Items) > 0 || len(probe.Data.Chunks) > 0
}

// TestReadPro_Execute_WithRealKnowledgeBase 集成验证：使用真实知识库服务（localhost:1318），
// 以 preview=true 读取真实文件，验证返回分块树预览，并输出体积对比。
// 文件不存在、知识库未运行或目标文件未被索引时跳过（不阻塞 CI）。
func TestReadPro_Execute_WithRealKnowledgeBase(t *testing.T) {
	targets := []string{
		"/Users/ray/workspaces/bega-labs/docs/todo.md",
		"/Users/ray/workspaces/bega-labs/docs/server-side/assessment.md",
		"/Users/ray/workspaces/bega-labs/docs/server-side/architecture.md",
	}

	// 探测知识库服务是否可用（带 2 秒超时，服务异常挂起时快速跳过而非阻塞）
	probeClient := &http.Client{Timeout: 2 * time.Second}
	probe, err := probeClient.Get("http://localhost:1318/api/tree?dir=/Users/ray/workspaces/bega-labs")
	if err != nil {
		t.Skip("知识库服务未运行，跳过集成验证")
	}
	probe.Body.Close()
	if probe.StatusCode != http.StatusOK {
		t.Skipf("知识库服务响应异常（HTTP %d），跳过集成验证", probe.StatusCode)
	}

	readPro := NewReadPro("http://localhost:1318")
	ctx := testCtxWithDir(t, "/Users/ray/workspaces/bega-labs")

	for _, target := range targets {
		fi, err := os.Stat(target)
		if err != nil || fi.IsDir() {
			t.Logf("跳过（文件不存在）: %s", target)
			continue
		}
		// 服务在线但文件未被索引（或服务未初始化）→ preview 按设计回退原 Read，
		// 无法验证分块树渲染，跳过而非误报失败。
		if !kbIndexed(t, target) {
			t.Skipf("文件未被知识库索引，跳过集成验证: %s", target)
		}

		result, err := readPro.Execute(ctx, map[string]any{"filePath": target, "preview": true})
		if err != nil {
			t.Fatalf("Execute 失败(%s): %v", target, err)
		}
		rr, ok := result.(*tools.ReadResult)
		if !ok {
			t.Fatalf("期望返回 *tools.ReadResult，实际=%T", result)
		}
		if rr.Data.Suggestion != "chunk_tree_preview" {
			t.Fatalf("preview=true 应返回分块树预览，实际 Suggestion=%q（note=%q）", rr.Data.Suggestion, rr.Data.Note)
		}
		contentLen := len(rr.Data.Content)
		ratio := float64(contentLen) / float64(fi.Size()) * 100
		t.Logf("文件 %s：原文件 %d 字节 → 分块树 %d 字节（仅 %.1f%%）",
			filepath.Base(target), fi.Size(), contentLen, ratio)
		if int64(contentLen) > fi.Size() {
			t.Errorf("分块树不应大于原文件：%d > %d", contentLen, fi.Size())
		}
	}
	t.Logf("分块树预览示例（todo.md，前 600 字符）:\n%s", truncateString(firstChunkTree(t, readPro, ctx, targets[0]), 600))
}

// firstChunkTree 读取单个真实文件的返回内容（供示例展示用）。
func firstChunkTree(t *testing.T, readPro tools.FuncTool, ctx context.Context, target string) string {
	t.Helper()
	result, err := readPro.Execute(ctx, map[string]any{"filePath": target, "preview": true})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	rr, ok := result.(*tools.ReadResult)
	if !ok {
		t.Fatalf("期望返回 *tools.ReadResult，实际=%T", result)
	}
	return rr.Data.Content
}

// TestReadPro_Timeout 确保整个流程受 ctx 超时约束（知识库查询超时不应挂起）。
func TestReadPro_Timeout(t *testing.T) {
	path := writeLargeFile(t)
	dir := filepath.Dir(path)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 客户端 ctx 取消时立即退出，避免 server.Close() 阻塞等待 handler
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer server.Close()

	readPro := NewReadPro(server.URL)
	baseCtx := testCtxWithDir(t, dir)
	ctx, cancel := context.WithTimeout(baseCtx, 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := readPro.Execute(ctx, map[string]any{"filePath": path, "preview": true})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	// 知识库查询超时后应回退原 Read（不返回错误、不挂起），耗时受超时约束
	if elapsed > 2*time.Second {
		t.Errorf("执行耗时 %v 过长，ctx 超时应快速回退", elapsed)
	}
}

// TestReadPro_NoPreview_NoKBQuery 验证只有 preview=true 才会查询知识库：
// 未打开 preview 的大文件（无 offset/limit）完全委托原 Read（file_too_large 引导），
// 知识库服务不会被请求。
func TestReadPro_NoPreview_NoKBQuery(t *testing.T) {
	path := writeLargeFile(t)
	dir := filepath.Dir(path)

	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"success":true,"data":{"page":1,"size":200,"total":1,"items":[
			{"id":"c1","parent_id":"","title":"分块","summary":"摘要","start_line":1,"end_line":100,"start_pos":0,"end_pos":1000}
		]}}`)
	}))
	defer server.Close()

	readPro := NewReadPro(server.URL)
	ctx := testCtxWithDir(t, dir)

	result, err := readPro.Execute(ctx, map[string]any{"filePath": path})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	rr, ok := result.(*tools.ReadResult)
	if !ok {
		t.Fatalf("期望返回 *tools.ReadResult，实际=%T", result)
	}
	// 未打开 preview → 委托原 Read → 大文件返回 file_too_large 引导
	if rr.Data.Suggestion != tools.SuggestionFileTooLarge {
		t.Errorf("未打开 preview 时应委托原 Read 返回 %q 引导，实际=%q", tools.SuggestionFileTooLarge, rr.Data.Suggestion)
	}
	if got := atomic.LoadInt32(&requests); got != 0 {
		t.Errorf("未打开 preview 时不应查询知识库，实际请求 %d 次", got)
	}
}

// TestReadPro_Preview_SmallFile 小文件（30 行以内）+ preview=true → preview 无意义，
// 直接委托原 Read 读取全文（全文都在读取阈值内），不查询知识库。
func TestReadPro_Preview_SmallFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "small.md")
	content := "# 标题\n\n小文件正文内容，不足三十行。\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("创建小文件失败: %v", err)
	}

	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"success":true,"data":{"page":1,"size":200,"total":1,"items":[
			{"id":"s1","parent_id":"","title":"小文件分块","summary":"完整内容","start_line":1,"end_line":5,"start_pos":0,"end_pos":100}
		]}}`)
	}))
	defer server.Close()

	readPro := NewReadPro(server.URL)
	ctx := testCtxWithDir(t, dir)

	result, err := readPro.Execute(ctx, map[string]any{"filePath": path, "preview": true})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	rr, ok := result.(*tools.ReadResult)
	if !ok {
		t.Fatalf("期望返回 *tools.ReadResult，实际=%T", result)
	}
	// 小文件 + preview → 直接读全文（success=true 且内容为原文）
	if !rr.Data.Success {
		t.Fatalf("小文件 preview 应降级为直接读全文，实际 success=false note=%q", rr.Data.Note)
	}
	if !strings.Contains(rr.Data.Content, "小文件正文内容") {
		t.Errorf("小文件应直接返回全文内容，实际=%q", truncateString(rr.Data.Content, 200))
	}
	if got := atomic.LoadInt32(&requests); got != 0 {
		t.Errorf("小文件 preview 不应查询知识库，实际请求 %d 次", got)
	}
}

// TestReadPro_Preview_ThirtyLinesBoundary 边界验证：恰好 30 行的文件 + preview
// → 仍视为小文件直接读全文；31 行以上才生效。
func TestReadPro_Preview_ThirtyLinesBoundary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "boundary.go")
	var sb strings.Builder
	for i := 0; i < 30; i++ {
		sb.WriteString(fmt.Sprintf("// 第 %d 行\n", i))
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("创建文件失败: %v", err)
	}

	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"success":true,"data":{"page":1,"size":200,"total":1,"items":[
			{"id":"b1","parent_id":"","title":"分块","summary":"摘要","start_line":1,"end_line":30,"start_pos":0,"end_pos":1000}
		]}}`)
	}))
	defer server.Close()

	readPro := NewReadPro(server.URL)
	ctx := testCtxWithDir(t, dir)

	result, err := readPro.Execute(ctx, map[string]any{"filePath": path, "preview": true})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	rr, ok := result.(*tools.ReadResult)
	if !ok {
		t.Fatalf("期望返回 *tools.ReadResult，实际=%T", result)
	}
	if !rr.Data.Success || !strings.Contains(rr.Data.Content, "第 0 行") {
		t.Errorf("恰好 30 行的文件 preview 应直接读全文，实际 note=%q", rr.Data.Note)
	}
	if got := atomic.LoadInt32(&requests); got != 0 {
		t.Errorf("恰好 30 行不应查询知识库，实际请求 %d 次", got)
	}
}

// TestReadPro_LargeImage_WithRange 大图片（超过原 Read 全量上限）+ offset/limit
// → 图片是二进制内容，不能按行精读（会输出乱码），应委托原 Read 保持其行为。
func TestReadPro_LargeImage_WithRange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.png")
	// 写入超过原 Read 全量上限（256KB）的二进制数据（重复 PNG 魔数块）
	data := bytes.Repeat([]byte{0x89, 0x50, 0x4E, 0x47}, 70*1024) // 280KB
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("写入大图片失败: %v", err)
	}

	readPro := NewReadPro("http://localhost:19999")
	ctx := testCtxWithDir(t, dir)

	result, err := readPro.Execute(ctx, map[string]any{"filePath": path, "offset": 1, "limit": 5})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}
	rr, ok := result.(*tools.ReadResult)
	if !ok {
		t.Fatalf("期望返回 *tools.ReadResult，实际=%T", result)
	}
	// 大图片应委托原 Read 返回 file_too_large 引导，而非按行输出乱码
	if rr.Data.Suggestion != tools.SuggestionFileTooLarge {
		t.Errorf("大图片应委托原 Read 返回 %q 引导，实际=%q", tools.SuggestionFileTooLarge, rr.Data.Suggestion)
	}
	if strings.Contains(rr.Data.Content, "\t") && strings.Contains(rr.Data.Content, "\x89") {
		t.Error("大图片不应按行精读输出二进制乱码")
	}
}
