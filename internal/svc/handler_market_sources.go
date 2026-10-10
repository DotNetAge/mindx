package svc

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/DotNetAge/mindx/internal/core/bundle"
)

// handleMarketSources 市场源清单 RPC：源目录恒从引导基址（cfg.market_api_base）
// 拉取——换源只切当前生效 base，不改目录视角，保证清单稳定且跨源可切回。
// base_url 为源服务根地址（换源 = 换此地址）；selected 按 base_url 与当前生效
// base（market_active_base，空 = 跟随引导；再空 = 客户端内置官方默认）匹配标注，
// 不按源 id 匹配（id 空间属于目录服务，与 active base 无对应关系）。
// 前端来源卡片视图直接消费此清单。
func (d *Daemon) handleMarketSources(_ context.Context, _ json.RawMessage) (any, error) {
	cfg := d.app.Config()
	dir := bundle.NewMarketClient(cfg.MarketAPIBase, "")
	dir.SetPAT(d.app.MarketPAT())
	sources, err := dir.Sources()
	if err != nil {
		return nil, err
	}
	current := strings.TrimRight(cfg.MarketActiveBase, "/")
	if current == "" {
		current = strings.TrimRight(cfg.MarketAPIBase, "/")
	}
	if current == "" {
		current = strings.TrimRight(bundle.DefaultMarketBase, "/")
	}
	out := make([]map[string]any, 0, len(sources))
	for _, s := range sources {
		out = append(out, map[string]any{
			"id":       strconv.FormatInt(s.ID, 10),
			"name":     s.Name,
			"base_url": strings.TrimRight(s.BaseURL, "/"),
			"tier":     s.Tier,
			"selected": strings.TrimRight(s.BaseURL, "/") == current,
		})
	}
	return map[string]any{"sources": out}, nil
}
