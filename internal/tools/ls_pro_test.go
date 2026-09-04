package tools

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DotNetAge/goharness/events"
	"github.com/DotNetAge/goharness/logging"
	"github.com/DotNetAge/goharness/sandbox"
	"github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/goharness/tools"
)

// newMockSessionStore 创建用于测试的 mock SessionStore
type mockSessionStore struct {
	sessions map[string]*session.SessionInfo
}

func newMockSessionStore() *mockSessionStore {
	return &mockSessionStore{
		sessions: make(map[string]*session.SessionInfo),
	}
}

func (m *mockSessionStore) Append(ctx context.Context, sessionID string, agentName string, sponsor string, message session.Message) error {
	return nil
}

func (m *mockSessionStore) Get(ctx context.Context, sessionID string) ([]session.Message, error) {
	return nil, nil
}

func (m *mockSessionStore) CurrentContext(ctx context.Context, agentName string, maxTokens int64) ([]session.Message, error) {
	return nil, nil
}

func (m *mockSessionStore) Delete(ctx context.Context, timestamp int64, sessionID string) error {
	return nil
}

func (m *mockSessionStore) Clear(ctx context.Context, sessionID string) error {
	return nil
}

func (m *mockSessionStore) SetSlideHandler(handler session.SlideHandler) {}

func (m *mockSessionStore) Close() error {
	return nil
}

func (m *mockSessionStore) ListSessions(ctx context.Context) ([]session.SessionInfo, error) {
	var result []session.SessionInfo
	for _, s := range m.sessions {
		result = append(result, *s)
	}
	return result, nil
}

func (m *mockSessionStore) Create(ctx context.Context, agentName string, opts ...session.SessionOption) (*session.SessionInfo, error) {
	info := &session.SessionInfo{
		SessionID: "test-session",
		AgentName: agentName,
	}
	for _, opt := range opts {
		opt(info)
	}
	m.sessions[info.SessionID] = info
	return info, nil
}

func (m *mockSessionStore) GetMeta(ctx context.Context, sessionID string) (*session.SessionInfo, error) {
	if s, ok := m.sessions[sessionID]; ok {
		return s, nil
	}
	return nil, os.ErrNotExist
}

func (m *mockSessionStore) ResolveSessionDir(sessionID string) (string, error) {
	return "/tmp/test-sessions/" + sessionID, nil
}

func (m *mockSessionStore) DeleteSession(ctx context.Context, sessionID string) error {
	delete(m.sessions, sessionID)
	return nil
}

func (m *mockSessionStore) GetCursor(ctx context.Context, sessionID string) (int, error) {
	return 0, nil
}

func (m *mockSessionStore) SetCursor(ctx context.Context, sessionID string, cursor int) error {
	return nil
}

func (m *mockSessionStore) SaveModifyFiles(sessionID string, files []string) error {
	return nil
}

func (m *mockSessionStore) GetModifyFiles(sessionID string) ([]string, error) {
	return nil, nil
}

func (m *mockSessionStore) UpdateMessages(ctx context.Context, sessionID string, cursor int, messages []session.Message) error {
	return nil
}

func (m *mockSessionStore) Truncate(ctx context.Context, sessionID string, keepCount int) error {
	return nil
}

// testCtxWithDir 创建带指定项目目录的测试上下文。
// 遵守 goharness 工具契约：ToolContext.Logger 必然非空（生产由 Runtime 注入）；
// 会话必须注入沙箱（安全决策统一收口到沙箱，未注入时工具拒绝执行）。
func testCtxWithDir(t *testing.T, projectDir string) context.Context {
	t.Helper()
	sessionStore := newMockSessionStore()
	sb, err := sandbox.NewSandbox(&sandbox.SandboxPolicy{
		AllowedDirs:       []string{projectDir},
		DeniedFileGlobs:   sandbox.DefaultDeniedFileGlobs(),
		DeniedDirGlobs:    sandbox.DefaultDeniedDirGlobs(),
		DeniedDevicePaths: sandbox.DefaultDeniedDevicePaths(),
	}, logging.NewNopLogger())
	if err != nil {
		t.Fatalf("创建沙箱失败: %v", err)
	}
	sess, err := session.New("test-agent", "", projectDir, sessionStore, logging.NewNopLogger(), session.WithSandbox(sb))
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	return tools.WithToolContext(context.Background(), &tools.ToolContext{
		Session:          sess,
		SessionWhitelist: sess.Whitelist(),
		Logger:           logging.NewNopLogger(),
		EmitEvent:        func(e events.ReactEvent) {},
	})
}

