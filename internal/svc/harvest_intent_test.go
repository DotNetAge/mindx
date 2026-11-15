package svc

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DotNetAge/goharness/tools"
)

func harvestTargets(ids ...string) []tools.HarvestTarget {
	out := make([]tools.HarvestTarget, 0, len(ids))
	for _, id := range ids {
		out = append(out, tools.HarvestTarget{TaskID: id, SessionID: "sub-" + id, AgentName: "tester", Title: "任务 " + id})
	}
	return out
}

func TestSessionDirOfAndIntentPath(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	// 空 ID / 不存在会话 → 空串
	if got := d.sessionDirOf(""); got != "" {
		t.Errorf("empty id dir = %q", got)
	}
	if got := d.sessionDirOf("ghost"); got != "" {
		t.Errorf("ghost dir = %q", got)
	}

	sid := mustCreateSession(t, d.app.SessDB(), "tester")
	dir := d.sessionDirOf(sid)
	if dir == "" {
		t.Fatal("valid session dir is empty")
	}
	path := d.harvestIntentPath(sid)
	if filepath.Base(path) != "session-harvest.json" || filepath.Dir(path) != dir {
		t.Errorf("intent path = %q, want <dir>/session-harvest.json", path)
	}
	// 不存在会话 → 空路径
	if got := d.harvestIntentPath("ghost"); got != "" {
		t.Errorf("ghost intent path = %q", got)
	}
}

func TestRecord_NewIntentAndInCallDedup(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	sid := mustCreateSession(t, d.app.SessDB(), "tester")

	// 单次调用内重复 TaskID 只登记一次
	if err := d.Record(sid, harvestTargets("t1", "t2", "t1")); err != nil {
		t.Fatalf("Record error = %v", err)
	}
	intent := d.loadHarvestIntent(d.harvestIntentPath(sid))
	if intent == nil {
		t.Fatal("intent file missing after Record")
	}
	if intent.MainSessionID != sid || intent.Status != harvestStatusPending {
		t.Errorf("intent = %+v", intent)
	}
	if len(intent.SubSessions) != 2 {
		t.Fatalf("subs = %d, want 2", len(intent.SubSessions))
	}
	if intent.SubSessions[0].TaskID != "t1" || intent.SubSessions[0].SessionID != "sub-t1" {
		t.Errorf("sub = %+v", intent.SubSessions[0])
	}
	if intent.SubSessions[0].RegisteredAt.IsZero() {
		t.Error("RegisteredAt should be set")
	}
}

func TestRecord_MergeAndAlreadyActive(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	sid := mustCreateSession(t, d.app.SessDB(), "tester")
	path := d.harvestIntentPath(sid)

	// 首次登记 t1
	if err := d.Record(sid, harvestTargets("t1")); err != nil {
		t.Fatalf("first record: %v", err)
	}
	first := d.loadHarvestIntent(path)

	// 合并新目标 t2（收割在飞时新派发不得丢失）
	if err := d.Record(sid, harvestTargets("t2")); err != nil {
		t.Fatalf("merge record: %v", err)
	}
	merged := d.loadHarvestIntent(path)
	if len(merged.SubSessions) != 2 {
		t.Fatalf("merged subs = %d, want 2", len(merged.SubSessions))
	}
	// t1 的登记时间保持不变（各自起算收割窗口）
	if !merged.SubSessions[0].RegisteredAt.Equal(first.SubSessions[0].RegisteredAt) {
		t.Error("existing target RegisteredAt should be preserved")
	}
	if !merged.UpdatedAt.After(first.UpdatedAt) {
		t.Error("UpdatedAt should advance on merge")
	}

	// 全部目标均已登记 → ErrHarvestAlreadyActive
	if err := d.Record(sid, harvestTargets("t1", "t2")); !errors.Is(err, tools.ErrHarvestAlreadyActive) {
		t.Errorf("duplicate record err = %v, want ErrHarvestAlreadyActive", err)
	}
}

func TestRecord_ErrorBranches(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	// 缺主会话 ID
	if err := d.Record("", harvestTargets("t1")); err == nil {
		t.Error("empty session id: expected error")
	}
	// 会话目录无法定位
	if err := d.Record("ghost", harvestTargets("t1")); err == nil || !strings.Contains(err.Error(), "无法定位主会话目录") {
		t.Errorf("ghost session err = %v", err)
	}
	// 空目标列表首次登记：合法（登记一个空意图）
	sid := mustCreateSession(t, d.app.SessDB(), "tester")
	if err := d.Record(sid, nil); err != nil {
		t.Fatalf("record with empty targets: %v", err)
	}
	// 空意图下重复登记任何目标都允许（known 集为空）
	if err := d.Record(sid, harvestTargets("t1")); err != nil {
		t.Errorf("record after empty intent: %v", err)
	}
}

func TestLoadHarvestIntent(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	// 不存在 / 损坏 → nil
	if got := d.loadHarvestIntent(filepath.Join(t.TempDir(), "nope.json")); got != nil {
		t.Errorf("missing file = %+v, want nil", got)
	}
	corrupt := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(corrupt, []byte("{broken"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := d.loadHarvestIntent(corrupt); got != nil {
		t.Errorf("corrupt file = %+v, want nil", got)
	}

	// 合法文件 → 解析成功
	valid := filepath.Join(t.TempDir(), "ok.json")
	if err := os.WriteFile(valid, []byte(`{"main_session_id":"s1","status":"pending"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if got := d.loadHarvestIntent(valid); got == nil || got.MainSessionID != "s1" {
		t.Errorf("valid file = %+v", got)
	}
}

func TestWriteAndDeleteHarvestIntent(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	path := filepath.Join(t.TempDir(), "nested", "session-harvest.json")
	intent := &harvestIntentFile{
		MainSessionID: "s1",
		Status:        harvestStatusReporting,
		SubSessions: []harvestIntentSub{
			{TaskID: "t1", RegisteredAt: time.Now()},
		},
		ReportingBatch: []string{"t1"},
	}
	if err := d.writeHarvestIntent(path, intent); err != nil {
		t.Fatalf("write error = %v", err)
	}
	// 原子写：不留 .tmp 残留
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("tmp file should be renamed away")
	}
	// 状态字段 round-trip
	got := d.loadHarvestIntent(path)
	if got == nil || got.Status != harvestStatusReporting || len(got.ReportingBatch) != 1 {
		t.Errorf("round-trip = %+v", got)
	}

	// 删除：存在删除 + 不存在幂等成功
	if err := d.deleteHarvestIntent(path); err != nil {
		t.Fatalf("delete error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file should be removed")
	}
	if err := d.deleteHarvestIntent(path); err != nil {
		t.Errorf("delete missing should be idempotent, got %v", err)
	}
}
