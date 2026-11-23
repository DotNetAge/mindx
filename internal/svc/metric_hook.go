package svc

import (
	"context"
	"strings"
	"time"

	agents "github.com/DotNetAge/goharness/agents"
	goharnessevents "github.com/DotNetAge/goharness/events"
	goharnesssession "github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/mindx/pkg/metric"
)

// metricRoundAccum 是同一用户轮被 AskUser / 授权挂起切开后的前段执行快照。
// 挂起段本身不是一轮：它的 token、时长、迭代数与技能序列要并入最终段，
// 否则后端 round 号会按「执行段」膨胀，与用户看到的轮次脱节。
type metricRoundAccum struct {
	TaskHint        string
	Tokens          int
	Duration        time.Duration
	Iterations      int
	Skills          []string
	RouteCorrection int
}

// stashMetricAccum 累积一个挂起段的执行量。
func (d *Daemon) stashMetricAccum(sid, taskHint string, data goharnessevents.ExecutionSummaryData, routeCorrection int, skills []string) {
	d.metricAccumMu.Lock()
	defer d.metricAccumMu.Unlock()
	if d.metricAccums == nil {
		d.metricAccums = map[string]*metricRoundAccum{}
	}
	a := d.metricAccums[sid]
	if a == nil {
		a = &metricRoundAccum{TaskHint: taskHint}
		d.metricAccums[sid] = a
	}
	if a.TaskHint == "" {
		a.TaskHint = taskHint
	}
	a.Tokens += data.TokensUsed.ActualTokens()
	a.Duration += data.TotalDuration
	a.Iterations += data.TotalIterations
	a.Skills = append(a.Skills, skills...)
	a.RouteCorrection += routeCorrection
}

// takeMetricAccum 取出并清空挂起段累计；没有累计时返回 nil。
func (d *Daemon) takeMetricAccum(sid string) *metricRoundAccum {
	d.metricAccumMu.Lock()
	defer d.metricAccumMu.Unlock()
	if d.metricAccums == nil {
		return nil
	}
	a := d.metricAccums[sid]
	delete(d.metricAccums, sid)
	return a
}

// clearMetricAccum 在真实新提问开始时清掉上一轮残留的挂起段累计。
func (d *Daemon) clearMetricAccum(sid string) {
	d.metricAccumMu.Lock()
	defer d.metricAccumMu.Unlock()
	if d.metricAccums != nil {
		delete(d.metricAccums, sid)
	}
}

// isSuspendedSummaryReason 判断本次执行是否以「等待用户输入」结束：
// AskUser 与工具授权都会收尾当前 Ask，但都属于同一用户轮的中断点，
// 不能各自落一条 round 事件。
func isSuspendedSummaryReason(reason string) bool {
	return reason == "ask_user_pending" || reason == "permission_pending"
}

// mergeMetricAccum 把挂起段的执行量并入最终段：token 取净量相加，
// 时长/迭代/技能序列累加，任务意图保留本轮最初的用户提问。
func mergeMetricAccum(
	data *goharnessevents.ExecutionSummaryData,
	taskHint *string,
	routeCorrection *int,
	skillsUsed *[]string,
	a *metricRoundAccum,
) {
	// round 事件只按净 token 计量：把前段净量并入 prompt 槽，
	// ActualTokens 会算出两段之和。
	data.TokensUsed = goharnesssession.TokenUsage{
		PromptTokens: a.Tokens + data.TokensUsed.ActualTokens(),
	}
	data.TotalDuration += a.Duration
	data.TotalIterations += a.Iterations
	*routeCorrection += a.RouteCorrection
	*skillsUsed = append(a.Skills, *skillsUsed...)
	if a.TaskHint != "" {
		*taskHint = a.TaskHint
	}
}

