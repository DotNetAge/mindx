package core

import (
	"sort"

	"github.com/DotNetAge/mindx/pkg/metric"
)

// 本文件是决策量表的聚合器（白皮书《决策量表：Agent 生态的绩效系统》第 4 章聚合层）：
// 输入窗口内的 metric 事件，输出四因子 + P 的结构化量表。全部为确定性计算，
// 禁止 LLM 心算——LLM 只负责解读本聚合器的 JSON 输出。

// Stat 是单个指标的统计量。分位数类指标填 Median/P90；比率类指标填
// Mean（0-1 占比），此时 Median/P90 与 Mean 同值便于统一渲染。
// Null 为 true 表示样本不足（低于最小样本阈值），其余字段无意义不得引用。
type Stat struct {
	Samples int     `json:"samples"`
	Mean    float64 `json:"mean"`
	Median  float64 `json:"median"`
	P90     float64 `json:"p90"`
	Null    bool    `json:"null"`
}

// MetricSamples 是窗口内的样本概览。
type MetricSamples struct {
	Rounds int `json:"rounds"` // round 事件数
	Judged int `json:"judged"` // 含生效判定的轮次数
}

// MetricWindow 是量表的统计窗口。
type MetricWindow struct {
	Since string `json:"since"`
	Until string `json:"until"`
}

// MetricReport 是 metric report 的 JSON 载荷。
// v1 填充 Speed/Effort 与 q_route_correction；判定类指标（Q 其余/Y/P）在 v2 判定层
// 上线前输出 Null=true 的占位——空值显式声明，禁止以缺失冒充零值。
type MetricReport struct {
	Window    MetricWindow     `json:"window"`
	Samples   MetricSamples    `json:"samples"`
	Speed     SpeedStats       `json:"speed"`
	Effort    EffortStats      `json:"effort"`
	Quality   QualityStats     `json:"quality"`
	Perceived PerceivedStats   `json:"perceived"`
	Yield     map[string]Stat  `json:"yield"`
	ByAgent   []AgentMetricRow `json:"by_agent,omitempty"`
	BySkill   []SkillMetricRow `json:"by_skill,omitempty"`
	Notes     []string         `json:"notes,omitempty"` // 口径说明（如简化口径声明）
}

// SpeedStats：s_rounds = 每次任务执行的循环数（ExecutionSummary.TotalIterations）；
// s_duration = 每次任务执行的时长（毫秒）。
type SpeedStats struct {
	SRounds   *Stat `json:"s_rounds"`
	SDuration *Stat `json:"s_duration"`
}

// EffortStats：e_tokens = 每次任务执行的有效 token（ActualTokens，缓存不计费部分已扣除）。
// e_cost 需要 pricing join，v2 交付（见指标字典）。
type EffortStats struct {
	ETokens *Stat `json:"e_tokens"`
	ECost   *Stat `json:"e_cost"`
}

// QualityStats：q_route_correction = 会话内相邻技能切换计数（v1 简化口径：
// 轮次技能序列中相邻不同技能的次数，按会话汇总）；q_task_success /
// q_rework_rate 为会话级比率（生效判定取会话最新轮次的判定——白皮书 4.3）。
type QualityStats struct {
	QTaskSuccess     *Stat `json:"q_task_success"`
	QReworkRate      *Stat `json:"q_rework_rate"`
	QRouteCorrection *Stat `json:"q_route_correction"`
}

// PerceivedStats：主观感知（P 维度，白皮书第 7 章）。p_round_agg = 轮次
// 赞踩聚合（user_feedback 判定中"赞"占比，事件满意度构念）；p_overall =
// 管理页整体评价"赞"占比（整体印象构念）。两构念分开呈现不混算（MTMM）。
type PerceivedStats struct {
	PRoundAgg *Stat `json:"p_round_agg"`
	POverall  *Stat `json:"p_overall"`
}

// AgentMetricRow 是 Agent 维度的下钻行。
type AgentMetricRow struct {
	Agent      string  `json:"agent"`
	Rounds     int     `json:"rounds"`
	TokensSum  int     `json:"tokens_sum"`
	DurationMS int64   `json:"duration_ms_sum"`
	Coverage   float64 `json:"coverage"` // 有判定轮次占比
}

// SkillMetricRow 是技能维度的下钻行（按轮次引用计数）。
type SkillMetricRow struct {
	Skill  string `json:"skill"`
	Rounds int    `json:"rounds"` // 引用该技能的轮次数
}

// minMetricSamples 是最小样本阈值（附录 B 第 1 条默认 10）。
const minMetricSamples = 10

