package rpc

import "encoding/json"

// MetricReportParams are the params for metric.report.
type MetricReportParams struct {
	WindowDays int    `json:"window_days,omitempty"` // 统计窗口（天），默认 7
	Agent      string `json:"agent,omitempty"`       // 下钻过滤：Agent 名
	Skill      string `json:"skill,omitempty"`       // 下钻过滤：技能名
}

// MetricReport 调用 metric.report，返回决策量表 JSON。
// 输出结构见 internal/core.MetricReport（四因子 + P 占位 + 下钻行 +
// 覆盖率声明）；样本不足的指标为 null，禁止以小样本冒充统计。
func (c *Client) MetricReport(params MetricReportParams) (json.RawMessage, error) {
	return c.CallWithTimeout("metric.report", params)
}
