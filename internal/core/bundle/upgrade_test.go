package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestLocalVersion 血统读取：三类包各从自己的定义文件取版本。
// 没有记录版本时必须返回 found=false 而不是报错或编一个——「来源不明」是
// 真实状态，升级闸要能把它和「版本相同」区分开。
func TestLocalVersion(t *testing.T) {
	root := t.TempDir()

	writeDoc := func(rel, docName, content string) string {
		dir := filepath.Join(root, rel)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("建目录失败: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, docName), []byte(content), 0644); err != nil {
			t.Fatalf("写文件失败: %v", err)
		}
		return dir
	}

	agentDir := writeDoc("agent", identityFileName, "---\nname: alpha\nversion: 1.2.3\n---\n\n正文\n")
	if v, ok, err := LocalVersion(KindAgent, agentDir); err != nil || !ok || v != "1.2.3" {
		t.Fatalf("Agent 版本读取错误：v=%q ok=%v err=%v", v, ok, err)
	}

	teamDir := writeDoc("team", teamDocFileName, "---\nname: squad\nversion: \"2.0.0\"\n---\n\n团队人设\n")
	if v, ok, err := LocalVersion(KindTeam, teamDir); err != nil || !ok || v != "2.0.0" {
		t.Fatalf("Team 版本读取错误（引号应被剥掉）：v=%q ok=%v err=%v", v, ok, err)
	}

	// 技能版本写在 metadata.version（build-market 取的正是这个位置）
	skillDir := writeDoc("skill", skillDocName, "---\nname: deploy\ndescription: 部署\nmetadata:\n  version: 0.9.1\n  name_zh: 部署\n---\n\n指令\n")
	if v, ok, err := LocalVersion(KindSkill, skillDir); err != nil || !ok || v != "0.9.1" {
		t.Fatalf("Skill 版本读取错误：v=%q ok=%v err=%v", v, ok, err)
	}

	// 无版本记录：found=false，不报错
	noVer := filepath.Join(root, "nover")
	if err := os.MkdirAll(noVer, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(noVer, identityFileName), []byte("---\nname: legacy\n---\n\n正文\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if v, ok, err := LocalVersion(KindAgent, noVer); err != nil || ok || v != "" {
		t.Fatalf("无版本记录应返回 ok=false：v=%q ok=%v err=%v", v, ok, err)
	}

	// 目录里没有定义文件：同样视为未记录，不是错误
	emptyDir := filepath.Join(root, "empty")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := LocalVersion(KindAgent, emptyDir); err != nil || ok {
		t.Fatalf("无定义文件应返回 ok=false：ok=%v err=%v", ok, err)
	}
}

// TestContentMatchesAgent 内容闸：与「已装那版」逐字节一致 ⇒ 没改过；
// 改一个字节 ⇒ 改过。这是升级时决定「直接覆盖」还是「问用户」的唯一依据。
func TestContentMatchesAgent(t *testing.T) {
	root, agents, skills := newTestStores(t)
	agent := mustSaveAgent(t, agents, "alpha", "原始行为规则", nil)
	agent.Meta.Version = "1.0.0"
	if err := agents.Save(agent); err != nil {
		t.Fatalf("保存 agent 失败: %v", err)
	}
	agent = agents.Get("alpha")

	pkg := filepath.Join(root, "alpha.mindpkg")
	if _, _, err := ExportAgent(agent, skills, pkg); err != nil {
		t.Fatalf("导出 Agent 包失败: %v", err)
	}

	opts := InstallOptions{GlobalDir: filepath.Join(root, "skills"), AgentsDir: filepath.Join(root, "agents")}
	same, err := ContentMatches(pkg, opts)
	if err != nil {
		t.Fatalf("内容比对失败: %v", err)
	}
	if !same {
		t.Fatal("刚导出的包与原目录应当逐字节一致")
	}

	// 改本地 SOUL.md：判定为已修改
	soulPath := filepath.Join(agent.Dir, soulFileName)
	if err := os.WriteFile(soulPath, []byte("我改过的行为规则"), 0644); err != nil {
		t.Fatalf("修改 SOUL.md 失败: %v", err)
	}
	same, err = ContentMatches(pkg, opts)
	if err != nil {
		t.Fatalf("内容比对失败: %v", err)
	}
	if same {
		t.Fatal("改过本地文件后应判定为不一致")
	}
}

// TestContentMatchesSkill 技能内容闸：全局库里的 SKILL.md 被改过即判为不一致。
func TestContentMatchesSkill(t *testing.T) {
	root, _, skills := newTestStores(t)
	mustWriteSkill(t, filepath.Join(root, "skills"), "deploy-check", "部署检查", map[string]string{
		"references/runbook.md": "手册",
	})
	if err := skills.ReloadGlobal(); err != nil {
		t.Fatalf("重载全局库失败: %v", err)
	}
	pkg := filepath.Join(root, "deploy-check.mindpkg")
	if _, err := ExportSkill(skills, "deploy-check", pkg); err != nil {
		t.Fatalf("导出技能包失败: %v", err)
	}

	globalDir := filepath.Join(root, "skills")
	if same, err := ContentMatches(pkg, InstallOptions{GlobalDir: globalDir}); err != nil || !same {
		t.Fatalf("未改动时应一致：same=%v err=%v", same, err)
	}
	if err := os.WriteFile(filepath.Join(globalDir, "deploy-check", "references", "runbook.md"),
		[]byte("改过的手册"), 0644); err != nil {
		t.Fatalf("修改资源失败: %v", err)
	}
	if same, _ := ContentMatches(pkg, InstallOptions{GlobalDir: globalDir}); same {
		t.Fatal("技能资源被改过后应判定为不一致")
	}
}

// TestDownloadVersionHitsVersionedEndpoint 按版本下载：必须打到
// /packages/{kind}/{name}/versions/{version}/download，且落盘内容与该版 sha256 一致。
// 缓存按版本分文件——latest 与本地那版常不是同一个包，共用文件名会每次校验失败重下。
func TestDownloadVersionHitsVersionedEndpoint(t *testing.T) {
	payload := []byte("versioned-package-bytes")
	sum := sha256.Sum256(payload)
	sha := hex.EncodeToString(sum[:])

	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotPath = req.URL.Path
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	client := NewMarketClient(srv.URL, t.TempDir())
	pkg := MarketPackage{
		Kind: KindAgent,
		Name: "alpha",
		Versions: []MarketVersion{
			{Version: "2.0.0", Sha256: sha, Size: int64(len(payload))},
			{Version: "1.0.0", Sha256: sha, Size: int64(len(payload))},
		},
	}
	path, err := client.DownloadVersion(pkg, "1.0.0")
	if err != nil {
		t.Fatalf("按版本下载失败: %v", err)
	}
	if gotPath != "/api/v1/packages/agent/alpha/versions/1.0.0/download" {
		t.Fatalf("下载路径不对：%s", gotPath)
	}
	if filepath.Base(path) != "alpha@1.0.0.mindpkg" {
		t.Fatalf("缓存文件应按版本分名，实际 %s", filepath.Base(path))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取缓存文件失败: %v", err)
	}
	if string(data) != string(payload) {
		t.Fatal("下载内容与期望不符")
	}

	// 已下架/不存在的版本：明确拒绝，不静默回退到 latest
	if _, err := client.DownloadVersion(pkg, "0.9.0"); err == nil {
		t.Fatal("不存在的版本应当报错")
	}
}
