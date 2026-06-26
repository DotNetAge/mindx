package svc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/mindx/pkg/rpc"
)

func (d *Daemon) handleSkillList(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.SkillListParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}

	type skillEntry struct {
		Name         string            `json:"name"`
		Description  string            `json:"description"`
		RootDir      string            `json:"root_dir,omitempty"`
		Source       string            `json:"source,omitempty"`
		Instructions string            `json:"instructions,omitempty"`
		Paths        []string          `json:"paths,omitempty"`
		Metadata     map[string]string `json:"metadata,omitempty"`
		License      string            `json:"license,omitempty"`
	}

	skillReg := d.app.SkillRegistry()
	if skillReg == nil {
		return []skillEntry{}, nil
	}
	skills := skillReg.ListSkills()

	result := make([]skillEntry, len(skills))
	for i, s := range skills {
		result[i] = skillEntry{
			Name:         s.Name,
			Description:  s.Description,
			RootDir:      s.RootDir,
			Source:       s.Source,
			Instructions: s.Instructions,
			Paths:        s.Paths,
			Metadata:     s.Metadata,
			License:      s.License,
		}
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

	skillReg := d.app.SkillRegistry()
	if skillReg == nil {
		return nil, fmt.Errorf("skill registry not available")
	}

	sk, err := skillReg.GetSkill(p.Name)
	if err != nil {
		return nil, fmt.Errorf("skill %q not found: %w", p.Name, err)
	}

	return sk, nil
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
