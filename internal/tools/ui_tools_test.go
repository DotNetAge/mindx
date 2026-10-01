package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DotNetAge/goharness/logging"
	"github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/goharness/tools"
)

// nopStore / nopLogger 是测试用空实现（工具执行仅读取会话 ProjectDir，
// 不触发存储与日志调用）。
type uiNopStore struct{ session.SessionStore }
type uiNopLogger struct{ logging.Logger }

// uiToolContext 构造绑定指定项目目录的 ToolContext。
func uiToolContext(t *testing.T, projectDir string) context.Context {
	t.Helper()
	sess, err := session.New("test-agent", "", projectDir, uiNopStore{}, uiNopLogger{})
	if err != nil {
		t.Fatalf("创建测试会话失败: %v", err)
	}
	return tools.WithToolContext(context.Background(), &tools.ToolContext{Session: sess})
}

// uiCapture 收集广播事件供断言。
type uiCapture struct {
	events []string
	data   []map[string]any
}

func (c *uiCapture) broadcast(event string, data map[string]any) {
	c.events = append(c.events, event)
	c.data = append(c.data, data)
}

// TestOpen_WorkspaceGate 验收：Open 工作区闸（相对路径解析落 ProjectDir、越界拒绝）与广播。
func TestOpen_WorkspaceGate(t *testing.T) {
	projectDir := t.TempDir()
	ctx := uiToolContext(t, projectDir)
	cap := &uiCapture{}

	tool := NewOpen(cap.broadcast)

	// 相对路径 → 解析为 ProjectDir 下绝对路径并广播
	res, err := tool.Execute(ctx, map[string]any{"path": "report.md"})
	if err != nil {
		t.Fatalf("相对路径应受理: %v", err)
	}
	if len(cap.events) != 1 || cap.events[0] != "file_open" {
		t.Fatalf("应广播 file_open, 实际 %v", cap.events)
	}
	if cap.data[0]["path"] != filepath.Join(projectDir, "report.md") {
		t.Fatalf("path 应解析为项目内绝对路径, 实际 %v", cap.data[0]["path"])
	}
	if res.(map[string]any)["accepted"] != true {
		t.Fatal("受理语义应返回 accepted=true")
	}

	// 越界路径拒绝且不广播
	if _, err = tool.Execute(ctx, map[string]any{"path": "../outside.md"}); err == nil {
		t.Fatal("越界路径应被拒绝")
	}
	if len(cap.events) != 1 {
		t.Fatalf("越界路径不应广播, 实际事件数 %d", len(cap.events))
	}

	// 缺 path 报错
	if _, err = tool.Execute(ctx, map[string]any{}); err == nil {
		t.Fatal("缺 path 应报错")
	}

	// 无 ToolContext（Session 为 nil）报引导错误
	if _, err = tool.Execute(context.Background(), map[string]any{"path": "a.md"}); err == nil ||
		!strings.Contains(err.Error(), "ProjectDir") {
		t.Fatalf("缺会话上下文应报 ProjectDir 引导错误, 实际 %v", err)
	}
}

// TestVisit_SchemeGate 验收：Visit 仅放行 http/https 并广播 link_open。
func TestVisit_SchemeGate(t *testing.T) {
	ctx := uiToolContext(t, t.TempDir())
	cap := &uiCapture{}
	tool := NewVisit(cap.broadcast)

	if _, err := tool.Execute(ctx, map[string]any{"url": "https://example.com/login"}); err != nil {
		t.Fatalf("https 链接应受理: %v", err)
	}
	if len(cap.events) != 1 || cap.events[0] != "link_open" {
		t.Fatalf("应广播 link_open, 实际 %v", cap.events)
	}

	// 非 http/https 拒绝
	for _, bad := range []string{"ftp://example.com", "file:///etc/passwd", "javascript:alert(1)"} {
		if _, err := tool.Execute(ctx, map[string]any{"url": bad}); err == nil {
			t.Fatalf("%s 应被拒绝", bad)
		}
	}
	if len(cap.events) != 1 {
		t.Fatalf("拒绝项不应广播, 实际事件数 %d", len(cap.events))
	}
}

// TestTerminalRun_CwdResolution 验收：TerminalRun 默认 cwd=ProjectDir、相对 cwd 解析与广播。
func TestTerminalRun_CwdResolution(t *testing.T) {
	projectDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectDir, "sub"), 0o755); err != nil {
		t.Fatalf("创建子目录失败: %v", err)
	}
	ctx := uiToolContext(t, projectDir)
	cap := &uiCapture{}
	tool := NewTerminalRun(cap.broadcast)

	// 默认 cwd = ProjectDir
	if _, err := tool.Execute(ctx, map[string]any{"command": "pnpm test"}); err != nil {
		t.Fatalf("默认 cwd 应受理: %v", err)
	}
	if cap.data[0]["cwd"] != projectDir {
		t.Fatalf("默认 cwd 应为项目目录, 实际 %v", cap.data[0]["cwd"])
	}
	if cap.events[0] != "terminal_run" {
		t.Fatalf("应广播 terminal_run, 实际 %v", cap.events[0])
	}

	// 相对 cwd 解析到项目内子目录
	if _, err := tool.Execute(ctx, map[string]any{"command": "ls", "cwd": "sub"}); err != nil {
		t.Fatalf("相对 cwd 应受理: %v", err)
	}
	if cap.data[1]["cwd"] != filepath.Join(projectDir, "sub") {
		t.Fatalf("相对 cwd 应解析为 %s, 实际 %v", filepath.Join(projectDir, "sub"), cap.data[1]["cwd"])
	}

	// 越界 cwd 拒绝
	if _, err := tool.Execute(ctx, map[string]any{"command": "ls", "cwd": ".."}); err == nil {
		t.Fatal("越界 cwd 应被拒绝")
	}

	// 缺 command 报错
	if _, err := tool.Execute(ctx, map[string]any{}); err == nil {
		t.Fatal("缺 command 应报错")
	}
}
