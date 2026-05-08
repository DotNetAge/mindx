package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/DotNetAge/goreact/core"
	"github.com/DotNetAge/goreact/reactor"
	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/google/uuid"
)

var atAgentRegex = regexp.MustCompile(`^@([\w-]+)\s+`)

func (a *App) defaultHandler(msg *gateway.Message) {
	// msg.Data contains the raw JSON-RPC params — extract the "text" field
	// from the user.message notification instead of using the raw JSON.
	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(msg.Data, &payload); err != nil || payload.Text == "" {
		a.logger.Warn("defaultHandler: missing or invalid text field",
			"data", string(msg.Data), "error", err)
		return
	}
	text := payload.Text

	agentName, content := parseAgentTarget(text)

	agent, err := a.resolveAgent(agentName)
	if err != nil {
		a.sendEvent(msg.ClientID, gateway.RespError, "错误", err.Error())
		return
	}

	sessionID := a.resolveSessionID(msg.SessionID)
	resolvedAgentName := agentName
	if resolvedAgentName == "" {
		resolvedAgentName = a.settings.MasterAgent
		if resolvedAgentName == "" {
			resolvedAgentName = "master"
		}
	}

	// === Trace root: ties gateway client_id → session_id → agent ===
	a.logger.Info("request start",
		"client_id", msg.ClientID,
		"session_id", sessionID,
		"agent", resolvedAgentName,
		"input_preview", truncate(content, 100),
	)

	eventCh, cancelEvents := agent.EventsFiltered(func(e core.ReactEvent) bool {
		switch e.Type {
		case core.ThinkingDelta, core.ThinkingDone, core.ActionStart,
			core.ActionProgress, core.ActionResult, core.FinalAnswer,
			core.ExecutionSummary, core.Error, core.SubtaskSpawned,
			core.SubtaskCompleted, core.ClarifyNeeded, core.PermissionRequest,
			core.PermissionDenied, core.CycleEnd, core.TaskSummary:
			return true
		default:
			return false
		}
	})
	defer cancelEvents()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for event := range eventCh {
			a.forwardEvent(msg.ClientID, event)
		}
	}()

	_, err = agent.Ask(sessionID, content)
	if err != nil {
		a.logger.Error("request failed", err,
			"client_id", msg.ClientID,
			"session_id", sessionID,
			"agent", resolvedAgentName,
		)
		a.sendEvent(msg.ClientID, gateway.RespError, "错误", err.Error())
	}

	<-done
	a.logger.Info("request done",
		"client_id", msg.ClientID,
		"session_id", sessionID,
		"agent", resolvedAgentName,
	)
}

func parseAgentTarget(text string) (agentName string, content string) {
	matches := atAgentRegex.FindStringSubmatch(text)
	if len(matches) == 2 {
		return matches[1], strings.TrimPrefix(text, matches[0])
	}
	return "", text
}

func (a *App) resolveSessionID(clientProvided string) string {
	if clientProvided != "" {
		return clientProvided
	}

	if a.sessDB != nil && a.settings.MasterAgent != "" {
		sid, err := a.sessDB.GetByRole(context.Background(), a.settings.MasterAgent)
		if err == nil && sid != nil && sid.SessionID != "" {
			a.logger.Info("resumed session from store", "agent", a.settings.MasterAgent, "session", sid.SessionID)
			return sid.SessionID
		}
	}

	sid := generateSessionID()
	a.logger.Info("created new session", "session", sid)
	return sid
}

