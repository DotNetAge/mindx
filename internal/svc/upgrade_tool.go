package svc

import (
	"fmt"
	"strings"

	mindxtools "github.com/DotNetAge/mindx/internal/tools"
)

// 本文件把 Daemon 的升级能力适配成 Agent 工具的接口（tools.BundleUpgradeBackend）。
//
// 为什么不直接在工具里做：tools 包不能反向依赖 svc（循环），而升级必须复用
// 「两把闸 + 备份 + 重载」这一条执行体——界面按钮、CLI、Agent 工具三条入口
// 一旦各写一遍，迟早会出现「界面说可升级、Agent 说没有」的分裂。

// upgradeToolBackend 升级工具的 daemon 适配器。
type upgradeToolBackend struct {
	d *Daemon
}

// CheckBundleUpdates 已装包与市场最新版本的对照（kind/name 皆空 = 三类全查）。
func (b upgradeToolBackend) CheckBundleUpdates(kind, name string) ([]mindxtools.BundleUpdateTarget, error) {
	k := strings.TrimSpace(kind)
	if k != "" && k != "agent" && k != "team" && k != "skill" {
		return nil, fmt.Errorf("kind 只支持 agent / team / skill（收到 %q）", kind)
	}
	items, errs := b.d.collectUpgradeItems(k, strings.TrimSpace(name))
	out := make([]mindxtools.BundleUpdateTarget, 0, len(items))
	for _, it := range items {
		out = append(out, mindxtools.BundleUpdateTarget{
			Kind:          it.Kind,
			Name:          it.Name,
			LocalVersion:  it.LocalVersion,
			LatestVersion: it.LatestVersion,
			Upgradable:    it.Upgradable,
			Modified:      it.Modified,
			Note:          it.Note,
		})
	}
	if len(errs) > 0 {
		// 部分失败不整体失败：能查的照样给模型，但把失败原因带回去，避免「查不到」
		// 被模型误读成「没有新版本」。
		return out, fmt.Errorf("部分包检查失败：%s", strings.Join(errs, "；"))
	}
	return out, nil
}

// UpgradeBundle 执行单包升级（与 bundle.upgrade RPC 同一执行体）。
func (b upgradeToolBackend) UpgradeBundle(kind, name string, force bool, version string) (mindxtools.BundleUpgradeOutcome, error) {
	res, err := b.d.applyUpgrade(strings.TrimSpace(kind), strings.TrimSpace(name), force, version)
	if err != nil {
		return mindxtools.BundleUpgradeOutcome{}, err
	}
	out := mindxtools.BundleUpgradeOutcome{
		Kind:        res.Item.Kind,
		Name:        res.Item.Name,
		Applied:     res.Applied,
		Status:      res.Status,
		FromVersion: res.Item.LocalVersion,
		ToVersion:   res.Item.LatestVersion,
		Backup:      res.Backup,
		Note:        res.Item.Note,
	}
	return out, nil
}
