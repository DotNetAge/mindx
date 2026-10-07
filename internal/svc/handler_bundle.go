package svc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/mindx/internal/core/bundle"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

// 本文件实现分发包安装导出闭环与 COS 静态市场的 RPC（PR-PROMPTS 第四节）：
//
//   - agent.export / skill.export：导出分发包（zip 容器，保真原则）；
//   - agent.import / skill.import：本地安装（同名冲突默认拒绝，覆盖需显式确认）；
//   - market.list：在线清单拉取（失败降级本地缓存并附原因）；
//   - market.install：下载 → sha256 校验 → 复用本地安装展开逻辑。

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
// JSON tag 为 snake_case（对齐 COS 清单格式），而前端契约（MarketPackageInfo）
// 为 camelCase，此处逐字段转换对齐（skillNames 等映射字段的前端读取键）。
type marketPackageDTO struct {
	Kind        string            `json:"kind"`
	Name        string            `json:"name"`
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
	DownloadURL string            `json:"downloadUrl,omitempty"`
}

// handleMarketList 市场浏览：带分页/过滤参数走服务端分页 API（主入口），
// 空参保持存量全量清单语义（过渡兼容，壳侧逐个切换期间不断供）。
func (d *Daemon) handleMarketList(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MarketListParams
	if params != nil {
		_ = json.Unmarshal(params, &p) // 参数解析失败按空参处理（兼容存量调用方）
	}
	paged := p.Page > 0 || p.PageSize > 0 || p.Q != "" || p.Kind != "" || p.Sort != ""

	var (
		packages  []bundle.MarketPackage
		source    string
		warning   string
		total     int
		page      int
		updatedAt string
	)
	if paged {
		res, err := d.app.Market().ListPage(bundle.MarketQuery{
			Page: p.Page, PageSize: p.PageSize, Kind: p.Kind, Q: p.Q, Sort: p.Sort,
		})
		if err != nil {
			return nil, err
		}
		packages, source, warning, total, page = res.Packages, res.Source, res.Warning, res.Total, res.Page
	} else {
		res, err := d.app.Market().List()
		if err != nil {
			return nil, err
		}
		packages, source, warning = res.Manifest.Packages, res.Source, res.Warning
		updatedAt = res.Manifest.UpdatedAt
	}
	if packages == nil {
		packages = []bundle.MarketPackage{}
	}
	dtos := make([]marketPackageDTO, 0, len(packages))
	for _, p := range packages {
		dtos = append(dtos, marketPackageDTO{
			Kind:        string(p.Kind),
			Name:        p.Name,
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
		})
	}
	resp := map[string]any{
		"packages": dtos,
		"source":   source,
		"warning":  warning,
	}
	if paged {
		resp["total"] = total
		resp["page"] = page
	} else {
		resp["updated_at"] = updatedAt
	}
	return resp, nil
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

	// 单包定位：服务端详情 API 优先（download_url 直连），全量清单回退（过渡兼容）
	target, err := d.app.Market().PackageByKindName(p.Kind, p.Name)
	if err != nil {
		return nil, err
	}

	pkgPath, err := d.app.Market().Download(*target)
	if err != nil {
		return nil, err
	}

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

	d.logger.Info("市场分发包安装完成", "kind", p.Kind, "name", p.Name)
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