func (a *App) forwardEvent(clientID string, event core.ReactEvent) {
	switch event.Type {
	case core.ThinkingDelta:
		text, ok := event.Data.(string)
		if !ok {
			a.logger.Warn("unexpected ThinkingDelta data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		a.gw.SendResponse(clientID, gateway.RespThinkingDelta, "思考中", text)

	case core.ThinkingDone:
		thought, ok := event.Data.(reactor.Thought)
		if !ok {
			a.logger.Warn("unexpected ThinkingDone data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		md := buildThinkingDoneMarkdown(thought)
		a.sendEvent(clientID, gateway.RespThinkingDone, "思考完成", md)

	case core.ActionStart:
		action, ok := event.Data.(core.ActionStartData)
		if !ok {
			a.logger.Warn("unexpected ActionStart data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		md := buildActionStartMarkdown(action)
		a.sendEvent(clientID, gateway.RespActionStart, "开始操作", md)

	case core.ActionProgress:
		progress, ok := event.Data.(string)
		if !ok {
			a.logger.Warn("unexpected ActionProgress data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		a.gw.SendResponse(clientID, gateway.RespActionProgress, "操作进度", progress)

	case core.ActionResult:
		result, ok := event.Data.(core.ActionResultData)
		if !ok {
			a.logger.Warn("unexpected ActionResult data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		md := buildActionResultMarkdown(result)
		a.sendEvent(clientID, gateway.RespActionResult, "操作结果", md)

	case core.SubtaskSpawned:
		info, ok := event.Data.(core.SubtaskInfo)
		if !ok {
			a.logger.Warn("unexpected SubtaskSpawned data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		md := buildSubtaskSpawnedMarkdown(info)
		a.sendEvent(clientID, gateway.RespSubtaskSpawned, "子任务生成", md)

	case core.SubtaskCompleted:
		result, ok := event.Data.(core.SubtaskResult)
		if !ok {
			a.logger.Warn("unexpected SubtaskCompleted data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		md := buildSubtaskCompletedMarkdown(result)
		a.sendEvent(clientID, gateway.RespSubtaskCompleted, "子任务完成", md)

	case core.FinalAnswer:
		answer, ok := event.Data.(string)
		if !ok {
			a.logger.Warn("unexpected FinalAnswer data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		a.sendEvent(clientID, gateway.RespFinalAnswer, "最终答案", answer)

	case core.ClarifyNeeded:
		question, ok := event.Data.(string)
		if !ok {
			a.logger.Warn("unexpected ClarifyNeeded data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		a.sendEvent(clientID, gateway.RespClarifyNeeded, "需要澄清", question)

	case core.PermissionRequest:
		req, ok := event.Data.(core.PermissionRequestData)
		if !ok {
			a.logger.Warn("unexpected PermissionRequest data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		md := buildPermissionRequestMarkdown(req)
		a.sendEvent(clientID, gateway.RespPermissionRequest, "权限请求", md)

	case core.PermissionDenied:
		reason, ok := event.Data.(string)
		if !ok {
			a.logger.Warn("unexpected PermissionDenied data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		a.sendEvent(clientID, gateway.RespPermissionDenied, "权限拒绝", reason)

	case core.ExecutionSummary:
		summary, ok := event.Data.(core.ExecutionSummaryData)
		if !ok {
			a.logger.Warn("unexpected ExecutionSummary data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		a.sendExecutionSummary(clientID, summary)

	case core.CycleEnd:
		cycle, ok := event.Data.(core.CycleInfo)
		if !ok {
			a.logger.Warn("unexpected CycleEnd data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		md := buildCycleEndMarkdown(cycle)
		a.sendEvent(clientID, gateway.RespCycleEnd, "循环结束", md)

	case core.TaskSummary:
		taskSummary, ok := event.Data.(core.TaskSummaryData)
		if !ok {
			a.logger.Warn("unexpected TaskSummary data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		md := buildTaskSummaryMarkdown(taskSummary)
		a.gw.SendResponse(clientID, gateway.RespTaskSummary, "任务总结", md,
			gateway.WithResponseMeta(map[string]interface{}{
				"input_tokens":  taskSummary.InputTokens,
				"output_tokens": taskSummary.OutputTokens,
			}))

	case core.Error:
		errMsg, ok := event.Data.(string)
		if !ok {
			a.logger.Warn("unexpected Error data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		a.sendEvent(clientID, gateway.RespError, "错误", errMsg)
	}
}

func (a *App) sendEvent(clientID string, respType gateway.ResponseType, title string, data string) {
	a.gw.SendResponse(clientID, respType, title, data)
}

func (a *App) sendExecutionSummary(clientID string, summary core.ExecutionSummaryData) {
	tableData := map[string]interface{}{
		"headers": []string{"Metric", "Value"},
		"rows": []map[string]string{
			{"metric": "Iterations", "value": fmt.Sprintf("%d", summary.TotalIterations)},
			{"metric": "Tool Calls", "value": fmt.Sprintf("%d", summary.ToolCalls)},
			{"metric": "Tools Used", "value": strings.Join(summary.ToolsUsed, ", ")},
			{"metric": "Duration", "value": formatDuration(summary.TotalDuration)},
			{"metric": "Tokens Used", "value": fmt.Sprintf("%d", summary.TokensUsed)},
			{"metric": "Termination", "value": summary.TerminationReason},
		},
	}
	a.gw.SendResponse(clientID, gateway.RespExecutionSummary, "执行摘要", tableData)
}

func generateSessionID() string {
	return fmt.Sprintf("sess_%s", uuid.New().String()[:8])
}

func buildThinkingDoneMarkdown(t reactor.Thought) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("### 思考完成\n\n"))
	b.WriteString(fmt.Sprintf("**决策**: `%s`  **置信度**: %.0f%%\n\n", t.Decision, t.Confidence*100))
	if t.Reasoning != "" {
		b.WriteString(fmt.Sprintf("**推理**: %s\n\n", t.Reasoning))
	}
	if t.ToolCalls != nil && len(t.ToolCalls) > 0 {
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

func buildActionStartMarkdown(action core.ActionStartData) string {
	paramsStr := formatParams(action.Params)
	return fmt.Sprintf("### ⚡ 调用工具: `%s`\n\n参数: %s\n", action.ToolName, paramsStr)
}

func buildActionResultMarkdown(result core.ActionResultData) string {
	var b strings.Builder
	if result.Success {
		b.WriteString(fmt.Sprintf("### ✅ `%s` 执行成功\n\n", result.ToolName))
		b.WriteString(fmt.Sprintf("**耗时**: %s\n\n", formatDuration(result.Duration)))
		if result.Result != "" {
			b.WriteString(fmt.Sprintf("**结果**:\n```\n%s\n```\n", truncate(result.Result, 500)))
		}
	} else {
		b.WriteString(fmt.Sprintf("### ❌ `%s` 执行失败\n\n", result.ToolName))
		b.WriteString(fmt.Sprintf("**错误**: %s\n", result.Error))
	}
	return b.String()
}

func buildSubtaskSpawnedMarkdown(info core.SubtaskInfo) string {
	return fmt.Sprintf("### 🌿 子任务生成: `%s`\n\n**Agent**: %s\n**描述**: %s\n", info.TaskID, info.AgentName, info.Description)
}

func buildSubtaskCompletedMarkdown(result core.SubtaskResult) string {
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

func buildPermissionRequestMarkdown(req core.PermissionRequestData) string {
	return fmt.Sprintf("### 🔒 权限请求: `%s`\n\n**原因**: %s\n**安全级别**: %d\n", req.ToolName, req.Reason, req.SecurityLevel)
}

func buildCycleEndMarkdown(cycle core.CycleInfo) string {
	return fmt.Sprintf("### 🔄 T-A-O 循环结束 (迭代 #%d, 耗时 %s)\n", cycle.Iteration, formatDuration(cycle.Duration))
}

func buildTaskSummaryMarkdown(ts core.TaskSummaryData) string {
	return fmt.Sprintf("### 📋 任务总结\n\n%s\n\n**Token**: 输入 %d / 输出 %d\n", ts.Summary, ts.InputTokens, ts.OutputTokens)
}

func formatParams(params map[string]any) string {
	if len(params) == 0 {
		return "(无)"
	}
	b, err := json.Marshal(params)
	if err != nil {
		return fmt.Sprintf("%v", params)
	}
	return truncate(string(b), 200)
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return d.Truncate(100 * time.Millisecond).String()
}

func truncate(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxLen]) + "..."
}
