package svc

import (
	"testing"
)

func TestParseAgentTarget(t *testing.T) {
	cases := []struct {
		in                     string
		agent, session, remain string
	}{
		{"普通消息", "", "", "普通消息"},
		{"@alpha", "alpha", "", ""},
		{"@alpha 你好", "alpha", "", "你好"},
		{"@alpha 01ARZ3NDEKTSV4RRFFQ69G5FAV 查一下", "alpha", "01ARZ3NDEKTSV4RRFFQ69G5FAV", "查一下"},
		{"@alpha 01ARZ3NDEKTSV4RRFFQ69G5FAV", "alpha", "01ARZ3NDEKTSV4RRFFQ69G5FAV", ""},
		{"@alpha not-ulid 内容", "alpha", "", "not-ulid 内容"},
	}
	for _, c := range cases {
		agent, session, remain := parseAgentTarget(c.in)
		if agent != c.agent || session != c.session || remain != c.remain {
			t.Errorf("parseAgentTarget(%q) = (%q,%q,%q), want (%q,%q,%q)",
				c.in, agent, session, remain, c.agent, c.session, c.remain)
		}
	}
}

func TestResolveSessionID(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	// 命令显式指定（非 "new"）最高优先
	if got := d.resolveSessionID("client-sid", "cmd-sid"); got != "cmd-sid" {
		t.Errorf("command priority = %q", got)
	}
	// 命令传 "new" 哨兵 → 交还下层逻辑（客户端值优先）
	if got := d.resolveSessionID("client-sid", "new"); got != "client-sid" {
		t.Errorf("client fallback = %q", got)
	}
	// 双空 → 无既有会话时新建
	got := d.resolveSessionID("", "")
	if got == "" {
		t.Error("bare path should generate a session id")
	}
}

func TestResolveLatestSessionForAgent(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	// 空参 / 不存在 agent → 空串
	if got := d.resolveLatestSessionForAgent(""); got != "" {
		t.Errorf("empty agent = %q", got)
	}
	if got := d.resolveLatestSessionForAgent("ghost"); got != "" {
		t.Errorf("ghost agent = %q", got)
	}

	s1 := mustCreateSession(t, d.app.SessDB(), "tester")
	s2 := mustCreateSession(t, d.app.SessDB(), "tester")
	mustCreateSession(t, d.app.SessDB(), "other-agent")

	got := d.resolveLatestSessionForAgent("tester")
	if got != s1 && got != s2 {
		t.Errorf("latest = %q, want one of %s/%s", got, s1, s2)
	}
	// LastActivityAt 落后于创建顺序时取最近者；两个会话创建间隔极短，
	// 只断言「属于该 agent」而不固定具体哪一个（时间戳相同则取遍历序）
}

func TestGetSessionMeta(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	sid := mustCreateSession(t, d.app.SessDB(), "tester")
	meta, err := d.getSessionMeta(sid)
	if err != nil || meta == nil || meta.SessionID != sid {
		t.Errorf("meta = %+v err = %v", meta, err)
	}
	if _, err := d.getSessionMeta("ghost"); err == nil {
		t.Error("missing session: expected error")
	}
}

func TestEnsureScheduleSession(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	// 已绑定且非哨兵 → 原样返回
	if got, err := d.ensureScheduleSession("tester", "bound-sid", "", "", ""); err != nil || got != "bound-sid" {
		t.Errorf("existing binding = %q, %v", got, err)
	}
	// 空绑定 → 新建（带项目目录）
	got, err := d.ensureScheduleSession("tester", "", t.TempDir(), "", "")
	if err != nil || got == "" {
		t.Fatalf("create = %q, %v", got, err)
	}
	meta, merr := d.getSessionMeta(got)
	if merr != nil || meta.ProjectDir == "" {
		t.Errorf("created meta project dir = %+v err = %v", meta, merr)
	}
	// "new" 哨兵 → 同样新建（project_dir 必填）
	got2, err := d.ensureScheduleSession("tester", "new", t.TempDir(), "", "")
	if err != nil || got2 == "" || got2 == got {
		t.Errorf("sentinel create = %q, %v", got2, err)
	}
}

func TestCancelSubAgents_NoRuntimes(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	// 空会话 / 无运行时缓存 → 0，不 panic
	if n := d.cancelSubAgents(""); n != 0 {
		t.Errorf("empty session = %d", n)
	}
	if n := d.cancelSubAgents("no-runtime-sid"); n != 0 {
		t.Errorf("no runtime = %d", n)
	}
}

func TestDaemonAccessors(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	if d.App() != d.app || d.Gateway() != d.gw || d.KVStore() != d.kvStore {
		t.Error("accessor mismatch")
	}
	_ = d.Scheduler()
	_ = d.SchedulerDB()
	if d.Addr() == "" || d.WSPath() == "" {
		t.Error("addr/ws path should not be empty")
	}
}
