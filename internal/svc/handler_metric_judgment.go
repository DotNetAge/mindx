package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/DotNetAge/mindx/pkg/metric"
)

// 本文件是决策量表的判定层（白皮书《决策量表：Agent 生态的绩效系统》第 7 章）：
// 三个判定入口 + 限频 + 回溯修正 + 文本汇入双系统。
//
// 判定优先级（7.4）：结构化用户反馈 > 用户文本 > Agent 上报 > 中性不计。
// 数据实现：同一事件的判定版本链按时间取最新为生效判定（LatestJudgment），
// 来源字段仅作统计维度——用户判定天然发生在 Agent 上报之后（轮次结束先于收尾）。

// ── round.rating：壳侧轮次赞/踩（主收集，7.2 四层收集面第一层） ──────────

type roundRatingParams struct {
	SessionID string `json:"session_id"`
	Value     string `json:"value"` // "up" | "down"
	Text      string `json:"text,omitempty"`
}

func (d *Daemon) handleRoundRating(_ context.Context, params json.RawMessage) (any, error) {
	var p roundRatingParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}
	outcome, err := ratingToOutcome(p.Value)
	if err != nil {
		return nil, err
	}

	store := d.app.MetricStore()
	// 定位：该会话时间最新的 round 事件——评价发生在轮次结束后立刻。
	//
	// 但「结束后立刻」存在一个窄窗口：若用户在最后一轮仍在输出时就点了赞踩，
	// ExecutionSummary 尚未发射（它在 runWithGoAgent 的 defer 里），该轮事件
	// 还不存在，latestRoundEvent 会命中**上一轮**——与 metric.record 同一类
	// 错位。故与 Agent 上报共用暂存机制：轮内暂存，轮末挂载。
	if d.sessionQueueFor(p.SessionID).Busy() {
		d.stashPendingJudgment(p.SessionID, pendingJudgment{
			Outcome: outcome,
			Source:  metric.SourceUserFeedback,
			At:      time.Now(),
		})
		notePath := d.writeMetricNote(p.SessionID, "", p.Text)
		d.logger.Info("metric: 轮次未结束时收到评价，暂存待该轮事件落盘后挂载",
			"session_id", p.SessionID, "value", p.Value)
		resp := map[string]any{"ok": true, "deferred": true}
		if notePath != "" {
			resp["note_path"] = notePath
		}
		return resp, nil
	}

	target, err := latestRoundEvent(store, p.SessionID)
	if err != nil {
		return nil, err
	}
	if err := store.AppendJudgment(context.Background(), target.ID, metric.JudgmentRecord{
		Value:  outcome,
		Source: metric.SourceUserFeedback,
		At:     time.Now(),
	}); err != nil {
		return nil, err
	}

	// 文本感受汇入双系统：偏好类 notes（白皮书 7.2 要点 3）。
	if notePath := d.writeMetricNote(p.SessionID, target.AgentName, p.Text); notePath != "" {
		return map[string]any{"ok": true, "note_path": notePath}, nil
	}
	return map[string]any{"ok": true}, nil
}

// ── agent.rate：管理页整体评价（辅收集，限频） ──────────────────────────

type agentRateParams struct {
	SubjectType string `json:"subject_type"` // agent | team | skill
	Name        string `json:"name"`
	Value       string `json:"value"` // "up" | "down"
	Text        string `json:"text,omitempty"`
}

