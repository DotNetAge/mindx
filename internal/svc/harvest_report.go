package svc

// 收割回灌通道：非阻塞 CollectResults 的收尾（定案二）。
//
// goharness 后台收割器（agents/collector.go）在全部子任务落定或达到总上限
//（30 分钟）时调用宿主注入的汇报回调（Runtime.harvestReporter → 本文件的
// reportHarvestToSession）。回灌以「系统收割」标记的消息经主会话串行队列
//（sessionQueueFor）程序化发起一轮执行——LLM 在此轮才得知各子任务结局并
// 向用户转述。用户间隙继续与主 Agent 聊天不干扰：回灌与用户消息走同一
// 串行队列，天然顺序化。

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/DotNetAge/goharness/tools"
)

// harvestCancelClientID 是收割回灌在 clientCancels 中的虚拟客户端键：
// 收割回灌不属于任何客户端，message.cancel 按会话取消时需一并查此集合。
const harvestCancelClientID = "__harvest__"

// reportHarvestToSession 收割报告回灌主会话（App.SetHarvestReporter 注入）。
// 报告格式化为「系统收割」消息正文后经 runBackgroundTurn 发起主会话一轮
// 执行。收割 goroutine 与用户对话异步并发，本函数任何失败只记日志，
// 绝不 panic（收割器有 recover，但通道自身也要收敛）。
func (d *Daemon) reportHarvestToSession(report tools.HarvestReport) {
	defer func() {
		if r := recover(); r != nil {
			d.logger.Error("harvest report: panic", fmt.Errorf("%v", r))
		}
	}()
	sid := report.MainSessionID
	if sid == "" || d.app == nil {
		return
	}

	// 执行 agent 按会话绑定裁定（与 defaultHandler 回退一致）：
	// 会话绑定 agent_name > 全局当前 agent。
	agentName := ""
	if meta, err := d.getSessionMeta(sid); err == nil && meta != nil {
		agentName = meta.AgentName
	}
	if agentName == "" {
		agentName = d.app.CurrentAgentName()
	}

	content := formatHarvestReport(report)
	d.logger.Info("harvest report: feeding back to main session",
		"session_id", sid, "agent", agentName, "targets", len(report.Targets))

	d.runBackgroundTurn(sid, agentName, content, harvestCancelClientID, func(err error) {
		if err != nil && !errors.Is(err, context.Canceled) {
			d.logger.Error("harvest report: feedback turn failed", err, "session_id", sid)
		}
	})
}

// formatHarvestReport 把收割报告格式化为「系统收割」消息正文：
// 内容进主会话上下文，LLM 在此轮才得知各子任务结局并向用户转述。
// 每个条目带子代理名称与会话锚点（session_id 落盘持久，LLM 后续轮次
// 可据此直达对应子代理对话）。
func formatHarvestReport(report tools.HarvestReport) string {
	var b strings.Builder
	b.WriteString("[系统收割] 派发给子代理的任务已全部结算（或收割达到总上限），以下是各任务的结果汇总。请基于这些结果继续推进，并把各任务的结局清楚地告知用户。\n")
	for _, t := range report.Targets {
		name := t.AgentName
		if t.Title != "" {
			name = fmt.Sprintf("%s（%s）", t.AgentName, t.Title)
		}
		switch t.Status {
		case "completed":
			fmt.Fprintf(&b, "\n- %s（会话 %s）：已完成\n%s", name, t.SessionID, t.Result)
		case "failed":
			fmt.Fprintf(&b, "\n- %s（会话 %s）：失败，原因：%s", name, t.SessionID, t.Error)
		default:
			// blocked / timeout：未落定，条目的 Error 已含具体说明
			fmt.Fprintf(&b, "\n- %s（会话 %s）：未完成——%s", name, t.SessionID, t.Error)
		}
	}
	return b.String()
}
