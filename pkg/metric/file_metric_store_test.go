package metric

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestAppendRoundAssignsIDAndTimestamp(t *testing.T) {
	dir := t.TempDir()
	store := NewFileMetricStore(dir)

	ev, err := store.AppendEvent(context.Background(), MetricEvent{
		Kind:       KindRound,
		SessionID:  "sess-1",
		AgentName:  "assistant",
		RoundNo:    1,
		DurationMS: 1200,
		Tokens:     500,
		Cost:       0.01,
		SkillsUsed: []string{"search"},
	})
	if err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	if ev.ID == "" {
		t.Fatal("expected auto-generated event ID")
	}
	if ev.TS.IsZero() {
		t.Fatal("expected auto-filled timestamp")
	}

	got, err := store.Query(context.Background(), MetricFilter{SessionID: "sess-1"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 1 || got[0].ID != ev.ID {
		t.Fatalf("expected 1 event with ID %s, got %+v", ev.ID, got)
	}
}

func TestJudgmentVersionChainAndLatestWins(t *testing.T) {
	dir := t.TempDir()
	store := NewFileMetricStore(dir)

	ev, err := store.AppendEvent(context.Background(), MetricEvent{
		Kind:      KindRound,
		SessionID: "sess-2",
		AgentName: "assistant",
		RoundNo:   1,
	})
	if err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}

	// 无判定时 LatestJudgment 返回 false。
	if _, ok := ev.LatestJudgment(); ok {
		t.Fatal("expected no judgment initially")
	}

	// Agent 上报成功（t0）。
	t0 := time.Now().Add(-2 * time.Hour)
	if err := store.AppendJudgment(context.Background(), ev.ID, JudgmentRecord{
		Value: OutcomeSuccess, Source: SourceAgentReport, At: t0,
	}); err != nil {
		t.Fatalf("AppendJudgment(agent): %v", err)
	}

	// 用户次日重开任务 → 聚合层回溯改写为返工（t1 晚于 t0）。
	t1 := time.Now()
	if err := store.AppendJudgment(context.Background(), ev.ID, JudgmentRecord{
		Value: OutcomeRework, Source: SourceReworkTrace, At: t1,
	}); err != nil {
		t.Fatalf("AppendJudgment(rework): %v", err)
	}

	// 版本链保留全部历史。
	got, err := store.Query(context.Background(), MetricFilter{SessionID: "sess-2"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	if len(got[0].Judgments) != 2 {
		t.Fatalf("expected 2 judgment records in chain, got %d", len(got[0].Judgments))
	}

	// 生效判定 = 时间最新的回溯记录（防乐观偏差固化）。
	latest, ok := got[0].LatestJudgment()
	if !ok || latest.Value != OutcomeRework || latest.Source != SourceReworkTrace {
		t.Fatalf("expected rework_trace to win, got %+v", latest)
	}
}

func TestAppendJudgmentUnknownEventFails(t *testing.T) {
	dir := t.TempDir()
	store := NewFileMetricStore(dir)

	err := store.AppendJudgment(context.Background(), "no-such-id", JudgmentRecord{
		Value: OutcomeFail, Source: SourceUserFeedback, At: time.Now(),
	})
	if err == nil {
		t.Fatal("expected error for unknown event ID")
	}
}

func TestQueryFilters(t *testing.T) {
	dir := t.TempDir()
	store := NewFileMetricStore(dir)

	seed := []MetricEvent{
		{Kind: KindRound, SessionID: "s1", AgentName: "alpha", Team: "core", RoundNo: 1},
		{Kind: KindRound, SessionID: "s2", AgentName: "beta", RoundNo: 1},
		{Kind: KindOverall, AgentName: "alpha"},
		{Kind: KindOverall, SkillName: "search"},
	}
	for _, ev := range seed {
		if _, err := store.AppendEvent(context.Background(), ev); err != nil {
			t.Fatalf("AppendEvent: %v", err)
		}
	}

	cases := []struct {
		name   string
		filter MetricFilter
		want   int
	}{
		{"by agent", MetricFilter{AgentName: "alpha"}, 2},
		{"by team", MetricFilter{Team: "core"}, 1},
		{"by kind overall", MetricFilter{Kind: KindOverall}, 2},
		{"by skill", MetricFilter{SkillName: "search"}, 1},
		{"by session", MetricFilter{SessionID: "s2"}, 1},
	}
	for _, tc := range cases {
		got, err := store.Query(context.Background(), tc.filter)
		if err != nil {
			t.Fatalf("%s: Query: %v", tc.name, err)
		}
		if len(got) != tc.want {
			t.Fatalf("%s: expected %d events, got %d", tc.name, tc.want, len(got))
		}
	}
}

func TestRetentionDropsOnlyOldEvents(t *testing.T) {
	dir := t.TempDir()
	store := NewFileMetricStore(dir)

	old := time.Now().Add(-100 * 24 * time.Hour)
	recent := time.Now().Add(-time.Hour)

	if _, err := store.AppendEvent(context.Background(), MetricEvent{Kind: KindRound, TS: old, SessionID: "old"}); err != nil {
		t.Fatalf("AppendEvent(old): %v", err)
	}
	if _, err := store.AppendEvent(context.Background(), MetricEvent{Kind: KindRound, TS: recent, SessionID: "new"}); err != nil {
		t.Fatalf("AppendEvent(recent): %v", err)
	}
	if err := store.LogConfounding(context.Background(), ConfoundingEvent{TS: old, Kind: "model_change", Detail: "old change"}); err != nil {
		t.Fatalf("LogConfounding: %v", err)
	}

	eventsRemoved, confoundingsRemoved, err := store.Retention(context.Background(), time.Now().Add(-90*24*time.Hour))
	if err != nil {
		t.Fatalf("Retention: %v", err)
	}
	if eventsRemoved != 1 || confoundingsRemoved != 1 {
		t.Fatalf("expected 1+1 removed, got %d+%d", eventsRemoved, confoundingsRemoved)
	}

	got, err := store.Query(context.Background(), MetricFilter{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 1 || got[0].SessionID != "new" {
		t.Fatalf("expected only the recent event to survive, got %+v", got)
	}
}

func TestPersistenceAcrossStoreInstances(t *testing.T) {
	dir := t.TempDir()
	storeA := NewFileMetricStore(dir)

	ev, err := storeA.AppendEvent(context.Background(), MetricEvent{
		Kind: KindRound, SessionID: "persist", AgentName: "assistant", RoundNo: 3,
	})
	if err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}

	// 新实例（模拟 daemon 重启）后数据仍在。
	storeB := NewFileMetricStore(dir)
	got, err := storeB.Query(context.Background(), MetricFilter{SessionID: "persist"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got) != 1 || got[0].ID != ev.ID {
		t.Fatalf("expected persisted event %s, got %+v", ev.ID, got)
	}
}

func TestConfoundingsWindowQuery(t *testing.T) {
	dir := t.TempDir()
	store := NewFileMetricStore(dir)

	base := time.Now().Add(-48 * time.Hour)
	for i := range 3 {
		if err := store.LogConfounding(context.Background(), ConfoundingEvent{
			TS:     base.Add(time.Duration(i) * 24 * time.Hour),
			Kind:   "model_change",
			Detail: fmt.Sprintf("change-%d", i),
		}); err != nil {
			t.Fatalf("LogConfounding: %v", err)
		}
	}

	since := time.Now().Add(-40 * time.Hour)
	until := time.Now()
	got, err := store.QueryConfoundings(context.Background(), since, until)
	if err != nil {
		t.Fatalf("QueryConfoundings: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 confoundings in window, got %d", len(got))
	}
}