// AggregateMetricReport 从窗口内的 metric 事件聚合量表。
// round 事件驱动 S/E 与判定类指标；overall 事件驱动 p_overall。
func AggregateMetricReport(events []metric.MetricEvent, since, until string) *MetricReport {
	report := &MetricReport{
		Window: MetricWindow{Since: since, Until: until},
		Yield:  map[string]Stat{"y_adoption": {Null: true}}, // 采纳信号 v2 接入
		Notes: []string{
			"v1 口径：q_route_correction 为相邻技能切换的简化计数；e_cost 需定价 join，v2 交付",
		},
	}

	var iterations []float64
	var durations []float64
	var tokens []float64
	var routeCorrections []float64

	// 会话级生效判定：每会话取最新 round 事件的判定（白皮书 4.3——
	// 返工回溯写入的 rework_trace 天然是最新记录，覆盖早先乐观判定）。
	sessionOutcome := map[string]metric.Outcome{}
	ratingUp, ratingTotal := 0, 0
	overallUp, overallTotal := 0, 0

	byAgent := map[string]*AgentMetricRow{}
	bySkill := map[string]*SkillMetricRow{}

	for _, ev := range events {
		if ev.Kind != metric.KindRound {
			// overall 事件：管理页整体评价（P 的整体印象构念）。
			if ev.Kind == metric.KindOverall {
				if j, ok := ev.LatestJudgment(); ok && j.Source == metric.SourceUserFeedback {
					overallTotal++
					if j.Value == metric.OutcomeSuccess {
						overallUp++
					}
				}
			}
			continue
		}

		report.Samples.Rounds++
		judged, hasJudgment := ev.LatestJudgment()
		if hasJudgment {
			report.Samples.Judged++
			sessionOutcome[ev.SessionID] = judged.Value
			// P 的轮次赞踩：仅统计用户结构化反馈源（user_feedback）。
			if judged.Source == metric.SourceUserFeedback {
				ratingTotal++
				if judged.Value == metric.OutcomeSuccess {
					ratingUp++
				}
			}
		}

		iterations = append(iterations, float64(ev.RoundIterations))
		durations = append(durations, float64(ev.DurationMS))
		tokens = append(tokens, float64(ev.Tokens))
		if rc := ev.RouteCorrection; rc > 0 {
			routeCorrections = append(routeCorrections, float64(rc))
		}

		row := byAgent[ev.AgentName]
		if row == nil {
			row = &AgentMetricRow{Agent: ev.AgentName}
			byAgent[ev.AgentName] = row
		}
		row.Rounds++
		row.TokensSum += ev.Tokens
		row.DurationMS += ev.DurationMS
		if hasJudgment {
			row.Coverage++
		}

		seen := map[string]bool{}
		for _, sk := range ev.SkillsUsed {
			if seen[sk] {
				continue
			}
			seen[sk] = true
			srow := bySkill[sk]
			if srow == nil {
				srow = &SkillMetricRow{Skill: sk}
				bySkill[sk] = srow
			}
			srow.Rounds++
		}
	}

	for _, row := range byAgent {
		if row.Rounds > 0 {
			row.Coverage = row.Coverage / float64(row.Rounds)
		}
	}
	report.ByAgent = sortAgentRows(byAgent)
	report.BySkill = sortSkillRows(bySkill)

	report.Speed.SRounds = buildStat(iterations)
	report.Speed.SDuration = buildStat(durations)
	report.Effort.ETokens = buildStat(tokens)
	// e_cost：需要 pricing join（模型定价表），v2 交付。
	report.Effort.ECost = &Stat{Null: true}
	report.Quality.QRouteCorrection = buildStat(routeCorrections)

	// 会话级比率：成功 / 返工（有判定会话为分母）。
	successCount, reworkCount := 0, 0
	for _, v := range sessionOutcome {
		switch v {
		case metric.OutcomeSuccess:
			successCount++
		case metric.OutcomeRework:
			reworkCount++
		}
	}
	report.Quality.QTaskSuccess = ratioStat(successCount, len(sessionOutcome))
	report.Quality.QReworkRate = ratioStat(reworkCount, len(sessionOutcome))
	report.Perceived.PRoundAgg = ratioStat(ratingUp, ratingTotal)
	report.Perceived.POverall = ratioStat(overallUp, overallTotal)

	return report
}

// ratioStat 构造比率型 Stat（Mean/Median/P90 同值，0-1 占比）；
// 分母不足阈值时为 Null——禁止以小样本冒充统计（白皮书 9 章通用口径）。
func ratioStat(numerator, denominator int) *Stat {
	if denominator < minMetricSamples {
		return &Stat{Samples: denominator, Null: true}
	}
	r := float64(numerator) / float64(denominator)
	return &Stat{Samples: denominator, Mean: r, Median: r, P90: r}
}

// buildStat 由样本集构造 Stat（分位数类：Mean 为算术均值）；样本不足阈值时返回 Null。
func buildStat(samples []float64) *Stat {
	if len(samples) < minMetricSamples {
		return &Stat{Samples: len(samples), Null: true}
	}
	mean := 0.0
	for _, v := range samples {
		mean += v
	}
	return &Stat{
		Samples: len(samples),
		Mean:    mean / float64(len(samples)),
		Median:  quantile(samples, 0.5),
		P90:     quantile(samples, 0.9),
	}
}

// quantile 返回最近秩分位数（nearest-rank，避免插值的过度平滑）。
func quantile(samples []float64, q float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	sorted := make([]float64, len(samples))
	copy(sorted, samples)
	sort.Float64s(sorted)
	idx := int(q * float64(len(sorted)))
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	if idx < 0 {
		idx = 0
	}
	return sorted[idx]
}

func sortAgentRows(m map[string]*AgentMetricRow) []AgentMetricRow {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	rows := make([]AgentMetricRow, 0, len(keys))
	for _, k := range keys {
		rows = append(rows, *m[k])
	}
	return rows
}

func sortSkillRows(m map[string]*SkillMetricRow) []SkillMetricRow {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	rows := make([]SkillMetricRow, 0, len(keys))
	for _, k := range keys {
		rows = append(rows, *m[k])
	}
	return rows
}
