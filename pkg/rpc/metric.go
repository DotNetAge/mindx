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

// RoundRatingParams are the params for round.rating（壳侧轮次赞/踩，主收集）。
type RoundRatingParams struct {
	SessionID string `json:"session_id"`
	Value     string `json:"value"` // "up" | "down"
	Text      string `json:"text,omitempty"`
}

// RoundRating 上报轮次主观评价：写入判定版本链（user_feedback 源），
// 可选文本汇入项目 notes"偏好"类。
func (c *Client) RoundRating(params RoundRatingParams) (json.RawMessage, error) {
	return c.CallWithTimeout("round.rating", params)
}

// AgentRateParams are the params for agent.rate（管理页整体评价，限频：同对象每日一次）。
type AgentRateParams struct {
	SubjectType string `json:"subject_type"` // agent | team | skill
	Name        string `json:"name"`
	Value       string `json:"value"` // "up" | "down"
	Text        string `json:"text,omitempty"`
}

// AgentRate 上报整体评价。同对象当日已评时返回 ok=false + reason=already_rated_today。
func (c *Client) AgentRate(params AgentRateParams) (json.RawMessage, error) {
	return c.CallWithTimeout("agent.rate", params)
}

// MetricRecordParams are the params for metric.record（Agent 收尾上报，降级判定源）。
type MetricRecordParams struct {
	SessionID string `json:"session_id"`
	Outcome   string `json:"outcome"` // success | fail | rework
	Note      string `json:"note,omitempty"`
}

// MetricRecord 上报任务判定；outcome=rework 时回溯标记同任务意图的历史会话。
func (c *Client) MetricRecord(params MetricRecordParams) (json.RawMessage, error) {
	return c.CallWithTimeout("metric.record", params)
}