// TestLSPro_Info 测试工具元信息
func TestLSPro_Info(t *testing.T) {
	lsPro := NewLSPro("http://localhost:1318")
	info := lsPro.Info()

	if info.Name != "Ls" {
		t.Errorf("期望 Name='Ls'，实际='%s'", info.Name)
	}

	if info.Description == "" {
		t.Error("Description 不应为空")
	}

	if len(info.Parameters) != 3 {
		t.Errorf("期望 3 个参数，实际=%d", len(info.Parameters))
	}

	// 验证参数名称
	paramNames := make(map[string]bool)
	for _, p := range info.Parameters {
		paramNames[p.Name] = true
	}
	expectedParams := []string{"path", "recursive", "show_hidden"}
	for _, name := range expectedParams {
		if !paramNames[name] {
			t.Errorf("缺少参数: %s", name)
		}
	}
}

// TestLSPro_Execute_WithKnowledgeBase 测试有知识库数据的路径（~/workspaces/sparrow）
func TestLSPro_Execute_WithKnowledgeBase(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("获取主目录失败: %v", err)
	}

	testDir := filepath.Join(homeDir, "workspaces", "sparrow")

	// 检查测试目录是否存在
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		t.Skipf("测试目录不存在: %s", testDir)
	}

	// 使用真实的知识库服务地址
	lsPro := NewLSPro("http://localhost:1318")

	ctx := testCtxWithDir(t, testDir)

	// 执行测试
	result, err := lsPro.Execute(ctx, map[string]any{})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}

	// 验证返回结果
	if result == nil {
		t.Fatal("返回结果不应为空")
	}

	// 知识库可用时应该返回树形结构（string 类型）
	// 知识库不可用时会降级到原生 LS（map 类型）
	resultStr, ok := result.(string)
	if !ok {
		// 降级到原生 LS，测试仍然通过
		t.Logf("知识库服务未运行，已降级到原生 LS 模式")
		resultMap, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("期望返回 string 或 map[string]any，实际=%T", result)
		}
		if success, ok := resultMap["success"].(bool); !ok || !success {
			t.Error("降级模式下期望 success=true")
		}
		return
	}

	// 知识库模式下应该返回树形结构（包含 ├── 或 └──）
	if !strings.Contains(resultStr, "──") {
		t.Error("知识库模式应返回树形结构，但未找到树形符号")
	}

	t.Logf("知识库返回结果（前 500 字符）:\n%s", truncateString(resultStr, 500))

	// 验证缓存机制：第二次调用应该返回缓存提示
	result2, err := lsPro.Execute(ctx, map[string]any{})
	if err != nil {
		t.Fatalf("第二次 Execute 失败: %v", err)
	}

	resultStr2, ok := result2.(string)
	if !ok {
		t.Fatalf("第二次调用期望返回 string，实际=%T", result2)
	}

	if !strings.Contains(resultStr2, "未发生变化") {
		t.Error("第二次调用应返回缓存提示")
	}
}

