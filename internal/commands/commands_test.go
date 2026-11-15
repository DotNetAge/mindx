package commands

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/i18n"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/DotNetAge/mindx/pkg/scheduler"
)

func init() {
	// 命令注册与错误文案依赖 i18n；测试固定 zh 保证断言确定性。
	i18n.MustInit("zh")
}

func TestNewRegistryContainsBuiltins(t *testing.T) {
	r := New()
	names := map[string]bool{}
	for _, m := range r.Metas() {
		names[m.Name] = true
	}
	for _, want := range []string{"help", "about", "init", "clear", "agents", "models", "skills", "job.add", "job.list", "job.del", "compact"} {
		if !names[want] {
			t.Fatalf("built-in command %q missing from New()", want)
		}
	}
}

func TestRegistryRegisterAllWith(t *testing.T) {
	r := &Registry{}
	r.Register(Meta{Name: "a"}, func(ctx *gateway.CommandContext) (any, error) { return "raw-a", nil })
	r.Register(Meta{Name: "b"}, func(ctx *gateway.CommandContext) (any, error) { return "raw-b", nil })

	// nil registerer：静默不动
	r.RegisterAllWith(nil, nil)

	var registered []string
	reg := func(meta Meta, handler func(ctx *gateway.CommandContext) (any, error)) {
		registered = append(registered, meta.Name)
	}
	r.RegisterAllWith(reg, nil)
	if len(registered) != 2 || registered[0] != "a" || registered[1] != "b" {
		t.Fatalf("registered = %v", registered)
	}

	// wrap 施加到每条命令
	r2 := &Registry{}
	r2.Register(Meta{Name: "a"}, func(ctx *gateway.CommandContext) (any, error) { return "raw", nil })
	var wrappedNames []string
	wrap := func(name string, h Handler) Handler {
		return func(ctx *gateway.CommandContext) (any, error) {
			wrappedNames = append(wrappedNames, name)
			return h(ctx)
		}
	}
	var calls int
	reg2 := func(meta Meta, handler func(ctx *gateway.CommandContext) (any, error)) {
		calls++
		res, err := handler(&gateway.CommandContext{})
		if err != nil || res != "raw" {
			t.Fatalf("wrapped handler result = %v, %v", res, err)
		}
	}
	r2.RegisterAllWith(reg2, wrap)
	if calls != 1 || len(wrappedNames) != 1 || wrappedNames[0] != "a" {
		t.Fatalf("wrap not applied: calls=%d wrapped=%v", calls, wrappedNames)
	}
}

func TestLocalRegistry(t *testing.T) {
	lr := NewLocal()
	metas := lr.Metas()
	if len(metas) != 3 {
		t.Fatalf("local commands = %d, want 3", len(metas))
	}
	for i, want := range []string{"help", "clear", "exit"} {
		if metas[i].Name != want {
			t.Fatalf("local command[%d] = %q, want %q", i, metas[i].Name, want)
		}
	}
}

func TestSystemPureHandlers(t *testing.T) {
	// about / init / clear 不读 ctx，直接经 Registry 注册的 handler 验证。
	r := New()
	handlers := map[string]Handler{}
	for _, c := range r.commands {
		handlers[c.meta.Name] = c.handler
	}
	for name := range handlers {
		switch name {
		case "about", "init", "clear":
			res, err := handlers[name](&gateway.CommandContext{})
			if err != nil || res == nil || res == "" {
				t.Fatalf("%s handler = (%v, %v)", name, res, err)
			}
		}
	}
}

// handlersOf 提取 Registry 里注册的命令 handler，用于直接驱动注册闭包。
func handlersOf(r *Registry) map[string]Handler {
	m := map[string]Handler{}
	for _, c := range r.commands {
		m[c.meta.Name] = c.handler
	}
	return m
}

func resetAllDeps(t *testing.T) {
	t.Helper()
	resetCatalogDeps(t)
	resetSchedulerDeps(t)
	resetCompactDeps(t)
}

