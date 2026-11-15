package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
	"kb_addr": func(cfg *core.MindxConfig, v any) {
		if s, ok := v.(string); ok {
			cfg.KBAddr = s
		}
	},
	"channel_url": func(cfg *core.MindxConfig, v any) {
		if s, ok := v.(string); ok {
			cfg.ChannelURL = s
		}
	},
	"market_source_id": func(cfg *core.MindxConfig, v any) {
		if s, ok := v.(string); ok {
			cfg.MarketSourceID = strings.TrimSpace(s)
		}
	},
}

func (d *Daemon) handleUserConfig(_ context.Context, params json.RawMessage) (any, error) {
	cfg := d.app.Config()

	// 写：接受任意已知字段，更新并持久化
	if len(params) > 0 {
		var updates map[string]any
		if err := json.Unmarshal(params, &updates); err != nil {
			// 畸形参数以前被静默当成「只读请求」，返回旧配置，调用方以为写入成功。
			return nil, fmt.Errorf("配置参数格式不对，这次修改没有生效。")
		}
		changed := false
		kbChanged := false
		channelChanged := false
		marketChanged := false
		oldMarketSourceID := cfg.MarketSourceID
		// 回滚快照：cfg 是共享指针，setter 已就地改写内存值。落盘失败时必须把
		// 内存改回去，否则「界面显示保存成功、重启又变回来」的内存/磁盘分叉。
		oldValues := make(map[string]any)
		for key, val := range updates {
			if setter, ok := userConfigFieldSetters[key]; ok {
				old := getConfigField(cfg, key)
				setter(cfg, val)
				new := getConfigField(cfg, key)
				if fmt.Sprintf("%v", old) != fmt.Sprintf("%v", new) {
					d.logger.Info("user.config: field changed", "key", key, "from", old, "to", new)
					changed = true
					oldValues[key] = old
					if key == "kb_addr" {
						kbChanged = true
					}
					if key == "channel_url" {
						channelChanged = true
					}
					if key == "market_source_id" {
						marketChanged = true
					}
				}
				continue
			}
			// 专用源邀请码特判（user.config 键名 market_pat 历史沿用，其本义是
			// market 访问令牌/AT）：写入密钥存储而非 mindx.json，
			// 明文严禁回读与入日志，只记录 set/unset 状态
			if key == "market_pat" {
				pat, ok := val.(string)
				if !ok {
					continue
				}
				pat = strings.TrimSpace(pat)
				if pat == d.app.MarketPAT() {
					continue
				}
				if err := d.app.SetMarketPAT(pat); err != nil {
					return nil, err
				}
				d.logger.Info("user.config: market_pat changed", "state", map[bool]string{true: "set", false: "unset"}[pat != ""])
				changed = true
				marketChanged = true
			}
		}
		if changed {
			if err := cfg.Save(); err != nil {
				d.logger.Error("user.config: 配置落盘失败，正在回滚内存值", err)
				for key, old := range oldValues {
					if setter, ok := userConfigFieldSetters[key]; ok {
						setter(cfg, old)
					}
				}
				return nil, fmt.Errorf("配置没能保存到磁盘，这次修改已还原。请检查磁盘空间或目录权限后重试。")
			}
			d.logger.Info("user.config: config saved successfully")
		}
		// 知识库地址变更后清空运行时缓存，使下一次 ResolveRuntime
		// 按新地址重新装配知识库工具（未配置时则不再装配）。
		if kbChanged {
			d.app.InvalidateRuntimes()
		}
		// 中继地址变更后通知 Channel 管理器按新地址重连（清空则断开离线）。
		if channelChanged {
			d.onChannelURLChanged(cfg.ChannelURL)
		}
		// 市场源/令牌变更后重建市场客户端并广播，前端货架重置重拉：
		// 换源以「连通探测成功」为唯一判据，失败回滚 source_id 并把
		// 错误透传前端（弹窗提示，不静默降级）。
		if marketChanged {
			if err := d.app.SetMarketSource(cfg.MarketSourceID); err != nil {
				cfg.MarketSourceID = oldMarketSourceID
				if saveErr := cfg.Save(); saveErr != nil {
					d.logger.Error("user.config: rollback save failed", fmt.Errorf("%w", saveErr))
				}
				return nil, err
			}
			d.broadcastUI("market_changed", map[string]any{"source_id": cfg.MarketSourceID})
		}
	}

	// 读：始终返回当前配置（邀请码只回是否已设置的布尔，明文严禁回读/落日志）
	result := map[string]any{
		"initialized":      cfg.Initialized,
		"last_agent":       cfg.LastAgent,
		"last_session_id":  cfg.LastSessionID,
		"default_model":    cfg.DefaultModel,
		"default_provider": cfg.DefaultProvider,
		"last_model":       cfg.LastModel,
		"kb_addr":          cfg.KBAddr,
		"channel_url":      cfg.ChannelURL,
		"market_source_id": cfg.MarketSourceID,
		"market_pat_set":   d.app.MarketPAT() != "",
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
	case "kb_addr":
		return cfg.KBAddr
	case "channel_url":
		return cfg.ChannelURL
	case "market_source_id":
		return cfg.MarketSourceID
	}
	return nil
}
