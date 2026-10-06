// Package metric 承载决策量表的计量数据模型与本地持久化设施。
//
// 数据模型对应白皮书《决策量表：Agent 生态的绩效系统》第 4 章五层管线中的
// 采集层与存储层：
//
//   - MetricEvent：轮次级计量事件（round，客观量自动埋点），判定以版本链
//     形式追加在同一条事件上（带版本、可回溯改写——白皮书 4.3）；
//     管理页整体评价为 overall 事件（对象为 Agent/Team/Skill）。
//   - ConfoundingEvent：混杂登记事件（模型版本变更、路由表大版本变更等），
//     供 metric compare 输出强制附带（白皮书 2.1 局限声明）。
//
// 存储采用单文件 YAML + 全量读写（复刻 pkg/session 的 FileTokenUsageStore 模式），
// 90 天滚动保留明细、聚合结果永久（白皮书 8.3 成本边界）。
package metric

import "time"

// Outcome 是一次判定结果的取值。
type Outcome string

const (
	// OutcomeSuccess 表示该轮次/任务被判定为成功。
	OutcomeSuccess Outcome = "success"
	// OutcomeFail 表示该轮次/任务被判定为失败。
	OutcomeFail Outcome = "fail"
	// OutcomeRework 表示该轮次/任务被判定为返工（需要重做或已被重做取代）。
	OutcomeRework Outcome = "rework"
)

// OutcomeSource 标识判定来源。聚合统计按来源分维度，回溯修正的判定
// 以时间为准取最新（见 LatestJudgment）。
type OutcomeSource string

const (
	// SourceUserFeedback 用户在壳侧的结构化反馈（轮次赞踩 / 管理页评价）。
	SourceUserFeedback OutcomeSource = "user_feedback"
	// SourceUserText 用户写入的文本感受（归 notes"偏好"类的同源数据）。
	SourceUserText OutcomeSource = "user_text"
	// SourceAgentReport Agent 收尾通过 mindx metric record 上报的自我判定。
	SourceAgentReport OutcomeSource = "agent_report"
	// SourceReworkTrace 同任务重开识别命中后由聚合层写入的客观回溯判定。
	SourceReworkTrace OutcomeSource = "rework_trace"
)

// EventKind 区分事件类别。
type EventKind string

const (
	// KindRound 轮次客观事件：每轮问答结束自动埋点（S/E 客观量）。
	KindRound EventKind = "round"
	// KindRating 轮次主观事件：用户对某轮次的赞/踩评价（判定追加到同一事件）。
	KindRating EventKind = "rating"
	// KindOverall 管理页整体评价事件：对象为 Agent/Team/Skill，非轮次粒度。
	KindOverall EventKind = "overall"
)

// JudgmentRecord 是一次判定记录。判定按时间生效（LatestJudgment 取最新），
// 版本链保留全部历史供审计——回溯修正不改写旧记录，只追加新记录。
type JudgmentRecord struct {
	Value  Outcome       `yaml:"value" json:"value"`
	Source OutcomeSource `yaml:"source" json:"source"`
	At     time.Time     `yaml:"at" json:"at"`
	Note   string        `yaml:"note,omitempty" json:"note,omitempty"`
}

// MetricEvent 是计量事件。一个轮次一条 round 事件（客观量在埋点时填充），
// 判定（用户赞踩 / Agent 上报 / 回溯修正）以版本链追加在同一条事件上；
// overall 事件独立成条（无轮次号）。
type MetricEvent struct {
	ID        string    `yaml:"id" json:"id"`
	TS        time.Time `yaml:"ts" json:"ts"`
	Kind      EventKind `yaml:"kind" json:"kind"`
	SessionID string    `yaml:"session_id,omitempty" json:"session_id,omitempty"`
	AgentName string    `yaml:"agent_name,omitempty" json:"agent_name,omitempty"`
	Team      string    `yaml:"team,omitempty" json:"team,omitempty"`
	RoundNo   int       `yaml:"round_no,omitempty" json:"round_no,omitempty"`
	SkillName string    `yaml:"skill_name,omitempty" json:"skill_name,omitempty"`

	// —— S/E 客观量（round 事件）——
	DurationMS int64    `yaml:"duration_ms,omitempty" json:"duration_ms,omitempty"`
	Tokens     int      `yaml:"tokens,omitempty" json:"tokens,omitempty"`
	Cost       float64  `yaml:"cost,omitempty" json:"cost,omitempty"`
	SkillsUsed []string `yaml:"skills_used,omitempty" json:"skills_used,omitempty"`

	// —— 判定版本链（rating / overall 事件；round 事件可被回溯追加）——
	Judgments []JudgmentRecord `yaml:"judgments,omitempty" json:"judgments,omitempty"`

	// —— 返工指向：本事件被识别为对某会话的返工时记录原会话 ID ——
	ReworkOf string `yaml:"rework_of,omitempty" json:"rework_of,omitempty"`

	// —— 任务意图提示（会话任务名，用于意图聚类与路由一致性度量）——
	TaskHint string `yaml:"task_hint,omitempty" json:"task_hint,omitempty"`
}

// LatestJudgment 返回事件的生效判定：时间最新的记录。
// 回溯修正（rework_trace）天然发生在后，因此覆盖早先的乐观判定；
// 来源字段仅作统计维度，不参与优先级比较。
// 无判定时返回 false。
func (e *MetricEvent) LatestJudgment() (JudgmentRecord, bool) {
	if len(e.Judgments) == 0 {
		return JudgmentRecord{}, false
	}
	latest := e.Judgments[0]
	for _, j := range e.Judgments[1:] {
		if j.At.After(latest.At) {
			latest = j
		}
	}
	return latest, true
}

// ConfoundingEvent 是混杂登记事件（白皮书 2.1 局限声明）：
// 在 metric compare 的基线窗口内发生的、可能污染归因的变更，
// 输出时强制附带，防止把模型升级的效果误记到路由表修订头上。
type ConfoundingEvent struct {
	TS     time.Time `yaml:"ts" json:"ts"`
	Kind   string    `yaml:"kind" json:"kind"`     // model_change / runtime_major / daemon_reinstall
	Detail string    `yaml:"detail" json:"detail"` // 人类可读的变更说明（如 "model: glm-4.5 -> glm-5"）
}

// MetricFilter 是 Query 的过滤条件；零值字段表示不限制。
type MetricFilter struct {
	SessionID string
	AgentName string
	Team      string
	SkillName string
	Kind      EventKind
	Since     time.Time
	Until     time.Time
}
