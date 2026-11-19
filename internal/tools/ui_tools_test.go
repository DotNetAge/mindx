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

// TestHandoff_BroadcastAndGuards 验收：交接事件参数透传 + 三道闸（自我交接/未安装/目录越界）。
func TestHandoff_BroadcastAndGuards(t *testing.T) {
	projectDir := t.TempDir()
	ctx := uiToolContext(t, projectDir)
	cap := &uiCapture{}

	exists := func(name string) bool { return name == "go-programmer" }
	tool := NewHandoff(cap.broadcast, exists)

	// 正常交接：广播 session_handoff，参数逐一透传，返回文案点明「不用继续」
	res, err := tool.Execute(ctx, map[string]any{
		"agent": "go-programmer", "task": "把导出改成流式", "mode": "resume",
	})
	if err != nil {
		t.Fatalf("正常交接应受理: %v", err)
	}
	if len(cap.events) != 1 || cap.events[0] != "session_handoff" {
		t.Fatalf("应广播 session_handoff, 实际 %v", cap.events)
	}
	d := cap.data[0]
	if d["agent"] != "go-programmer" || d["task"] != "把导出改成流式" || d["mode"] != "resume" {
		t.Fatalf("事件参数不符: %v", d)
	}
	if d["project_dir"] != projectDir {
		t.Fatalf("project_dir 应默认当前项目目录, 实际 %v", d["project_dir"])
	}
	note, _ := res.(map[string]any)["note"].(string)
	if !strings.Contains(note, "go-programmer") || !strings.Contains(note, "不用继续") {
		t.Fatalf("返回文案需点明接手方与收尾语义, 实际 %q", note)
	}

	// deliver 默认 draft：事件带 deliver，返回文案点明「等用户按发送键」
	if d["deliver"] != "draft" {
		t.Fatalf("deliver 应默认 draft, 实际 %v", d["deliver"])
	}
	if !strings.Contains(note, "发送键") {
		t.Fatalf("draft 文案需说明等用户按发送键, 实际 %q", note)
	}

	// deliver=send：直接发出，文案不再提发送键
	cap.events, cap.data = nil, nil
	if _, err := tool.Execute(ctx, map[string]any{
		"agent": "go-programmer", "task": "y", "deliver": "send",
	}); err != nil {
		t.Fatalf("send 应受理: %v", err)
	}
	if cap.data[0]["deliver"] != "send" {
		t.Fatalf("deliver 未透传: %v", cap.data[0]["deliver"])
	}

	// 闸 1：交接给当前会话自己的智能体
	if _, err := tool.Execute(ctx, map[string]any{"agent": "test-agent", "task": "x"}); err == nil {
		t.Fatal("交接给自己应报错")
	}

	// 闸 2：目标未安装
	if _, err := tool.Execute(ctx, map[string]any{"agent": "ghost", "task": "x"}); err == nil {
		t.Fatal("交接给未安装的智能体应报错")
	}

	// 闸 3：目录越界
	if _, err := tool.Execute(ctx, map[string]any{
		"agent": "go-programmer", "task": "x", "project_dir": "../outside",
	}); err == nil {
		t.Fatal("越界目录应报错")
	}

	// 闸 5：deliver 非法
	if _, err := tool.Execute(ctx, map[string]any{
		"agent": "go-programmer", "task": "x", "deliver": "later",
	}); err == nil {
		t.Fatal("非法 deliver 应报错")
	}

	// 闸 4：mode 非法
	if _, err := tool.Execute(ctx, map[string]any{
		"agent": "go-programmer", "task": "x", "mode": "bad",
	}); err == nil {
		t.Fatal("非法 mode 应报错")
	}
}
