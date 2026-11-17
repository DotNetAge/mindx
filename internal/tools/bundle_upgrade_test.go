package tools

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeUpgradeBackend 记录调用参数的假后端（工具层只验契约与话术，不碰真磁盘）。
type fakeUpgradeBackend struct {
	items    []BundleUpdateTarget
	checkErr error

	upgradeStatus map[string]string // name → status
	upgradeErr    map[string]error

	adoptSummary AdoptSummary
	adoptErr     error

	calls []struct {
		kind  string
		name  string
		force bool
		ver   string
	}
}

func (f *fakeUpgradeBackend) CheckBundleUpdates(kind, name string) ([]BundleUpdateTarget, error) {
	f.calls = append(f.calls, struct {
		kind  string
		name  string
		force bool
		ver   string
	}{kind: kind, name: name})
	if f.checkErr != nil {
		return nil, f.checkErr
	}
	var out []BundleUpdateTarget
	for _, it := range f.items {
		if kind != "" && it.Kind != kind {
			continue
		}
		if name != "" && it.Name != name {
			continue
		}
		out = append(out, it)
	}
	return out, nil
}

func (f *fakeUpgradeBackend) UpgradeBundle(kind, name string, force bool, version string) (BundleUpgradeOutcome, error) {
	f.calls = append(f.calls, struct {
		kind  string
		name  string
		force bool
		ver   string
	}{kind: kind, name: name, force: force, ver: version})
	if err := f.upgradeErr[name]; err != nil {
		return BundleUpgradeOutcome{}, err
	}
	status := f.upgradeStatus[name]
	if status == "" {
		status = "upgraded"
	}
	return BundleUpgradeOutcome{
		Kind: kind, Name: name, Status: status,
		Applied: status == "upgraded", FromVersion: "1.0.0", ToVersion: "1.1.0",
	}, nil
}

func (f *fakeUpgradeBackend) AdoptPackages(kind, name string, dryRun bool) (AdoptSummary, error) {
	f.calls = append(f.calls, struct {
		kind  string
		name  string
		force bool
		ver   string
	}{kind: kind, name: name})
	if f.adoptErr != nil {
		return AdoptSummary{}, f.adoptErr
	}
	return f.adoptSummary, nil
}

func modFlag(v bool) *bool { return &v }

// TestBundleUpgradeCheck 检查：只读，把可升级与「改没改过」如实回报。
func TestBundleUpgradeCheck(t *testing.T) {
	fe := &fakeUpgradeBackend{items: []BundleUpdateTarget{
		{Kind: "agent", Name: "alpha", LocalVersion: "1.0.0", LatestVersion: "1.1.0", Upgradable: true, Modified: modFlag(false)},
		{Kind: "team", Name: "squad", LocalVersion: "2.0.0", LatestVersion: "2.0.0"},
	}}
	got, err := NewBundleUpgrade(fe).Execute(context.Background(), map[string]any{"action": "check"})
	if err != nil {
		t.Fatalf("check 失败：%v", err)
	}
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("返回值类型不对：%T", got)
	}
	if m["total"] != 2 || m["upgradable"] != 1 {
		t.Fatalf("统计不对：%+v", m)
	}

	// 指定 kind/name 只查那一个
	if _, err := NewBundleUpgrade(fe).Execute(context.Background(), map[string]any{
		"action": "check", "kind": "agent", "name": "alpha",
	}); err != nil {
		t.Fatalf("单包 check 失败：%v", err)
	}
	last := fe.calls[len(fe.calls)-1]
	if last.kind != "agent" || last.name != "alpha" {
		t.Fatalf("单包 check 参数没透传：%+v", last)
	}
}

