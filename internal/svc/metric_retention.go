package svc

import (
	"context"
	"time"
)

// metricRetentionKeep 计量明细的保留窗（与白皮书口径一致：明细 90 天，
// 聚合结果由上层另行持久化，不受滚动影响）。
const metricRetentionKeep = 90 * 24 * time.Hour

// startMetricRetention 启动时清一次、之后每天清一次。
// 之所以要主动调度：Retention 只在写接口里被定义、从没人调用，明细文件
// 会无限增长；而每次 Append 都是全量重写，历史越长写入越慢。
func (d *Daemon) startMetricRetention(ctx context.Context) {
	store := d.app.MetricStore()
	if store == nil {
		return
	}
	sweep := func() {
		events, confoundings, err := store.Retention(ctx, time.Now().Add(-metricRetentionKeep))
		if err != nil {
			d.logger.Warn("metric retention failed", "error", err)
			return
		}
		if events > 0 || confoundings > 0 {
			d.logger.Info("metric retention done", "events_removed", events, "confoundings_removed", confoundings)
		}
	}
	go func() {
		sweep()
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sweep()
			}
		}
	}()
}
