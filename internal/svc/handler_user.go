package svc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/DotNetAge/mindx/internal/core"
)

// userConfigFieldSetters maps JSON key → setter on MindxConfig.
// 新增可写字段只需在这里加一行，无需修改 handler 逻辑。
var userConfigFieldSetters = map[string]func(cfg *core.MindxConfig, val any){
	"last_agent": func(cfg *core.MindxConfig, v any) {
		if s, ok := v.(string); ok {
			cfg.LastAgent = s
		}
	},
	"last_session_id": func(cfg *core.MindxConfig, v any) {
		if s, ok := v.(string); ok {
			cfg.LastSessionID = s
		}
	},
	"last_model": func(cfg *core.MindxConfig, v any) {
		if s, ok := v.(string); ok {
			cfg.LastModel = s
		}
	},
	"default_model": func(cfg *core.MindxConfig, v any) {
		if s, ok := v.(string); ok {
			cfg.DefaultModel = s
		}
	},
	"default_provider": func(cfg *core.MindxConfig, v any) {
		if s, ok := v.(string); ok {
			cfg.DefaultProvider = s
		}
	},
	"language": func(cfg *core.MindxConfig, v any) {
		if s, ok := v.(string); ok {
			cfg.Language = s
		}
	},
	"auto_indexing": func(cfg *core.MindxConfig, v any) {
		if b, ok := v.(bool); ok {
			cfg.AutoIndexing = b
		}
	},
}

func (d *Daemon) handleUserConfig(_ context.Context, params json.RawMessage) (any, error) {
	cfg := d.app.Config()

	// 写：接受任意已知字段，更新并持久化
	if len(params) > 0 {
		var updates map[string]any
		if err := json.Unmarshal(params, &updates); err == nil {
			changed := false
			for key, val := range updates {
				if setter, ok := userConfigFieldSetters[key]; ok {
					old := getConfigField(cfg, key)
					setter(cfg, val)
					new := getConfigField(cfg, key)
					if fmt.Sprintf("%v", old) != fmt.Sprintf("%v", new) {
						d.logger.Info("user.config: field changed", "key", key, "from", old, "to", new)
						changed = true
					}
				}
			}
			if changed {
				if err := cfg.Save(); err != nil {
					d.logger.Error("user.config: failed to save config", fmt.Errorf("%w", err))
				} else {
					d.logger.Info("user.config: config saved successfully")
				}
			}
		}
	}

	// 读：始终返回当前配置
	result := map[string]any{
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
	if cfg.Language != "" {
		result["language"] = cfg.Language
	}
	if cfg.AutoIndexing {
		result["auto_indexing"] = cfg.AutoIndexing
	}

	d.logger.Info("user.config called", "result", result)

	return result, nil
}

// getConfigField 通过 JSON key 读取当前值（用于对比是否变化）。
func getConfigField(cfg *core.MindxConfig, key string) any {
	switch key {
	case "last_agent":
		return cfg.LastAgent
	case "last_session_id":
		return cfg.LastSessionID
	case "last_model":
		return cfg.LastModel
	case "default_model":
		return cfg.DefaultModel
	case "default_provider":
		return cfg.DefaultProvider
	case "language":
		return cfg.Language
	case "auto_indexing":
		return cfg.AutoIndexing
	}
	return nil
}
