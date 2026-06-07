package svc

import (
	"fmt"
	"strings"

	goreactevents "github.com/DotNetAge/goreact/events"
	"github.com/DotNetAge/gort/pkg/gateway"
)

func (d *Daemon) sendEvent(clientID, sessionID string, respType gateway.ResponseType, title string, data string) {
	d.gw.SendResponse(clientID, respType, title, data, gateway.WithSessionID(sessionID))
}

func (d *Daemon) sendExecutionSummary(clientID, sessionID string, summary goreactevents.ExecutionSummaryData) {
	d.logger.Info("[SSE-TRACE L5] sendExecutionSummary: total_tokens="+fmt.Sprint(summary.TokensUsed.TotalTokens)+
		" input="+fmt.Sprint(summary.TokensUsed.InputTokens)+
		" output="+fmt.Sprint(summary.TokensUsed.OutputTokens))
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
	d.gw.SendResponse(clientID, gateway.RespExecutionSummary, "执行摘要", tableData,
		gateway.WithSessionID(sessionID),
		gateway.WithResponseMeta(map[string]any{
			"tokens_used": map[string]any{
				"total_tokens":   summary.TokensUsed.TotalTokens,
				"input_tokens":  summary.TokensUsed.InputTokens,
				"output_tokens": summary.TokensUsed.OutputTokens,
				"cached_tokens":  summary.TokensUsed.CachedTokens,
				"reasoning_tokens": summary.TokensUsed.ReasoningTokens,
			},
			"iterations":    summary.TotalIterations,
			"tool_calls":    summary.ToolCalls,
			"duration":      summary.TotalDuration.String(),
		}))
}

// Markdown builders for event messages

func buildSubtaskSpawnedMarkdown(info goreactevents.SubtaskInfo) string {
	md := fmt.Sprintf("### 🌿 子任务生成: `%s`\n\n**Agent**: %s\n**描述**: %s\n", info.TaskID, info.AgentName, info.Description)
	if info.Timeout != "" {
		md += fmt.Sprintf("**超时**: %s\n", info.Timeout)
	}
	return md
}

func buildSubtaskCompletedMarkdown(result goreactevents.SubtaskResult) string {
	var b strings.Builder
	if result.Success {
		b.WriteString(fmt.Sprintf("### ✅ 子任务完成: `%s`\n\n", result.TaskID))
		b.WriteString(fmt.Sprintf("**回答**: %s\n", truncate(result.Answer, 300)))
	} else {
		b.WriteString(fmt.Sprintf("### ❌ 子任务失败: `%s`\n\n", result.TaskID))
		b.WriteString(fmt.Sprintf("**错误**: %s\n", result.Error))
	}
	return b.String()
}

func buildTaskSummaryMarkdown(ts goreactevents.TaskSummaryData) string {
	return fmt.Sprintf("### 📋 任务总结\n\n%s\n\n**Token**: 输入 %d / 输出 %d / 总计 %d\n", ts.Summary, ts.TokenUsage.InputTokens, ts.TokenUsage.OutputTokens, ts.TokenUsage.TotalTokens)
}
