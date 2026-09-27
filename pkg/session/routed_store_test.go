package session

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	goharnesssession "github.com/DotNetAge/goharness/session"
)

// mustCreateRoutedSession 经路由器创建一个归属 projectDir 的会话并追加一条用户消息。
func mustCreateRoutedSession(t *testing.T, r *RoutedSessionStore, projectDir, agentName string) string {
	t.Helper()
	info, err := r.Create(context.Background(), agentName, goharnesssession.WithProjectDirOption(projectDir))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := r.Append(context.Background(), info.SessionID, agentName, "", goharnesssession.Message{
		Role:      "user",
		Content:   "hello",
		Timestamp: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("append message: %v", err)
	}
	return info.SessionID
}

func TestRoutedCreateFallsIntoProjectDir(t *testing.T) {
	registryFile := filepath.Join(t.TempDir(), "data", "session_dirs.json")
	r, err := NewRoutedSessionStore(registryFile)
	if err != nil {
		t.Fatalf("NewRoutedSessionStore: %v", err)
	}
	projectDir := t.TempDir()

	sid := mustCreateRoutedSession(t, r, projectDir, "agent-a")

	// 会话实体必须落在 <project_dir>/.sessions/<agent>/<sessionID>/
	sessionDir := filepath.Join(projectDir, ".sessions", "agent-a", sid)
	if _, statErr := os.Stat(filepath.Join(sessionDir, "meta.json")); statErr != nil {
		t.Fatalf("会话未落到工作目录分片 %s: %v", sessionDir, statErr)
	}
	// project_dir 随 meta 持久化
	meta, err := LoadSessionMeta(sessionDir)
	if err != nil {
		t.Fatalf("load meta: %v", err)
	}
	if filepath.Clean(meta.ProjectDir) != filepath.Clean(projectDir) {
		t.Fatalf("project_dir = %q, want %q", meta.ProjectDir, projectDir)
	}
}

func TestRoutedCreateRequiresProjectDir(t *testing.T) {
	r, err := NewRoutedSessionStore(filepath.Join(t.TempDir(), "dirs.json"))
	if err != nil {
		t.Fatalf("NewRoutedSessionStore: %v", err)
	}
	if _, err := r.Create(context.Background(), "agent-a"); err == nil {
		t.Fatal("无 project_dir 的 Create 应报错，实际成功")
	}
}

func TestRoutedLocateAcrossProjectDirs(t *testing.T) {
	registryFile := filepath.Join(t.TempDir(), "data", "session_dirs.json")
	r, err := NewRoutedSessionStore(registryFile)
	if err != nil {
		t.Fatalf("NewRoutedSessionStore: %v", err)
	}
	dirA, dirB := t.TempDir(), t.TempDir()
	sidA := mustCreateRoutedSession(t, r, dirA, "agent-a")
	sidB := mustCreateRoutedSession(t, r, dirB, "agent-b")

	// 跨目录按 sessionID 定位：读、改标题、删消息均落对分片
	if err := r.RenameSession(sidB, "新标题"); err != nil {
		t.Fatalf("rename session in dirB: %v", err)
	}
	metaA, err := r.GetSessionMeta(sidA)
	if err != nil {
		t.Fatalf("get meta A: %v", err)
	}
	if metaA.Title == "新标题" {
		t.Fatal("dirA 会话标题不应被 sidB 的改名波及")
	}
	metaB, err := r.GetSessionMeta(sidB)
	if err != nil {
		t.Fatalf("get meta B: %v", err)
	}
	if metaB.Title != "新标题" {
		t.Fatalf("dirB 会话标题 = %q, want 新标题", metaB.Title)
	}

	msgs, err := r.Get(context.Background(), sidA)
	if err != nil || len(msgs) != 1 {
		t.Fatalf("get messages A: msgs=%d err=%v", len(msgs), err)
	}

	// 未命中 sessionID 语义与旧版一致
	if _, err := r.GetSessionMeta("sess-not-exist"); err == nil {
		t.Fatal("未命中 GetSessionMeta 应返回错误")
	}
	if _, err := r.Get(context.Background(), "sess-not-exist"); err != nil {
		t.Fatalf("未命中 Get 应返回空列表而非错误: %v", err)
	}
}

func TestRoutedListSessionsUnionAndVanishedDir(t *testing.T) {
	registryFile := filepath.Join(t.TempDir(), "data", "session_dirs.json")
	r, err := NewRoutedSessionStore(registryFile)
	if err != nil {
		t.Fatalf("NewRoutedSessionStore: %v", err)
	}
	dirA, dirB := t.TempDir(), t.TempDir()
	mustCreateRoutedSession(t, r, dirA, "agent-a")
	time.Sleep(10 * time.Millisecond) // 保证 LastActivityAt 可比较
	mustCreateRoutedSession(t, r, dirB, "agent-b")

	sessions, err := r.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("len(sessions) = %d, want 2（跨目录并集）", len(sessions))
	}
	// 按 LastActivityAt 降序：后创建的 dirB 会话在前
	if sessions[0].AgentName != "agent-b" {
		t.Fatalf("sessions[0].AgentName = %q, want agent-b（降序）", sessions[0].AgentName)
	}

	// 工作目录消失 → 其分片不可见（TODO 设计：目录消失会话即消失）
	if err := os.RemoveAll(dirB); err != nil {
		t.Fatalf("remove dirB: %v", err)
	}
	sessions, err = r.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("list after vanish: %v", err)
	}
	if len(sessions) != 1 || sessions[0].AgentName != "agent-a" {
		t.Fatalf("目录消失后 len=%d，应只剩 agent-a 的会话", len(sessions))
	}
}

