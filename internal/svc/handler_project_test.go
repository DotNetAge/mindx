package svc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DotNetAge/goharness/logging"
	goharnesssession "github.com/DotNetAge/goharness/session"
	mindxses "github.com/DotNetAge/mindx/pkg/session"
)

// mustCreateProjectSession 在指定项目目录下创建带 Sponsor 的会话并追加一条消息
// （Append 会刷新 meta 的 UpdatedAt，作为项目聚合的活跃时间来源）。
func mustCreateProjectSession(t *testing.T, sessDB *mindxses.RoutedSessionStore, projectDir, agentName, sponsor string) string {
	t.Helper()
	opts := []goharnesssession.SessionOption{goharnesssession.WithProjectDirOption(projectDir)}
	if sponsor != "" {
		opts = append(opts, goharnesssession.WithSponsorOption(sponsor))
	}
	info, err := goharnesssession.CreateSession(context.Background(), sessDB, agentName, opts...)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	sess, loadErr := goharnesssession.Load(context.Background(), info.SessionID, agentName, sessDB, logging.DefaultLogger())
	if loadErr != nil {
		t.Fatalf("load session: %v", loadErr)
	}
	msg := goharnesssession.Message{
		Role:      "user",
		Content:   "hello",
		Timestamp: time.Now().UnixMilli(),
	}
	_ = sess.Append(context.Background(), msg)
	return info.SessionID
}

// mustCreateLeaderAgent 落盘一个带 members 的负责人 Agent 并重载注册表。
func mustCreateLeaderAgent(t *testing.T, d *Daemon, name string) {
	t.Helper()
	agentsDir := filepath.Join(d.app.Settings().UserPreferences(), "agents")
	agentDir := filepath.Join(agentsDir, name)
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatalf("create agent dir: %v", err)
	}
	content := fmt.Sprintf(`---
name: %s
role: Lead
description: test leader
members:
  - worker-a
  - worker-b
---
`, name)
	if err := os.WriteFile(filepath.Join(agentDir, "IDENTITY.md"), []byte(content), 0644); err != nil {
		t.Fatalf("write agent file: %v", err)
	}
	if err := d.app.ReloadAgents(); err != nil {
		t.Fatalf("ReloadAgents() error = %v", err)
	}
	if got := d.app.Agents().Get(name); got == nil || !got.IsLeader() {
		t.Fatalf("leader agent %q 未注册或未派生为负责人", name)
	}
}

func TestHandleProjectList_Empty(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	result, err := d.handleProjectList(context.Background(), nil)
	if err != nil {
		t.Fatalf("handleProjectList error = %v", err)
	}
	projects, ok := result.([]projectListResult)
	if !ok {
		t.Fatalf("expected []projectListResult, got %T", result)
	}
	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(projects))
	}
}

// TestHandleProjectList_Dedupe 验证按 project_dir 去重聚合：
// 同一项目多会话合并为一条（count 累加、last_agent/last_activity_at 取最近活跃）。
func TestHandleProjectList_Dedupe(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()
	sessDB := d.app.SessDB()

	projectA := filepath.Join(t.TempDir(), "alpha")
	projectB := filepath.Join(t.TempDir(), "beta")

	mustCreateProjectSession(t, sessDB, projectB, "agent-a", "agent-y")
	time.Sleep(10 * time.Millisecond) // 保证 LastActivityAt 顺序可比较
	mustCreateProjectSession(t, sessDB, projectA, "agent-a", "")
	time.Sleep(10 * time.Millisecond)
	mustCreateProjectSession(t, sessDB, projectA, "agent-b", "agent-x")

	result, err := d.handleProjectList(context.Background(), nil)
	if err != nil {
		t.Fatalf("handleProjectList error = %v", err)
	}
	projects, ok := result.([]projectListResult)
	if !ok {
		t.Fatalf("expected []projectListResult, got %T", result)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d: %+v", len(projects), projects)
	}

	// 降序：projectA（最后活跃）在前
	if projects[0].ProjectDir != projectA {
		t.Fatalf("expected projectA first, got %+v", projects[0])
	}
	a := projects[0]
	if a.Name != "alpha" {
		t.Errorf("name = %q, want alpha", a.Name)
	}
	if a.SessionCount != 2 {
		t.Errorf("session_count = %d, want 2", a.SessionCount)
	}
	if a.LastAgent != "agent-x" {
		t.Errorf("last_agent = %q, want agent-x", a.LastAgent)
	}

	b := projects[1]
	if b.ProjectDir != projectB || b.Name != "beta" || b.SessionCount != 1 {
		t.Errorf("projectB 聚合错误: %+v", b)
	}
	if b.LastAgent != "agent-y" {
		t.Errorf("last_agent = %q, want agent-y", b.LastAgent)
	}
}

// TestHandleProjectList_Leader 验证 leader 派生：Sponsor 命中团队负责人即该项目 leader；
// 未命中（普通 agent 或用户亲自发起）则 leader 为空。
func TestHandleProjectList_Leader(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()
	mustCreateLeaderAgent(t, d, "boss")
	sessDB := d.app.SessDB()

	projectA := filepath.Join(t.TempDir(), "alpha") // 负责人 + 普通成员都出现过
	projectB := filepath.Join(t.TempDir(), "beta")  // 仅普通成员 → leader 空

	mustCreateProjectSession(t, sessDB, projectA, "agent-a", "boss")
	mustCreateProjectSession(t, sessDB, projectA, "agent-b", "agent-x")
	mustCreateProjectSession(t, sessDB, projectB, "agent-a", "agent-x")

	result, err := d.handleProjectList(context.Background(), nil)
	if err != nil {
		t.Fatalf("handleProjectList error = %v", err)
	}
	projects, ok := result.([]projectListResult)
	if !ok {
		t.Fatalf("expected []projectListResult, got %T", result)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}

	byDir := make(map[string]projectListResult, len(projects))
	for _, p := range projects {
		byDir[p.ProjectDir] = p
	}
	if a := byDir[projectA]; a.Leader != "boss" {
		t.Errorf("projectA leader = %q, want boss", a.Leader)
	}
	if b := byDir[projectB]; b.Leader != "" {
		t.Errorf("projectB leader = %q, want 空（未委派负责人）", b.Leader)
	}
}
