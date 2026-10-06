package core

import (
	"sort"

	"github.com/DotNetAge/mindx/pkg/metric"
)

// 本文件是决策量表的聚合器（白皮书《决策量表：Agent 生态的绩效系统》第 4 章聚合层）：
// 输入窗口内的 metric 事件，输出四因子 + P 的结构化量表。全部为确定性计算，
// 禁止 LLM 心算——LLM 只负责解读本聚合器的 JSON 输出。

// Stat 是单个指标的统计量：样本量 + 中位数 + P90。
// Null 为 true 表示样本不足（低于最小样本阈值），Median/P90 无意义不得引用。
type Stat struct {
	Samples int     `json:"samples"`
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
	Yield     map[string]Stat  `json:"yield"`
	Perceived map[string]Stat  `json:"perceived"`
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
// 轮次技能序列中相邻不同技能的次数，按会话汇总）。完整口径见指标字典。
type QualityStats struct {
	QRouteCorrection *Stat `json:"q_route_correction"`
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

// AggregateMetricReport 从窗口内的 round 事件聚合量表。
// now 用于生成窗口Until；事件由调用方先行过滤窗口。
func AggregateMetricReport(events []metric.MetricEvent, since, until string) *MetricReport {
	report := &MetricReport{
		Window:    MetricWindow{Since: since, Until: until},
		Yield:     map[string]Stat{"y_adoption": {Null: true}},
		Perceived: map[string]Stat{"p_round_agg": {Null: true}, "p_overall": {Null: true}},
		Notes: []string{
			"v1 口径：q_route_correction 为相邻技能切换的简化计数；e_cost 需定价 join，v2 交付",
		},
	}

	var iterations []float64
	var durations []float64
	var tokens []float64
	var routeCorrections []float64

	byAgent := map[string]*AgentMetricRow{}
	bySkill := map[string]*SkillMetricRow{}

	for _, ev := range events {
		if ev.Kind != metric.KindRound {
			continue
		}
		report.Samples.Rounds++
		if _, judged := ev.LatestJudgment(); judged {
			report.Samples.Judged++
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
		if _, judged := ev.LatestJudgment(); judged {
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

	return report
}

// buildStat 由样本集构造 Stat；样本不足阈值时返回 Null。
func buildStat(samples []float64) *Stat {
	if len(samples) < minMetricSamples {
		return &Stat{Samples: len(samples), Null: true}
	}
	return &Stat{
		Samples: len(samples),
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
