package svc

import (
	"context"
	"encoding/json"
	"fmt"

	goreactcore "github.com/DotNetAge/goreact/core"
)

func (d *Daemon) handleModelList(_ context.Context, _ json.RawMessage) (any, error) {
	models := d.app.Models()
	if models == nil {
		return []goreactcore.ModelConfig{}, nil
	}
	list := models.List()
	if list == nil {
		return []goreactcore.ModelConfig{}, nil
	}
	return list, nil
}

type modelGetParams struct {
	Name string `json:"name"`
}

func (d *Daemon) handleModelGet(_ context.Context, params json.RawMessage) (any, error) {
	var p modelGetParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
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
