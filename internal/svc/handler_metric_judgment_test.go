package svc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DotNetAge/mindx/pkg/metric"
)

func TestRatingToOutcome(t *testing.T) {
	if v, err := ratingToOutcome("up"); err != nil || v != metric.OutcomeSuccess {
		t.Errorf("up = %v, %v", v, err)
	}
	if v, err := ratingToOutcome("like"); err != nil || v != metric.OutcomeSuccess {
		t.Errorf("like = %v, %v", v, err)
	}
	if v, err := ratingToOutcome("down"); err != nil || v != metric.OutcomeFail {
		t.Errorf("down = %v, %v", v, err)
	}
	if v, err := ratingToOutcome("dislike"); err != nil || v != metric.OutcomeFail {
		t.Errorf("dislike = %v, %v", v, err)
	}
	if _, err := ratingToOutcome("meh"); err == nil {
		t.Error("invalid value: expected error")
	}
}

func TestOverallSubjectMatches(t *testing.T) {
	judged := metric.MetricEvent{
		AgentName: "alpha",
		SkillName: "skill-a",
		Team:      "team-a",
		Judgments: []metric.JudgmentRecord{{Value: metric.OutcomeSuccess}},
	}
	unjudged := metric.MetricEvent{AgentName: "alpha"}

	if !overallSubjectMatches(judged, "agent", "alpha") {
		t.Error("agent match expected")
	}
	if overallSubjectMatches(judged, "agent", "beta") {
		t.Error("agent mismatch should not match")
	}
	if !overallSubjectMatches(judged, "skill", "skill-a") {
		t.Error("skill match expected")
	}
	if !overallSubjectMatches(judged, "team", "team-a") {
		t.Error("team match expected")
	}
	if overallSubjectMatches(judged, "unknown", "alpha") {
		t.Error("unknown subject type should not match")
	}
	if overallSubjectMatches(unjudged, "agent", "alpha") {
		t.Error("event without judgments should not match")
	}
}

// metricTestDaemon 带 metric store 与两个 round 事件的会话（agent=tester）。
func metricTestDaemon(t *testing.T) (*Daemon, string) {
	t.Helper()
	d, cleanup := newTestDaemon(t)
	t.Cleanup(cleanup)

	sid := mustCreateSession(t, d.app.SessDB(), "tester")
	store := d.app.MetricStore()
	if store == nil {
		t.Fatal("MetricStore() is nil")
	}
	if _, err := store.AppendEvent(context.Background(), metric.MetricEvent{
		Kind: metric.KindRound, SessionID: sid, AgentName: "tester", RoundNo: 1,
	}); err != nil {
		t.Fatalf("append round event: %v", err)
	}
	return d, sid
}

func TestLatestRoundEvent(t *testing.T) {
	d, sid := metricTestDaemon(t)
	store := d.app.MetricStore()

	// 无 round 事件的会话 → 报错
	if _, err := latestRoundEvent(store, "ghost-session"); err == nil {
		t.Error("no round event: expected error")
	}

	// 追加第二条（时间更新），应取最新
	ev2, err := store.AppendEvent(context.Background(), metric.MetricEvent{
		Kind: metric.KindRound, SessionID: sid, AgentName: "tester", RoundNo: 2,
	})
	if err != nil {
		t.Fatalf("append second round event: %v", err)
	}
	target, err := latestRoundEvent(store, sid)
	if err != nil {
		t.Fatalf("latestRoundEvent error = %v", err)
	}
	if target.ID != ev2.ID {
		t.Errorf("latest = %s, want %s", target.ID, ev2.ID)
	}
}

