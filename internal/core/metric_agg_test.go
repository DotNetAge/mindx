package core

import (
	"testing"
	"time"

	"github.com/DotNetAge/mindx/pkg/metric"
)

func metricRound(id string, agent string, iterations int, durationMS int64, tokens int, skills []string) metric.MetricEvent {
	return metric.MetricEvent{
		ID:              id,
		Kind:            metric.KindRound,
		SessionID:       "sess-" + id,
		AgentName:       agent,
		RoundIterations: iterations,
		DurationMS:      durationMS,
		Tokens:          tokens,
		SkillsUsed:      skills,
		RouteCorrection: countSwitches(skills),
		TS:              time.Now(),
	}
}

// countSwitches 复刻埋点闭包的相邻技能切换计数口径。
func countSwitches(skills []string) int {
	count := 0
	for i := 1; i < len(skills); i++ {
		if skills[i] != skills[i-1] {
			count++
		}
	}
	return count
}

func TestAggregateMetricReportAboveThreshold(t *testing.T) {
	// 12 个样本（超过阈值 10）：iterations 1..12，中位 6/7 之间取秩 6。
	var events []metric.MetricEvent
	for i := 1; i <= 12; i++ {
		events = append(events, metricRound(
			string(rune('a'+i)), "assistant", i, int64(i*100), i*10,
			[]string{"search", "SkillB"}))
	}

	report := AggregateMetricReport(events, "since", "until")

	if report.Samples.Rounds != 12 {
		t.Fatalf("expected 12 rounds, got %d", report.Samples.Rounds)
	}
	if report.Samples.Judged != 0 {
		t.Fatalf("expected 0 judged (no judgments), got %d", report.Samples.Judged)
	}
	if report.Speed.SRounds.Null {
		t.Fatal("s_rounds should be above threshold")
	}
	if report.Speed.SRounds.Median != 6 && report.Speed.SRounds.Median != 7 {
		t.Fatalf("expected median around 6-7, got %v", report.Speed.SRounds.Median)
	}
	if report.Speed.SRounds.P90 != 11 {
		t.Fatalf("expected p90 = 11 (nearest-rank), got %v", report.Speed.SRounds.P90)
	}
	if report.Effort.ETokens.Null {
		t.Fatal("e_tokens should be above threshold")
	}
	if report.Effort.ECost == nil || !report.Effort.ECost.Null {
		t.Fatal("e_cost is v2, expected null placeholder")
	}
	// 技能序列 [search, SkillB] 的相邻切换 = 1。
	if report.Quality.QRouteCorrection.Null {
		t.Fatal("q_route_correction should be above threshold")
	}
	if report.Quality.QRouteCorrection.Median != 1 {
		t.Fatalf("expected route correction median 1, got %v", report.Quality.QRouteCorrection.Median)
	}
}

func TestAggregateMetricReportBelowThresholdIsNull(t *testing.T) {
	events := []metric.MetricEvent{metricRound("a", "assistant", 3, 100, 50, nil)}

	report := AggregateMetricReport(events, "since", "until")

	if !report.Speed.SRounds.Null {
		t.Fatal("expected s_rounds null below threshold")
	}
	if report.Speed.SRounds.Samples != 1 {
		t.Fatalf("expected samples 1, got %d", report.Speed.SRounds.Samples)
	}
	// Yield/Perceived 在 v2 前为 null 占位。
	if report.Yield["y_adoption"].Null != true {
		t.Fatal("expected y_adoption null placeholder")
	}
	if report.Perceived["p_round_agg"].Null != true {
		t.Fatal("expected p_round_agg null placeholder")
	}
}

func TestAggregateMetricReportByAgentAndSkill(t *testing.T) {
	events := []metric.MetricEvent{
		metricRound("1", "alpha", 2, 100, 10, []string{"search"}),
		metricRound("2", "alpha", 3, 200, 20, []string{"search", "SkillB"}),
		metricRound("3", "beta", 4, 300, 30, nil),
	}

	report := AggregateMetricReport(events, "since", "until")

	if len(report.ByAgent) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(report.ByAgent))
	}
	// 排序后 alpha 在前。
	if report.ByAgent[0].Agent != "alpha" || report.ByAgent[0].Rounds != 2 {
		t.Fatalf("unexpected first agent row: %+v", report.ByAgent[0])
	}
	if report.ByAgent[0].TokensSum != 30 {
		t.Fatalf("expected alpha tokens_sum 30, got %d", report.ByAgent[0].TokensSum)
	}
	// 技能引用：search 被两轮引用，SkillB 一轮。
	if len(report.BySkill) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(report.BySkill))
	}
	if report.BySkill[0].Skill != "SkillB" || report.BySkill[0].Rounds != 1 {
		t.Fatalf("unexpected first skill row: %+v", report.BySkill[0])
	}
}

func TestAggregateIgnoresNonRoundEvents(t *testing.T) {
	events := []metric.MetricEvent{
		{Kind: metric.KindOverall, AgentName: "alpha"},
		metricRound("1", "alpha", 2, 100, 10, nil),
	}

	report := AggregateMetricReport(events, "since", "until")

	if report.Samples.Rounds != 1 {
		t.Fatalf("expected only 1 round event counted, got %d", report.Samples.Rounds)
	}
}
