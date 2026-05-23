package svc

import (
	"context"
	"encoding/json"
	"fmt"

	goreactcore "github.com/DotNetAge/goreact/core"
)

func (d *Daemon) handleAgentList(_ context.Context, params json.RawMessage) (any, error) {
	agents := d.app.Agents()
	if agents == nil {
		return []goreactcore.AgentConfig{}, nil
	}
	list := agents.List()
	if list == nil {
		return []goreactcore.AgentConfig{}, nil
	}

	type agentEntry struct {
		Name         string         `json:"name"`
		Role         string         `json:"role,omitempty"`
		Description  string         `json:"description"`
		Introduction string         `json:"introduction,omitempty"`
		Model        string         `json:"model"`
		Skills       []string       `json:"skills,omitempty"`
		Body         string         `json:"body,omitempty"`
		Meta         map[string]any `json:"meta,omitempty"`
	}

	result := make([]agentEntry, len(list))
	for i, a := range list {
		result[i] = agentEntry{
			Name:         a.Name,
			Role:         a.Role,
			Description:  a.Description,
			Introduction: a.Introduction,
			Model:        a.Model,
			Skills:       a.Skills,
			Body:         a.Body,
			Meta:         a.Meta,
		}
	}
	return result, nil
}

type agentGetParams struct {
	Name string `json:"name"`
}

func (d *Daemon) handleAgentGet(_ context.Context, params json.RawMessage) (any, error) {
	var p agentGetParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	agents := d.app.Agents()
	if agents == nil {
		return nil, fmt.Errorf("agent registry not available")
	}

	cfg := agents.Get(p.Name)
	if cfg == nil {
		return nil, fmt.Errorf("agent %q not found", p.Name)
	}

	return cfg, nil
}

type agentCreateParams struct {
	Name         string         `json:"name"`
	Role         string         `json:"role"`
	Description  string         `json:"description"`
	Introduction string         `json:"introduction,omitempty"`
	Model        string         `json:"model"`
	Skills       []string       `json:"skills,omitempty"`
	Body         string         `json:"body"`
	Meta         map[string]any `json:"meta,omitempty"`
}

func (d *Daemon) handleAgentCreate(_ context.Context, params json.RawMessage) (any, error) {
	var p agentCreateParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if p.Role == "" {
		return nil, fmt.Errorf("role is required")
	}
	if p.Description == "" {
		return nil, fmt.Errorf("description is required")
	}
	if p.Model == "" {
		return nil, fmt.Errorf("model is required")
	}
	if p.Body == "" {
		return nil, fmt.Errorf("body is required")
	}

	agents := d.app.Agents()
	if agents == nil {
		return nil, fmt.Errorf("agent registry not available")
	}

	existing := agents.Get(p.Name)
	if existing != nil {
		return nil, fmt.Errorf("agent %q already exists", p.Name)
	}

	newAgent := goreactcore.AgentConfig{
		Name:         p.Name,
		Role:         p.Role,
		Description:  p.Description,
		Introduction: p.Introduction,
		Model:        p.Model,
		Skills:       p.Skills,
		Body:         p.Body,
		Meta:         p.Meta,
	}

	if err := agents.SaveTo(&newAgent); err != nil {
		return nil, fmt.Errorf("failed to create agent config: %w", err)
	}

	return map[string]string{
		"status":     "ok",
		"agent_name": newAgent.Name,
		"message":    "agent created successfully",
	}, nil
}

type agentUpdateParams struct {
	Name         string         `json:"name"`
	Role         string         `json:"role,omitempty"`
	Description  string         `json:"description,omitempty"`
	Introduction string         `json:"introduction,omitempty"`
	Model        string         `json:"model,omitempty"`
	Skills       []string       `json:"skills,omitempty"`
	Body         string         `json:"body,omitempty"`
	Meta         map[string]any `json:"meta,omitempty"`
}

func (d *Daemon) handleAgentUpdate(_ context.Context, params json.RawMessage) (any, error) {
	var p agentUpdateParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	agents := d.app.Agents()
	if agents == nil {
		return nil, fmt.Errorf("agent registry not available")
	}

	existing := agents.Get(p.Name)
	if existing == nil {
		return nil, fmt.Errorf("agent %q not found", p.Name)
	}

	updated := *existing

	if p.Role != "" {
		updated.Role = p.Role
	}
	if p.Description != "" {
		updated.Description = p.Description
	}
	if p.Model != "" {
		updated.Model = p.Model
	}
	if p.Skills != nil {
		updated.Skills = p.Skills
	}
	if p.Introduction != "" {
		updated.Introduction = p.Introduction
	}
	if p.Body != "" {
		updated.Body = p.Body
	}
	if p.Meta != nil {
		updated.Meta = p.Meta
	}

	if err := agents.SaveTo(&updated); err != nil {
		return nil, fmt.Errorf("failed to save agent config: %w", err)
	}

	return map[string]string{
		"status":     "ok",
		"agent_name": updated.Name,
		"message":    "agent config updated",
	}, nil
}
