package svc

import (
	"context"
	"encoding/json"
	"testing"

	goharnesssession "github.com/DotNetAge/goharness/session"
	mindxses "github.com/DotNetAge/mindx/pkg/session"
)

// mustCreateChildSession 建一条带父会话归属的派生会话（仅落 meta，不发消息）。
func mustCreateChildSession(t *testing.T, sessDB *mindxses.RoutedSessionStore, projectDir, agentName, sponsor, parentID string) string {
	t.Helper()
	info, err := goharnesssession.CreateSession(context.Background(), sessDB, agentName,
		goharnesssession.WithProjectDirOption(projectDir),
		goharnesssession.WithSponsorOption(sponsor),
		goharnesssession.WithParentSessionOption(parentID),
	)
	if err != nil {
		t.Fatalf("create child session: %v", err)
	}
	return info.SessionID
}

// TestHandleSessionDelete_Cascade 删除父会话必须级联删除其派生子树：
// 父会话没了，子会话在会话树里没有入口，留着只会变成「删了还在」。
// 用例覆盖两层深度（子会话自己再派工）与「不误删别的父会话的子会话」。
func TestHandleSessionDelete_Cascade(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()
	sessDB := d.app.SessDB()
	dir := t.TempDir()

	parent := mustCreateProjectSession(t, sessDB, dir, "assistant", "")
	child := mustCreateChildSession(t, sessDB, dir, "worker", "assistant", parent)
	grandchild := mustCreateChildSession(t, sessDB, dir, "worker-2", "worker", child)
	otherParent := mustCreateProjectSession(t, sessDB, dir, "assistant", "")
	otherChild := mustCreateChildSession(t, sessDB, dir, "worker", "assistant", otherParent)

	params, err := json.Marshal(map[string]string{"session_id": parent})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	res, err := d.handleSessionDelete(context.Background(), params)
	if err != nil {
		t.Fatalf("handleSessionDelete error = %v", err)
	}
	payload, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type %T", res)
	}
	deleted, ok := payload["deleted_session_ids"].([]string)
	if !ok {
		t.Fatalf("deleted_session_ids missing, got %#v", payload["deleted_session_ids"])
	}
	if len(deleted) != 3 {
		t.Fatalf("expected 3 deleted sessions (parent+child+grandchild), got %v", deleted)
	}
	remaining := map[string]bool{}
	sessions, err := goharnesssession.ListSessions(context.Background(), sessDB)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	for _, si := range sessions {
		remaining[si.SessionID] = true
	}
	for _, gone := range []string{parent, child, grandchild} {
		if remaining[gone] {
			t.Errorf("session %q 应已删除，仍出现在列表", gone)
		}
	}
	for _, keep := range []string{otherParent, otherChild} {
		if !remaining[keep] {
			t.Errorf("session %q 不属于该父会话，不应被级联删除", keep)
		}
	}
}

// TestHandleSessionList_ParentSessionID 会话列表必须透出父会话归属：
// 客户端会话树直接吃 session.list，字段丢失等于树永远只有一层。
func TestHandleSessionList_ParentSessionID(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()
	sessDB := d.app.SessDB()
	dir := t.TempDir()

	parent := mustCreateProjectSession(t, sessDB, dir, "assistant", "")
	child := mustCreateChildSession(t, sessDB, dir, "worker", "assistant", parent)

	res, err := d.handleSessionList(context.Background(), nil)
	if err != nil {
		t.Fatalf("handleSessionList error = %v", err)
	}
	sessions, ok := res.([]goharnesssession.SessionInfo)
	if !ok {
		t.Fatalf("unexpected result type %T", res)
	}
	found := false
	for _, si := range sessions {
		if si.SessionID != child {
			continue
		}
		found = true
		if si.ParentSessionID != parent {
			t.Errorf("child parent_session_id = %q, want %q", si.ParentSessionID, parent)
		}
	}
	if !found {
		t.Fatalf("child session %q 未出现在会话列表", child)
	}
}

// TestSetParentSessionID_Rewrite 复用回写：子会话归属可被改写为新的父会话，
// 且不破坏其余元数据（标题、目录）。
func TestSetParentSessionID_Rewrite(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()
	sessDB := d.app.SessDB()
	dir := t.TempDir()

	first := mustCreateProjectSession(t, sessDB, dir, "assistant", "")
	second := mustCreateProjectSession(t, sessDB, dir, "assistant", "")
	child := mustCreateChildSession(t, sessDB, dir, "worker", "assistant", first)

	if err := sessDB.SetParentSessionID(context.Background(), child, second); err != nil {
		t.Fatalf("SetParentSessionID error = %v", err)
	}
	meta, err := sessDB.GetMeta(context.Background(), child)
	if err != nil {
		t.Fatalf("GetMeta error = %v", err)
	}
	if meta.ParentSessionID != second {
		t.Errorf("parent_session_id = %q, want %q", meta.ParentSessionID, second)
	}
	if meta.ProjectDir != dir {
		t.Errorf("project_dir 被破坏: %q", meta.ProjectDir)
	}
	if meta.Sponsor != "assistant" {
		t.Errorf("sponsor 被破坏: %q", meta.Sponsor)
	}
}
