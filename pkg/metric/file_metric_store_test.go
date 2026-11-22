package metric

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 计量文件含用户任务原文：新建写入必须是 0600，历史 0644 文件在打开时被收紧。
func TestMetricFilesArePrivate(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "metric_events.yml"), []byte("[]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewFileMetricStore(dir)

	info, err := os.Stat(filepath.Join(dir, "metric_events.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("existing file perm = %o, want 600", perm)
	}

	if _, err := store.AppendEvent(context.Background(), MetricEvent{Kind: KindRound, SessionID: "s"}); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	info, err = os.Stat(filepath.Join(dir, "metric_events.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("rewritten file perm = %o, want 600", perm)
	}
}

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

// TestJudgmentSourcePriority 覆盖生效判定的来源层级语义（文档与实现的一致性）。
//
// 白皮书 7.4：结构化用户反馈 > 用户文本感受 > Agent 收尾上报。
// 原先实现只按 At 取最新，来源不参与比较——于是一条晚到的 Agent 上报能
// 静默抹掉用户当时的明确判定，「优先级」形同虚设。本测试固定修正后的行为。
func TestJudgmentSourcePriority(t *testing.T) {
	earlier := time.Now().Add(-2 * time.Hour)
	later := time.Now()

	cases := []struct {
		name string
		js   []JudgmentRecord
		want OutcomeSource
	}{
		{
			name: "晚到的 Agent 上报不得抹掉用户反馈",
			js: []JudgmentRecord{
				{Value: OutcomeFail, Source: SourceUserFeedback, At: earlier},
				{Value: OutcomeSuccess, Source: SourceAgentReport, At: later},
			},
			want: SourceUserFeedback,
		},
		{
			name: "晚到的用户文本同样低于结构化反馈",
			js: []JudgmentRecord{
				{Value: OutcomeFail, Source: SourceUserFeedback, At: earlier},
				{Value: OutcomeSuccess, Source: SourceUserText, At: later},
			},
			want: SourceUserFeedback,
		},
		{
			name: "用户文本高于 Agent 上报",
			js: []JudgmentRecord{
				{Value: OutcomeSuccess, Source: SourceAgentReport, At: earlier},
				{Value: OutcomeFail, Source: SourceUserText, At: later},
			},
			want: SourceUserText,
		},
		{
			name: "早到的用户判定依旧压过晚到的返工回溯",
			js: []JudgmentRecord{
				{Value: OutcomeSuccess, Source: SourceUserFeedback, At: earlier},
				{Value: OutcomeRework, Source: SourceReworkTrace, At: later},
			},
			want: SourceUserFeedback,
		},
		{
			name: "同级按时间：返工回溯覆盖 Agent 的乐观成功",
			js: []JudgmentRecord{
				{Value: OutcomeSuccess, Source: SourceAgentReport, At: earlier},
				{Value: OutcomeRework, Source: SourceReworkTrace, At: later},
			},
			want: SourceReworkTrace,
		},
		{
			name: "同级按时间：用户改主意以最后一次为准",
			js: []JudgmentRecord{
				{Value: OutcomeFail, Source: SourceUserFeedback, At: earlier},
				{Value: OutcomeSuccess, Source: SourceUserFeedback, At: later},
			},
			want: SourceUserFeedback,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ev := MetricEvent{Kind: KindRound, Judgments: c.js}
			j, ok := ev.LatestJudgment()
			if !ok {
				t.Fatal("expected a judgment")
			}
			if j.Source != c.want {
				t.Errorf("生效来源 = %s, want %s", j.Source, c.want)
			}
		})
	}
}