func TestRegistrationClosures(t *testing.T) {
	resetAllDeps(t)

	// catalog：deps 未配置时经注册闭包返回错误（覆盖闭包与 handler 的未配置分支）
	handlers := handlersOf(New())
	for _, name := range []string{"agents", "models", "skills"} {
		if _, err := handlers[name](&gateway.CommandContext{}); err == nil {
			t.Fatalf("%s closure with nil deps should error", name)
		}
	}

	// scheduler：job.add / job.del 闭包走通成功路径（不触发 RespondWithType）
	st := newTestSchedulerStore(t)
	schedulerDeps = SchedulerDeps{SchedulerDB: func() *scheduler.FileSchedulerStore { return st }}
	if _, err := handlers["job.add"](&gateway.CommandContext{Args: `@大秘 expr="0 0 9 * * *" new 闭包测试`}); err != nil {
		t.Fatalf("job.add closure: %v", err)
	}
	entries, _ := st.List(context.Background())
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if _, err := handlers["job.del"](&gateway.CommandContext{Args: "id=" + entries[0].ID}); err != nil {
		t.Fatalf("job.del closure: %v", err)
	}

	// compact：闭包走「无活跃会话」分支
	compactDeps = CompactDeps{CompactSession: func(ctx context.Context, params rpc.SessionCompactParams) (map[string]any, error) {
		return nil, nil
	}}
	if _, err := handlers["compact"](&gateway.CommandContext{}); err == nil {
		t.Fatal("compact closure without session should error")
	}
}

func TestSetDeps(t *testing.T) {
	SetCatalogDeps(CatalogDeps{ListAgents: func() ([]map[string]string, error) { return nil, nil }})
	if catalogDeps.ListAgents == nil {
		t.Fatal("SetCatalogDeps not stored")
	}
	SetSchedulerDeps(SchedulerDeps{})
	SetCompactDeps(CompactDeps{})
}

// ---- catalog ----

func resetCatalogDeps(t *testing.T) {
	t.Cleanup(func() { catalogDeps = CatalogDeps{} })
}

func TestCatalogNotConfigured(t *testing.T) {
	resetCatalogDeps(t)
	catalogDeps = CatalogDeps{}
	for name, h := range map[string]func(ctx *gateway.CommandContext) (any, error){
		"agents": handleAgents,
		"models": handleModels,
		"skills": handleSkills,
	} {
		if _, err := h(&gateway.CommandContext{}); err == nil {
			t.Fatalf("%s with nil deps should error", name)
		}
	}
}

func TestCatalogDepsErrorPropagates(t *testing.T) {
	resetCatalogDeps(t)
	boom := errors.New("backend down")
	catalogDeps = CatalogDeps{ListAgents: func() ([]map[string]string, error) { return nil, boom }}
	if _, err := handleAgents(&gateway.CommandContext{}); !errors.Is(err, boom) {
		t.Fatalf("handleAgents err = %v, want backend error", err)
	}
}

func TestCatalogHelpers(t *testing.T) {
	if got := displayName(map[string]string{"nick_name": "梁工", "name": "liang"}); got != "梁工" {
		t.Fatalf("displayName nick = %q", got)
	}
	if got := displayName(map[string]string{"name": "liang"}); got != "liang" {
		t.Fatalf("displayName fallback = %q", got)
	}

	items := []map[string]string{
		{"nick_name": "大秘", "name": "dashu", "role": "chief", "description": "总管"},
		{"name": "bare"},
	}
	rows := toAgentTableRows(items)
	if len(rows) != 2 || rows[0][0] != "大秘" || rows[0][1] != "chief" || rows[0][2] != "总管" || rows[1][0] != "bare" {
		t.Fatalf("toAgentTableRows = %v", rows)
	}

	rows2 := toTableRows(items, "name", "description")
	if len(rows2) != 2 || rows2[0][0] != "dashu" || rows2[0][1] != "总管" || rows2[1][0] != "bare" || rows2[1][1] != "" {
		t.Fatalf("toTableRows = %v", rows2)
	}
}

// ---- scheduler ----

