package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/DotNetAge/mindx/internal/core/teamstore"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

// teamDetailResult 是 team.get / team.list 的返回投影：Meta 全量字段内联展开，
// 附加 Profile 正文（团队人设，三端消费冻结口径之一部分）。
type teamDetailResult struct {
	teamstore.TeamMeta
	Profile string `json:"profile"`
}

func (d *Daemon) handleTeamList(_ context.Context, _ json.RawMessage) (any, error) {
	teams := d.app.Teams()
	if teams == nil {
		return []teamDetailResult{}, nil
	}
	list := teams.List()
	result := make([]teamDetailResult, 0, len(list))
	for _, t := range list {
		result = append(result, teamDetailResult{TeamMeta: t.Meta, Profile: t.Profile})
	}
	return result, nil
}

func (d *Daemon) handleTeamGet(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.TeamGetParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	team := d.app.Teams().Get(p.Name)
	if team == nil {
		return nil, fmt.Errorf("team %q not found", p.Name)
	}
	return teamDetailResult{TeamMeta: team.Meta, Profile: team.Profile}, nil
}

func (d *Daemon) handleTeamUpdate(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.TeamUpdateParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	teams := d.app.Teams()

	// Leader 非空校验（新建或换负责人）：团队会话 resolve 到 Leader，缺省无意义
	existing := teams.Get(p.Name)
	leader := ""
	if existing != nil {
		leader = existing.Meta.Leader
	}
	if p.Leader != nil {
		leader = strings.TrimSpace(*p.Leader)
	}
	if strings.TrimSpace(leader) == "" {
		return nil, fmt.Errorf("leader is required（团队必须任命负责人，团队会话 resolve 到该 Agent）")
	}

	// Leader 必须指向存在的 Agent（引用有效性校验；成员列表为纯引用，允许先建队后补成员）
	if agents := d.app.Agents(); agents != nil && agents.Get(leader) == nil {
		return nil, fmt.Errorf("未找到智能体 %q，请确认负责人名称是否正确", leader)
	}

	updated := teamstore.Team{Meta: teamstore.TeamMeta{Name: p.Name}}
	if existing != nil {
		updated = *existing
	}
	if p.NickName != nil {
		updated.Meta.NickName = strings.TrimSpace(*p.NickName)
	}
	if p.Description != nil {
		updated.Meta.Description = *p.Description
	}
	if p.Leader != nil {
		updated.Meta.Leader = strings.TrimSpace(*p.Leader)
	}
	if p.Members != nil {
		updated.Meta.Members = p.Members
	}
	if p.Icon != nil {
		updated.Meta.Icon = strings.TrimSpace(*p.Icon)
	}
	if p.Category != nil {
		updated.Meta.Category = strings.TrimSpace(*p.Category)
	}
	if p.Profile != nil {
		updated.Profile = strings.TrimSpace(*p.Profile)
	}
	if p.Hired != nil {
		updated.Meta.Hired = *p.Hired
	}
	updated.Meta.Leader = leader

	if err := teams.Save(&updated); err != nil {
		return nil, fmt.Errorf("failed to save team: %w", err)
	}

	return map[string]string{
		"status":    "ok",
		"team_name": updated.Meta.Name,
		"message":   "team config updated",
	}, nil
}

func (d *Daemon) handleTeamDelete(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.TeamDeleteParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if err := d.app.Teams().Remove(p.Name); err != nil {
		return nil, err
	}
	// 仅删团队本身：被引用的 Agent 不动；已存在的团队会话绑定悬空由注入侧
	// 容错降级（不注入团队段），日志告警由此处补充
	d.logger.Warn("team deleted: existing team sessions will degrade to plain agent sessions", "team", p.Name)
	return map[string]string{"status": "ok", "message": "team deleted"}, nil
}

func (d *Daemon) handleTeamHire(_ context.Context, params json.RawMessage) (any, error) {
	return d.setTeamHired(params, true)
}

func (d *Daemon) handleTeamFire(_ context.Context, params json.RawMessage) (any, error) {
	return d.setTeamHired(params, false)
}

func (d *Daemon) setTeamHired(params json.RawMessage, hired bool) (any, error) {
	var p rpc.TeamHireParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if err := d.app.Teams().SetHired(p.Name, hired); err != nil {
		return nil, err
	}
	action := "hired"
	if !hired {
		action = "fired"
	}
	return map[string]string{
		"status":    "ok",
		"team_name": p.Name,
		"message":   "team " + action,
	}, nil
}
