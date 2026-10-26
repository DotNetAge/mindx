package svc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/DotNetAge/mindx/internal/core/bundle"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

// 本文件实现分发包安装导出闭环与市场 RPC（PR-PROMPTS 第四节）：
//
//   - agent.export / skill.export：导出分发包（zip 容器，保真原则）；
//   - agent.import / skill.import：本地安装（同名冲突默认拒绝，覆盖需显式确认）；
//   - market.list：服务端分页 API 浏览（清单机制已废除，无回退）；
//   - market.install：下载 → sha256 校验 → 复用本地安装展开逻辑；
//   - market.pack / market.publish：打包预览与发布新版本（市场生态 §3）；
//   - market.submit / market.submissions.list / market.submissions.decide：
//     进化版本投递与包主审核（市场生态 §4）。

// handleAgentExport 导出 Agent 分发包到指定路径（out_path 来自前端保存对话框）。
func (d *Daemon) handleAgentExport(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.AgentExportParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" || p.OutPath == "" {
		return nil, fmt.Errorf("name 与 out_path 均为必填")
	}

	agent := d.app.Agents().Get(p.Name)
	if agent == nil {
		return nil, fmt.Errorf("agent %q not found", p.Name)
	}

	manifest, warnings, err := bundle.ExportAgent(agent, d.app.Skills(), p.OutPath)
	if err != nil {
		return nil, err
	}
	result := map[string]any{
		"status":   "ok",
		"name":     manifest.Name,
		"out_path": p.OutPath,
		"skills":   manifest.Skills,
	}
	if len(warnings) > 0 {
		result["warnings"] = warnings
	}
	d.logger.Info("Agent 分发包导出完成", "name", p.Name, "out_path", p.OutPath)
	return result, nil
}

// handleAgentImport 从本地分发包安装 Agent（展开规则见 bundle.Install）。
func (d *Daemon) handleAgentImport(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.AgentImportParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	res, err := bundle.Install(p.Path, bundle.InstallOptions{
		AgentsDir: d.app.Agents().Dir(),
		TeamsDir:  d.app.Teams().Dir(),
		GlobalDir: d.app.Skills().GlobalDir(),
		Overwrite: p.Overwrite,
	})
	if err != nil {
		return nil, err
	}

	// 安装改变了 agents 与全局技能库内容，重载注册表并失效运行时缓存
	if err := d.app.ReloadAgents(); err != nil {
		return nil, fmt.Errorf("分发包已落盘但智能体重载失败：%w", err)
	}
	if err := d.app.ReloadTeams(); err != nil {
		return nil, fmt.Errorf("分发包已落盘但团队重载失败：%w", err)
	}
	if err := d.app.ReloadSkills(); err != nil {
		return nil, fmt.Errorf("分发包已落盘但技能库重载失败：%w", err)
	}
	d.app.InvalidateRuntimes()

	d.logger.Info("Agent 分发包安装完成", "name", res.Name, "overwrite", res.Overwritten)
	return res, nil
}

// handleSkillExport 导出 Skill 分发包到指定路径。
func (d *Daemon) handleSkillExport(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.SkillExportParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" || p.OutPath == "" {
		return nil, fmt.Errorf("name 与 out_path 均为必填")
	}

	manifest, err := bundle.ExportSkill(d.app.Skills(), p.Name, p.OutPath)
	if err != nil {
		return nil, err
	}
	d.logger.Info("Skill 分发包导出完成", "name", p.Name, "out_path", p.OutPath)
	return map[string]any{
		"status":   "ok",
		"name":     manifest.Name,
		"out_path": p.OutPath,
	}, nil
}

// handleSkillImport 从本地分发包安装技能（直接进全局库）。
func (d *Daemon) handleSkillImport(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.SkillImportParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	res, err := bundle.Install(p.Path, bundle.InstallOptions{
		GlobalDir: d.app.Skills().GlobalDir(),
		Overwrite: p.Overwrite,
	})
	if err != nil {
		return nil, err
	}
	if err := d.app.ReloadSkills(); err != nil {
		return nil, fmt.Errorf("分发包已落盘但技能库重载失败：%w", err)
	}
	d.app.InvalidateRuntimes()

	d.logger.Info("Skill 分发包安装完成", "name", res.Name, "overwrite", res.Overwritten)
	// 安装即触发准备流程（requires.oauth 授权 / install 命令确认随响应带回）
	return installWithPrepare{InstallResult: *res, Prepares: d.prepareInstalledSkills(res.SkillsGlobal)}, nil
}

