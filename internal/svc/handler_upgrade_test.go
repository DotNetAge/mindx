package svc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DotNetAge/mindx/internal/core/agentstore"
	"github.com/DotNetAge/mindx/internal/core/bundle"
	"github.com/DotNetAge/mindx/internal/core/skillstore"
)

// 升级链路的端到端用例（两把闸 + 保留/覆盖二选一）。
//
// 这里必须用可编程的市场服务：本功能的正确性全在「拿哪一份字节去比、去装」，
// 用假数据只能测出参数拼装对不对，测不出「比错了版本」这类真正的缺陷。

// fakeMarket 一个最小市场服务：一个包、两个版本、可下载。
type fakeMarket struct {
	srv      *httptest.Server
	kind     string
	name     string
	versions []bundle.MarketVersion
	files    map[string][]byte // version → package bytes
}

func newFakeMarket(t *testing.T, kind, name string) *fakeMarket {
	t.Helper()
	fm := &fakeMarket{kind: kind, name: name, files: map[string][]byte{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/packages/"+kind+"/"+name, func(w http.ResponseWriter, _ *http.Request) {
		vers := make([]map[string]any, 0, len(fm.versions))
		for _, v := range fm.versions {
			vers = append(vers, map[string]any{
				"version": v.Version, "sha256": v.Sha256, "size": v.Size,
				"deleted": v.Deleted, "published_at": "2026-11-01T00:00:00Z",
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"kind": kind, "name": name, "description": "测试包",
			"versions": vers, "download_url": fm.srv.URL + "/packages/" + kind + "/" + name + ".mindpkg",
		})
	})
	mux.HandleFunc("/api/v1/packages/"+kind+"/"+name+"/versions/", func(w http.ResponseWriter, req *http.Request) {
		// /api/v1/packages/<kind>/<name>/versions/<ver>/download
		parts := strings.Split(strings.TrimPrefix(req.URL.Path, "/api/v1/packages/"+kind+"/"+name+"/versions/"), "/")
		data, ok := fm.files[parts[0]]
		if !ok {
			http.NotFound(w, req)
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(data)
	})
	mux.HandleFunc("/packages/"+kind+"/"+name+".mindpkg", func(w http.ResponseWriter, _ *http.Request) {
		latest := fm.versions[0] // 服务端按发布时间倒序，首条即最新
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(fm.files[latest.Version])
	})
	fm.srv = httptest.NewServer(mux)
	t.Cleanup(fm.srv.Close)
	return fm
}

// publish 登记一个版本（sha256 由包字节算出，与真实服务端口径一致）。
func (fm *fakeMarket) publish(version string, data []byte) {
	sum := sha256.Sum256(data)
	v := bundle.MarketVersion{
		Version: version, Sha256: hex.EncodeToString(sum[:]), Size: int64(len(data)),
	}
	fm.files[version] = data
	// 最新放最前（服务端语义）
	fm.versions = append([]bundle.MarketVersion{v}, fm.versions...)
}

// exportAgentPkg 把某个 agent 目录导成包字节（模拟线上那一版的真实内容）。
func exportAgentPkg(t *testing.T, agent *agentstore.Agent, skills *skillstore.Store) []byte {
	t.Helper()
	out := filepath.Join(t.TempDir(), agent.Name()+".mindpkg")
	if _, _, err := bundle.ExportAgent(agent, skills, out); err != nil {
		t.Fatalf("导出 Agent 包失败: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("读取导出包失败: %v", err)
	}
	return data
}

// TestBundleUpgrade_Flow 三个连续场景，正好覆盖「可升级 → 直接升」与
// 「本地改过 → 需要确认 → 覆盖」两条路径，以及升级后本地版本号确实变了。
func TestBundleUpgrade_Flow(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	// 1) 本机装一个 1.0.0 的 agent（内容 = 线上 1.0.0 的原始内容）
	agents := d.app.Agents()
	agent := &agentstore.Agent{
		Meta: agentstore.AgentMeta{
			Name: "up-alpha", Version: "1.0.0", Role: "测试", Description: "升级用例",
		},
		Soul: "原始行为规则",
	}
	if err := agents.Save(agent); err != nil {
		t.Fatalf("保存 agent 失败: %v", err)
	}
	agent = agents.Get("up-alpha")

	skills := d.app.Skills()
	v1 := exportAgentPkg(t, agent, skills)

	// 2) 线上再发一版 2.0.0（内容不同）
	agent.Meta.Version = "2.0.0"
	agent.Soul = "新版行为规则"
	if err := agents.Save(agent); err != nil {
		t.Fatalf("保存 2.0.0 失败: %v", err)
	}
	v2 := exportAgentPkg(t, agents.Get("up-alpha"), skills)

	// 本机回到 1.0.0 状态（模拟「装了 1.0.0 还没升级」）
	agent.Meta.Version = "1.0.0"
	agent.Soul = "原始行为规则"
	if err := agents.Save(agent); err != nil {
		t.Fatalf("回滚到 1.0.0 失败: %v", err)
	}

	fm := newFakeMarket(t, "agent", "up-alpha")
	fm.publish("1.0.0", v1)
	fm.publish("2.0.0", v2)
	if err := d.app.SetMarketBaseForTest(fm.srv.URL); err != nil {
		t.Fatalf("设置测试市场失败: %v", err)
	}

	// 3) 检查：可升级，且本地未被改动（内容与 1.0.0 逐字节一致）
	raw, err := d.handleBundleUpgradeCheck(context.Background(), mustJSON(t, map[string]string{
		"kind": "agent", "name": "up-alpha",
	}))
	if err != nil {
		t.Fatalf("升级检查失败: %v", err)
	}
	check := raw.(map[string]any)
	items := check["items"].([]UpgradeItem)
	if len(items) != 1 {
		t.Fatalf("应只有 1 条检查结果，实际 %d", len(items))
	}
	it := items[0]
	if !it.Upgradable || it.LocalVersion != "1.0.0" || it.LatestVersion != "2.0.0" {
		t.Fatalf("版本闸判定不对：%+v", it)
	}
	if it.Modified == nil || *it.Modified {
		t.Fatalf("内容与已装版本一致时应判定为未修改：%+v", it)
	}

	// 4) 升级：未改动 → 直接覆盖
	res, err := d.handleBundleUpgrade(context.Background(), mustJSON(t, map[string]any{
		"kind": "agent", "name": "up-alpha",
	}))
	if err != nil {
		t.Fatalf("升级失败: %v", err)
	}
	up := res.(UpgradeResult)
	if !up.Applied || up.Status != "upgraded" {
		t.Fatalf("应升级成功：%+v", up)
	}
	if up.Item.LocalVersion != "2.0.0" {
		t.Fatalf("升级后本地版本应为 2.0.0，实际 %q", up.Item.LocalVersion)
	}
	if _, statErr := os.Stat(up.Backup); statErr != nil {
		t.Fatalf("覆盖前应留备份，实际不可读（%s）：%v", up.Backup, statErr)
	}

	// 5) 再检查：已是最新
	raw, err = d.handleBundleUpgradeCheck(context.Background(), mustJSON(t, map[string]string{
		"kind": "agent", "name": "up-alpha",
	}))
	if err != nil {
		t.Fatalf("二次检查失败: %v", err)
	}
	if again := raw.(map[string]any)["items"].([]UpgradeItem)[0]; again.Upgradable {
		t.Fatalf("升级后不应再报可升级：%+v", again)
	}
}

// TestBundleUpgrade_NeedsConfirm 本地改过的包：默认拒绝覆盖（保留），
// force 才覆盖。这条是「不做三方合并」的具体落点——选择权在用户手里。
func TestBundleUpgrade_NeedsConfirm(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	agents := d.app.Agents()
	agent := &agentstore.Agent{
		Meta: agentstore.AgentMeta{Name: "edit-alpha", Version: "1.0.0", Role: "测试", Description: "改动用例"},
		Soul: "原始规则",
	}
	if err := agents.Save(agent); err != nil {
		t.Fatalf("保存 agent 失败: %v", err)
	}
	skills := d.app.Skills()
	v1 := exportAgentPkg(t, agents.Get("edit-alpha"), skills)

	// 线上 2.0.0
	cur := agents.Get("edit-alpha")
	cur.Meta.Version = "2.0.0"
	cur.Soul = "线上新版"
	if err := agents.Save(cur); err != nil {
		t.Fatalf("保存 2.0.0 失败: %v", err)
	}
	v2 := exportAgentPkg(t, agents.Get("edit-alpha"), skills)

	// 本机：恢复成 1.0.0 的内容，然后用户改了 SOUL.md
	cur.Meta.Version = "1.0.0"
	cur.Soul = "原始规则"
	if err := agents.Save(cur); err != nil {
		t.Fatalf("回滚失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agents.Get("edit-alpha").Dir, "SOUL.md"), []byte("我自己改的规则"), 0644); err != nil {
		t.Fatalf("改写 SOUL.md 失败: %v", err)
	}

	fm := newFakeMarket(t, "agent", "edit-alpha")
	fm.publish("1.0.0", v1)
	fm.publish("2.0.0", v2)
	if err := d.app.SetMarketBaseForTest(fm.srv.URL); err != nil {
		t.Fatalf("设置测试市场失败: %v", err)
	}

	raw, err := d.handleBundleUpgradeCheck(context.Background(), mustJSON(t, map[string]string{
		"kind": "agent", "name": "edit-alpha",
	}))
	if err != nil {
		t.Fatalf("检查失败: %v", err)
	}
	it := raw.(map[string]any)["items"].([]UpgradeItem)[0]
	if it.Modified == nil || !*it.Modified {
		t.Fatalf("本地改过应判定为已修改：%+v", it)
	}

	// 不带 force：不落盘，返回 needs_confirm
	res, err := d.handleBundleUpgrade(context.Background(), mustJSON(t, map[string]any{
		"kind": "agent", "name": "edit-alpha",
	}))
	if err != nil {
		t.Fatalf("升到一半失败: %v", err)
	}
	if got := res.(UpgradeResult); got.Applied || got.Status != "needs_confirm" {
		t.Fatalf("改过的包默认应拒绝覆盖：%+v", got)
	}
	if soul, _ := os.ReadFile(filepath.Join(agents.Get("edit-alpha").Dir, "SOUL.md")); string(soul) != "我自己改的规则" {
		t.Fatalf("needs_confirm 时不应改动本地文件，实际：%q", string(soul))
	}

	// 带 force：覆盖
	res, err = d.handleBundleUpgrade(context.Background(), mustJSON(t, map[string]any{
		"kind": "agent", "name": "edit-alpha", "force": true,
	}))
	if err != nil {
		t.Fatalf("强制升级失败: %v", err)
	}
	if got := res.(UpgradeResult); !got.Applied {
		t.Fatalf("force 应覆盖成功：%+v", got)
	}
	if soul, _ := os.ReadFile(filepath.Join(agents.Get("edit-alpha").Dir, "SOUL.md")); string(soul) != "线上新版" {
		t.Fatalf("force 后应为线上内容，实际：%q", string(soul))
	}
}

// TestBundleUpgrade_LocalOnly 市场里没有这个包（404）：不是错误，标注清楚即可。
func TestBundleUpgrade_LocalOnly(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	agents := d.app.Agents()
	if err := agents.Save(&agentstore.Agent{
		Meta: agentstore.AgentMeta{Name: "solo-alpha", Version: "1.0.0", Role: "测试", Description: "本地独有"},
		Soul: "规则",
	}); err != nil {
		t.Fatalf("保存 agent 失败: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	}))
	defer srv.Close()
	if err := d.app.SetMarketBaseForTest(srv.URL); err != nil {
		t.Fatalf("设置测试市场失败: %v", err)
	}

	raw, err := d.handleBundleUpgradeCheck(context.Background(), mustJSON(t, map[string]string{
		"kind": "agent", "name": "solo-alpha",
	}))
	if err != nil {
		t.Fatalf("404 不应算错误: %v", err)
	}
	it := raw.(map[string]any)["items"].([]UpgradeItem)[0]
	if it.Upgradable || !strings.Contains(it.Note, "市场中无同名包") {
		t.Fatalf("本地独有包应标注「市场中无同名包」且不可升级：%+v", it)
	}
}

// TestUpgradeToolBackend Agent 工具与 RPC 走同一条执行体：
// 工具侧只要能把「检查 → 执行」跑通，就不可能出现「界面说可升级、Agent 说没有」。
func TestUpgradeToolBackend(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	agents := d.app.Agents()
	if err := agents.Save(&agentstore.Agent{
		Meta: agentstore.AgentMeta{Name: "tool-alpha", Version: "1.0.0", Role: "测试", Description: "工具入口用例"},
		Soul: "原始规则",
	}); err != nil {
		t.Fatalf("保存 agent 失败: %v", err)
	}
	skills := d.app.Skills()
	v1 := exportAgentPkg(t, agents.Get("tool-alpha"), skills)

	cur := agents.Get("tool-alpha")
	cur.Meta.Version = "2.0.0"
	cur.Soul = "线上新版"
	if err := agents.Save(cur); err != nil {
		t.Fatalf("保存 2.0.0 失败: %v", err)
	}
	v2 := exportAgentPkg(t, agents.Get("tool-alpha"), skills)

	cur.Meta.Version = "1.0.0"
	cur.Soul = "原始规则"
	if err := agents.Save(cur); err != nil {
		t.Fatalf("回滚失败: %v", err)
	}

	fm := newFakeMarket(t, "agent", "tool-alpha")
	fm.publish("1.0.0", v1)
	fm.publish("2.0.0", v2)
	if err := d.app.SetMarketBaseForTest(fm.srv.URL); err != nil {
		t.Fatalf("设置测试市场失败: %v", err)
	}

	backend := upgradeToolBackend{d: d}
	items, err := backend.CheckBundleUpdates("agent", "tool-alpha")
	if err != nil {
		t.Fatalf("工具检查失败: %v", err)
	}
	if len(items) != 1 || !items[0].Upgradable || items[0].LocalVersion != "1.0.0" {
		t.Fatalf("工具检查结果不对：%+v", items)
	}

	out, err := backend.UpgradeBundle("agent", "tool-alpha", false, "")
	if err != nil {
		t.Fatalf("工具升级失败: %v", err)
	}
	if !out.Applied || out.Status != "upgraded" {
		t.Fatalf("工具升级结果不对：%+v", out)
	}
	if out.ToVersion != "2.0.0" {
		t.Fatalf("升级后目标版本应为 2.0.0，实际 %q", out.ToVersion)
	}

	// 非法 kind 在适配层就挡掉，不落到 daemon 逻辑里
	if _, err := backend.CheckBundleUpdates("plugin", ""); err == nil {
		t.Fatal("非法 kind 应报错")
	}
}

// TestBundleAdopt 存量纳管：本地内容与社区某一版逐字节一致才写版本号。
//
// 三条边界都在这里钉死：①一致 ⇒ 写入且能立刻被版本闸看见；②本地改过 ⇒ 不写，
// 如实说认不出来；③即使本地已雇佣（hired: true），也不影响「内容一致」的判定
// ——hired 是运行态，不参与内容比对。
func TestBundleAdopt(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	agents := d.app.Agents()
	if err := agents.Save(&agentstore.Agent{
		Meta: agentstore.AgentMeta{Name: "adopt-alpha", Role: "测试", Description: "纳管用例"},
		Soul: "原始规则",
	}); err != nil {
		t.Fatalf("保存 agent 失败: %v", err)
	}
	if err := agents.SetHired("adopt-alpha", true); err != nil {
		t.Fatalf("设置雇佣态失败: %v", err)
	}
	skills := d.app.Skills()
	v1 := exportAgentPkg(t, agents.Get("adopt-alpha"), skills)

	// 模拟「版本字段出现之前的老安装」：磁盘上的 IDENTITY.md 没有 version
	stripVersionField(t, agents.Get("adopt-alpha").Dir)

	// 社区里有同名包 1.0.0，内容与本地一致
	fm := newFakeMarket(t, "agent", "adopt-alpha")
	fm.publish("1.0.0", v1)
	if err := d.app.SetMarketBaseForTest(fm.srv.URL); err != nil {
		t.Fatalf("设置测试市场失败: %v", err)
	}

	raw, err := d.handleBundleAdopt(context.Background(), mustJSON(t, map[string]any{
		"kind": "agent", "name": "adopt-alpha",
	}))
	if err != nil {
		t.Fatalf("纳管失败: %v", err)
	}
	res := raw.(map[string]any)
	if res["adopted"] != 1 {
		t.Fatalf("应纳管 1 个包，实际 %+v", res)
	}
	items := res["items"].([]AdoptItem)
	if len(items) != 1 || items[0].Version != "1.0.0" || !items[0].Adopted {
		t.Fatalf("纳管结果不对：%+v", items)
	}
	// 落盘后可被版本闸读到，且雇佣态仍在
	v, ok, err := bundle.LocalVersion(bundle.KindAgent, agents.Get("adopt-alpha").Dir)
	if err != nil || !ok || v != "1.0.0" {
		t.Fatalf("版本号未写入正确：v=%q ok=%v err=%v", v, ok, err)
	}
	data, _ := os.ReadFile(filepath.Join(agents.Get("adopt-alpha").Dir, "IDENTITY.md"))
	if !strings.Contains(string(data), "hired: true") {
		t.Fatalf("纳管不应改动雇佣态：%s", string(data))
	}

	// 本地改过的包：认不出来，不写
	if err := os.WriteFile(filepath.Join(agents.Get("adopt-alpha").Dir, "SOUL.md"), []byte("我改过的规则"), 0644); err != nil {
		t.Fatalf("改写失败: %v", err)
	}
	stripVersionField(t, agents.Get("adopt-alpha").Dir)
	raw, err = d.handleBundleAdopt(context.Background(), mustJSON(t, map[string]any{
		"kind": "agent", "name": "adopt-alpha",
	}))
	if err != nil {
		t.Fatalf("二次纳管失败: %v", err)
	}
	res = raw.(map[string]any)
	if res["adopted"] != 0 {
		t.Fatalf("改过的包不应被纳管：%+v", res)
	}
	for _, it := range res["items"].([]AdoptItem) {
		if it.Name == "adopt-alpha" && !strings.Contains(it.Reason, "不一致") {
			t.Fatalf("应说明为何认不出来：%+v", it)
		}
	}
}

// TestBundleAdoptLocalOnly 社区没有同名包（本地自建 / 已下架）：照样给起点 1.0.0，
// 理由写清楚——这不是「猜它是哪一版」，而是给自建实体一个显式起点。
func TestBundleAdoptLocalOnly(t *testing.T) {
	d, cleanup := newTestDaemon(t)
	defer cleanup()

	agents := d.app.Agents()
	if err := agents.Save(&agentstore.Agent{
		Meta: agentstore.AgentMeta{Name: "solo", Role: "测试", Description: "自建"},
		Soul: "规则",
	}); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	stripVersionField(t, agents.Get("solo").Dir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	}))
	defer srv.Close()
	if err := d.app.SetMarketBaseForTest(srv.URL); err != nil {
		t.Fatalf("设置测试市场失败: %v", err)
	}

	raw, err := d.handleBundleAdopt(context.Background(), mustJSON(t, map[string]any{
		"kind": "agent", "name": "solo",
	}))
	if err != nil {
		t.Fatalf("纳管失败: %v", err)
	}
	items := raw.(map[string]any)["items"].([]AdoptItem)
	if len(items) != 1 || !items[0].Adopted || items[0].Version != "1.0.0" {
		t.Fatalf("自建包应起版 1.0.0：%+v", items)
	}
	if !strings.Contains(items[0].Reason, "社区无同名包") {
		t.Fatalf("理由要说清是自建包起版：%+v", items[0])
	}
}

// stripVersionField 从 IDENTITY.md 里删掉 version 行，模拟版本字段出现之前的老安装。
func stripVersionField(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, "IDENTITY.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	var kept []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "version:") {
			continue
		}
		kept = append(kept, line)
	}
	if err := os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0644); err != nil {
		t.Fatalf("写回失败: %v", err)
	}
}
