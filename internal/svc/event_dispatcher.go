package svc

import (
	"fmt"
	"strings"

	goharnessevents "github.com/DotNetAge/goharness/events"
	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/i18n"
)

func (d *Daemon) sendEvent(clientID, sessionID string, respType gateway.ResponseType, title string, data string, opts ...gateway.ResponseOption) {
	if d.gw == nil {
		return
	}
	allOpts := append([]gateway.ResponseOption{gateway.WithSessionID(sessionID)}, opts...)
	_ = d.gw.SendResponse(clientID, respType, title, data, allOpts...)
}

// broadcastScheduleEvent sends a schedule.job_event notification to all connected clients.
func (d *Daemon) broadcastScheduleEvent(sessionID, agent, eventType string, data any) {
	if d.gw == nil {
		return
	}
	d.gw.BroadcastNotification("schedule.job_event", map[string]any{
		"session_id": sessionID,
		"agent":      agent,
		"type":       eventType,
		"data":       data,
	})
}

func (d *Daemon) sendExecutionSummary(clientID, sessionID string, summary goharnessevents.ExecutionSummaryData, agentName string) {
	if d.gw == nil {
		return
	}
	tokensUsed := summary.TokensUsed
	// Effective token consumption: cached/reused tokens should not be counted as billed usage.
	effectiveTotal := tokensUsed.InputTokens + tokensUsed.OutputTokens - tokensUsed.CachedTokens
	if effectiveTotal < 0 {
		effectiveTotal = 0
	}
	d.logger.Debug("sendExecutionSummary",
		"effective_total", effectiveTotal,
		"input", tokensUsed.InputTokens,
		"output", tokensUsed.OutputTokens,
		"cached", tokensUsed.CachedTokens)
	tokenValue := fmt.Sprintf("%d (in:%d out:%d cached:%d reasoning:%d)",
		effectiveTotal, tokensUsed.InputTokens, tokensUsed.OutputTokens,
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
				"total_tokens":     effectiveTotal,
				"input_tokens":     tokensUsed.InputTokens,
				"output_tokens":    tokensUsed.OutputTokens,
				"cached_tokens":    tokensUsed.CachedTokens,
				"reasoning_tokens": tokensUsed.ReasoningTokens,
			},
			"iterations": summary.TotalIterations,
			"tool_calls": summary.ToolCalls,
			"duration":   summary.TotalDuration.String(),
			"agent_name": agentName,
		}))
}

// Markdown builders for event messages

func buildSubtaskSpawnedMarkdown(info goharnessevents.SubtaskInfo) string {
	md := fmt.Sprintf("### %s: `%s`\n\n**Agent**: %s\n**%s**: %s\n", i18n.T("svc.md.subtask.spawned"), info.TaskID, info.AgentName, i18n.T("svc.md.subtask.description"), info.Description)
	if info.Timeout != "" {
		md += fmt.Sprintf(i18n.T("svc.md.subtask.timeout"), info.Timeout)
	}
	return md
}

func buildSubtaskCompletedMarkdown(result goharnessevents.SubtaskResult) string {
	var b strings.Builder
	if result.Success {
		b.WriteString(fmt.Sprintf("### %s: `%s`\n\n", i18n.T("svc.md.subtask.completed"), result.TaskID))
		b.WriteString(fmt.Sprintf("**%s**: %s\n", i18n.T("svc.md.subtask.answer"), truncate(result.Answer, 300)))
	} else {
		b.WriteString(fmt.Sprintf("### %s: `%s`\n\n", i18n.T("svc.md.subtask.failed"), result.TaskID))
		b.WriteString(fmt.Sprintf(i18n.T("svc.md.subtask.error"), result.Error))
	}
	return b.String()
}

func buildTaskSummaryMarkdown(ts goharnessevents.TaskSummaryData) string {
	return fmt.Sprintf("### %s\n\n%s\n",
		i18n.T("svc.md.task.summary"), ts.Summary)
}

// formatTokenCount converts a large number to a human-readable K/M format.
//
//	25858 → "25.9K"
//	271   → "271"
//	1500000 → "1.5M"
func formatTokenCount(n int) string {
	if n >= 1_000_000 {
		m := float64(n) / 1_000_000
		return fmt.Sprintf("%.1fM", m)
	}
	if n >= 1_000 {
		k := float64(n) / 1_000
		return fmt.Sprintf("%.1fK", k)
	}
	return fmt.Sprintf("%d", n)
}