func newTestSchedulerStore(t *testing.T) *scheduler.FileSchedulerStore {
	t.Helper()
	st, err := scheduler.NewFileSchedulerStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func resetSchedulerDeps(t *testing.T) {
	t.Cleanup(func() { schedulerDeps = SchedulerDeps{} })
}

func TestHandleJobAdd_NotConfigured(t *testing.T) {
	resetSchedulerDeps(t)
	schedulerDeps = SchedulerDeps{}
	if _, err := handleJobAdd(&gateway.CommandContext{Args: "@a expr=\"0 0 9 * * *\" new hi"}); err == nil {
		t.Fatal("job.add without deps should error")
	}
}

func TestHandleJobAdd_EmptyArgs(t *testing.T) {
	resetSchedulerDeps(t)
	st := newTestSchedulerStore(t)
	schedulerDeps = SchedulerDeps{SchedulerDB: func() *scheduler.FileSchedulerStore { return st }}
	if _, err := handleJobAdd(&gateway.CommandContext{Args: "   "}); err == nil {
		t.Fatal("job.add with empty args should error")
	}
}

func TestHandleJobAdd_ParseErrors(t *testing.T) {
	resetSchedulerDeps(t)
	st := newTestSchedulerStore(t)
	schedulerDeps = SchedulerDeps{SchedulerDB: func() *scheduler.FileSchedulerStore { return st }}

	cases := []string{
		`expr="0 0 9 * * *" new hi`,     // 缺 @agent
		`@大秘 new hi`,                    // 缺 expr
		`@大秘 expr="0 0 9 * * *"`,        // 缺内容
		`@大秘 expr="0 9 * * 1-5" new hi`, // 5 字段 cron 不支持
	}
	for _, args := range cases {
		if _, err := handleJobAdd(&gateway.CommandContext{Args: args}); err == nil {
			t.Fatalf("job.add %q should error", args)
		}
	}
}

func TestHandleJobAdd_Success(t *testing.T) {
	resetSchedulerDeps(t)
	st := newTestSchedulerStore(t)
	schedulerDeps = SchedulerDeps{SchedulerDB: func() *scheduler.FileSchedulerStore { return st }}

	ctx := &gateway.CommandContext{Args: `@大秘 expr="0 0 9 * * *" new 早报 每天九点汇报`}
	res, err := handleJobAdd(ctx)
	if err != nil {
		t.Fatalf("handleJobAdd: %v", err)
	}
	out, ok := res.(string)
	if !ok || !strings.Contains(out, "(auto)") || !strings.Contains(out, "@大秘") {
		t.Fatalf("handleJobAdd output = %v", res)
	}

	entries, err := st.List(context.Background())
	if err != nil || len(entries) != 1 {
		t.Fatalf("store after add: %v entries, err %v", len(entries), err)
	}
	e := entries[0]
	if e.Agent != "大秘" || e.SessionID != "" || e.CronExpr != "0 0 9 * * *" ||
		e.Content != "早报 每天九点汇报" || !e.Enabled || e.ID == "" {
		t.Fatalf("saved entry = %+v", e)
	}
}

func TestHandleJobAdd_ExplicitSessionIDKept(t *testing.T) {
	resetSchedulerDeps(t)
	st := newTestSchedulerStore(t)
	schedulerDeps = SchedulerDeps{SchedulerDB: func() *scheduler.FileSchedulerStore { return st }}

	ctx := &gateway.CommandContext{Args: `@大秘 expr="0 0 9 * * *" sess-1 检查邮件`}
	if _, err := handleJobAdd(ctx); err != nil {
		t.Fatalf("handleJobAdd: %v", err)
	}
	entries, _ := st.List(context.Background())
	if len(entries) != 1 || entries[0].SessionID != "sess-1" {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestHandleJobDel(t *testing.T) {
	resetSchedulerDeps(t)
	st := newTestSchedulerStore(t)

	// 未配置
	schedulerDeps = SchedulerDeps{}
	if _, err := handleJobDel(&gateway.CommandContext{Args: "id=x"}); err == nil {
		t.Fatal("job.del without deps should error")
	}

	schedulerDeps = SchedulerDeps{SchedulerDB: func() *scheduler.FileSchedulerStore { return st }}

	// 缺 id
	if _, err := handleJobDel(&gateway.CommandContext{Args: ""}); err == nil {
		t.Fatal("job.del without id should error")
	}

	// 不存在的 id
	if _, err := handleJobDel(&gateway.CommandContext{Args: "id=nope"}); err == nil {
		t.Fatal("job.del unknown id should error")
	}

	// 成功：先存后删
	entry := &scheduler.ScheduleEntry{ID: "abc123", Agent: "大秘", Content: "日报", CronExpr: "0 0 9 * * *", Enabled: true}
	if err := st.Save(context.Background(), entry); err != nil {
		t.Fatal(err)
	}
	res, err := handleJobDel(&gateway.CommandContext{Args: "id=abc123"})
	if err != nil {
		t.Fatalf("handleJobDel: %v", err)
	}
	if out, _ := res.(string); !strings.Contains(out, "abc123") || !strings.Contains(out, "@大秘") {
		t.Fatalf("handleJobDel output = %v", res)
	}
	if _, err := st.Load(context.Background(), "abc123"); err == nil {
		t.Fatal("entry should be deleted")
	}
}

func TestHandleJobList_NotConfigured(t *testing.T) {
	resetSchedulerDeps(t)
	schedulerDeps = SchedulerDeps{}
	if _, err := handleJobList(&gateway.CommandContext{}); err == nil {
		t.Fatal("job.list without deps should error")
	}
}

func TestParseJobAddArgs(t *testing.T) {
	// 完整参数：agent / expr / dir / session / content
	agent, sess, content, expr, dir, err := parseJobAddArgs(`@大秘 expr="0 0 9 * * *" dir="/tmp/a b" sess-1 检查邮件`)
	if err != nil {
		t.Fatalf("parseJobAddArgs: %v", err)
	}
	if agent != "大秘" || sess != "sess-1" || content != "检查邮件" || expr != "0 0 9 * * *" || dir != "/tmp/a b" {
		t.Fatalf("parsed = %q %q %q %q %q", agent, sess, content, expr, dir)
	}

	// project= 是 dir= 的别名
	_, _, _, _, dir, err = parseJobAddArgs(`@a expr="0 0 9 * * *" project=/tmp/x s c`)
	if err != nil || dir != "/tmp/x" {
		t.Fatalf("project= alias: dir=%q err=%v", dir, err)
	}

	// 仅一个内容片段：回退为 content，session 置 "new"
	_, sess, content, _, _, err = parseJobAddArgs(`@a expr="0 0 9 * * *" 巡检`)
	if err != nil || sess != "new" || content != "巡检" {
		t.Fatalf("single content part: sess=%q content=%q err=%v", sess, content, err)
	}

	// 多内容片段：首段为 session，其余合并为 content
	_, sess, content, _, _, err = parseJobAddArgs(`@a expr="0 0 9 * * *" sess-2 巡检 服务器`)
	if err != nil || sess != "sess-2" || content != "巡检 服务器" {
		t.Fatalf("multi content parts: sess=%q content=%q err=%v", sess, content, err)
	}

	// 缺 agent / 缺 expr
	if _, _, _, _, _, err := parseJobAddArgs(`expr="0 0 9 * * *" s c`); err == nil {
		t.Fatal("missing agent should error")
	}
	if _, _, _, _, _, err := parseJobAddArgs(`@a s c`); err == nil {
		t.Fatal("missing expr should error")
	}
}

func TestTruncateString(t *testing.T) {
	if got := truncateString("short", 10); got != "short" {
		t.Fatalf("short = %q", got)
	}
	if got := truncateString("exactly10!", 10); got != "exactly10!" {
		t.Fatalf("exact = %q", got)
	}
	if got := truncateString("0123456789ABC", 10); got != "0123456789..." {
		t.Fatalf("truncated = %q", got)
	}
}

func TestSplitArgsAndParseCommandArgs(t *testing.T) {
	// splitArgs：引号内空格不切分，引号字符保留
	parts := splitArgs(`@a "hello world" 'x y' tail`)
	want := []string{"@a", `"hello world"`, `'x y'`, "tail"}
	if len(parts) != len(want) {
		t.Fatalf("splitArgs = %v", parts)
	}
	for i := range want {
		if parts[i] != want[i] {
			t.Fatalf("splitArgs[%d] = %q, want %q", i, parts[i], want[i])
		}
	}

	// parseCommandArgs：key=value，值剥引号；无等号的片段忽略
	args := parseCommandArgs(`id="job 1" enabled=true bare`)
	if args["id"] != "job 1" || args["enabled"] != "true" {
		t.Fatalf("parseCommandArgs = %v", args)
	}
	if _, ok := args["bare"]; ok {
		t.Fatalf("bare fragment should be ignored: %v", args)
	}
	if got := parseCommandArgs(""); len(got) != 0 {
		t.Fatalf("empty args = %v", got)
	}
}

// ---- compact ----

func resetCompactDeps(t *testing.T) {
	t.Cleanup(func() { compactDeps = CompactDeps{} })
}

func TestHandleCompact(t *testing.T) {
	resetCompactDeps(t)

	// 未配置
	compactDeps = CompactDeps{}
	if _, err := handleCompact(&gateway.CommandContext{SessionID: "s1"}); err == nil {
		t.Fatal("compact without deps should error")
	}

	// 无活跃会话
	compactDeps = CompactDeps{CompactSession: func(ctx context.Context, params rpc.SessionCompactParams) (map[string]any, error) {
		return nil, nil
	}}
	if _, err := handleCompact(&gateway.CommandContext{}); err == nil {
		t.Fatal("compact without session should error")
	}

	// 依赖失败：错误包装后透传
	boom := errors.New("llm unavailable")
	compactDeps = CompactDeps{CompactSession: func(ctx context.Context, params rpc.SessionCompactParams) (map[string]any, error) {
		if params.SessionID != "sess-9" {
			t.Fatalf("CompactSession session = %q, want sess-9", params.SessionID)
		}
		return nil, boom
	}}
	if _, err := handleCompact(&gateway.CommandContext{SessionID: "sess-9"}); !errors.Is(err, boom) {
		t.Fatalf("compact err = %v, want wrapped boom", err)
	}
}
