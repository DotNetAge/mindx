package svc

import (
	"encoding/json"
	"fmt"
	"strings"

	goreactevents "github.com/DotNetAge/goreact/events"
	"github.com/DotNetAge/gort/pkg/gateway"
)

func (d *Daemon) forwardEvent(clientID string, event goreactevents.ReactEvent) {
	sid := event.SessionID
	switch event.Type {
	case goreactevents.ThinkingDelta:
		text, ok := event.Data.(string)
		if !ok {
			d.logger.Warn("unexpected ThinkingDelta data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.gw.SendResponse(clientID, gateway.RespThinkingDelta, "思考中", text, gateway.WithSessionID(sid))

	case goreactevents.ThinkingDone:
		d.sendEvent(clientID, sid, gateway.RespThinkingDone, "思考完成", "思考阶段已完成")

	case goreactevents.ContentDelta:
		text, ok := event.Data.(string)
		if !ok {
			d.logger.Warn("unexpected ContentDelta data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.gw.SendResponse(clientID, gateway.RespMarkdown, "输出中", text, gateway.WithSessionID(sid))

	case goreactevents.ToolUseDelta:
		data, ok := event.Data.(goreactevents.ToolUseDeltaData)
		if !ok {
			d.logger.Warn("unexpected ToolUseDelta data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.gw.SendResponse(clientID, gateway.RespText, "工具参数", map[string]any{
			"index":     data.Index,
			"id":        data.ID,
			"name":      data.Name,
			"arguments": data.Arguments,
		}, gateway.WithSessionID(sid))

	case goreactevents.ToolExecStart:
		data, ok := event.Data.(goreactevents.ToolExecStartData)
		if !ok {
			d.logger.Warn("unexpected ToolExecStart data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.gw.SendResponse(clientID, gateway.RespActionStart, "工具开始", map[string]any{
			"tool_name": data.ToolName,
			"params":    data.Params,
		}, gateway.WithSessionID(sid))

	case goreactevents.ToolExecEnd:
		data, ok := event.Data.(goreactevents.ToolExecEndData)
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

	case goreactevents.SubtaskSpawned:
		info, ok := event.Data.(goreactevents.SubtaskInfo)
		if !ok {
			d.logger.Warn("unexpected SubtaskSpawned data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		md := buildSubtaskSpawnedMarkdown(info)
		d.sendEvent(clientID, sid, gateway.RespSubtaskSpawned, "子任务生成", md)

	case goreactevents.SubtaskCompleted:
		result, ok := event.Data.(goreactevents.SubtaskResult)
		if !ok {
			d.logger.Warn("unexpected SubtaskCompleted data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		md := buildSubtaskCompletedMarkdown(result)
		d.sendEvent(clientID, sid, gateway.RespSubtaskCompleted, "子任务完成", md)

	case goreactevents.FinalAnswer:
		answer, ok := event.Data.(string)
		if !ok {
			d.logger.Warn("unexpected FinalAnswer data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.sendEvent(clientID, sid, gateway.RespFinalAnswer, "最终答案", answer)

	case goreactevents.PermissionRequest:
		req, ok := event.Data.(goreactevents.PermissionRequestData)
		if !ok {
			d.logger.Warn("unexpected PermissionRequest data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		md := buildPermissionRequestMarkdown(req)
		d.sendEvent(clientID, sid, gateway.RespPermissionRequest, "权限请求", md)

	case goreactevents.AskUserRequest:
		req, ok := event.Data.(goreactevents.AskUserRequestData)
		if !ok {
			d.logger.Warn("unexpected AskUserRequest data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		jsonData, _ := json.Marshal(req)
		d.sendEvent(clientID, sid, gateway.RespForm, "需要澄清", string(jsonData))

	case goreactevents.PermissionDenied:
		reason, ok := event.Data.(string)
		if !ok {
			d.logger.Warn("unexpected PermissionDenied data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.sendEvent(clientID, sid, gateway.RespPermissionDenied, "权限拒绝", reason)

	case goreactevents.ExecutionSummary:
		summary, ok := event.Data.(goreactevents.ExecutionSummaryData)
		if !ok {
			d.logger.Warn("unexpected ExecutionSummary data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.sendExecutionSummary(clientID, sid, summary)

	case goreactevents.CycleEnd:
		cycle, ok := event.Data.(goreactevents.CycleInfo)
		if !ok {
			d.logger.Warn("unexpected CycleEnd data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		md := buildCycleEndMarkdown(cycle)
		d.sendEvent(clientID, sid, gateway.RespCycleEnd, "循环结束", md)

	case goreactevents.TaskSummary:
		taskSummary, ok := event.Data.(goreactevents.TaskSummaryData)
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

	case goreactevents.LLMTimeout:
		data, ok := event.Data.(goreactevents.LLMTimeoutData)
		if !ok {
			d.logger.Warn("unexpected LLMTimeout data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.gw.SendResponse(clientID, gateway.RespError, "超时", map[string]any{
			"session_id": data.SessionID,
			"timeout":    data.Timeout.String(),
			"elapsed":    data.Elapsed.String(),
			"error":      data.Error,
		}, gateway.WithSessionID(sid))

	case goreactevents.AgentTalkStart:
		info, ok := event.Data.(goreactevents.AgentTalkInfo)
		if !ok {
			d.logger.Warn("unexpected AgentTalkStart data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.gw.SendResponse(clientID, gateway.RespAgentTalkStart, "Agent对话开始", map[string]any{
			"to": info.To, "message": info.Message,
		}, gateway.WithSessionID(sid))

	case goreactevents.AgentTalkEnd:
		result, ok := event.Data.(goreactevents.AgentTalkResult)
		if !ok {
			d.logger.Warn("unexpected AgentTalkEnd data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.gw.SendResponse(clientID, gateway.RespAgentTalkEnd, "Agent对话结束", map[string]any{
			"to": result.To, "reply": result.Reply, "error": result.Error,
		}, gateway.WithSessionID(sid))

	case goreactevents.Compaction:
		data, ok := event.Data.(goreactevents.CompactionData)
		if !ok {
			d.logger.Warn("unexpected Compaction data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.gw.SendResponse(clientID, gateway.RespCompaction, "上下文压缩", map[string]any{
			"session_id": data.SessionID, "messages_slid": data.MessagesSlid, "remaining_after": data.RemainingAfter, "window_size": data.WindowSize,
		}, gateway.WithSessionID(sid))

	case goreactevents.MaxTurnsReached:
		data, ok := event.Data.(goreactevents.MaxTurnsReachedData)
		if !ok {
			d.logger.Warn("unexpected MaxTurnsReached data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.gw.SendResponse(clientID, gateway.RespMaxTurnsReached, "达到最大轮次", map[string]any{
			"turns_completed": data.TurnsCompleted, "max_turns": data.MaxTurns, "suggestion": data.Suggestion,
		}, gateway.WithSessionID(sid))

	case goreactevents.Error:
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

func (d *Daemon) sendExecutionSummary(clientID, sessionID string, summary goreactevents.ExecutionSummaryData) {
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

func buildPermissionRequestMarkdown(req goreactevents.PermissionRequestData) string {
	md := fmt.Sprintf("### 🔒 权限请求: `%s`\n\n**原因**: %s\n**安全级别**: %d\n", req.ToolName, req.Reason, req.SecurityLevel)
	if len(req.Params) > 0 {
		paramsJSON, _ := json.MarshalIndent(req.Params, "", "  ")
		md += fmt.Sprintf("**参数**:\n```json\n%s\n```\n", string(paramsJSON))
	}
	return md
}

func buildCycleEndMarkdown(cycle goreactevents.CycleInfo) string {
	md := fmt.Sprintf("### 🔄 思考循环结束 (迭代 #%d, 耗时 %s)\n", cycle.Iteration, formatDuration(cycle.Duration))
	if cycle.TerminationReason != "" {
		md += fmt.Sprintf("**终止原因**: %s\n", cycle.TerminationReason)
	}
	return md
}

func buildTaskSummaryMarkdown(ts goreactevents.TaskSummaryData) string {
	return fmt.Sprintf("### 📋 任务总结\n\n%s\n\n**Token**: 输入 %d / 输出 %d / 总计 %d\n", ts.Summary, ts.TokenUsage.InputTokens, ts.TokenUsage.OutputTokens, ts.TokenUsage.TotalTokens)
}
