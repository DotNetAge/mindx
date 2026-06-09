package svc

import (
	"context"
	"encoding/json"
)

func (d *Daemon) handleUserConfig(_ context.Context, params json.RawMessage) (any, error) {
	cfg := d.app.Config()

	result := map[string]interface{}{
		"initialized":      cfg.Initialized,
		"last_agent":       cfg.LastAgent,
		"last_session_id":  cfg.LastSessionID,
		"default_model":    cfg.DefaultModel,
		"default_provider": cfg.DefaultProvider,
		"last_model":       cfg.LastModel,
		"embedder_model":   cfg.EmbedderModel,
	}

	if cfg.PermissionRules != nil {
		result["permission_rules"] = cfg.PermissionRules
	}

	d.logger.Info("user.config called", "result", result)

	return result, nil
}
