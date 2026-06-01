package svc

import (
	"context"
	"encoding/json"
	"fmt"

	goreactconfig "github.com/DotNetAge/goreact/config"
)

func (d *Daemon) handleModelList(_ context.Context, _ json.RawMessage) (any, error) {
	models := d.app.Models()
	if models == nil {
		return []goreactconfig.ModelConfig{}, nil
	}
	list := models.List()
	if list == nil {
		return []goreactconfig.ModelConfig{}, nil
	}
	return list, nil
}

type modelGetParams struct {
	Name string `json:"name"`
}

func (d *Daemon) handleModelGet(_ context.Context, params json.RawMessage) (any, error) {
	var p modelGetParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	models := d.app.Models()
	if models == nil {
		return nil, fmt.Errorf("model registry not available")
	}

	cfg := models.Get(p.Name)
	if cfg == nil {
		return nil, fmt.Errorf("model %q not found", p.Name)
	}

	return cfg, nil
}

type modelSwitchParams struct {
	Name     string `json:"name"`
	Provider string `json:"provider,omitempty"`
}

func (d *Daemon) handleModelSwitch(_ context.Context, params json.RawMessage) (any, error) {
	var p modelSwitchParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	models := d.app.Models()
	if models == nil {
		return nil, fmt.Errorf("model registry not available")
	}

	cfg := models.Get(p.Name)
	if cfg == nil {
		return nil, fmt.Errorf("model %q not found", p.Name)
	}

	d.app.Config().DefaultModel = p.Name
	if p.Provider != "" {
		d.app.Config().DefaultProvider = p.Provider
	}
	if err := d.app.Config().Save(); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	return map[string]any{
		"name":     p.Name,
		"provider": cfg.Provider,
		"message":  fmt.Sprintf("Switched to model %q", p.Name),
	}, nil
}
