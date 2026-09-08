package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/DotNetAge/mindx/pkg/rpc"
)

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
