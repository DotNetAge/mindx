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

// TestIsSystemTaskHint 覆盖系统生成文本的判定边界。
// 这类文本在多个不相关会话里必然逐字相同，一旦进入匹配域就会造成误命中。
func TestIsSystemTaskHint(t *testing.T) {
	cases := []struct {
		hint string
		want bool
	}{
		{"[系统收割] 以下子任务已结算（或达到各自收割时长上限）……", true},
		{"[系统] 定时任务触发", true},
		{"", true},
		{"   ", true},
		{"帮我重构这个模块的存储层", false},
		{"宠物事件链路验收A：请只回复两个字——好的", false},
	}
	for _, c := range cases {
		if got := isSystemTaskHint(c.hint); got != c.want {
			t.Errorf("isSystemTaskHint(%q) = %v, want %v", c.hint, got, c.want)
		}
	}
}

// TestTraceReworkSkipsSystemTaskHint 验证返工回溯不会因系统文本把无关会话串成一串。
// 回归的是实测缺陷：216 个 round 事件里 36 个（16.7%）task_hint 是「[系统收割]」，
// 精确匹配能命中它们，等于一次返工把一批无关会话全判成 rework。
func TestTraceReworkSkipsSystemTaskHint(t *testing.T) {
	d, sid := metricTestDaemon(t)
	store := d.app.MetricStore()
	ctx := context.Background()

	const sysHint = "[系统收割] 以下子任务已结算，是它们的结果汇总。请基于这些结果继续推进。"
	target := metric.MetricEvent{
		Kind: metric.KindRound, SessionID: sid, AgentName: "tester", TaskHint: sysHint, RoundNo: 1,
	}
	target, _ = store.AppendEvent(ctx, target)

	// 两个与本任务毫无关系、但 task_hint 同为系统文本的会话
	var others []string
	for i := 0; i < 2; i++ {
		oid := mustCreateSession(t, d.app.SessDB(), "tester")
		if _, err := store.AppendEvent(ctx, metric.MetricEvent{
			Kind: metric.KindRound, SessionID: oid, AgentName: "tester", TaskHint: sysHint,
		}); err != nil {
			t.Fatalf("append unrelated event: %v", err)
		}
		others = append(others, oid)
	}

	if got := d.traceReworkToPriorSessions(store, target); got != 0 {
		t.Errorf("revisited = %d, want 0（系统文本不得回溯）", got)
	}
	for _, oid := range others {
		evs, _ := store.Query(ctx, metric.MetricFilter{SessionID: oid, Kind: metric.KindRound})
		if j, ok := evs[0].LatestJudgment(); ok {
			t.Errorf("会话 %s 被误判为 %s/%s", oid, j.Value, j.Source)
		}
	}
}

