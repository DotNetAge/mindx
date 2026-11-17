package svc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/DotNetAge/mindx/internal/core/bundle"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

// 本文件实现升级链路 L3（定案：mindx-market/docs/设计定案-市场包血统与升级链路.md）。
//
// 两把闸各管一件事，缺一不可：
//
//   - 版本闸：本地定义文件 frontmatter 的 version ≠ 市场最新版本 ⇒ 有新版；
//   - 内容闸：拿「本地记录的那一版」的包字节与盘上内容逐字节比对 ⇒ 判断本地改没改过。
//
// 内容闸必须比「已装的那一版」，不能比最新版——比最新版会把「有新版本」误判成
// 「被本地改过」。取旧版字节依赖服务端按版本下载端点（L2），所以这条链是有序的。
//
// 升级只做「保留 / 覆盖」二选一：改过的包默认拒绝升级（保留），用户确认后 force 覆盖。
// 不做三方合并——合并是猜测，不是事实。覆盖前一律先整目录快照到
// <runtime>/.bundle-backups/<kind>/<name>/<ts>/，出问题可整目录换回。

// UpgradeItem 单个已装包的升级对照行。
type UpgradeItem struct {
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	LocalVersion  string `json:"local_version,omitempty"`
	LatestVersion string `json:"latest_version,omitempty"`
	Upgradable    bool   `json:"upgradable"`
	// Modified 本地内容相对「已装版本」是否被改过。nil = 无法判定
	// （本地没记版本，或本地那版已在市场下架取不回字节）。
	Modified *bool  `json:"modified,omitempty"`
	Note     string `json:"note,omitempty"`
}

// UpgradeResult 单包升级结果。
type UpgradeResult struct {
	Applied bool        `json:"applied"`
	Status  string      `json:"status"` // upgraded | up_to_date | needs_confirm | rolled_back
	Item    UpgradeItem `json:"item"`
	Backup  string      `json:"backup,omitempty"`
}

// upgradeKinds 支持升级的包类型（与 market.install 同一集合）。
var upgradeKinds = []string{"agent", "team", "skill"}

// bundleInstallOptions 安装/比对共用的目录配置（覆盖语义由调用方按场景决定）。
func (d *Daemon) bundleInstallOptions(overwrite bool) bundle.InstallOptions {
	return bundle.InstallOptions{
		GlobalDir: d.app.Skills().GlobalDir(),
		AgentsDir: d.app.Agents().Dir(),
		TeamsDir:  d.app.Teams().Dir(),
		Overwrite: overwrite,
	}
}

// installedBundleDirs 本地已装包 → 目录（来源是各注册表的实际加载结果，
// 不按名字拼路径：目录名大小写与历史布局由注册表说了算）。
func (d *Daemon) installedBundleDirs(kind string) map[string]string {
	out := make(map[string]string)
	switch kind {
	case "agent":
		for _, a := range d.app.Agents().List() {
			out[a.Name()] = a.Dir
		}
	case "team":
		for _, t := range d.app.Teams().List() {
			out[t.Name()] = t.Dir
		}
	case "skill":
		globalDir := d.app.Skills().GlobalDir()
		for _, s := range d.app.Skills().Global().List() {
			out[s.Name] = filepath.Join(globalDir, s.Name)
		}
	}
	return out
}

