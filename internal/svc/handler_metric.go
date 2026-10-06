package svc

import (
	"context"
	"encoding/json"
	"time"

	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/pkg/metric"
	"github.com/DotNetAge/mindx/pkg/rpc"
)

// handleMetricReport 处理 metric.report：聚合窗口内的 round 事件，
// 输出决策量表（四因子 + P 占位 + Team→Agent→Skill 下钻）。
// 全部确定性计算（白皮书第 4 章聚合层），LLM 只负责解读 JSON。
func (d *Daemon) handleMetricReport(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.MetricReportParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}

	windowDays := p.WindowDays
	if windowDays <= 0 {
		windowDays = 7
	}

	until := time.Now()
	since := until.AddDate(0, 0, -windowDays)

	store := d.app.MetricStore()
	if store == nil {
		return map[string]any{"error": "metric store unavailable"}, nil
	}

	events, err := store.Query(context.Background(), metric.MetricFilter{
		Kind:      metric.KindRound,
		AgentName: p.Agent,
		SkillName: p.Skill,
		Since:     since,
		Until:     until,
	})
	if err != nil {
		return nil, err
	}

	return core.AggregateMetricReport(events,
		since.Format(time.RFC3339), until.Format(time.RFC3339)), nil
}
