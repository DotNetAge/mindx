package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/DotNetAge/goharness/skill"
	"github.com/DotNetAge/mindx/internal/core/skillstore"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

// skillEntry 是 skill.* RPC 的返回结构（level 标识技能库层级）。
type skillEntry struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	RootDir      string `json:"root_dir,omitempty"`
	Source       string `json:"source,omitempty"`
	Instructions string `json:"instructions,omitempty"`
	Level        string `json:"level"`
	// Metadata frontmatter metadata 原始键值（name_zh / version 等展示字段；
	// 注册表存储的是瘦身 Skill 不含该字段，此处按 RootDir 轻量读取补齐）。
	Metadata map[string]any `json:"metadata,omitempty"`
}

// projectSkillEntry 是项目级技能发现结果的精简投影（不含指令正文）。
// 军规：项目动态技能绝不进入系统提示词；此列表仅供晋升界面展示，
// 载入由 Skill 工具按需回退解析（ResolveProject）。
type projectSkillEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	RootDir     string `json:"root_dir,omitempty"`
}

// handleSkillList 列举技能库。作用域由参数决定（PR-PROMPTS 三级库）：
//   - ProjectDir 非空 → 项目级库（发现式扫描，供晋升界面展示）；
//   - AgentName 非空 → Agent 级私有库；
//   - 两者均为空 → 全局库。
func (d *Daemon) handleSkillList(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.SkillListParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}

	skills := d.app.Skills()

	// 项目级：发现式扫描（仅供晋升界面展示，不挂载会话）
	if p.ProjectDir != "" {
		reg, errs := skills.DiscoverProject(p.ProjectDir)
		result := make([]projectSkillEntry, 0, len(reg.List()))
		for _, sk := range reg.List() {
			result = append(result, projectSkillEntry{
				Name:        sk.Name,
				Description: sk.Description,
				RootDir:     sk.RootDir,
			})
		}
		if len(errs) > 0 {
			warnings := make([]string, 0, len(errs))
			for _, err := range errs {
				warnings = append(warnings, err.Error())
			}
			return map[string]any{"skills": result, "warnings": warnings}, nil
		}
		return map[string]any{"skills": result}, nil
	}

	// Agent 级 / 全局级
	var list []*skill.Skill
	var level string
	if p.AgentName != "" {
		list = skills.AgentSkills(p.AgentName).List()
		level = string(skillstore.LevelAgent)
	} else {
		list = skills.Global().List()
		level = string(skillstore.LevelGlobal)
	}

	result := make([]skillEntry, 0, len(list))
	for _, sk := range list {
		result = append(result, skillEntry{
			Name:         sk.Name,
			Description:  sk.Description,
			RootDir:      sk.RootDir,
			Source:       sk.Source,
			Instructions: sk.Instructions,
			Level:        level,
			Metadata:     skillstore.LoadSkillMetadata(sk.RootDir),
		})
	}
	return result, nil
}

func (d *Daemon) handleSkillGet(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.SkillGetParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	sk, err := d.app.Skills().Global().GetSkill(p.Name)
	if err != nil {
		return nil, fmt.Errorf("skill %q not found: %w", p.Name, err)
	}

	return sk, nil
}

func (d *Daemon) handleSkillDelete(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.SkillDeleteParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	sk, err := d.app.Skills().Global().GetSkill(p.Name)
	if err != nil {
		return nil, fmt.Errorf("skill %q not found: %w", p.Name, err)
	}
	if sk.RootDir == "" {
		return nil, fmt.Errorf("skill %q has no filesystem root, cannot delete", p.Name)
	}

	// 删除技能目录（SKILL.md 与 references/scripts 资源），随后重载注册表
	if err := os.RemoveAll(sk.RootDir); err != nil {
		return nil, fmt.Errorf("failed to remove skill %q: %w", p.Name, err)
	}
	if err := d.app.ReloadSkills(); err != nil {
		return nil, fmt.Errorf("skill removed but reload failed: %w", err)
	}

	return map[string]string{
		"status":  "ok",
		"message": "skill " + p.Name + " deleted",
	}, nil
}

func (d *Daemon) handleSkillReload(_ context.Context, params json.RawMessage) (any, error) {
	if err := d.app.ReloadSkills(); err != nil {
		return nil, fmt.Errorf("skill reload failed: %w", err)
	}
	return map[string]string{
		"status":  "ok",
		"message": "skills reloaded successfully",
	}, nil
}

// handleSkillPromote 晋升技能（PR-PROMPTS 第三节：一切晋升由用户裁决）。
// 项目级 → Agent 级 / 全局级、Agent 级 → 全局级；晋升为纯复制搬运，
// 目标同名默认拒绝，覆盖需用户在界面确认（overwrite=true）。
func (d *Daemon) handleSkillPromote(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.SkillPromoteParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if p.From == "" || p.To == "" {
		return nil, fmt.Errorf("from 和 to 均为必填")
	}

	err := d.app.Skills().Promote(skillstore.PromoteOptions{
		SkillName:  p.Name,
		From:       skillstore.SkillLevel(p.From),
		To:         skillstore.SkillLevel(p.To),
		AgentName:  p.AgentName,
		ProjectDir: p.ProjectDir,
		Overwrite:  p.Overwrite,
	})
	if err != nil {
		return nil, err
	}

	// 晋升改变了全局/Agent 级库内容，运行时缓存中的技能快照已过期，
	// 全部失效后下一次 ResolveRuntime 按新库重新组装。
	d.app.InvalidateRuntimes()

	d.logger.Info("技能晋升完成", "name", p.Name, "from", p.From, "to", p.To)
	return map[string]string{
		"status":  "ok",
		"name":    p.Name,
		"from":    p.From,
		"to":      p.To,
		"message": "技能晋升完成",
	}, nil
}