func TestHandleRoundRating(t *testing.T) {
	d, sid := metricTestDaemon(t)

	// 参数校验
	if _, err := d.handleRoundRating(context.Background(), mustParams(t, roundRatingParams{})); err == nil {
		t.Error("empty session_id: expected error")
	}
	if _, err := d.handleRoundRating(context.Background(), mustParams(t, roundRatingParams{SessionID: sid, Value: "meh"})); err == nil {
		t.Error("invalid value: expected error")
	}

	// 踩 → fail 判定（无文本，无 note_path）
	result, err := d.handleRoundRating(context.Background(), mustParams(t, roundRatingParams{SessionID: sid, Value: "down"}))
	if err != nil {
		t.Fatalf("handleRoundRating error = %v", err)
	}
	if result.(map[string]any)["ok"] != true {
		t.Errorf("result = %#v, want ok=true", result)
	}

	// 赞 + 文本 → 判定 + note 落盘
	result, err = d.handleRoundRating(context.Background(), mustParams(t, roundRatingParams{
		SessionID: sid, Value: "up", Text: "  希望以后回答更简洁  ",
	}))
	if err != nil {
		t.Fatalf("handleRoundRating with text error = %v", err)
	}
	m := result.(map[string]any)
	if m["note_path"] == nil || m["note_path"] == "" {
		t.Fatalf("expected note_path, got %#v", m)
	}
	notePath := m["note_path"].(string)
	content, readErr := os.ReadFile(notePath)
	if readErr != nil || !strings.Contains(string(content), "希望以后回答更简洁") {
		t.Errorf("note content missing user text: %v", readErr)
	}

	// 判定版本链：最新为 user_feedback up
	store := d.app.MetricStore()
	events, _ := store.Query(context.Background(), metric.MetricFilter{SessionID: sid, Kind: metric.KindRound})
	var found bool
	for _, ev := range events {
		if j, ok := ev.LatestJudgment(); ok && j.Source == metric.SourceUserFeedback && j.Value == metric.OutcomeSuccess {
			found = true
		}
	}
	if !found {
		t.Error("expected latest user_feedback success judgment on round event")
	}
}

func TestWriteMetricNote(t *testing.T) {
	d, sid := metricTestDaemon(t)

	// 空文本静默跳过
	if got := d.writeMetricNote(sid, "tester", "   "); got != "" {
		t.Errorf("empty text = %q, want empty", got)
	}
	// 无法定位项目目录（不存在的会话）静默跳过
	if got := d.writeMetricNote("ghost-session", "tester", "hello"); got != "" {
		t.Errorf("ghost session = %q, want empty", got)
	}

	// 正常落盘
	path := d.writeMetricNote(sid, "tester", "喜欢这个回答")
	if path == "" {
		t.Fatal("expected note path")
	}
	if fi, err := os.Stat(path); err != nil || fi.IsDir() {
		t.Fatalf("note file missing: %v", err)
	}
	// 文件应位于会话项目目录的 .agents/notes 下
	if !strings.Contains(filepath.Base(path), "tester-偏好-用户主观反馈.md") {
		t.Errorf("note name = %q", filepath.Base(path))
	}
}

func TestHandleAgentRate(t *testing.T) {
	d, _ := metricTestDaemon(t)
	ctx := context.Background()

	// 参数校验
	if _, err := d.handleAgentRate(ctx, mustParams(t, agentRateParams{Name: "", Value: "up"})); err == nil {
		t.Error("empty name: expected error")
	}
	if _, err := d.handleAgentRate(ctx, mustParams(t, agentRateParams{Name: "alpha", Value: "side"})); err == nil {
		t.Error("invalid value: expected error")
	}
	if _, err := d.handleAgentRate(ctx, mustParams(t, agentRateParams{Name: "alpha", Value: "up", SubjectType: "model"})); err == nil {
		t.Error("invalid subject_type: expected error")
	}

	// 首次评价成功
	result, err := d.handleAgentRate(ctx, mustParams(t, agentRateParams{SubjectType: "agent", Name: "alpha", Value: "up", Text: "很稳"}))
	if err != nil {
		t.Fatalf("handleAgentRate error = %v", err)
	}
	if result.(map[string]any)["ok"] != true {
		t.Errorf("result = %#v, want ok=true", result)
	}

	// 当日重复评价 → 限频拒绝（ok=false + already_rated_today）
	result, err = d.handleAgentRate(ctx, mustParams(t, agentRateParams{SubjectType: "agent", Name: "alpha", Value: "down"}))
	if err != nil {
		t.Fatalf("duplicate rate error = %v", err)
	}
	m := result.(map[string]any)
	if m["ok"] != false || m["reason"] != "already_rated_today" {
		t.Errorf("duplicate rate = %#v, want already_rated_today", m)
	}

	// 不同对象不受限频影响；skill/team 各成功一次
	for _, c := range []struct{ subject, name string }{
		{"agent", "beta"}, {"skill", "skill-a"}, {"team", "team-a"},
	} {
		if _, err := d.handleAgentRate(ctx, mustParams(t, agentRateParams{SubjectType: c.subject, Name: c.name, Value: "up"})); err != nil {
			t.Errorf("rate %s/%s error = %v", c.subject, c.name, err)
		}
	}
}

