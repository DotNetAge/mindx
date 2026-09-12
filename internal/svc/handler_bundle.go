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
	Role        string            `json:"role,omitempty"`
	Category    string            `json:"category,omitempty"`
	Skills      []string          `json:"skills,omitempty"`
	SkillNames  map[string]string `json:"skillNames,omitempty"`
	SkillDescs  map[string]string `json:"skillDescs,omitempty"`
	Version     string            `json:"version,omitempty"`
	File        string            `json:"file"`
	Sha256      string            `json:"sha256"`
	Size        int64             `json:"size,omitempty"`
}

// handleMarketList 拉取市场清单（在线优先，失败降级本地缓存并附原因）。
func (d *Daemon) handleMarketList(_ context.Context, _ json.RawMessage) (any, error) {
	res, err := d.app.Market().List()
	if err != nil {
		return nil, err
	}
	packages := res.Manifest.Packages
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
			Role:        p.Role,
			Category:    p.Category,
			Skills:      p.Skills,
			SkillNames:  p.SkillNames,
			SkillDescs:  p.SkillDescs,
			Version:     p.Version,
			File:        p.File,
			Sha256:      p.Sha256,
			Size:        p.Size,
		})
	}
	return map[string]any{
		"packages":   dtos,
		"source":     res.Source,
		"warning":    res.Warning,
		"updated_at": res.Manifest.UpdatedAt,
	}, nil
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

	// 清单中定位目标技能包（与 market.install 同口径；详情预览仅面向技能包）
	res, err := d.app.Market().List()
	if err != nil {
		return nil, err
	}
	var target *bundle.MarketPackage
	for i := range res.Manifest.Packages {
		pkg := &res.Manifest.Packages[i]
		if pkg.Kind == bundle.KindSkill && pkg.Name == p.Name {
			target = pkg
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("市场中不存在技能包 %q", p.Name)
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
func (d *Daemon) handleMarketInstall(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MarketInstallParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" || (p.Kind != "agent" && p.Kind != "skill") {
		return nil, fmt.Errorf("kind（agent/skill）与 name 均为必填")
	}

	// 清单中定位目标包（同时为下载与安装提供元数据）
	res, err := d.app.Market().List()
	if err != nil {
		return nil, err
	}
	var target *bundle.MarketPackage
	for i := range res.Manifest.Packages {
		pkg := &res.Manifest.Packages[i]
		if string(pkg.Kind) == p.Kind && pkg.Name == p.Name {
			target = pkg
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("市场中不存在 %s 类型包 %q", p.Kind, p.Name)
	}

	pkgPath, err := d.app.Market().Download(*target)
	if err != nil {
		return nil, err
	}

	installOpts := bundle.InstallOptions{
		GlobalDir: d.app.Skills().GlobalDir(),
		// 在线安装首次落地不做覆盖；目标已存在时由前端二次确认后携带 overwrite 重试
		Overwrite: p.Overwrite,
	}
	if bundle.PackageKind(p.Kind) == bundle.KindAgent {
		installOpts.AgentsDir = d.app.Agents().Dir()
	}
	installed, err := bundle.Install(pkgPath, installOpts)
	if err != nil {
		return nil, err
	}

	// 重载与失效逻辑与本地导入一致
	if installed.Kind == bundle.KindAgent {
		if err := d.app.ReloadAgents(); err != nil {
			return nil, fmt.Errorf("分发包已落盘但智能体重载失败：%w", err)
		}
	}
	if err := d.app.ReloadSkills(); err != nil {
		return nil, fmt.Errorf("分发包已落盘但技能库重载失败：%w", err)
	}
	d.app.InvalidateRuntimes()

	d.logger.Info("市场分发包安装完成", "kind", p.Kind, "name", p.Name)
	return installed, nil
}
