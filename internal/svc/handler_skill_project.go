package svc

import (
	"context"
	"encoding/json"
	"fmt"

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
	Loaded   bool           `json:"loaded,omitempty"` // 仅项目级技能使用：是否已确认载入当前会话
}

// projectSkillEntry 是项目级技能发现结果的精简投影（不含指令正文）。
type projectSkillEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	RootDir     string `json:"root_dir,omitempty"`
	Loaded      bool   `json:"loaded"`
}

// sessionProjectDir 读取会话的项目目录；会话不存在或未记录项目目录时返回空串。
func (d *Daemon) sessionProjectDir(sessionID string) string {
	if d.app.SessDB() == nil || sessionID == "" {
		return ""
	}
	meta, err := d.app.SessDB().GetMeta(context.Background(), sessionID)
	if err != nil || meta == nil {
		return ""
	}
	return meta.ProjectDir
}

// projectOverlayFor 返回会话已确认载入的项目技能覆盖注册表；未载入返回 nil。
func (d *Daemon) projectOverlayFor(sessionID string) *skillstore.Registry {
	if v, ok := d.projectSkills.Load(sessionID); ok {
		if reg, ok := v.(*skillstore.Registry); ok {
			return reg
		}
	}
	return nil
}

// handleSkillList 列举技能库。作用域由参数决定（PR-PROMPTS 三级库）：
//   - ProjectDir 非空 → 项目级库（发现式扫描，Loaded 标记会话内已载入状态）；
//   - AgentName 非空 → Agent 级私有库；
//   - 两者均为空 → 全局库。
func (d *Daemon) handleSkillList(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.SkillListParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}

	skills := d.app.Skills()

	// 项目级：发现式扫描，附带会话内载入状态
	if p.ProjectDir != "" {
		reg, errs := skills.DiscoverProject(p.ProjectDir)
		sessionID := p.SessionID
		overlay := d.projectOverlayFor(sessionID)
		loaded := make(map[string]bool)
		if overlay != nil {
			for _, sk := range overlay.List() {
				loaded[sk.Name] = true
			}
		}
		result := make([]projectSkillEntry, 0, len(reg.List()))
		for _, sk := range reg.List() {
			result = append(result, projectSkillEntry{
				Name:        sk.Name,
				Description: sk.Description,
				RootDir:     sk.RootDir,
				Loaded:      loaded[sk.Name],
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

// handleSkillProjectLoad 将项目级技能批量确认载入会话（一次性挂载）。
// names 为空表示全部载入；载入结果仅对当前会话生效（会话覆盖注册表），
// 不写入任何持久化配置，daemon 重启后由前端重新发现确认。
func (d *Daemon) handleSkillProjectLoad(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.SkillProjectLoadParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	projectDir := d.sessionProjectDir(p.SessionID)
	if projectDir == "" {
		return nil, fmt.Errorf("会话 %q 未关联项目目录，无法载入项目技能", p.SessionID)
	}

	reg, errs := d.app.Skills().DiscoverProject(projectDir)
	if len(errs) > 0 {
		warnings := make([]string, 0, len(errs))
		for _, err := range errs {
			warnings = append(warnings, err.Error())
		}
		d.logger.Warn("项目技能发现存在无法装载的技能", "session_id", p.SessionID, "warnings", warnings)
	}

	available := reg.List()

	// names 为空 → 全部载入（批量确认语义）；否则按名单过滤
	var selected []*skill.Skill
	wanted := make(map[string]bool, len(p.Names))
	for _, n := range p.Names {
		wanted[n] = true
	}
	for _, sk := range available {
		if len(wanted) == 0 || wanted[sk.Name] {
			selected = append(selected, sk)
			delete(wanted, sk.Name)
		}
	}
	if len(wanted) > 0 {
		missing := make([]string, 0, len(wanted))
		for n := range wanted {
			missing = append(missing, n)
		}
		return nil, fmt.Errorf("项目技能库中不存在以下技能：%v", missing)
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("项目 %s 下没有可载入的技能", projectDir)
	}

	// 组装会话覆盖注册表并记录（daemon 在每轮 Ask 重建会话时应用）
	overlay := skillstore.NewRegistry()
	loaded := make([]string, 0, len(selected))
	for _, sk := range selected {
		if err := overlay.RegisterSkill(sk); err != nil {
			return nil, fmt.Errorf("注册项目技能 %q 失败：%w", sk.Name, err)
		}
		loaded = append(loaded, sk.Name)
	}
	d.projectSkills.Store(p.SessionID, overlay)

	d.logger.Info("项目技能已载入会话", "session_id", p.SessionID, "project_dir", projectDir, "skills", loaded)
	return map[string]any{
		"session_id":  p.SessionID,
		"project_dir": projectDir,
		"loaded":      loaded,
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