func TestRoutedRegistryDedupAndReload(t *testing.T) {
	registryFile := filepath.Join(t.TempDir(), "data", "session_dirs.json")
	r, err := NewRoutedSessionStore(registryFile)
	if err != nil {
		t.Fatalf("NewRoutedSessionStore: %v", err)
	}
	projectDir := t.TempDir()
	mustCreateRoutedSession(t, r, projectDir, "agent-a")
	sid2 := mustCreateRoutedSession(t, r, projectDir, "agent-a") // 同目录第二会话

	// 同一目录多次 Create，清单去重后仅一条
	data, readErr := os.ReadFile(registryFile)
	if readErr != nil {
		t.Fatalf("read registry: %v", readErr)
	}
	var dirs []string
	if err := json.Unmarshal(data, &dirs); err != nil {
		t.Fatalf("unmarshal registry: %v", err)
	}
	if len(dirs) != 1 {
		t.Fatalf("清单条目数 = %d, want 1（去重）: %v", len(dirs), dirs)
	}

	// 新路由器实例（重启语义）从清单恢复后仍可定位既有会话
	r2, err := NewRoutedSessionStore(registryFile)
	if err != nil {
		t.Fatalf("NewRoutedSessionStore reload: %v", err)
	}
	meta, err := r2.GetSessionMeta(sid2)
	if err != nil {
		t.Fatalf("reload 后定位既有会话失败: %v", err)
	}
	if meta.SessionID != sid2 {
		t.Fatalf("sessionID = %q, want %q", meta.SessionID, sid2)
	}
}

func TestMigrateLegacySessions(t *testing.T) {
	legacyDir := t.TempDir()
	dirA, dirB := t.TempDir(), t.TempDir()

	// 旧布局：两个可归属会话 + 一个无 project_dir 的留守会话
	legacyA := filepath.Join(legacyDir, "agent-a", "sess-old-a")
	legacyB := filepath.Join(legacyDir, "agent-b", "sess-old-b")
	legacyOrphan := filepath.Join(legacyDir, "agent-c", "sess-orphan")
	for dir, pd := range map[string]string{
		legacyA: dirA, legacyB: dirB, legacyOrphan: "",
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("mkdir legacy: %v", err)
		}
		pdValue := pd
		if pdValue == "" {
			pdValue = filepath.Join(dirA, "unused") // 占位，删除后模拟空 project_dir
		}
		info := &goharnesssession.SessionInfo{SessionID: filepath.Base(dir), ProjectDir: pdValue}
		if dir == legacyOrphan {
			info.ProjectDir = "" // 无归属：留守
		}
		if err := SaveSessionMeta(dir, info); err != nil {
			t.Fatalf("save meta: %v", err)
		}
	}

	var registered []string
	moved, kept, err := MigrateLegacySessions(legacyDir, func(pd string) error {
		registered = append(registered, pd)
		return nil
	})
	if err != nil {
		t.Fatalf("MigrateLegacySessions: %v", err)
	}
	if moved != 2 || kept != 1 {
		t.Fatalf("moved=%d kept=%d, want 2/1", moved, kept)
	}
	if len(registered) != 2 {
		t.Fatalf("登记次数 = %d, want 2", len(registered))
	}

	// 可归属会话落新位置且原目录消失
	for _, tc := range []struct{ old, agent, id string }{
		{legacyA, "agent-a", "sess-old-a"},
		{legacyB, "agent-b", "sess-old-b"},
	} {
		pd := filepath.Clean(map[string]string{"agent-a": dirA, "agent-b": dirB}[tc.agent])
		newDir := filepath.Join(pd, ".sessions", tc.agent, tc.id)
		if _, statErr := os.Stat(filepath.Join(newDir, "meta.json")); statErr != nil {
			t.Fatalf("会话未搬迁到 %s: %v", newDir, statErr)
		}
		if _, statErr := os.Stat(tc.old); !os.IsNotExist(statErr) {
			t.Fatalf("旧目录应已消失 %s", tc.old)
		}
	}
	// 留守会话仍在原地
	if _, statErr := os.Stat(filepath.Join(legacyOrphan, "meta.json")); statErr != nil {
		t.Fatal("无归属会话应留守原地")
	}
	// 存在留守时旧目录不得更名备份
	if _, statErr := os.Stat(legacyDir); statErr != nil {
		t.Fatal("存在留守会话时旧目录应保留原位")
	}
}

func TestMigrateLegacyRenamesDirWhenFullyMoved(t *testing.T) {
	legacyDir := t.TempDir()
	dirA := t.TempDir()
	legacyA := filepath.Join(legacyDir, "agent-a", "sess-old-a")
	if err := os.MkdirAll(legacyA, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := SaveSessionMeta(legacyA, &goharnesssession.SessionInfo{SessionID: "sess-old-a", ProjectDir: dirA}); err != nil {
		t.Fatalf("save meta: %v", err)
	}

	if _, kept, err := MigrateLegacySessions(legacyDir, func(string) error { return nil }); err != nil || kept != 0 {
		t.Fatalf("kept=%d err=%v, want 0/nil", kept, err)
	}
	if _, statErr := os.Stat(legacyDir + ".migrated.bak"); statErr != nil {
		t.Fatal("全部搬迁后旧目录应更名为 .migrated.bak")
	}
}

func TestMigrateLegacySessionsNoop(t *testing.T) {
	moved, kept, err := MigrateLegacySessions(filepath.Join(t.TempDir(), "not-exist"), func(string) error { return nil })
	if err != nil || moved != 0 || kept != 0 {
		t.Fatalf("不存在的源目录应 no-op: moved=%d kept=%d err=%v", moved, kept, err)
	}
}
