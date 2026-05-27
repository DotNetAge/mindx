package svc

import (
	"context"
	"encoding/json"
	"fmt"

)

type skillListParams struct {
	AgentName string `json:"agent_name,omitempty"`
}

func (d *Daemon) handleSkillList(_ context.Context, params json.RawMessage) (any, error) {
	var p skillListParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}

	skills := d.app.SkillRegistry().ListSkills()

	type skillEntry struct {
		Name         string            `json:"name"`
		Description  string            `json:"description"`
		RootDir      string            `json:"root_dir,omitempty"`
		Source       string            `json:"source,omitempty"`
		Instructions string            `json:"instructions,omitempty"`
		Paths        []string          `json:"paths,omitempty"`
		Metadata     map[string]string `json:"metadata,omitempty"`
	}

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
		}
	}
	return result, nil
}

type skillGetParams struct {
	Name      string `json:"name"`
	AgentName string `json:"agent_name,omitempty"`
}

func (d *Daemon) handleSkillGet(_ context.Context, params json.RawMessage) (any, error) {
	var p skillGetParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	sk, err := d.app.SkillRegistry().GetSkill(p.Name)
	if err != nil {
		return nil, fmt.Errorf("skill %q not found: %w", p.Name, err)
	}

	return sk, nil
}