// checkBundleUpgrade 单包升级检查（版本闸 + 内容闸）。
func (d *Daemon) checkBundleUpgrade(kind, name string) (UpgradeItem, error) {
	item := UpgradeItem{Kind: kind, Name: name}
	dirs := d.installedBundleDirs(kind)
	dir, ok := dirs[name]
	if !ok {
		return item, fmt.Errorf("本地没有已安装的%s %q", upgradeKindLabel(kind), name)
	}
	local, found, err := bundle.LocalVersion(bundle.PackageKind(kind), dir)
	if err != nil {
		return item, err
	}
	item.LocalVersion = local
	if !found {
		// 没有记录版本就不必问市场：判不出「是不是新版」，也判不出「改没改过」。
		// 顺带把全量检查的网络往返砍掉一大截——存量包普遍没有 version。
		item.Note = "本地未记录版本（旧安装或第三方包），重装一次即获得版本血统"
		return item, nil
	}

	pkg, err := d.app.Market().PackageByKindName(kind, name)
	if err != nil {
		// 404 = 市场里确实没有（本地独有 / 已下架）：不是错误，如实标注。
		// 其它错误（网络/鉴权）必须上抛——把连不上说成「没有这个包」会让人以为被下架了。
		if errors.Is(err, bundle.ErrMarketPackageNotFound) {
			item.Note = "市场中无同名包（本地独有或已下架）"
			return item, nil
		}
		return item, fmt.Errorf("查询市场失败：%w", err)
	}
	item.LatestVersion = pkg.Version
	item.Upgradable = pkg.Version != "" && pkg.Version != local

	switch {
	case pkg.Version == "":
		item.Note = "市场未下发版本号"
	default:
		// 内容闸：只有能取回「本地记录的那一版」字节才判定得出来
		if _, ok := pkg.FindVersion(local); !ok {
			item.Note = "本地记录的版本已在市场下架，无法取回该版字节做比对"
			break
		}
		path, derr := d.app.Market().DownloadVersion(*pkg, local)
		if derr != nil {
			item.Note = "取回本地版本包失败：" + derr.Error()
			break
		}
		same, cerr := bundle.ContentMatches(path, d.bundleInstallOptions(false))
		if cerr != nil {
			item.Note = "内容比对失败：" + cerr.Error()
			break
		}
		modified := !same
		item.Modified = &modified
	}
	return item, nil
}

// upgradeKindLabel 包类型中文名（错误与提示文案用）。
func upgradeKindLabel(kind string) string {
	switch kind {
	case "agent":
		return "智能体"
	case "team":
		return "团队"
	case "skill":
		return "技能"
	}
	return kind
}

// upgradeCheckWorkers 全量检查的并发上限。
// 每个包要打一次市场详情 API（再可能下一次包），几百个包串行跑会让人以为界面卡死；
// 但也不能不限并发——服务端与本地磁盘都不喜欢突发流量。
const upgradeCheckWorkers = 8

// collectUpgradeItems 全量或单包检查（并发有界）。
// RPC 与 Agent 工具共用它：两条入口必须给出同一份判定，否则「界面说可升级、Agent 说没有」。
func (d *Daemon) collectUpgradeItems(kind, name string) ([]UpgradeItem, []string) {
	kinds := upgradeKinds
	if kind != "" {
		kinds = []string{kind}
	}

	type target struct{ kind, name string }
	var targets []target
	for _, k := range kinds {
		dirs := d.installedBundleDirs(k)
		if name != "" {
			targets = append(targets, target{kind: k, name: name})
			continue
		}
		for n := range dirs {
			targets = append(targets, target{kind: k, name: n})
		}
	}

	// 结果按下标回填：并发只影响耗时，不影响输出顺序（顺序稳定才好比对两次检查）
	items := make([]UpgradeItem, len(targets))
	failed := make([]string, len(targets))
	sem := make(chan struct{}, upgradeCheckWorkers)
	var wg sync.WaitGroup
	for i, tg := range targets {
		wg.Add(1)
		go func(i int, tg target) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			item, err := d.checkBundleUpgrade(tg.kind, tg.name)
			if err != nil {
				failed[i] = fmt.Sprintf("%s/%s: %v", tg.kind, tg.name, err)
				return
			}
			items[i] = item
		}(i, tg)
	}
	wg.Wait()

	out := make([]UpgradeItem, 0, len(items))
	errs := make([]string, 0, len(failed))
	for i, it := range items {
		if failed[i] != "" {
			errs = append(errs, failed[i])
			continue
		}
		out = append(out, it)
	}
	return out, errs
}

// handleBundleUpgradeCheck 检查已装包是否有新版本。kind/name 皆空 = 全量检查。
func (d *Daemon) handleBundleUpgradeCheck(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.BundleUpgradeCheckParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Kind != "" && p.Name == "" {
		return nil, fmt.Errorf("指定 kind 时必须同时给出 name")
	}
	items, errs := d.collectUpgradeItems(p.Kind, p.Name)
	upgradable := 0
	for _, it := range items {
		if it.Upgradable {
			upgradable++
		}
	}
	resp := map[string]any{
		"items":      items,
		"total":      len(items),
		"upgradable": upgradable,
	}
	if len(errs) > 0 {
		resp["errors"] = errs
	}
	return resp, nil
}

