package svc

import (
	"fmt"
	"strings"

	goreactevents "github.com/DotNetAge/goreact/events"
	"github.com/DotNetAge/mindx/internal/i18n"
	"github.com/DotNetAge/gort/pkg/gateway"
)

func (d *Daemon) sendEvent(clientID, sessionID string, respType gateway.ResponseType, title string, data string) {
	_ = d.gw.SendResponse(clientID, respType, title, data, gateway.WithSessionID(sessionID))
}

func (d *Daemon) sendExecutionSummary(clientID, sessionID string, summary goreactevents.ExecutionSummaryData) {
	d.logger.Info("[SSE-TRACE L5] sendExecutionSummary: total_tokens=" + fmt.Sprint(summary.TokensUsed.TotalTokens) +
		" input=" + fmt.Sprint(summary.TokensUsed.InputTokens) +
		" output=" + fmt.Sprint(summary.TokensUsed.OutputTokens))
	tableData := map[string]any{
		"headers": []string{"Metric", "Value"},
		"rows": []map[string]string{
			{"metric": "Iterations", "value": fmt.Sprintf("%d", summary.TotalIterations)},
			{"metric": "Tool Calls", "value": fmt.Sprintf("%d", summary.ToolCalls)},
			{"metric": "Tools Used", "value": strings.Join(summary.ToolsUsed, ", ")},
			{"metric": "Duration", "value": formatDuration(summary.TotalDuration)},
			{"metric": "Tokens Used", "value": fmt.Sprintf("%d (in:%d out:%d cached:%d reasoning:%d)", summary.TokensUsed.TotalTokens, summary.TokensUsed.InputTokens, summary.TokensUsed.OutputTokens, summary.TokensUsed.CachedTokens, summary.TokensUsed.ReasoningTokens)},
			{"metric": "Termination", "value": summary.TerminationReason},
		},
	}
	_ = d.gw.SendResponse(clientID, gateway.RespExecutionSummary, i18n.T("svc.event.execution.summary"), tableData,
		gateway.WithSessionID(sessionID),
		gateway.WithResponseMeta(map[string]any{
			"tokens_used": map[string]any{
				"total_tokens":     summary.TokensUsed.TotalTokens,
				"input_tokens":     summary.TokensUsed.InputTokens,
				"output_tokens":    summary.TokensUsed.OutputTokens,
				"cached_tokens":    summary.TokensUsed.CachedTokens,
				"reasoning_tokens": summary.TokensUsed.ReasoningTokens,
			},
			"iterations": summary.TotalIterations,
			"tool_calls": summary.ToolCalls,
			"duration":   summary.TotalDuration.String(),
		}))
}

// Markdown builders for event messages

func buildSubtaskSpawnedMarkdown(info goreactevents.SubtaskInfo) string {
	md := fmt.Sprintf("### %s: `%s`\n\n**Agent**: %s\n**%s**: %s\n", i18n.T("svc.md.subtask.spawned"), info.TaskID, info.AgentName, i18n.T("svc.md.subtask.description"), info.Description)
	if info.Timeout != "" {
		md += fmt.Sprintf(i18n.T("svc.md.subtask.timeout"), info.Timeout)
	}
	return md
}

func buildSubtaskCompletedMarkdown(result goreactevents.SubtaskResult) string {
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

func buildTaskSummaryMarkdown(ts goreactevents.TaskSummaryData) string {
	return fmt.Sprintf("### %s\n\n%s\n\n**%s**: %s %d / %s %d / %s %d\n",
		i18n.T("svc.md.task.summary"), ts.Summary,
		i18n.T("svc.md.task.token"), i18n.T("svc.md.token.input"), ts.TokenUsage.InputTokens,
		i18n.T("svc.md.token.output"), ts.TokenUsage.OutputTokens,
		i18n.T("svc.md.token.total"), ts.TokenUsage.TotalTokens)
}
