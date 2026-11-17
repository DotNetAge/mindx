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
		// 比的是「这个实体的定义」，不是整包字节：包内附带的技能副本与全局技能库
		// 各有自己的版本链，拿它们的差异判定「这个包被改过」是错的（详见 definition.go）
		same, cerr := bundle.DefinitionMatches(path, dir, d.bundleInstallOptions(false))
		if cerr != nil {
			item.Note = "内容比对失败：" + cerr.Error()
			break
		}
		modified := !same
		item.Modified = &modified
	}
	return item, nil
}

// validUpgradeKind 包类型白名单（升级/纳管三条入口共用）。
func validUpgradeKind(kind string) bool {
	return kind == "agent" || kind == "team" || kind == "skill"
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
	if p.Kind != "" && !validUpgradeKind(p.Kind) {
		return nil, fmt.Errorf("kind 只支持 agent / team / skill（收到 %q）", p.Kind)
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
	if !validUpgradeKind(p.Kind) {
		return nil, fmt.Errorf("kind 只支持 agent / team / skill（收到 %q）", p.Kind)
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

// ─── 存量纳管（bundle.adopt）───
//
// 只补一个字段，不动任何内容：本地内容与市场某一版逐字节一致 ⇒ 这个版本号是事实；
// 一个版本都不匹配 ⇒ 如实说「认不出来」，不写。定案「不猜」的边界就落在这里。

// AdoptItem 单个包的纳管结果。
type AdoptItem struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Adopted bool   `json:"adopted"`
	// Already 原本就有版本记录（无需纳管）：不计入 pending，也不出现在结果里
	Already bool   `json:"already,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// adoptOne 尝试为一个无版本记录的包补写版本号。
func (d *Daemon) adoptOne(kind, name string, dryRun bool) (AdoptItem, error) {
	item := AdoptItem{Kind: kind, Name: name}
	dir, ok := d.installedBundleDirs(kind)[name]
	if !ok {
		return item, fmt.Errorf("本地没有已安装的%s %q", upgradeKindLabel(kind), name)
	}
	cur, found, err := bundle.LocalVersion(bundle.PackageKind(kind), dir)
	if err != nil {
		return item, err
	}
	if found {
		item.Version = cur
		item.Already = true
		item.Reason = "已有版本记录，无需纳管"
		return item, nil
	}

	pkg, err := d.app.Market().PackageByKindName(kind, name)
	if err != nil {
		if errors.Is(err, bundle.ErrMarketPackageNotFound) {
			// 社区里没有同名包 = 本地自建（或已下架）。它没有「对应哪一版」的问题，
			// 但也不该永远没有起点：按本地自有内容起版 1.0.0，之后要发布就是常规流程。
			// 技能不套用这条——技能版本约定写在 SKILL.md 的 metadata.version，由技能链管。
			if kind == "skill" {
				item.Reason = "社区中没有同名包，无从确认版本"
				return item, nil
			}
			item.Version = bundle.DefaultVersion
			if dryRun {
				item.Reason = "社区无同名包，可按本地内容起版 " + bundle.DefaultVersion + "（未写入）"
				return item, nil
			}
			if werr := bundle.WriteLocalVersion(bundle.PackageKind(kind), dir, bundle.DefaultVersion); werr != nil {
				return item, fmt.Errorf("写入版本号失败：%w", werr)
			}
			item.Adopted = true
			item.Reason = "社区无同名包，按本地内容起版 " + bundle.DefaultVersion
			return item, nil
		}
		return item, fmt.Errorf("查询社区失败：%w", err)
	}
	ver, matched, err := bundle.MatchInstalledVersion(bundle.PackageKind(kind), dir, *pkg,
		d.bundleInstallOptions(false), d.app.Market())
	if err != nil {
		return item, err
	}
	if !matched {
		item.Reason = "本地内容与社区任何版本都不一致（可能本地改过，或来源不同），不写版本号"
		return item, nil
	}
	item.Version = ver
	if dryRun {
		item.Reason = "可纳管（未写入）"
		return item, nil
	}
	if err := bundle.WriteLocalVersion(bundle.PackageKind(kind), dir, ver); err != nil {
		return item, fmt.Errorf("写入版本号失败：%w", err)
	}
	item.Adopted = true
	return item, nil
}

// handleBundleAdopt 把「版本字段出现之前装上的包」纳入版本管理。
// kind/name 皆空 = 全量尝试；dry_run 只报告不写盘。
func (d *Daemon) handleBundleAdopt(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.BundleAdoptParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Kind != "" && !validUpgradeKind(p.Kind) {
		return nil, fmt.Errorf("kind 只支持 agent / team / skill（收到 %q）", p.Kind)
	}
	if p.Kind == "" && p.Name != "" {
		return nil, fmt.Errorf("指定 name 时必须同时给出 kind")
	}
	kinds := upgradeKinds
	if p.Kind != "" {
		kinds = []string{p.Kind}
	}

	type target struct{ kind, name string }
	var targets []target
	for _, k := range kinds {
		dirs := d.installedBundleDirs(k)
		if p.Name != "" {
			targets = append(targets, target{kind: k, name: p.Name})
			continue
		}
		for n := range dirs {
			targets = append(targets, target{kind: k, name: n})
		}
	}

	items := make([]AdoptItem, len(targets))
	failed := make([]string, len(targets))
	sem := make(chan struct{}, upgradeCheckWorkers)
	var wg sync.WaitGroup
	for i, tg := range targets {
		wg.Add(1)
		go func(i int, tg target) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			it, err := d.adoptOne(tg.kind, tg.name, p.DryRun)
			if err != nil {
				failed[i] = fmt.Sprintf("%s/%s: %v", tg.kind, tg.name, err)
				return
			}
			items[i] = it
		}(i, tg)
	}
	wg.Wait()

	adopted, pending := 0, 0
	out := make([]AdoptItem, 0, len(items))
	errs := make([]string, 0, len(failed))
	for i, it := range items {
		if failed[i] != "" {
			errs = append(errs, failed[i])
			continue
		}
		if it.Adopted {
			adopted++
		}
		// pending = 「能认出版本但这次没写」（dry_run，或调用方没让写）。
		// 本来就有版本的包不算 pending——那会让人以为有活没干。
		if it.Version != "" && !it.Adopted && !it.Already {
			pending++
		}
		// 已有版本的包不进结果：全量纳管时那是绝大多数，列出来只会淹没有效信息
		if !it.Already && (it.Adopted || it.Reason != "") {
			out = append(out, it)
		}
	}

	// 写盘过的包要把内存里的版本刷出来（否则列表/检查仍显示未纳管）
	if adopted > 0 {
		for _, k := range kinds {
			if err := d.reloadBundles(k); err != nil {
				errs = append(errs, fmt.Sprintf("纳管已写入但重载失败（%s）：%v", k, err))
			}
		}
	}

	resp := map[string]any{
		"items":   out,
		"adopted": adopted,
		"pending": pending,
	}
	if len(errs) > 0 {
		resp["errors"] = errs
	}
	return resp, nil
}
