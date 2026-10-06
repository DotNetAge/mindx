package svc

import (
	"context"
	"strings"

	agents "github.com/DotNetAge/goharness/agents"
	goharnessevents "github.com/DotNetAge/goharness/events"
	"github.com/DotNetAge/mindx/pkg/metric"
)

// withMetricHook 在 AskBuilder 上追加决策计量埋点
// （白皮书《决策量表：Agent 生态的绩效系统》第 4 章采集层）。
// AskBuilder.OnXxx 为追加语义，与既有 emitter 并存互不影响。
//
// 闭包状态覆盖单次 Ask 生命周期：
//   - ToolStart 累积技能调用序列（工具名 "Skill"，参数 name 为技能名），
//     并统计相邻技能切换次数（q_route_correction 的 v1 简化口径）；
//   - ExecutionSummary 落一条 round 事件（S/E 客观量 + 技能序列 + 任务意图提示）。
//
// 归属规则（白皮书 7.2 要点 5）：事件归负责交付的主 Agent——"负责人对结果负责"
// 的计量化。子会话（SubAgent 委托）的事件不在此处重复计量：其质量由委托链
// 客观埋点（t_delegation_rebound，M2）承担，主观评价归属主 Agent。
func (d *Daemon) withMetricHook(
	b *agents.AskBuilder,
	sid, agentName string,
	getSubSessionID func() string,
	taskHint string,
) *agents.AskBuilder {
	store := d.app.MetricStore()
	if store == nil {
		return b
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
		if sub := getSubSessionID(); sub != "" && sub != sid {
			return
		}
		// 轮号 = 该会话既有 round 事件数 + 1（判定按 session + 轮号定位事件，
		// daemon 重启后计数自然从存量恢复）。
		roundNo := 1
		if existing, err := store.Query(context.Background(), metric.MetricFilter{
			SessionID: sid, Kind: metric.KindRound,
		}); err == nil {
			roundNo = len(existing) + 1
		}
		ev := metric.MetricEvent{
			Kind:            metric.KindRound,
			SessionID:       sid,
			AgentName:       agentName,
			RoundNo:         roundNo,
			DurationMS:      data.TotalDuration.Milliseconds(),
			Tokens:          data.TokensUsed.ActualTokens(),
			RoundIterations: data.TotalIterations,
			RouteCorrection: routeCorrection,
			SkillsUsed:      skillsUsed,
			TaskHint:        truncateHint(taskHint),
		}
		if _, err := store.AppendEvent(context.Background(), ev); err != nil {
			d.logger.Warn("metric: 写入轮次事件失败",
				"session_id", sid, "error", err)
		}
	})

	return b
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