// withMetricHook 在 AskBuilder 上追加决策计量埋点
// （白皮书《决策量表：Agent 生态的绩效系统》第 4 章采集层）。
// AskBuilder.OnXxx 为追加语义，与既有 emitter 并存互不影响。
//
// 闭包状态覆盖单次 Ask 生命周期：
//   - ToolStart 累积技能调用序列（工具名 "Skill"，参数 name 为技能名），
//     并统计相邻技能切换次数（q_route_correction 的 v1 简化口径）；
//   - ExecutionSummary 落一条 round 事件（S/E 客观量 + 技能序列 + 任务意图提示）。
//
// continuation 表示本次 Ask 由 AskUser 作答或授权决策触发，属于上一用户轮的
// 续跑而非新轮：挂起段先暂存，最终段合并后只落一条 round 事件。
//
// 归属规则（白皮书 7.2 要点 5）：事件归负责交付的主 Agent——"负责人对结果负责"
// 的计量化。子会话（SubAgent 委托）的事件不在此处重复计量：其质量由委托链
// 客观埋点（t_delegation_rebound，M2）承担，主观评价归属主 Agent。
func (d *Daemon) withMetricHook(
	b *agents.AskBuilder,
	sid, agentName string,
	getSubSessionID func() string,
	taskHint string,
	continuation bool,
) *agents.AskBuilder {
	store := d.app.MetricStore()
	if store == nil {
		return b
	}
	if !continuation {
		d.clearMetricAccum(sid)
	}

	var skillsUsed []string
	var routeCorrection int
	var lastSkill string

	b.OnToolStart(func(data goharnessevents.ToolExecStartData) {
		// 子会话的工具调用不计入主会话轮次事件（见归属规则）。
		if sub := getSubSessionID(); sub != "" && sub != sid {
			return
		}
		if data.ToolName != "Skill" {
			return
		}
		skillName := extractSkillName(data.Params)
		if skillName == "" {
			return
		}
		if lastSkill != "" && lastSkill != skillName {
			routeCorrection++
		}
		lastSkill = skillName
		skillsUsed = append(skillsUsed, skillName)
	})

	b.OnExecutionSummary(func(data goharnessevents.ExecutionSummaryData) {
		// 子会话摘要不落轮次事件：主 Agent 的对外交付才是计量对象。
		// 子会话里暂存的判定同样没有可挂载的事件，按子会话自己的 ID 丢弃，
		// 不能误伤主会话在同一轮里已暂存的判定。
		if sub := getSubSessionID(); sub != "" && sub != sid {
			d.discardPendingJudgments(sub, "sub-session summary skipped")
			return
		}
		// AskUser / 工具授权挂起只是同一用户轮的中断点，不是新一轮：
		// 暂存本段执行量，等续跑收尾时合并成一条 round 事件。
		if isSuspendedSummaryReason(data.TerminationReason) {
			d.stashMetricAccum(sid, taskHint, data, routeCorrection, skillsUsed)
			return
		}
		// 授权魔术词被路由到子会话时，主会话本轮没有实际工作：
		// 已有挂起段就合并成一条收尾事件，没有就完全不落轮，避免零迭代假轮。
		if continuation && data.TerminationReason == "magicword_consumed" {
			a := d.takeMetricAccum(sid)
			if a == nil {
				return
			}
			mergeMetricAccum(&data, &taskHint, &routeCorrection, &skillsUsed, a)
		} else if a := d.takeMetricAccum(sid); a != nil {
			mergeMetricAccum(&data, &taskHint, &routeCorrection, &skillsUsed, a)
		}
		// 计量是旁路能力：落盘失败只丢这一条轮次事件，不打断会话主流程。
		_, _ = d.recordRoundEvent(store, roundEventInput{
			SessionID:       sid,
			AgentName:       agentName,
			TaskHint:        taskHint,
			Data:            data,
			RouteCorrection: routeCorrection,
			SkillsUsed:      skillsUsed,
		})
	})

	return b
}

// roundEventInput 是一次轮次事件落盘所需的全部输入。
type roundEventInput struct {
	SessionID       string
	AgentName       string
	TaskHint        string
	Data            goharnessevents.ExecutionSummaryData
	RouteCorrection int
	SkillsUsed      []string
}

// recordRoundEvent 落盘一条 round 事件，并立即挂载该轮暂存的判定。
//
// 抽出为独立方法是为了让「写入 → 挂载」这条因果在测试里可被真实执行，
// 而不是靠外部模拟——判定挂载点一旦错位，run 全绿也毫无意义。
func (d *Daemon) recordRoundEvent(store *metric.FileMetricStore, in roundEventInput) (metric.MetricEvent, error) {
	// 轮号 = 该会话既有 round 事件数 + 1（判定按 session + 轮号定位事件，
	// daemon 重启后计数自然从存量恢复）。
	roundNo := 1
	if existing, err := store.Query(context.Background(), metric.MetricFilter{
		SessionID: in.SessionID, Kind: metric.KindRound,
	}); err == nil {
		roundNo = len(existing) + 1
	}
	ev := metric.MetricEvent{
		Kind:            metric.KindRound,
		SessionID:       in.SessionID,
		AgentName:       in.AgentName,
		RoundNo:         roundNo,
		DurationMS:      in.Data.TotalDuration.Milliseconds(),
		Tokens:          in.Data.TokensUsed.ActualTokens(),
		RoundIterations: in.Data.TotalIterations,
		RouteCorrection: in.RouteCorrection,
		SkillsUsed:      in.SkillsUsed,
		TaskHint:        truncateHint(in.TaskHint),
	}
	saved, err := store.AppendEvent(context.Background(), ev)
	if err != nil {
		d.logger.Warn("metric: 写入轮次事件失败",
			"session_id", in.SessionID, "error", err)
		// 这一轮没有事件可挂：暂存的判定不能漂到下一轮。
		d.discardPendingJudgments(in.SessionID, "append round event failed")
		return metric.MetricEvent{}, err
	}
	// 轮内上报的判定此刻才可挂载：该轮事件已存在，且正是 Agent 所处的
	// 这一轮。放在写入成功之后、且不在 goroutine 里——flush 依赖本轮
	// 完整字段（task_hint / agent），事件未落盘时无从定位。
	d.flushPendingJudgments(store, saved)
	return saved, nil
}

// extractSkillName 从 Skill 工具的参数中提取技能名（参数字段 "name"）。
func extractSkillName(params map[string]any) string {
	raw, ok := params["name"]
	if !ok {
		return ""
	}
	name, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(name)
}

// truncateHint 截断任务意图提示（会话任务名/用户输入首行），
// 控制事件体积——明细不存对话原文，仅存聚类所需的意图提示（8.4 隐私条款）。
func truncateHint(hint string) string {
	hint = strings.TrimSpace(hint)
	if idx := strings.IndexAny(hint, "\r\n"); idx > 0 {
		hint = hint[:idx]
	}
	const maxLen = 120
	runes := []rune(hint)
	if len(runes) > maxLen {
		hint = string(runes[:maxLen])
	}
	return hint
}