// handleTeamExport 导出团队整包（team/TEAM.md + 成员 Agent 实际生效内容）。
// 成员缺失时记入 warnings（引用降级语义，不阻断导出）。
func (d *Daemon) handleTeamExport(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.TeamExportParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" || p.OutPath == "" {
		return nil, fmt.Errorf("name 与 out_path 均为必填")
	}

	team := d.app.Teams().Get(p.Name)
	if team == nil {
		return nil, fmt.Errorf("team %q not found", p.Name)
	}

	manifest, warnings, err := bundle.ExportTeam(team, d.app.Agents(), d.app.Skills(), p.OutPath)
	if err != nil {
		return nil, err
	}
	result := map[string]any{
		"status":   "ok",
		"name":     manifest.Name,
		"out_path": p.OutPath,
		"members":  manifest.Members,
	}
	if len(warnings) > 0 {
		result["warnings"] = warnings
	}
	d.logger.Info("Team 分发包导出完成", "name", p.Name, "out_path", p.OutPath, "members", len(manifest.Members))
	return result, nil
}

// handleTeamImport 从本地团队分发包安装（整队落位，展开规则见 bundle.Install）。
func (d *Daemon) handleTeamImport(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.TeamImportParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	res, err := bundle.Install(p.Path, bundle.InstallOptions{
		AgentsDir: d.app.Agents().Dir(),
		TeamsDir:  d.app.Teams().Dir(),
		GlobalDir: d.app.Skills().GlobalDir(),
		Overwrite: p.Overwrite,
	})
	if err != nil {
		return nil, err
	}

	// 整队落位改变 agents/teams/全局技能库，全部重载并失效运行时缓存
	if err := d.app.ReloadAgents(); err != nil {
		return nil, fmt.Errorf("分发包已落盘但智能体重载失败：%w", err)
	}
	if err := d.app.ReloadTeams(); err != nil {
		return nil, fmt.Errorf("分发包已落盘但团队重载失败：%w", err)
	}
	if err := d.app.ReloadSkills(); err != nil {
		return nil, fmt.Errorf("分发包已落盘但技能库重载失败：%w", err)
	}
	d.app.InvalidateRuntimes()

	d.logger.Info("Team 分发包安装完成", "name", res.Name, "members", len(res.Members), "overwrite", res.Overwritten)
	return res, nil
}