func (d *Daemon) handleAgentRate(_ context.Context, params json.RawMessage) (any, error) {
	var p agentRateParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	outcome, err := ratingToOutcome(p.Value)
	if err != nil {
		return nil, err
	}

	store := d.app.MetricStore()

	// 限频（附录 B 第 9 条，v1 口径：同对象每日一次）：当日已有该对象的
	// overall 判定即拒绝——控制评价密度，防冲动评价扰动数据。
	dayStart := time.Now().Truncate(24 * time.Hour)
	existing, err := store.Query(context.Background(), metric.MetricFilter{
		Kind: metric.KindOverall, Since: dayStart,
	})
	if err != nil {
		return nil, err
	}
	for _, ev := range existing {
		if overallSubjectMatches(ev, p.SubjectType, p.Name) {
			return map[string]any{
				"ok":     false,
				"reason": "already_rated_today",
			}, nil
		}
	}

	ev := metric.MetricEvent{Kind: metric.KindOverall}
	// 对象复用归属字段（kind=overall 时为评价对象）：
	// agent → AgentName；skill → SkillName；team → Team。
	switch p.SubjectType {
	case "agent":
		ev.AgentName = p.Name
	case "skill":
		ev.SkillName = p.Name
	case "team":
		ev.Team = p.Name
	default:
		return nil, fmt.Errorf("subject_type must be agent, team, or skill")
	}
	ev, err = store.AppendEvent(context.Background(), ev)
	if err != nil {
		return nil, err
	}
	if err := store.AppendJudgment(context.Background(), ev.ID, metric.JudgmentRecord{
		Value:  outcome,
		Source: metric.SourceUserFeedback,
		At:     time.Now(),
		Note:   p.Text, // 管理页评价无会话上下文，文本仅存事件 Note（不出网）
	}); err != nil {
		return nil, err
	}

	return map[string]any{"ok": true}, nil
}

// ── metric.record：Agent 收尾上报（降级判定源） ─────────────────────────

type metricRecordParams struct {
	SessionID string `json:"session_id"`
	Outcome   string `json:"outcome"` // success | fail | rework
	Note      string `json:"note,omitempty"`
}

func (d *Daemon) handleMetricRecord(_ context.Context, params json.RawMessage) (any, error) {
	var p metricRecordParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	outcome := metric.Outcome(p.Outcome)
	switch outcome {
	case metric.OutcomeSuccess, metric.OutcomeFail, metric.OutcomeRework:
	default:
		return nil, fmt.Errorf("outcome must be success, fail, or rework")
	}
	if p.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	store := d.app.MetricStore()

	// 轮次内上报：目标轮次的 round 事件此刻尚不存在，不能就地定位。
	//
	// round 事件由 withMetricHook 在 OnExecutionSummary 里落盘，而该事件在
	// runWithGoAgent 的 defer 中发射——即整轮结束后才写。Agent 却是在轮内
	// 调本接口的。若此处走 latestRoundEvent，必然错位：第 1 轮（新会话）
	// 直接找不到事件而丢失判定，第 N 轮则落到第 N-1 轮上（实测样本：判定
	// 讲的是 round 10 之后的事，却挂在 round 9「社区就有这个技能」上）。
	//
	// 因此轮内上报只暂存，由该轮 round 事件落盘后的 flushPendingJudgments
	// 挂载——判定与它评的那一轮严格对齐，task_hint 不再错配。
	if d.sessionQueueFor(p.SessionID).Busy() {
		pending := pendingJudgment{
			Outcome: outcome,
			Source:  metric.SourceAgentReport,
			Note:    p.Note,
			At:      time.Now(), // 取轮内时刻：用户轮末反馈天然更晚，仍按时间覆盖
		}
		d.stashPendingJudgment(p.SessionID, pending)
		d.logger.Info("metric: 轮次内上报，判定暂存待该轮事件落盘后挂载",
			"session_id", p.SessionID, "outcome", outcome)
		return map[string]any{"ok": true, "revisited": 0, "deferred": true}, nil
	}

	// 轮次外上报（补记历史、复盘对旧会话下判定）：目标轮次已落盘，就地定位。
	target, err := latestRoundEvent(store, p.SessionID)
	if err != nil {
		return nil, err
	}
	if err := store.AppendJudgment(context.Background(), target.ID, metric.JudgmentRecord{
		Value:  outcome,
		Source: metric.SourceAgentReport,
		At:     time.Now(),
		Note:   p.Note,
	}); err != nil {
		return nil, err
	}

	// 返工回溯（白皮书 4.3 判定版本化）：本会话被判定为返工时，
	// 同 Agent、同任务意图的历史会话一并标记——修正其乐观的"成功"判定。
	revisited := 0
	if outcome == metric.OutcomeRework && target.TaskHint != "" {
		revisited = d.traceReworkToPriorSessions(store, target)
	}
	return map[string]any{"ok": true, "revisited": revisited}, nil
}

// ── 轮次内判定的暂存与挂载 ───────────────────────────────────────────────

// pendingJudgment 是一次待挂载的判定（轮内上报时暂存）。
type pendingJudgment struct {
	Outcome metric.Outcome
	Source  metric.OutcomeSource
	Note    string
	At      time.Time
}

