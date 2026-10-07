package svc

import (
	"context"
	"encoding/json"
	"strconv"
)

// handleMarketSources 市场服务端 GET /api/v1/sources 的 RPC 转发：返回可用市场源清单
// （配置了商户访问令牌时可见商户专用源）。selected 标注当前选中源
// （cfg.market_source_id；空 = 官方 Default）。前端换源面板直接消费此清单。
func (d *Daemon) handleMarketSources(_ context.Context, _ json.RawMessage) (any, error) {
	sources, err := d.app.Market().Sources()
	if err != nil {
		return nil, err
	}
	cfg := d.app.Config()
	out := make([]map[string]any, 0, len(sources))
	for _, s := range sources {
		idStr := strconv.FormatInt(s.ID, 10)
		selected := cfg.MarketSourceID == idStr || (cfg.MarketSourceID == "" && s.ID == 1)
		out = append(out, map[string]any{
			"id":       idStr,
			"name":     s.Name,
			"tier":     s.Tier,
			"selected": selected,
		})
	}
	return map[string]any{"sources": out}, nil
}
