package svc

import (
	"encoding/json"
	"fmt"
	"strings"

	goreactcore "github.com/DotNetAge/goreact/core"
	"github.com/DotNetAge/goreact/reactor"
	"github.com/DotNetAge/gort/pkg/gateway"
)

func (d *Daemon) forwardEvent(clientID string, event goreactcore.ReactEvent) {
	sid := event.SessionID
	switch event.Type {
	case goreactcore.ThinkingDelta:
		text, ok := event.Data.(string)
		if !ok {
			d.logger.Warn("unexpected ThinkingDelta data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.gw.SendResponse(clientID, gateway.RespThinkingDelta, "思考中", text, gateway.WithSessionID(sid))

	case goreactcore.ThinkingDone:
		thought, ok := event.Data.(*reactor.Thought)
		if !ok {
			d.logger.Warn("unexpected ThinkingDone data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		md := buildThinkingDoneMarkdown(*thought)
		d.sendEvent(clientID, sid, gateway.RespThinkingDone, "思考完成", md)

	case goreactcore.ActionStart:
		action, ok := event.Data.(goreactcore.ActionStartData)
		if !ok {
			d.logger.Warn("unexpected ActionStart data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.gw.SendResponse(clientID, gateway.RespActionStart, "开始操作", 	map[string]any{
			"tool_count":       action.ToolCount,
			"tool_names":       action.ToolNames,
			"predicted_tokens": action.TotalPredictedTokens,
			"iteration":        action.Iteration,
		}, gateway.WithSessionID(sid))

	case goreactcore.ToolExecStart:
		data, ok := event.Data.(goreactcore.ToolExecStartData)
		if !ok {
			d.logger.Warn("unexpected ToolExecStart data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.gw.SendResponse(clientID, gateway.RespActionStart, "工具开始", map[string]any{
			"tool_name": data.ToolName,
			"params":    data.Params,
		}, gateway.WithSessionID(sid))

	case goreactcore.ToolExecEnd:
		data, ok := event.Data.(goreactcore.ToolExecEndData)
		if !ok {
			d.logger.Warn("unexpected ToolExecEnd data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.gw.SendResponse(clientID, gateway.RespActionResult, "工具结果", map[string]any{
			"tool_name": data.ToolName,
			"success":   data.Success,
			"result":    data.Result,
			"error":     data.Error,
			"duration":  data.Duration.String(),
		}, gateway.WithSessionID(sid))

	case goreactcore.ActionProgress:
		progress, ok := event.Data.(goreactcore.ActionProgressData)
		if !ok {
			d.logger.Warn("unexpected ActionProgress data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.gw.SendResponse(clientID, gateway.RespActionProgress, "操作进度", map[string]any{
			"completed": progress.CompletedCount,
			"total":     progress.TotalCount,
			"status":    progress.Status,
		}, gateway.WithSessionID(sid))

	case goreactcore.ActionEnd:
		data, ok := event.Data.(goreactcore.ActionEndData)
		if !ok {
			d.logger.Warn("unexpected ActionEnd data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.gw.SendResponse(clientID, gateway.RespActionEnd, "操作完成", map[string]any{
			"total":   data.TotalTools,
			"success": data.SuccessCount,
			"failed":  data.FailedCount,
			"summary": data.Summary,
		}, gateway.WithSessionID(sid))

	case goreactcore.SubtaskSpawned:
		info, ok := event.Data.(goreactcore.SubtaskInfo)
		if !ok {
			d.logger.Warn("unexpected SubtaskSpawned data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		md := buildSubtaskSpawnedMarkdown(info)
		d.sendEvent(clientID, sid, gateway.RespSubtaskSpawned, "子任务生成", md)

	case goreactcore.SubtaskCompleted:
		result, ok := event.Data.(goreactcore.SubtaskResult)
		if !ok {
			d.logger.Warn("unexpected SubtaskCompleted data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		md := buildSubtaskCompletedMarkdown(result)
		d.sendEvent(clientID, sid, gateway.RespSubtaskCompleted, "子任务完成", md)

	case goreactcore.FinalAnswer:
		answer, ok := event.Data.(string)
		if !ok {
			d.logger.Warn("unexpected FinalAnswer data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.sendEvent(clientID, sid, gateway.RespFinalAnswer, "最终答案", answer)

	case goreactcore.ClarifyNeeded:
		question, ok := event.Data.(string)
		if !ok {
			d.logger.Warn("unexpected ClarifyNeeded data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.sendEvent(clientID, sid, gateway.RespClarifyNeeded, "需要澄清", question)

	case goreactcore.PermissionRequest:
		req, ok := event.Data.(goreactcore.PermissionRequestData)
		if !ok {
			d.logger.Warn("unexpected PermissionRequest data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		md := buildPermissionRequestMarkdown(req)
		d.sendEvent(clientID, sid, gateway.RespPermissionRequest, "权限请求", md)

	case goreactcore.AskUserRequest:
		req, ok := event.Data.(goreactcore.AskUserRequestData)
		if !ok {
			d.logger.Warn("unexpected AskUserRequest data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		jsonData, _ := json.Marshal(req)
		d.sendEvent(clientID, sid, gateway.RespForm, "需要澄清", string(jsonData))

	case goreactcore.PermissionDenied:
		reason, ok := event.Data.(string)
		if !ok {
			d.logger.Warn("unexpected PermissionDenied data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.sendEvent(clientID, sid, gateway.RespPermissionDenied, "权限拒绝", reason)

	case goreactcore.ExecutionSummary:
		summary, ok := event.Data.(goreactcore.ExecutionSummaryData)
		if !ok {
			d.logger.Warn("unexpected ExecutionSummary data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.sendExecutionSummary(clientID, sid, summary)

	case goreactcore.CycleEnd:
		cycle, ok := event.Data.(goreactcore.CycleInfo)
		if !ok {
			d.logger.Warn("unexpected CycleEnd data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		md := buildCycleEndMarkdown(cycle)
		d.sendEvent(clientID, sid, gateway.RespCycleEnd, "循环结束", md)

	case goreactcore.TaskSummary:
		taskSummary, ok := event.Data.(goreactcore.TaskSummaryData)
		if !ok {
			d.logger.Warn("unexpected TaskSummary data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		md := buildTaskSummaryMarkdown(taskSummary)
		d.gw.SendResponse(clientID, gateway.RespTaskSummary, "任务总结", md,
			gateway.WithSessionID(sid),
			gateway.WithResponseMeta(map[string]any{
				"input_tokens":  taskSummary.TokenUsage.InputTokens,
				"output_tokens": taskSummary.TokenUsage.OutputTokens,
			}))

	case goreactcore.Error:
		errMsg, ok := event.Data.(string)
		if !ok {
			d.logger.Warn("unexpected Error data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.sendEvent(clientID, sid, gateway.RespError, "错误", errMsg)
	}
}

func (d *Daemon) sendEvent(clientID, sessionID string, respType gateway.ResponseType, title string, data string) {
	d.gw.SendResponse(clientID, respType, title, data, gateway.WithSessionID(sessionID))
}

func (d *Daemon) sendExecutionSummary(clientID, sessionID string, summary goreactcore.ExecutionSummaryData) {
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
	d.gw.SendResponse(clientID, gateway.RespExecutionSummary, "执行摘要", tableData, gateway.WithSessionID(sessionID))
}

// Markdown builders for event messages

func buildThinkingDoneMarkdown(t reactor.Thought) string {
	var b strings.Builder
	fmt.Fprintf(&b, "### 思考完成\n\n")
	b.WriteString(fmt.Sprintf("**决策**: `%s`  **置信度**: %.0f%%\n\n", t.Decision, t.Confidence*100))
	if t.Reasoning != "" {
		b.WriteString(fmt.Sprintf("**推理**: %s\n\n", t.Reasoning))
	}
	if len(t.ToolCalls) > 0 {
		b.WriteString("**即将调用工具**:\n\n")
		for toolName, params := range t.ToolCalls {
			b.WriteString(fmt.Sprintf("- `%s` — `%v`\n", toolName, params))
		}
		b.WriteString("\n")
	}
	if t.ClarificationQuestion != "" {
		b.WriteString(fmt.Sprintf("**问题**: %s\n\n", t.ClarificationQuestion))
	}
	return b.String()
}

func buildSubtaskSpawnedMarkdown(info goreactcore.SubtaskInfo) string {
	return fmt.Sprintf("### 🌿 子任务生成: `%s`\n\n**Agent**: %s\n**描述**: %s\n", info.TaskID, info.AgentName, info.Description)
}

func buildSubtaskCompletedMarkdown(result goreactcore.SubtaskResult) string {
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

func buildPermissionRequestMarkdown(req goreactcore.PermissionRequestData) string {
	return fmt.Sprintf("### 🔒 权限请求: `%s`\n\n**原因**: %s\n**安全级别**: %d\n", req.ToolName, req.Reason, req.SecurityLevel)
}

func buildCycleEndMarkdown(cycle goreactcore.CycleInfo) string {
	return fmt.Sprintf("### 🔄 T-A-O 循环结束 (迭代 #%d, 耗时 %s)\n", cycle.Iteration, formatDuration(cycle.Duration))
}

func buildTaskSummaryMarkdown(ts goreactcore.TaskSummaryData) string {
	return fmt.Sprintf("### 📋 任务总结\n\n%s\n\n**Token**: 输入 %d / 输出 %d / 总计 %d\n", ts.Summary, ts.TokenUsage.InputTokens, ts.TokenUsage.OutputTokens, ts.TokenUsage.TotalTokens)
}