// stashPendingJudgment 把判定挂到该会话的待挂载队列上。
// 同一会话一轮内多次上报时按追加顺序保留，全部挂到该轮事件上（版本链语义，
// 最新者生效），不丢任何一条。
func (d *Daemon) stashPendingJudgment(sessionID string, pj pendingJudgment) {
	v, _ := d.pendingJudgments.LoadOrStore(sessionID, &pendingJudgmentList{})
	list := v.(*pendingJudgmentList)
	list.mu.Lock()
	list.items = append(list.items, pj)
	list.mu.Unlock()
}

// takePendingJudgments 取出并清空该会话的待挂载判定。
func (d *Daemon) takePendingJudgments(sessionID string) []pendingJudgment {
	v, ok := d.pendingJudgments.Load(sessionID)
	if !ok {
		return nil
	}
	d.pendingJudgments.Delete(sessionID)
	list := v.(*pendingJudgmentList)
	list.mu.Lock()
	defer list.mu.Unlock()
	return list.items
}

// pendingJudgmentCount 返回该会话暂存的判定条数（只读，不消费）。
func (d *Daemon) pendingJudgmentCount(sessionID string) int {
	v, ok := d.pendingJudgments.Load(sessionID)
	if !ok {
		return 0
	}
	list := v.(*pendingJudgmentList)
	list.mu.Lock()
	defer list.mu.Unlock()
	return len(list.items)
}

// flushPendingJudgments 在该轮 round 事件落盘后立即挂载轮内上报的判定。
// 由 withMetricHook 的 OnExecutionSummary 在该事件写入成功后调用。
// 这是轮次归因的唯一正确挂载点：此刻事件已存在，且正是 Agent 所处的这一轮。
func (d *Daemon) flushPendingJudgments(store *metric.FileMetricStore, ev metric.MetricEvent) {
	items := d.takePendingJudgments(ev.SessionID)
	if len(items) == 0 {
		return
	}
	var lastRework bool
	for _, pj := range items {
		if err := store.AppendJudgment(context.Background(), ev.ID, metric.JudgmentRecord{
			Value:  pj.Outcome,
			Source: pj.Source,
			At:     pj.At,
			Note:   pj.Note,
		}); err != nil {
			d.logger.Warn("metric: 挂载轮内判定失败",
				"session_id", ev.SessionID, "event_id", ev.ID, "error", err)
			continue
		}
		if pj.Outcome == metric.OutcomeRework {
			lastRework = true
		}
	}
	// 返工回溯延后到此刻执行：它依赖本轮的 task_hint 与 agent 归属，
	// 而这两个字段只有事件落盘后才确定。
	if lastRework && ev.TaskHint != "" {
		d.traceReworkToPriorSessions(store, ev)
	}
}

// pendingJudgmentList 是单个会话的待挂载判定队列（sync.Map 值）。
type pendingJudgmentList struct {
	mu    sync.Mutex
	items []pendingJudgment
}

// traceReworkToPriorSessions 把返工回溯到同 Agent、同任务意图的其他会话：
// 向其最近 round 事件追加 rework_trace 判定（版本链追加，不改写历史）。
// 返回受影响会话数。
//
// 匹配域排除系统生成文本：TaskHint 取的是「该轮用户说了什么」，但有一部分轮次
// 没有用户消息，task_hint 落的是系统注入的收割提示（实测 216 个 round 事件里
// 36 个即 16.7% 以「[系统收割]」开头）。这类文本在多个不相关会话里必然逐字相同，
// 一旦参与匹配就会把互不相干的会话串成同一任务意图——一次返工把几十个无关会话
// 全判成 rework。宁可漏配，不可错配。
func (d *Daemon) traceReworkToPriorSessions(store *metric.FileMetricStore, target metric.MetricEvent) int {
	// target 本身是系统文本时无从判断任务意图，直接不回溯。
	if isSystemTaskHint(target.TaskHint) {
		d.logger.Info("metric: 目标任务名为系统生成文本，跳过返工回溯",
			"session_id", target.SessionID)
		return 0
	}
	prior, err := store.Query(context.Background(), metric.MetricFilter{
		Kind: metric.KindRound, AgentName: target.AgentName,
	})
	if err != nil {
		return 0
	}
	affected := map[string]bool{}
	for _, ev := range prior {
		if ev.SessionID == target.SessionID || ev.TaskHint != target.TaskHint {
			continue
		}
		if isSystemTaskHint(ev.TaskHint) {
			continue
		}
		if err := store.AppendJudgment(context.Background(), ev.ID, metric.JudgmentRecord{
			Value:  metric.OutcomeRework,
			Source: metric.SourceReworkTrace,
			At:     time.Now(),
			Note:   fmt.Sprintf("同任务意图重开：见会话 %s", target.SessionID),
		}); err != nil {
			d.logger.Warn("metric: 返工回溯失败", "session_id", ev.SessionID, "error", err)
			continue
		}
		affected[ev.SessionID] = true
	}
	return len(affected)
}