// TestLSPro_Execute_TildePath_Resolved 回归测试：~ 路径在查询知识库前必须展开为绝对路径。
// 修复前 "~/workspaces/sparrow" 原样发给知识库服务，服务端按字面量匹配不到索引数据，
// 返回空目录树导致误回退原生列表（误报"知识库未启用"）。
func TestLSPro_Execute_TildePath_Resolved(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		t.Skipf("无法获取用户主目录: %v", err)
	}

	var gotDir string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotDir = r.URL.Query().Get("dir")
		w.Header().Set("Content-Type", "application/json")
		// 返回非空目录树，确保 Execute 走知识库分支
		fmt.Fprintf(w, `{"success":true,"data":{"name":"sparrow","path":"%s","size":0,"is_dir":true,"chunks":null,"children":[{"name":"docs","path":"%s/docs","size":0,"is_dir":true,"summary":"测试摘要","chunks":null,"children":null}]}}`, gotDir, gotDir)
	}))
	defer server.Close()

	lsPro := NewLSPro(server.URL)
	projectDir := filepath.Join(homeDir, "workspaces", "sparrow")
	ctx := testCtxWithDir(t, projectDir)

	// 使用子路径：避免与既有测试（TestLSPro_Execute_WithKnowledgeBase）的目录缓存 key 冲突
	result, err := lsPro.Execute(ctx, map[string]any{"path": "~/workspaces/sparrow/docs"})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}

	// 修复后应展开为绝对路径
	wantDir := filepath.Join(homeDir, "workspaces", "sparrow", "docs")
	if gotDir != wantDir {
		t.Errorf("知识库查询 dir 参数=%q，期望展开为 %q", gotDir, wantDir)
	}
	if strings.HasPrefix(gotDir, "~/") {
		t.Errorf("知识库查询 dir 参数未展开 ~：%q", gotDir)
	}

	// 应命中知识库分支返回目录树（string 类型）
	if _, ok := result.(string); !ok {
		t.Errorf("期望返回知识库目录树（string），实际=%T", result)
	}
}

// TestLSPro_Execute_WithoutKnowledgeBase 测试无知识库数据的路径（回退到原生 LS）
func TestLSPro_Execute_WithoutKnowledgeBase(t *testing.T) {
	// 使用临时目录（不会有知识库数据）
	tmpDir := t.TempDir()

	// 创建一些测试文件
	testFiles := []string{"file1.txt", "file2.go", "README.md"}
	for _, f := range testFiles {
		if err := os.WriteFile(filepath.Join(tmpDir, f), []byte("test"), 0644); err != nil {
			t.Fatalf("创建测试文件失败: %v", err)
		}
	}

	// 使用不存在的知识库地址，强制回退到原生 LS
	lsPro := NewLSPro("http://localhost:19999")

	ctx := testCtxWithDir(t, tmpDir)

	// 执行测试
	result, err := lsPro.Execute(ctx, map[string]any{})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}

	// 验证返回结果
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("期望返回 map[string]any（原生 LS 模式），实际=%T", result)
	}

	// 验证 success 字段
	if success, ok := resultMap["success"].(bool); !ok || !success {
		t.Error("期望 success=true")
	}

	// 验证 path 字段
	if path, ok := resultMap["path"].(string); !ok || path != tmpDir {
		t.Errorf("期望 path='%s'，实际='%v'", tmpDir, resultMap["path"])
	}

	// 验证 items 字段
	items, ok := resultMap["items"].([]map[string]any)
	if !ok {
		t.Fatalf("期望 items 为 []map[string]any，实际=%T", resultMap["items"])
	}

	if len(items) != len(testFiles) {
		t.Errorf("期望 %d 个项目，实际=%d", len(testFiles), len(items))
	}

	// 验证每个 item 包含必要字段
	for _, item := range items {
		if _, ok := item["name"]; !ok {
			t.Error("item 缺少 'name' 字段")
		}
		if _, ok := item["type"]; !ok {
			t.Error("item 缺少 'type' 字段")
		}
		if _, ok := item["size"]; !ok {
			t.Error("item 缺少 'size' 字段")
		}
	}

	// 验证 message 字段包含回退提示
	if msg, ok := resultMap["message"].(string); !ok || !strings.Contains(msg, "知识库未启用") {
		t.Error("回退模式的 message 应包含'知识库未启用'提示")
	}
}