// TestBundleUpgradeCheckEmpty 没有可升级时给出人话 summary（模型不必自己拼）。
func TestBundleUpgradeCheckEmpty(t *testing.T) {
	fe := &fakeUpgradeBackend{items: []BundleUpdateTarget{
		{Kind: "agent", Name: "alpha", LocalVersion: "1.0.0", LatestVersion: "1.0.0"},
	}}
	got, err := NewBundleUpgrade(fe).Execute(context.Background(), map[string]any{"action": "check"})
	if err != nil {
		t.Fatalf("check 失败：%v", err)
	}
	if s, _ := got.(map[string]any)["summary"].(string); !strings.Contains(s, "最新") {
		t.Fatalf("无可升级时应给出 summary，实际 %+v", got)
	}
}

// TestBundleUpgradeNeedsConfirm 本地改过的包：工具不覆盖，返回 needs_confirm + 原因 + 提示。
func TestBundleUpgradeNeedsConfirm(t *testing.T) {
	fe := &fakeUpgradeBackend{
		items:         []BundleUpdateTarget{{Kind: "agent", Name: "alpha", LocalVersion: "1.0.0", LatestVersion: "1.1.0", Upgradable: true, Modified: modFlag(true)}},
		upgradeStatus: map[string]string{"alpha": "needs_confirm"},
	}
	got, err := NewBundleUpgrade(fe).Execute(context.Background(), map[string]any{
		"action": "upgrade", "kind": "agent", "name": "alpha",
	})
	if err != nil {
		t.Fatalf("upgrade 失败：%v", err)
	}
	m := got.(map[string]any)
	if m["needs_confirm"] != 1 || m["upgraded"] != 0 {
		t.Fatalf("needs_confirm 统计不对：%+v", m)
	}
	if hint, _ := m["hint"].(string); !strings.Contains(hint, "force") {
		t.Fatalf("应提示如何继续（征得同意后用 force）：%+v", m)
	}
	results := m["results"].([]BundleUpgradeOutcome)
	if len(results) != 1 || !strings.Contains(results[0].Note, "修改过") {
		t.Fatalf("应说明需要确认的原因：%+v", results)
	}
	// 未确认时不得替调用方加 force
	if fe.calls[len(fe.calls)-1].force {
		t.Fatal("未经用户同意不得传 force")
	}
}

// TestBundleUpgradeAll 批量升级：只处理可升级的，单个失败不影响其余。
func TestBundleUpgradeAll(t *testing.T) {
	fe := &fakeUpgradeBackend{
		items: []BundleUpdateTarget{
			{Kind: "agent", Name: "alpha", LocalVersion: "1.0.0", LatestVersion: "1.1.0", Upgradable: true},
			{Kind: "agent", Name: "beta", LocalVersion: "1.0.0", LatestVersion: "1.1.0", Upgradable: true},
			{Kind: "agent", Name: "gamma", LocalVersion: "1.0.0", LatestVersion: "1.0.0"}, // 已最新
		},
		upgradeErr: map[string]error{"beta": errors.New("下载失败")},
	}
	got, err := NewBundleUpgrade(fe).Execute(context.Background(), map[string]any{
		"action": "upgrade", "kind": "agent",
	})
	if err != nil {
		t.Fatalf("批量升级失败：%v", err)
	}
	m := got.(map[string]any)
	if m["upgraded"] != 1 || m["skipped"] != 1 {
		t.Fatalf("批量统计不对：%+v", m)
	}
	results := m["results"].([]BundleUpgradeOutcome)
	if len(results) != 2 {
		t.Fatalf("应只为 2 个可升级包出结果，实际 %d", len(results))
	}
	var sawFailed bool
	for _, r := range results {
		if r.Name == "beta" && r.Status == "failed" {
			sawFailed = true
		}
	}
	if !sawFailed {
		t.Fatalf("单个包失败应如实回报而不是吞掉：%+v", results)
	}
}

