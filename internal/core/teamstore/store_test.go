package teamstore

import (
	"os"
	"path/filepath"
	"testing"
)

func mustLoad(t *testing.T, dir string) *Store {
	t.Helper()
	store, _, err := Load(dir)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	return store
}

// round-trip：Save 写盘后 Load 读回，frontmatter 强类型与 Profile 正文不丢。
func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := mustLoad(t, dir)

	team := &Team{
		Meta: TeamMeta{
			Name:        "alpha",
			NickName:    "阿尔法",
			Description: "产品研发突击队",
			Leader:      "architect",
			Members:     []string{"architect", "flutter-dev", "backend-dev"},
			Icon:        "rocket",
			Category:    "产品研发",
			Hired:       true,
		},
		Profile: "## 团队定位\n\n我们是一支端到端交付团队。",
	}
	if err := store.Save(team); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}

	// 目录名小写
	if _, err := os.Stat(filepath.Join(dir, "alpha", "TEAM.md")); err != nil {
		t.Fatalf("TEAM.md 未落盘: %v", err)
	}

	reloaded := mustLoad(t, dir)
	got := reloaded.Get("alpha")
	if got == nil {
		t.Fatal("重载后未找到团队 alpha")
	}
	if got.Meta.NickName != "阿尔法" || got.Meta.Leader != "architect" || !got.Meta.Hired {
		t.Fatalf("frontmatter round-trip 不一致: %+v", got.Meta)
	}
	if len(got.Meta.Members) != 3 || got.Meta.Members[1] != "flutter-dev" {
		t.Fatalf("members round-trip 不一致: %v", got.Meta.Members)
	}
	if got.Profile == "" || !filepath.IsAbs(got.Dir) {
		t.Fatalf("Profile/Dir 异常: profile=%q dir=%q", got.Profile, got.Dir)
	}
}

// Load 容错：单个坏目录不拖垮整体，错误记入报告。
func TestLoadToleratesBrokenDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "good"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "good", "TEAM.md"), []byte("---\nname: good\nleader: a\n---\n正文"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "broken"), 0755); err != nil {
		t.Fatal(err)
	}
	// 缺少 frontmatter 的坏文件
	if err := os.WriteFile(filepath.Join(dir, "broken", "TEAM.md"), []byte("没有 frontmatter"), 0644); err != nil {
		t.Fatal(err)
	}

	store, loadErrs, err := Load(dir)
	if err == nil {
		t.Fatal("存在加载失败时应返回聚合错误（与 agentstore 语义一致）")
	}
	if store == nil {
		t.Fatal("聚合错误时 store 仍应可用")
	}
	if len(loadErrs) != 1 {
		t.Fatalf("期望 1 条加载错误, 实得 %v", loadErrs)
	}
	if store.Get("good") == nil {
		t.Fatal("正常团队未被加载")
	}
	if store.Get("broken") != nil {
		t.Fatal("坏目录不应进入注册表")
	}
}

// 目录不存在时返回空注册表（首次启动语义）。
func TestLoadMissingDirReturnsEmpty(t *testing.T) {
	store := mustLoad(t, filepath.Join(t.TempDir(), "nonexistent"))
	if len(store.List()) != 0 {
		t.Fatal("期望空注册表")
	}
}

// dir 为空必须报错（必传参数非空检查）。
func TestLoadRejectsEmptyDir(t *testing.T) {
	if _, _, err := Load(""); err == nil {
		t.Fatal("空 dir 应返回错误")
	}
}

// SetHired 翻转雇佣标记并持久化。
func TestSetHired(t *testing.T) {
	dir := t.TempDir()
	store := mustLoad(t, dir)
	if err := store.Save(&Team{Meta: TeamMeta{Name: "alpha", Leader: "a"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetHired("alpha", true); err != nil {
		t.Fatalf("SetHired 失败: %v", err)
	}
	if !store.Get("alpha").Meta.Hired {
		t.Fatal("内存标记未翻转")
	}
	if !mustLoad(t, dir).Get("alpha").Meta.Hired {
		t.Fatal("持久化标记未翻转")
	}
	if err := store.SetHired("ghost", true); err == nil {
		t.Fatal("未知团队应报错")
	}
}

// Remove 删除磁盘目录与内存注册，且不触碰 Agent（引用方自行容错）。
func TestRemove(t *testing.T) {
	dir := t.TempDir()
	store := mustLoad(t, dir)
	if err := store.Save(&Team{Meta: TeamMeta{Name: "alpha", Leader: "a"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Remove("alpha"); err != nil {
		t.Fatalf("Remove 失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "alpha")); !os.IsNotExist(err) {
		t.Fatal("磁盘目录未删除")
	}
	if store.Get("alpha") != nil {
		t.Fatal("内存注册未移除")
	}
	if err := store.Remove("alpha"); err == nil {
		t.Fatal("重复删除应报错")
	}
}

// FindByLeader / TeamOf 反查语义。
func TestFindByLeaderAndTeamOf(t *testing.T) {
	dir := t.TempDir()
	store := mustLoad(t, dir)
	if err := store.Save(&Team{Meta: TeamMeta{Name: "alpha", Leader: "architect", Members: []string{"dev"}}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&Team{Meta: TeamMeta{Name: "beta", Leader: "pm", Members: []string{"architect"}}}); err != nil {
		t.Fatal(err)
	}

	if got := store.FindByLeader("architect"); got == nil || got.Meta.Name != "alpha" {
		t.Fatalf("FindByLeader = %v, 期望 alpha", got)
	}
	if got := store.FindByLeader("dev"); got != nil {
		t.Fatalf("成员不是负责人, 期望 nil, 实得 %v", got)
	}
	teams := store.TeamOf("architect")
	if len(teams) != 2 || teams[0].Meta.Name != "alpha" || teams[1].Meta.Name != "beta" {
		t.Fatalf("TeamOf = %v, 期望 [alpha beta]", teams)
	}
}