// TestLSPro_Execute_Recursive 测试递归参数
func TestLSPro_Execute_Recursive(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建子目录和文件
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("创建子目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("test"), 0644); err != nil {
		t.Fatalf("创建嵌套文件失败: %v", err)
	}

	// 使用不存在的知识库地址，强制回退到原生 LS
	lsPro := NewLSPro("http://localhost:19999")
	ctx := testCtxWithDir(t, tmpDir)

	// 测试 recursive=true
	result, err := lsPro.Execute(ctx, map[string]any{"recursive": true})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("期望返回 map[string]any，实际=%T", result)
	}

	items, ok := resultMap["items"].([]map[string]any)
	if !ok {
		t.Fatalf("期望 items 为 []map[string]any，实际=%T", resultMap["items"])
	}

	// 找到子目录项，验证它有 children
	foundSubDir := false
	for _, item := range items {
		if item["name"] == "subdir" {
			foundSubDir = true
			if children, ok := item["children"].([]map[string]any); !ok || len(children) == 0 {
				t.Error("递归模式下子目录应有 children")
			}
			break
		}
	}
	if !foundSubDir {
		t.Error("未找到子目录项")
	}
}

// TestLSPro_Execute_ShowHidden 测试隐藏文件参数
func TestLSPro_Execute_ShowHidden(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建普通文件和隐藏文件
	if err := os.WriteFile(filepath.Join(tmpDir, "visible.txt"), []byte("test"), 0644); err != nil {
		t.Fatalf("创建文件失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".hidden"), []byte("test"), 0644); err != nil {
		t.Fatalf("创建隐藏文件失败: %v", err)
	}

	lsPro := NewLSPro("http://localhost:19999")
	ctx := testCtxWithDir(t, tmpDir)

	// 测试 show_hidden=false（默认）
	result1, err := lsPro.Execute(ctx, map[string]any{})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}

	resultMap1 := result1.(map[string]any)
	items1 := resultMap1["items"].([]map[string]any)

	// 应该只有 1 个可见文件
	if len(items1) != 1 {
		t.Errorf("默认模式下期望 1 个项目，实际=%d", len(items1))
	}

	// 测试 show_hidden=true
	result2, err := lsPro.Execute(ctx, map[string]any{"show_hidden": true})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}

	resultMap2 := result2.(map[string]any)
	items2 := resultMap2["items"].([]map[string]any)

	// 应该有 2 个文件（包括隐藏文件）
	if len(items2) != 2 {
		t.Errorf("show_hidden=true 模式下期望 2 个项目，实际=%d", len(items2))
	}
}

// TestLSPro_Execute_InvalidPath 测试无效路径
func TestLSPro_Execute_InvalidPath(t *testing.T) {
	lsPro := NewLSPro("http://localhost:19999")

	// 使用不存在的路径
	ctx := testCtxWithDir(t, "/nonexistent/path/that/does/not/exist")

	// 执行测试，应该返回错误
	_, err := lsPro.Execute(ctx, map[string]any{})
	if err == nil {
		t.Error("期望对不存在的路径返回错误，但得到了 nil")
	}
}

// TestLSPro_Execute_ExplicitPath 测试显式指定路径参数
func TestLSPro_Execute_ExplicitPath(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建测试文件
	if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	lsPro := NewLSPro("http://localhost:19999")

	// 会话目录与查询目录一致（避免安全检查失败）
	ctx := testCtxWithDir(t, tmpDir)

	// 显式指定 path 参数（查询子目录）
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("创建子目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "nested.txt"), []byte("nested"), 0644); err != nil {
		t.Fatalf("创建嵌套文件失败: %v", err)
	}

	result, err := lsPro.Execute(ctx, map[string]any{"path": subDir})
	if err != nil {
		t.Fatalf("Execute 失败: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("期望返回 map[string]any，实际=%T", result)
	}

	// 验证返回的 path 是显式指定的路径
	if path, ok := resultMap["path"].(string); !ok || path != subDir {
		t.Errorf("期望 path='%s'，实际='%v'", subDir, resultMap["path"])
	}

	// 验证返回的项目
	items, ok := resultMap["items"].([]map[string]any)
	if !ok {
		t.Fatalf("期望 items 为 []map[string]any，实际=%T", resultMap["items"])
	}

	if len(items) != 1 {
		t.Errorf("期望 1 个项目，实际=%d", len(items))
	}
}
