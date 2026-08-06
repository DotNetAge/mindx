package svc

import (
	"fmt"
	"strings"

	goharnessevents "github.com/DotNetAge/goharness/events"
	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/i18n"
	"github.com/DotNetAge/mindx/pkg/scheduler"
)

func (d *Daemon) sendEvent(clientID, sessionID string, respType gateway.ResponseType, title string, data string, opts ...gateway.ResponseOption) {
	if d.gw == nil {
		return
	}
	allOpts := append([]gateway.ResponseOption{gateway.WithSessionID(sessionID)}, opts...)
	_ = d.gw.SendResponse(clientID, respType, title, data, allOpts...)
}

// broadcastJobLifecycle 统一广播调度任务生命周期事件（job_started/completed/failed/missed）。
// payload 携带 entry_id/run_id/agent/session_id/content 等完整信息，
// meta.agent_name 供前端取智能体名。
func (d *Daemon) broadcastJobLifecycle(method string, info scheduler.JobLifecycleInfo) {
	if d.gw == nil {
		return
	}
	d.gw.BroadcastNotification(method, map[string]any{
		"session_id": info.SessionID,
		"agent":      info.Agent,
		"type":       info.Status,
		"data":       info,
		"meta":       map[string]any{"agent_name": info.Agent},
	})
}

func (d *Daemon) sendExecutionSummary(clientID, sessionID string, summary goharnessevents.ExecutionSummaryData, agentName string) {
	if d.gw == nil {
		return
	}
	tokensUsed := summary.TokensUsed
	// Effective token consumption: cached/reused tokens should not be counted as billed usage.
	effectiveTotal := tokensUsed.ActualTokens()
	d.logger.Debug("sendExecutionSummary",
		"effective_total", effectiveTotal,
		"input", tokensUsed.PromptTokens,
		"output", tokensUsed.CompletionTokens,
		"cached", tokensUsed.CachedTokens)
	rawTotal := tokensUsed.PromptTokens + tokensUsed.CompletionTokens
	tokenValue := fmt.Sprintf("%d (in:%d out:%d cached:%d reasoning:%d)",
		effectiveTotal, tokensUsed.PromptTokens, tokensUsed.CompletionTokens,
		tokensUsed.CachedTokens, tokensUsed.ReasoningTokens)
	tableData := map[string]any{
		"headers": []string{"Metric", "Value"},
		"rows": []map[string]string{
			{"metric": "Iterations", "value": fmt.Sprintf("%d", summary.TotalIterations)},
			{"metric": "Tool Calls", "value": fmt.Sprintf("%d", summary.ToolCalls)},
			{"metric": "Tools Used", "value": strings.Join(summary.ToolsUsed, ", ")},
			{"metric": "Duration", "value": formatDuration(summary.TotalDuration)},
			{"metric": "Tokens Used", "value": tokenValue},
			{"metric": "Termination", "value": summary.TerminationReason},
		},
	}
	_ = d.gw.SendResponse(clientID, gateway.RespExecutionSummary, i18n.T("svc.event.execution.summary"), tableData,
		gateway.WithSessionID(sessionID),
		gateway.WithResponseMeta(map[string]any{
			"tokens_used": map[string]any{
				"total_tokens":      rawTotal,
				"actual_tokens":     effectiveTotal,
				"prompt_tokens":     tokensUsed.PromptTokens,
				"completion_tokens": tokensUsed.CompletionTokens,
				"cached_tokens":     tokensUsed.CachedTokens,
				"reasoning_tokens":  tokensUsed.ReasoningTokens,
			},
			"iterations": summary.TotalIterations,
			"tool_calls": summary.ToolCalls,
			"duration":   summary.TotalDuration.String(),
			"agent_name": agentName,
		}))
}

func buildTaskSummaryMarkdown(ts goharnessevents.TaskSummaryData) string {
	return fmt.Sprintf("### %s\n\n%s\n",
		i18n.T("svc.md.task.summary"), ts.Summary)
}