// handleBundleUpgrade 升级单个包（缺省升到市场最新；给 version 即回滚到该版）。
func (d *Daemon) handleBundleUpgrade(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.BundleUpgradeParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Kind == "" || p.Name == "" {
		return nil, fmt.Errorf("kind 与 name 均为必填")
	}
	return d.applyUpgrade(p.Kind, p.Name, p.Force, p.Version)
}

// applyUpgrade 执行单包升级（RPC 与 Agent 工具共用同一条执行体）。
func (d *Daemon) applyUpgrade(kind, name string, force bool, version string) (UpgradeResult, error) {
	item, err := d.checkBundleUpgrade(kind, name)
	if err != nil {
		return UpgradeResult{}, err
	}

	pkg, err := d.app.Market().PackageByKindName(kind, name)
	if err != nil {
		return UpgradeResult{}, err
	}
	target := strings.TrimSpace(version)
	if target == "" {
		target = pkg.Version
	}
	if target == "" {
		return UpgradeResult{}, fmt.Errorf("市场未下发 %s/%s 的版本号，无法确定升级目标", kind, name)
	}

	status := "upgraded"
	if target == item.LatestVersion {
		status = "upgraded"
	} else if item.LocalVersion != "" && target != item.LocalVersion {
		status = "rolled_back"
	}

	if target == item.LocalVersion {
		return UpgradeResult{Status: "up_to_date", Item: item}, nil
	}
	// 内容闸：改动过或判不出来，默认不覆盖（保留），由用户决定
	if item.Modified == nil || *item.Modified {
		if !force {
			return UpgradeResult{Status: "needs_confirm", Item: item}, nil
		}
	}

	var pkgPath string
	if target == pkg.Version {
		pkgPath, err = d.app.Market().Download(*pkg)
	} else {
		pkgPath, err = d.app.Market().DownloadVersion(*pkg, target)
	}
	if err != nil {
		return UpgradeResult{}, err
	}

	dir := d.installedBundleDirs(kind)[name]
	backup, err := d.backupBundleDir(kind, name, dir)
	if err != nil {
		return UpgradeResult{}, fmt.Errorf("升级前备份失败，已中止（未改动本地文件）：%w", err)
	}

	if _, err := bundle.Install(pkgPath, d.bundleInstallOptions(true)); err != nil {
		return UpgradeResult{}, err
	}
	if err := d.reloadBundles(kind); err != nil {
		return UpgradeResult{}, fmt.Errorf("包已落盘但重载失败（备份在 %s）：%w", backup, err)
	}

	newVer, _, _ := bundle.LocalVersion(bundle.PackageKind(kind), dir)
	item.LocalVersion = newVer
	item.LatestVersion = pkg.Version
	item.Upgradable = pkg.Version != "" && pkg.Version != newVer
	item.Modified = nil
	item.Note = ""

	d.logger.Info("分发包升级完成", "kind", kind, "name", name,
		"from", item.LocalVersion, "to", target, "backup", backup)
	return UpgradeResult{Applied: true, Status: status, Item: item, Backup: backup}, nil
}

// reloadBundles 按包类型重载注册表并失效运行时（与安装路径同一套动作）。
func (d *Daemon) reloadBundles(kind string) error {
	if kind == "agent" || kind == "team" {
		if err := d.app.ReloadAgents(); err != nil {
			return err
		}
	}
	if kind == "team" {
		if err := d.app.ReloadTeams(); err != nil {
			return err
		}
	}
	if err := d.app.ReloadSkills(); err != nil {
		return err
	}
	d.app.InvalidateRuntimes()
	return nil
}

// backupBundleDir 覆盖前把整目录快照到 <runtime>/.bundle-backups/<kind>/<name>/<ts>/。
// 快照目录在注册表扫描范围之外（agents/ 的兄弟目录），不会被当成包实体加载。
func (d *Daemon) backupBundleDir(kind, name, srcDir string) (string, error) {
	root := filepath.Join(filepath.Dir(d.app.Agents().Dir()), ".bundle-backups")
	dest := filepath.Join(root, kind, name, time.Now().Format("20060102-150405"))
	if err := copyTree(srcDir, dest); err != nil {
		return "", err
	}
	return dest, nil
}

// copyTree 递归复制目录（保留权限；符号链接按内容复制，不跟随出界）。
func copyTree(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("备份源不是目录：%s", src)
	}
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = in.Close() }()
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			_ = out.Close()
			return err
		}
		return out.Close()
	})
}