// TestMetricRecordRoundAttribution 固定轮次归因的正确语义。
//
// 缺陷根源是写入时序，不是「只能落在最新一轮」这句话能覆盖的：
// round 事件由 withMetricHook 在 OnExecutionSummary 里落盘，而该事件在
// runWithGoAgent 的 defer 中发射，即**整轮结束后**才写入；Agent 却是在
// **轮次之内**调 metric record 的。若就地 latestRoundEvent：
//
//  1. 新会话第 1 轮内上报 → 该会话尚无任何 round 事件 → 判定整条丢失。
//  2. 多轮会话第 N 轮内上报 → 只能定位到第 N-1 轮，判定落到上一轮的事件上，
//     task_hint 与判定的 note 必然错配一轮（真实数据里就是这样：判定讲的是
//     round 10 之后的事，却挂在 round 9「社区就有这个技能」上）。
//
// 修正：轮内上报只暂存，由该轮事件落盘后的 flushPendingJudgments 挂载。
func TestMetricRecordRoundAttribution(t *testing.T) {
	d, sid := metricTestDaemon(t)
	ctx := context.Background()
	store := d.app.MetricStore()

	// ── 面 1：新会话第 1 轮内上报，此时该会话零 round 事件 ──
	freshSid := mustCreateSession(t, d.app.SessDB(), "tester")
	d.sessionQueueFor(freshSid).Enqueue(func() { select {} }) // 标记为执行中
	result, err := d.handleMetricRecord(ctx, mustParams(t, metricRecordParams{
		SessionID: freshSid, Outcome: "fail", Note: "问题：… 否决：… 修正：… 验证：…",
	}))
	if err != nil {
		t.Fatalf("轮内上报不应报错（应暂存而非丢失）：%v", err)
	}
	if m := result.(map[string]any); m["deferred"] != true {
		t.Errorf("轮内上报应标记 deferred，实际 %#v", m)
	}
	// 暂存后该会话仍无 round 事件——判定确实还没落地，等轮末挂载
	if evs, _ := store.Query(ctx, metric.MetricFilter{SessionID: freshSid, Kind: metric.KindRound}); len(evs) != 0 {
		t.Errorf("暂存阶段不应凭空产生 round 事件，实际 %d 条", len(evs))
	}

	// 轮末事件落盘 → 判定挂到**这一轮**（round 1），not 丢失
	first, err := store.AppendEvent(ctx, metric.MetricEvent{
		Kind: metric.KindRound, SessionID: freshSid, AgentName: "tester",
		RoundNo: 1, TaskHint: "第一轮的任务",
	})
	if err != nil {
		t.Fatalf("append round 1: %v", err)
	}
	d.flushPendingJudgments(store, first)

	evs, _ := store.Query(ctx, metric.MetricFilter{SessionID: freshSid, Kind: metric.KindRound})
	if len(evs) != 1 {
		t.Fatalf("round 事件数 = %d, want 1", len(evs))
	}
	j, ok := evs[0].LatestJudgment()
	if !ok {
		t.Fatal("轮末应挂载暂存的判定，实际判定丢失")
	}
	if j.Value != metric.OutcomeFail || j.Source != metric.SourceAgentReport {
		t.Errorf("挂载的判定 = %s/%s, want fail/agent_report", j.Value, j.Source)
	}

	// ── 面 2：多轮会话，在第 2 轮内上报 ──
	// 第 1 轮已落盘
	if _, err := store.AppendEvent(ctx, metric.MetricEvent{
		Kind: metric.KindRound, SessionID: sid, AgentName: "tester",
		RoundNo: 1, TaskHint: "上一轮的任务",
	}); err != nil {
		t.Fatalf("append round 1: %v", err)
	}
	d.sessionQueueFor(sid).Enqueue(func() { select {} }) // 第 2 轮执行中
	if _, err := d.handleMetricRecord(ctx, mustParams(t, metricRecordParams{
		SessionID: sid, Outcome: "fail", Note: "本轮（第 2 轮）才失败的证据",
	})); err != nil {
		t.Fatalf("record error = %v", err)
	}
	// 此刻第 1 轮不得被写入判定（否则就是错配一轮）
	round1, _ := store.Query(ctx, metric.MetricFilter{SessionID: sid, Kind: metric.KindRound})
	for _, ev := range round1 {
		if ev.RoundNo == 1 {
			if _, ok := ev.LatestJudgment(); ok {
				t.Error("第 2 轮内的判定被错写到第 1 轮——错配一轮的缺陷复现")
			}
		}
	}
	// 第 2 轮事件落盘 → 判定挂到第 2 轮
	second, err := store.AppendEvent(ctx, metric.MetricEvent{
		Kind: metric.KindRound, SessionID: sid, AgentName: "tester",
		RoundNo: 2, TaskHint: "第 2 轮的任务",
	})
	if err != nil {
		t.Fatalf("append round 2: %v", err)
	}
	d.flushPendingJudgments(store, second)

	all, _ := store.Query(ctx, metric.MetricFilter{SessionID: sid, Kind: metric.KindRound})
	var withJudgment []int
	for _, ev := range all {
		if _, ok := ev.LatestJudgment(); ok {
			withJudgment = append(withJudgment, ev.RoundNo)
		}
	}
	if len(withJudgment) != 1 || withJudgment[0] != 2 {
		t.Errorf("判定应且仅应落在 round 2，实际落在 %v", withJudgment)
	}
}