// marketPackageDTO 是 market.list 响应的包条目投影：bundle.MarketPackage 的
// JSON tag 为 snake_case（对齐 COS 清单格式），现役消费方为 mindx-work 市场
// 插件（MarketPkg，snake_case），字段键与之逐一对齐（skillNames 例外——
// 两个前端的历史读取键同为 camelCase，保持不变）。
type marketPackageDTO struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	// Title 展示标题（kind=plugin；服务端 meta 快照透出。货架卡片主名用标题，
	// name 是反向域 id（com.mindxwork.gitgraph）不宜直接示人，前端按 title 优先回退 name）
	Title       string            `json:"title,omitempty"`
	Description string            `json:"description,omitempty"`
	Icon        string            `json:"icon,omitempty"`
	NickName    string            `json:"nick_name,omitempty"`
	Role        string            `json:"role,omitempty"`
	Category    string            `json:"category,omitempty"`
	Leader      string            `json:"leader,omitempty"`
	Members     []string          `json:"members,omitempty"`
	Skills      []string          `json:"skills,omitempty"`
	SkillNames  map[string]string `json:"skillNames,omitempty"`
	SkillDescs  map[string]string `json:"skillDescs,omitempty"`
	Version     string            `json:"version,omitempty"`
	File        string            `json:"file"`
	Sha256      string            `json:"sha256"`
	Size        int64             `json:"size,omitempty"`
	DownloadURL string            `json:"download_url,omitempty"`
	// 市场信号（§1/§2：下载计数与评分聚合，分页契约只增不破坏）
	DownloadCnt int64   `json:"download_count,omitempty"`
	RatingAvg   float64 `json:"rating_avg,omitempty"`
	RatingCnt   int64   `json:"rating_count,omitempty"`
	// 插件元数据（kind=plugin；安装确认弹窗与插件页卡片的信任呈现依据）
	Author      string   `json:"author,omitempty"`
	URL         string   `json:"url,omitempty"`
	Repo        string   `json:"repo,omitempty"`
	License     string   `json:"license,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	Fingerprint string   `json:"fingerprint,omitempty"`
	PublishedAt string   `json:"published_at,omitempty"`
	// 衍生谱系（§4）：卡片「进化自 XXX」小字（原创不透出）
	DerivedFrom *bundle.DerivedRef `json:"derived_from,omitempty"`
}

// handleMarketList 市场浏览：全部走服务端分页 API（清单机制已废除，
// 无参请求视为第一页默认页大小）。响应恒为分页形态 {packages,total,page,source}。
// org_slug 非空时浏览该组织的专用源（仅本人在册组织，服务端权限闸门校验）。
func (d *Daemon) handleMarketList(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MarketListParams
	if params != nil {
		_ = json.Unmarshal(params, &p) // 参数解析失败按空参处理
	}

	q := bundle.MarketQuery{Page: p.Page, PageSize: p.PageSize, Kind: p.Kind, Q: p.Q, Sort: p.Sort}
	orgSlug := strings.TrimSpace(p.OrgSlug)
	var res *bundle.MarketPage
	var err error
	if orgSlug != "" {
		res, err = d.app.Market().ListOrgPackages(orgSlug, q)
	} else {
		res, err = d.app.Market().ListPage(q)
	}
	if err != nil {
		return nil, err
	}
	packages := res.Packages
	if packages == nil {
		packages = []bundle.MarketPackage{}
	}
	dtos := make([]marketPackageDTO, 0, len(packages))
	for _, p := range packages {
		dtos = append(dtos, marketPackageDTO{
			Kind:        string(p.Kind),
			Name:        p.Name,
			Title:       p.Title,
			Description: p.Description,
			Icon:        p.Icon,
			NickName:    p.NickName,
			Role:        p.Role,
			Category:    p.Category,
			Leader:      p.Leader,
			Members:     p.Members,
			Skills:      p.Skills,
			SkillNames:  p.SkillNames,
			SkillDescs:  p.SkillDescs,
			Version:     p.Version,
			File:        p.File,
			Sha256:      p.Sha256,
			Size:        p.Size,
			DownloadURL: p.DownloadURL,
			DownloadCnt: p.DownloadCount,
			RatingAvg:   p.RatingAvg,
			RatingCnt:   p.RatingCount,
			Author:      p.Author,
			URL:         p.URL,
			Repo:        p.Repo,
			License:     p.License,
			Permissions: p.Permissions,
			Fingerprint: p.Fingerprint,
			PublishedAt: p.PublishedAt,
			DerivedFrom: p.DerivedFrom,
		})
	}
	return map[string]any{
		"packages": dtos,
		"total":    res.Total,
		"page":     res.Page,
		"source":   res.Source,
		"org_slug": orgSlug,
		"warning":  "",
	}, nil
}

// handleMarketOrgs 当前账号所在组织清单（M7 组织源）：发布目标选择与货架切换的数据源。
// 未绑定账号报错提示先绑定；已在册组织含冻结态（前端置灰不可选）。
func (d *Daemon) handleMarketOrgs(_ context.Context, _ json.RawMessage) (any, error) {
	orgs, err := d.app.Market().MeOrgs()
	if err != nil {
		return nil, err
	}
	if orgs == nil {
		orgs = []bundle.MarketOrg{}
	}
	return map[string]any{"orgs": orgs}, nil
}

// handleMarketPackageRead 读取市场技能包的 SKILL.md 原文（详情预览用，不安装不落库）。
// 包文件经 Market.Download 落缓存并做 sha256 校验，重复预览命中缓存不重复下载。
func (d *Daemon) handleMarketPackageRead(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MarketPackageReadParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	// 单包定位：服务端详情 API 优先，清单回退（与 market.install 同口径；预览仅面向技能包）
	target, err := d.app.Market().PackageByKindName(string(bundle.KindSkill), p.Name)
	if err != nil {
		return nil, err
	}

	pkgPath, err := d.app.Market().Download(*target)
	if err != nil {
		return nil, err
	}
	content, err := bundle.ReadSkillDoc(pkgPath, p.Name)
	if err != nil {
		return nil, err
	}
	return map[string]any{"name": p.Name, "content": content}, nil
}

// handleMarketInstall 从市场下载分发包（sha256 校验）并安装。
// 安装即触发准备流程（requires.oauth 授权 / install 命令块）；skip_deps 跳过
// 准备；auto_install 时安装命令免确认直接执行（CLI --auto 契约）。
func (d *Daemon) handleMarketInstall(ctx context.Context, params json.RawMessage) (any, error) {
	var p rpc.MarketInstallParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" || (p.Kind != "agent" && p.Kind != "skill" && p.Kind != "team") {
		return nil, fmt.Errorf("kind（agent/skill/team）与 name 均为必填")
	}
	t0 := time.Now()

	// 单包定位：服务端详情 API 优先（download_url 直连），全量清单回退（过渡兼容）；
	// org_slug 非空时定位该组织专用源中的包（仅本人在册组织可见）
	orgSlug := strings.TrimSpace(p.OrgSlug)
	var target *bundle.MarketPackage
	var err error
	if orgSlug != "" {
		target, err = d.app.Market().OrgPackageByKindName(orgSlug, p.Kind, p.Name)
	} else {
		target, err = d.app.Market().PackageByKindName(p.Kind, p.Name)
	}
	if err != nil {
		return nil, err
	}
	tDetail := time.Since(t0)

	pkgPath, err := d.app.Market().Download(*target)
	if err != nil {
		return nil, err
	}
	tDownload := time.Since(t0)

	installOpts := bundle.InstallOptions{
		GlobalDir: d.app.Skills().GlobalDir(),
		TeamsDir:  d.app.Teams().Dir(),
		// 在线安装首次落地不做覆盖；目标已存在时由前端二次确认后携带 overwrite 重试
		Overwrite: p.Overwrite,
	}
	if bundle.PackageKind(p.Kind) == bundle.KindAgent || bundle.PackageKind(p.Kind) == bundle.KindTeam {
		installOpts.AgentsDir = d.app.Agents().Dir()
	}
	installed, err := bundle.Install(pkgPath, installOpts)
	if err != nil {
		return nil, err
	}
	tInstall := time.Since(t0)

	// 重载与失效逻辑与本地导入一致
	if installed.Kind == bundle.KindAgent || installed.Kind == bundle.KindTeam {
		if err := d.app.ReloadAgents(); err != nil {
			return nil, fmt.Errorf("分发包已落盘但智能体重载失败：%w", err)
		}
	}
	if installed.Kind == bundle.KindTeam {
		if err := d.app.ReloadTeams(); err != nil {
			return nil, fmt.Errorf("分发包已落盘但团队重载失败：%w", err)
		}
	}
	if err := d.app.ReloadSkills(); err != nil {
		return nil, fmt.Errorf("分发包已落盘但技能库重载失败：%w", err)
	}
	d.app.InvalidateRuntimes()

	d.logger.Info("市场分发包安装完成",
		"kind", p.Kind, "name", p.Name,
		"耗时", map[string]any{
			"定位":   tDetail.String(),
			"下载":   (tDownload - tDetail).String(),
			"解包安装": (tInstall - tDownload).String(),
			"重载失效": (time.Since(t0) - tInstall).String(),
		})
	// 安装即触发准备流程（agent 包携带的技能也落入全局库，一并准备）；
	// skip_deps 显式跳过准备；auto_install 时安装命令免确认直接执行（CLI --auto 契约）
	var prepares []skillPrepareResult
	if !p.SkipDeps {
		if p.AutoInstall {
			prepares = d.prepareAndAutoInstall(ctx, installed.SkillsGlobal)
		} else {
			prepares = d.prepareInstalledSkills(installed.SkillsGlobal)
		}
	}
	return installWithPrepare{InstallResult: *installed, Prepares: prepares}, nil
}

// ─── 发布与投递（市场生态 §3/§4）───

// marketBuildOverrides 组装发布覆盖项（publish/submit 共用）：谱系三字段须成对出现
// （derived_kind + derived_name），指向包自身的声明在打包校验中拒绝。
func marketBuildOverrides(version, license, derivedKind, derivedName, derivedVersion string) (bundle.PublishOverrides, error) {
	ov := bundle.PublishOverrides{Version: version, License: license}
	dk, dn := strings.TrimSpace(derivedKind), strings.TrimSpace(derivedName)
	if dk == "" && dn == "" {
		return ov, nil
	}
	if dk == "" || dn == "" {
		return ov, fmt.Errorf("衍生声明必须同时提供 derived_kind 与 derived_name")
	}
	ov.DerivedFrom = &bundle.DerivedRef{Kind: dk, Name: dn, Version: strings.TrimSpace(derivedVersion)}
	return ov, nil
}

// marketBuildPackage 按类型打包分发包字节（pack/publish/submit 共用打包入口）。
func (d *Daemon) marketBuildPackage(kind, name string, ov *bundle.PublishOverrides) ([]byte, *bundle.Manifest, []string, error) {
	switch bundle.PackageKind(kind) {
	case bundle.KindAgent:
		agent := d.app.Agents().Get(name)
		if agent == nil {
			return nil, nil, nil, fmt.Errorf("智能体 %q 不存在", name)
		}
		return bundle.BuildAgentBytes(agent, d.app.Skills(), ov)
	case bundle.KindTeam:
		team := d.app.Teams().Get(name)
		if team == nil {
			return nil, nil, nil, fmt.Errorf("团队 %q 不存在", name)
		}
		return bundle.BuildTeamBytes(team, d.app.Agents(), d.app.Skills(), ov)
	case bundle.KindSkill:
		data, manifest, err := bundle.BuildSkillBytes(d.app.Skills(), name, ov)
		if err != nil {
			return nil, nil, nil, err
		}
		return data, manifest, nil, nil
	default:
		return nil, nil, nil, fmt.Errorf("kind（agent/skill/team）非法")
	}
}

// handleMarketPack 打包分发包（不入库）：返回清单摘要与包大小/内容摘要，
// 供发布表单预填（包名、描述、分类、技能装配与默认体积）。
func (d *Daemon) handleMarketPack(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MarketPackParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	data, manifest, _, err := d.marketBuildPackage(p.Kind, p.Name, nil)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	resp := map[string]any{
		"kind":        p.Kind,
		"name":        manifest.Name,
		"description": manifest.Description,
		"category":    manifest.Category,
		"skills":      manifest.Skills,
		"members":     manifest.Members,
		"size":        len(data),
		"sha256":      hex.EncodeToString(sum[:]),
	}
	if manifest.Version != "" {
		resp["version"] = manifest.Version
	}
	if len(manifest.SkillNames) > 0 {
		resp["skill_names"] = manifest.SkillNames
	}
	return resp, nil
}

// handleMarketPublish 打包并发布为新版本（§3）：版本/许可/谱系声明写入包内清单后
// 字节直传服务端（服务端复算 sha256 并做衍生谱系权威校验：存在性、不自指、防环）。
// org_slug 非空时发布至该组织的专用源（仅本人在册组织，全体成员可发布）。
func (d *Daemon) handleMarketPublish(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MarketPublishParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if strings.TrimSpace(p.Version) == "" {
		return nil, fmt.Errorf("version 为必填")
	}
	ov, err := marketBuildOverrides(p.Version, p.License, p.DerivedKind, p.DerivedName, p.DerivedVersion)
	if err != nil {
		return nil, err
	}
	// 发布名覆盖（§4.2 衍生改名发布）：403 包名冲突后以新名再发布时携带；
	// 包内清单 name 随覆盖值落盘（zip 内固定目录前缀不含包名，安装端按清单落位）
	publishName := strings.TrimSpace(p.PublishName)
	if publishName != "" {
		ov.Name = publishName
	}
	data, _, warnings, err := d.marketBuildPackage(p.Kind, p.Name, &ov)
	if err != nil {
		return nil, err
	}
	for _, w := range warnings {
		d.logger.Warn("发布打包告警", "kind", p.Kind, "name", p.Name, "warning", w)
	}
	// 目标源分派：org_slug 非空 = 组织专用源；空 = 公共资源社区（存量行为不变）
	orgSlug := strings.TrimSpace(p.OrgSlug)
	var res *bundle.PublishResult
	if orgSlug != "" {
		res, err = d.app.Market().PublishToOrg(orgSlug, p.Kind, p.Name, data, p.Version, publishName)
	} else {
		res, err = d.app.Market().Publish(p.Kind, p.Name, data, p.Version, publishName)
	}
	if err != nil {
		return nil, err
	}
	d.logger.Info("分发包发布完成", "kind", p.Kind, "name", p.Name, "version", res.Version, "created", res.Created, "org", orgSlug)
	return map[string]any{
		"package_id": res.PackageID,
		"version":    res.Version,
		"sha256":     res.Sha256,
		"created":    res.Created,
		"warnings":   warnings,
	}, nil
}

// handleMarketUnpublish 作者下架自己包的某版本（服务端软删，仅包主可操作）。
// kind/name/version 三要素必填；组织源版本下架服务端暂无端点，不支持 org 维度。
func (d *Daemon) handleMarketUnpublish(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MarketUnpublishParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if strings.TrimSpace(p.Kind) == "" || strings.TrimSpace(p.Name) == "" {
		return nil, fmt.Errorf("kind 与 name 为必填")
	}
	if strings.TrimSpace(p.Version) == "" {
		return nil, fmt.Errorf("version 为必填")
	}
	if err := d.app.Market().DeleteVersion(p.Kind, p.Name, p.Version); err != nil {
		return nil, err
	}
	d.logger.Info("分发包版本下架完成", "kind", p.Kind, "name", p.Name, "version", p.Version)
	return map[string]any{"deleted": p.Version}, nil
}

// handleMarketSubmit 投递进化版本到包主收件箱（§4 回流路径）：投递目标 = 原包名，
// 打包清单身份与目标天然一致；版本号必填（服务端校验），谱系声明可选透传。
func (d *Daemon) handleMarketSubmit(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MarketSubmitParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if strings.TrimSpace(p.Version) == "" {
		return nil, fmt.Errorf("version 为必填")
	}
	ov, err := marketBuildOverrides(p.Version, "", p.DerivedKind, p.DerivedName, p.DerivedVersion)
	if err != nil {
		return nil, err
	}
	// 投递 = 同名包的进化版：谱系指向投递目标自身无意义且服务端采纳时必拒，前置拦截
	if ov.DerivedFrom != nil && ov.DerivedFrom.Kind == p.Kind && ov.DerivedFrom.Name == p.Name {
		return nil, fmt.Errorf("投递清单的谱系声明不能指向投递目标自身")
	}
	data, _, warnings, err := d.marketBuildPackage(p.Kind, p.Name, &ov)
	if err != nil {
		return nil, err
	}
	for _, w := range warnings {
		d.logger.Warn("投递打包告警", "kind", p.Kind, "name", p.Name, "warning", w)
	}
	res, err := d.app.Market().Submit(p.Kind, p.Name, p.Note, p.Version, data)
	if err != nil {
		return nil, err
	}
	d.logger.Info("进化版本投递完成", "kind", p.Kind, "name", p.Name, "version", res.Version, "submission_id", res.SubmissionID)
	return map[string]any{
		"submission_id": res.SubmissionID,
		"version":       res.Version,
		"sha256":        res.Sha256,
		"created":       res.Created,
	}, nil
}

// handleMarketSubmissionsList 拉取名下包的投递收件箱（仅包主；服务端校验 owner）。
func (d *Daemon) handleMarketSubmissionsList(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MarketSubmissionsListParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Kind == "" || p.Name == "" {
		return nil, fmt.Errorf("kind 与 name 均为必填")
	}
	subs, err := d.app.Market().Submissions(p.Kind, p.Name)
	if err != nil {
		return nil, err
	}
	if subs == nil {
		subs = []bundle.Submission{}
	}
	return map[string]any{"submissions": subs}, nil
}

// handleMarketSubmissionDecide 包主审核投递：采纳（服务端代转正式发布管线）或拒绝。
// 重复决策由服务端以 409 invalid_state 拒绝，错误原样冒泡。
func (d *Daemon) handleMarketSubmissionDecide(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MarketSubmissionDecideParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Kind == "" || p.Name == "" || p.ID <= 0 {
		return nil, fmt.Errorf("kind、name 与 id 均为必填")
	}
	res, err := d.app.Market().DecideSubmission(p.Kind, p.Name, p.ID, p.Accept)
	if err != nil {
		return nil, err
	}
	if p.Accept {
		d.logger.Info("投递已采纳并发布", "kind", p.Kind, "name", p.Name, "submission_id", p.ID, "version", res.Version)
	} else {
		d.logger.Info("投递已拒绝", "kind", p.Kind, "name", p.Name, "submission_id", p.ID)
	}
	return res, nil
}