func TestHandleMetricRecord(t *testing.T) {
	d, sid := metricTestDaemon(t)
	ctx := context.Background()

	// 参数校验
	if _, err := d.handleMetricRecord(ctx, mustParams(t, metricRecordParams{SessionID: sid, Outcome: "so-so"})); err == nil {
		t.Error("invalid outcome: expected error")
	}
	if _, err := d.handleMetricRecord(ctx, mustParams(t, metricRecordParams{Outcome: "success"})); err == nil {
		t.Error("empty session_id: expected error")
	}

	// 成功上报 → agent_report 判定
	result, err := d.handleMetricRecord(ctx, mustParams(t, metricRecordParams{SessionID: sid, Outcome: "success", Note: "done"}))
	if err != nil {
		t.Fatalf("handleMetricRecord error = %v", err)
	}
	if m := result.(map[string]any); m["ok"] != true || m["revisited"].(int) != 0 {
		t.Errorf("result = %#v, want ok=true revisited=0", m)
	}

	// 返工上报 + 同任务意图的历史会话 → 回溯 revisited
	store := d.app.MetricStore()
	// 追加带 TaskHint 的最新 round 事件（latestRoundEvent 取它为定位目标）
	if _, err := store.AppendEvent(ctx, metric.MetricEvent{
		Kind: metric.KindRound, SessionID: sid, AgentName: "tester", TaskHint: "fix-login", RoundNo: 3,
	}); err != nil {
		t.Fatalf("append task-hint event: %v", err)
	}

	// 另一个会话：同 Agent 同 TaskHint
	otherSid := mustCreateSession(t, d.app.SessDB(), "tester")
	if _, err := store.AppendEvent(ctx, metric.MetricEvent{
		Kind: metric.KindRound, SessionID: otherSid, AgentName: "tester", TaskHint: "fix-login",
	}); err != nil {
		t.Fatalf("append other session event: %v", err)
	}

	result, err = d.handleMetricRecord(ctx, mustParams(t, metricRecordParams{SessionID: sid, Outcome: "rework"}))
	if err != nil {
		t.Fatalf("rework record error = %v", err)
	}
	if m := result.(map[string]any); m["revisited"].(int) < 1 {
		t.Errorf("revisited = %#v, want >=1", m["revisited"])
	}

	// 历史会话的 round 事件应带 rework_trace 判定
	prior, _ := store.Query(ctx, metric.MetricFilter{SessionID: otherSid, Kind: metric.KindRound})
	if len(prior) == 0 {
		t.Fatal("prior session event missing")
	}
	j, ok := prior[0].LatestJudgment()
	if !ok || j.Source != metric.SourceReworkTrace || j.Value != metric.OutcomeRework {
		t.Errorf("prior judgment = %+v ok=%v, want rework_trace/rework", j, ok)
	}
}

func TestTraceReworkToPriorSessions(t *testing.T) {
	d, sid := metricTestDaemon(t)
	store := d.app.MetricStore()
	ctx := context.Background()

	target := metric.MetricEvent{
		Kind: metric.KindRound, SessionID: sid, AgentName: "tester", TaskHint: "task-x", RoundNo: 3,
	}
	target, _ = store.AppendEvent(ctx, target)

	// 同 Agent 同 TaskHint 的历史会话
	otherSid := mustCreateSession(t, d.app.SessDB(), "tester")
	if _, err := store.AppendEvent(ctx, metric.MetricEvent{
		Kind: metric.KindRound, SessionID: otherSid, AgentName: "tester", TaskHint: "task-x",
	}); err != nil {
		t.Fatalf("append other session event: %v", err)
	}
	// 不同 TaskHint 的会话不受影响
	unrelatedSid := mustCreateSession(t, d.app.SessDB(), "tester")
	if _, err := store.AppendEvent(ctx, metric.MetricEvent{
		Kind: metric.KindRound, SessionID: unrelatedSid, AgentName: "tester", TaskHint: "other-task",
	}); err != nil {
		t.Fatalf("append unrelated session event: %v", err)
	}

	if got := d.traceReworkToPriorSessions(store, target); got != 1 {
		t.Errorf("revisited = %d, want 1", got)
	}

	// 历史会话的 round 事件应带 rework_trace 判定（重读 store 验证）
	evs, _ := store.Query(ctx, metric.MetricFilter{SessionID: otherSid, Kind: metric.KindRound})
	j, ok := evs[0].LatestJudgment()
	if !ok || j.Source != metric.SourceReworkTrace || j.Value != metric.OutcomeRework {
		t.Errorf("prior judgment = %+v ok=%v, want rework_trace/rework", j, ok)
	}
	// 不同 TaskHint 的会话不带回溯判定
	unrelated, _ := store.Query(ctx, metric.MetricFilter{SessionID: unrelatedSid, Kind: metric.KindRound})
	if _, ok := unrelated[0].LatestJudgment(); ok {
		t.Error("unrelated task hint should not be traced")
	}
}