// TestBundleUpgradeAllNothingToDo 批量升级但无包可升：不产生任何执行调用。
func TestBundleUpgradeAllNothingToDo(t *testing.T) {
	fe := &fakeUpgradeBackend{items: []BundleUpdateTarget{
		{Kind: "agent", Name: "alpha", LocalVersion: "1.0.0", LatestVersion: "1.0.0"},
	}}
	got, err := NewBundleUpgrade(fe).Execute(context.Background(), map[string]any{
		"action": "upgrade", "kind": "agent",
	})
	if err != nil {
		t.Fatalf("失败：%v", err)
	}
	if m := got.(map[string]any); m["upgraded"] != 0 {
		t.Fatalf("无包可升时应为 0：%+v", m)
	}
	if len(fe.calls) != 1 { // 只有一次 check
		t.Fatalf("无包可升时不应调用 upgrade，实际调用 %d 次", len(fe.calls))
	}
}

// TestBundleUpgradeArgsGuard 参数守卫：非法 kind / version 缺 name / 缺 action 都要明确报错。
func TestBundleUpgradeArgsGuard(t *testing.T) {
	fe := &fakeUpgradeBackend{}
	tool := NewBundleUpgrade(fe)
	ctx := context.Background()

	if _, err := tool.Execute(ctx, map[string]any{}); err == nil {
		t.Fatal("缺 action 应报错")
	}
	if _, err := tool.Execute(ctx, map[string]any{"action": "check", "kind": "plugin"}); err == nil {
		t.Fatal("非法 kind 应报错")
	}
	if _, err := tool.Execute(ctx, map[string]any{"action": "upgrade", "name": "alpha"}); err == nil {
		t.Fatal("upgrade 缺 kind 应报错")
	}
	if _, err := tool.Execute(ctx, map[string]any{"action": "upgrade", "kind": "agent", "version": "1.0.0"}); err == nil {
		t.Fatal("version 未给 name 应报错")
	}
	if _, err := tool.Execute(ctx, map[string]any{"action": "nope", "kind": "agent"}); err == nil {
		t.Fatal("非法 action 应报错")
	}
}

// TestBundleUpgradeCheckFailure 市场不可达：报错（不能静默返回空列表，否则会被读成「没新版本」）。
func TestBundleUpgradeCheckFailure(t *testing.T) {
	fe := &fakeUpgradeBackend{checkErr: errors.New("网络不可达")}
	_, err := NewBundleUpgrade(fe).Execute(context.Background(), map[string]any{"action": "check"})
	if err == nil {
		t.Fatal("检查失败应报错")
	}
	if !strings.Contains(err.Error(), "网络不可达") {
		t.Fatalf("错误应带原始原因：%v", err)
	}
}

// TestBundleUpgradeNoBackend 未注入后端（TUI 等场景）：明确报错而不是空指针。
func TestBundleUpgradeNoBackend(t *testing.T) {
	_, err := NewBundleUpgrade(nil).Execute(context.Background(), map[string]any{"action": "check"})
	if err == nil {
		t.Fatal("无后端应报错")
	}
}

// TestBundleUpgradeAdopt 纳管动作：把后端结果原样交回模型，不做二次解读。
func TestBundleUpgradeAdopt(t *testing.T) {
	fe := &fakeUpgradeBackend{adoptSummary: AdoptSummary{
		Items: []AdoptRow{
			{Kind: "agent", Name: "alpha", Version: "1.0.0", Adopted: true},
			{Kind: "agent", Name: "beta", Reason: "本地内容与社区任何版本都不一致（可能本地改过，或来源不同），不写版本号"},
		},
		Adopted: 1,
	}}
	got, err := NewBundleUpgrade(fe).Execute(context.Background(), map[string]any{
		"action": "adopt", "kind": "agent",
	})
	if err != nil {
		t.Fatalf("adopt 失败：%v", err)
	}
	summary, ok := got.(AdoptSummary)
	if !ok {
		t.Fatalf("返回值类型不对：%T", got)
	}
	if summary.Adopted != 1 || len(summary.Items) != 2 {
		t.Fatalf("纳管结果没透传：%+v", summary)
	}
	if last := fe.calls[len(fe.calls)-1]; last.kind != "agent" || last.name != "" {
		t.Fatalf("纳管参数没透传：%+v", last)
	}
}