// ── 内部工具 ─────────────────────────────────────────────────────────────

// systemTaskHintPrefixes 是系统注入文本的前缀。这些轮次没有用户消息，
// task_hint 落的是框架自己生成的提示，不代表用户任务意图。
var systemTaskHintPrefixes = []string{"[系统收割]", "[系统", "[收割]"}

// isSystemTaskHint 判断任务名是否为系统生成文本。
// 用途：把系统文本排除出返工回溯的匹配域（见 traceReworkToPriorSessions）。
func isSystemTaskHint(hint string) bool {
	h := strings.TrimSpace(hint)
	if h == "" {
		return true // 空任务名同样不可比
	}
	for _, p := range systemTaskHintPrefixes {
		if strings.HasPrefix(h, p) {
			return true
		}
	}
	return false
}

// latestRoundEvent 返回会话时间最新的 round 事件（评价/上报的定位目标）。
func latestRoundEvent(store *metric.FileMetricStore, sessionID string) (metric.MetricEvent, error) {
	events, err := store.Query(context.Background(), metric.MetricFilter{
		SessionID: sessionID, Kind: metric.KindRound,
	})
	if err != nil {
		return metric.MetricEvent{}, err
	}
	if len(events) == 0 {
		return metric.MetricEvent{}, fmt.Errorf("no metric round event for session %q", sessionID)
	}
	sort.Slice(events, func(i, j int) bool { return events[i].TS.After(events[j].TS) })
	return events[0], nil
}

// ratingToOutcome 把赞/踩映射为判定：up → success、down → fail。
func ratingToOutcome(v string) (metric.Outcome, error) {
	switch v {
	case "up", "like":
		return metric.OutcomeSuccess, nil
	case "down", "dislike":
		return metric.OutcomeFail, nil
	default:
		return "", fmt.Errorf("value must be up or down")
	}
}

// overallSubjectMatches 判断 overall 事件是否属于指定对象（限频查询）。
func overallSubjectMatches(ev metric.MetricEvent, subjectType, name string) bool {
	if len(ev.Judgments) == 0 {
		return false
	}
	switch subjectType {
	case "agent":
		return ev.AgentName == name
	case "skill":
		return ev.SkillName == name
	case "team":
		return ev.Team == name
	}
	return false
}

// writeMetricNote 把用户文本感受写入项目 notes（偏好类，四元组命名），
// 汇入既有双系统记录流（白皮书 7.2 要点 3）。text 为空或无法定位项目目录
// 时返回空串（静默跳过——文本是可选补充，不阻塞评价主流程）。
func (d *Daemon) writeMetricNote(sessionID, agentName, text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	projectDir := ""
	if meta, err := d.app.SessDB().GetMeta(context.Background(), sessionID); err == nil {
		projectDir = meta.ProjectDir
	}
	if projectDir == "" {
		return ""
	}

	agentsDir := filepath.Join(projectDir, ".agents", "notes")
	name := fmt.Sprintf("%s-%s-偏好-用户主观反馈.md",
		time.Now().Format("20060102-150405"), agentName)
	path := filepath.Join(agentsDir, name)

	content := fmt.Sprintf("# 用户主观反馈（%s）\n\n- 记录时间：%s\n- Agent：%s\n- 来源：决策量表评价\n\n%s\n",
		time.Now().Format("2006-01-02 15:04:05"), time.Now().Format("2006-01-02 15:04:05"), agentName, text)

	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		d.logger.Warn("metric: 写入 notes 失败（无法创建目录）", "error", err)
		return ""
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		d.logger.Warn("metric: 写入 notes 失败", "path", path, "error", err)
		return ""
	}
	return path
}