// TestRecordRoundEventMountsStashedJudgment 走生产路径验证「轮内暂存 → 轮末挂载」。
//
// 与 TestMetricRecordRoundAttribution 的分工：那个测试手工调 flushPendingJudgments，
// 只证明挂载逻辑本身正确；本测试经由 recordRoundEvent（withMetricHook 的真实
// 落盘路径）触发，证明这条因果是**接上的**——否则 flush 写成死代码也照样全绿。
func TestRecordRoundEventMountsStashedJudgment(t *testing.T) {
	d, sid := metricTestDaemon(t)
	ctx := context.Background()
	store := d.app.MetricStore()

	// 清掉 metricTestDaemon 预置的 round 1，让轮号从 1 起算更直观
	d.sessionQueueFor(sid).Enqueue(func() { select {} }) // 会话执行中

	const note = "问题：错把缓存失效当成数据丢失 否决：加大超时 修正：查 cursor 语义 验证：重跑通过"
	// 轮内上报：暂存，不落盘
	if _, err := d.handleMetricRecord(ctx, mustParams(t, metricRecordParams{
		SessionID: sid, Outcome: "rework", Note: note,
	})); err != nil {
		t.Fatalf("轮内上报 error = %v", err)
	}
	// 暂存后尚未落盘，flush 也未发生
	if got := d.pendingJudgmentCount(sid); got != 1 {
		t.Fatalf("暂存判定数 = %d, want 1", got)
	}

	// 轮末：走生产落盘路径
	saved, err := d.recordRoundEvent(store, roundEventInput{
		SessionID: sid, AgentName: "tester", TaskHint: "修 cursor 缺陷",
	})
	if err != nil {
		t.Fatalf("recordRoundEvent error = %v", err)
	}

	// 判定必须挂到这条事件上，且暂存队列清空
	evs, _ := store.Query(ctx, metric.MetricFilter{SessionID: sid, Kind: metric.KindRound})
	var target *metric.MetricEvent
	for i := range evs {
		if evs[i].ID == saved.ID {
			target = &evs[i]
		}
	}
	if target == nil {
		t.Fatal("落盘的 round 事件未查到")
	}
	j, ok := target.LatestJudgment()
	if !ok {
		t.Fatal("轮末未挂载暂存的判定——flush 未接线")
	}
	if j.Value != metric.OutcomeRework || j.Source != metric.SourceAgentReport {
		t.Errorf("挂载判定 = %s/%s, want rework/agent_report", j.Value, j.Source)
	}
	if !strings.Contains(j.Note, "问题：") || !strings.Contains(j.Note, "验证：") {
		t.Errorf("note 应随判定完整挂载，实际 %q", j.Note)
	}
	if left := d.pendingJudgmentCount(sid); left != 0 {
		t.Errorf("挂载后暂存队列应清空，剩 %d 条", left)
	}
}

// TestRoundEventNumbersAreSequential 验证轮号按存量递增（判定定位依赖它）。
func TestRoundEventNumbersAreSequential(t *testing.T) {
	d, sid := metricTestDaemon(t)
	store := d.app.MetricStore()

	// metricTestDaemon 已预置 round 1
	saved, err := d.recordRoundEvent(store, roundEventInput{
		SessionID: sid, AgentName: "tester", TaskHint: "第二轮",
	})
	if err != nil {
		t.Fatalf("recordRoundEvent error = %v", err)
	}
	if saved.RoundNo != 2 {
		t.Errorf("轮号 = %d, want 2（存量 1 条 + 1）", saved.RoundNo)
	}
}
