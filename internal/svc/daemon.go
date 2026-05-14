package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	goreactcore "github.com/DotNetAge/goreact/core"
	"github.com/DotNetAge/goreact/reactor"
	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/pkg/logging"
	"github.com/DotNetAge/mindx/pkg/scheduler"
	"github.com/DotNetAge/mindx/pkg/session"
	"github.com/google/uuid"
)

var atAgentRegex = regexp.MustCompile(`^@([\w-]+)(?:\s+([\w-]+))?\s+(.+)$`)

type Daemon struct {
	app         *core.App
	gw          *gateway.Server
	scheduler   *scheduler.Scheduler
	schedulerDB *scheduler.FileSchedulerStore
	addr        string
	wsPath      string
	logger      logging.Logger
}

func NewDaemon(app *core.App, addr, wsPath string) *Daemon {
	logDir := logging.ResolveLogDir()
	logger := logging.DefaultZapLogger(&logging.ZapConfig{
		Filename:   filepath.Join(logDir, "mindx-daemon.log"),
		MaxSize:    100,
		MaxBackups: 7,
		MaxAge:     30,
		Compress:   true,
		Console:    true,
	})

	schedulerDB, _ := scheduler.NewFileSchedulerStore(app.Settings().SchedulesDir())

	d := &Daemon{
		app:         app,
		addr:        addr,
		wsPath:      wsPath,
		schedulerDB: schedulerDB,
		logger:      logger,
	}

	d.scheduler = scheduler.NewScheduler(schedulerDB, d.executeScheduleCommand, logger)
	return d
}

func (d *Daemon) Start(ctx context.Context) error {
	if d.gw == nil {
		d.initGateway()
	}

	if d.scheduler != nil {
		if err := d.scheduler.Start(ctx); err != nil {
			d.logger.Warn("Scheduler failed to start", "error", err)
		}
	}

	d.logger.Info("MindX daemon starting", "addr", fmt.Sprintf("ws://localhost%s%s", d.addr, d.wsPath))

	if err := d.gw.Start(); err != nil {
		return fmt.Errorf("gateway start failed: %w", err)
	}

	<-ctx.Done()
	d.logger.Info("Shutting down")

	if err := d.gw.StopAllChannels(ctx); err != nil {
		d.logger.Warn("failed to stop channels", "error", err)
	}

	if d.scheduler != nil {
		d.scheduler.Stop()
	}

	return d.gw.Shutdown(ctx)
}

func (d *Daemon) TestStart(ctx context.Context) error {
	if d.gw == nil {
		d.initGateway()
	}

	if d.scheduler != nil {
		if err := d.scheduler.Start(ctx); err != nil {
			d.logger.Warn("scheduler failed to start", "error", err)
		}
	}

	if err := d.gw.Start(); err != nil {
		return fmt.Errorf("gateway start failed: %w", err)
	}

	return nil
}

func (d *Daemon) TestStop(ctx context.Context) error {
	if d.gw == nil {
		return nil
	}

	if err := d.gw.StopAllChannels(ctx); err != nil {
		d.logger.Warn("failed to stop channels", "error", err)
	}

	if d.scheduler != nil {
		d.scheduler.Stop()
	}

	return d.gw.Shutdown(ctx)
}

func (d *Daemon) initGateway() {
	d.gw = gateway.New(
		gateway.WithAddr(d.addr),
		gateway.WithPath(d.wsPath),
		gateway.WithHandler(d.defaultHandler),
	)
}

func (d *Daemon) executeScheduleCommand(ctx context.Context, agent string, sessionID string, content string, projectDir string) error {
	originalCWD, _ := os.Getwd()

	targetDir := projectDir
	if targetDir == "" {
		meta := d.restoreSessionEnvironment(sessionID)
		if meta != nil {
			targetDir = meta.ProjectWorkingDir
		}
	}

	if targetDir != "" {
		if err := os.Chdir(targetDir); err != nil {
			d.logger.Warn("failed to chdir to project dir, using current dir",
				"project_dir", targetDir,
				"error", err,
			)
		} else {
			defer func() {
				if restoreErr := os.Chdir(originalCWD); restoreErr != nil {
					d.logger.Warn("failed to restore cwd after scheduled task",
						"original", originalCWD,
						"error", restoreErr,
					)
				}
			}()
			os.Setenv("MINDX_PROJECT_DIR", targetDir)
			os.Setenv("MINDX_SESSION_ID", sessionID)
			d.logger.Info("set execution context for scheduled task",
				"session_id", sessionID,
				"project_dir", targetDir,
				"original_cwd", originalCWD,
			)
		}
	}

	resolvedAgent, err := d.app.ResolveAgent(agent)
	if err != nil {
		return fmt.Errorf("resolve agent %q: %w", agent, err)
	}
	if sessionID == "" || sessionID == "new" {
		sessionID = generateSessionID()
	}
	_, err = resolvedAgent.Ask(sessionID, content)
	if err != nil {
		return fmt.Errorf("execute scheduled message for @%s (session: %s): %w", agent, sessionID, err)
	}
	return nil
}

// restoreSessionEnvironment loads session metadata and restores the project directory.
// Returns nil if the session metadata cannot be found (e.g., sessions created before this feature).
func (d *Daemon) restoreSessionEnvironment(sessionID string) *session.SessionMeta {
	if d.app == nil || d.app.SessDB() == nil {
		return nil
	}
	meta, err := d.app.SessDB().GetSessionMeta(sessionID)
	if err != nil {
		d.logger.Debug("could not load session meta for scheduled task",
			"session_id", sessionID,
			"error", err,
		)
		return nil
	}
	return meta
}

func (d *Daemon) defaultHandler(msg *gateway.Message) {
	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(msg.Data, &payload); err != nil || payload.Text == "" {
		d.logger.Warn("defaultHandler: missing or invalid text field",
			"data", string(msg.Data), "error", err)
		return
	}
	text := payload.Text

	agentName, providedSessionID, content := parseAgentTarget(text)

	agent, err := d.app.ResolveAgent(agentName)
	if err != nil {
		d.sendEvent(msg.ClientID, msg.SessionID, gateway.RespError, "错误", err.Error())
		return
	}

	sessionID := d.resolveSessionID(msg.SessionID, providedSessionID)
	resolvedAgentName := agentName
	if resolvedAgentName == "" {
		resolvedAgentName = d.app.Settings().MasterAgent
		if resolvedAgentName == "" {
			resolvedAgentName = "master"
		}
	}

	d.logger.Info("request start",
		"client_id", msg.ClientID,
		"session_id", sessionID,
		"agent", resolvedAgentName,
		"input_preview", truncate(content, 100),
	)

	eventCh, cancelEvents := agent.EventsFiltered(func(e goreactcore.ReactEvent) bool {
		switch e.Type {
		case goreactcore.ThinkingDelta, goreactcore.ThinkingDone, goreactcore.ActionStart,
			goreactcore.ActionProgress, goreactcore.ActionResult, goreactcore.FinalAnswer,
			goreactcore.ExecutionSummary, goreactcore.Error, goreactcore.SubtaskSpawned,
			goreactcore.SubtaskCompleted, goreactcore.ClarifyNeeded, goreactcore.PermissionRequest,
			goreactcore.PermissionDenied, goreactcore.CycleEnd, goreactcore.TaskSummary:
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
			d.forwardEvent(msg.ClientID, event)
		}
	}()

	_, err = agent.Ask(sessionID, content)
	if err != nil {
		d.logger.Error("request failed", err,
			"client_id", msg.ClientID,
			"session_id", sessionID,
			"agent", resolvedAgentName,
		)
		d.sendEvent(msg.ClientID, sessionID, gateway.RespError, "错误", err.Error())
	}

	<-done
	d.logger.Info("request done",
		"client_id", msg.ClientID,
		"session_id", sessionID,
		"agent", resolvedAgentName,
	)
}

func parseAgentTarget(text string) (agentName string, sessionID string, content string) {
	matches := atAgentRegex.FindStringSubmatch(text)
	if len(matches) >= 4 {
		agentName = matches[1]
		sessionID = matches[2]
		content = matches[3]
		return
	}
	if len(matches) == 2 {
		agentName = matches[1]
		content = strings.TrimPrefix(text, matches[0])
		return
	}
	return "", "", text
}

func (d *Daemon) resolveSessionID(clientProvided string, commandProvided string) string {
	if commandProvided != "" && commandProvided != "new" {
		d.logger.Info("using session_id from command", "session", commandProvided)
		return commandProvided
	}
	if clientProvided != "" {
		return clientProvided
	}

	if d.app.SessionDB() != nil && d.app.Settings().MasterAgent != "" {
		sid, err := d.app.SessionDB().GetByRole(context.Background(), d.app.Settings().MasterAgent)
		if err == nil && sid != nil && sid.SessionID != "" {
			d.logger.Info("resumed session from store", "agent", d.app.Settings().MasterAgent, "session", sid.SessionID)
			return sid.SessionID
		}
	}

	sid := generateSessionID()
	d.logger.Info("created new session", "session", sid)
	return sid
}

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
		d.gw.SendResponse(clientID, gateway.RespActionStart, "开始操作", map[string]interface{}{
			"tool_name":        action.ToolName,
			"params":           action.Params,
			"predicted_tokens": action.PredictedTokens,
			"iteration":        action.Iteration,
		}, gateway.WithSessionID(sid))

	case goreactcore.ActionProgress:
		progress, ok := event.Data.(string)
		if !ok {
			d.logger.Warn("unexpected ActionProgress data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.gw.SendResponse(clientID, gateway.RespActionProgress, "操作进度", progress, gateway.WithSessionID(sid))

	case goreactcore.ActionResult:
		result, ok := event.Data.(goreactcore.ActionResultData)
		if !ok {
			d.logger.Warn("unexpected ActionResult data type", "type", fmt.Sprintf("%T", event.Data))
			return
		}
		d.gw.SendResponse(clientID, gateway.RespActionResult, "操作结果", map[string]interface{}{
			"tool_name": result.ToolName,
			"success":   result.Success,
			"result":    result.Result,
			"error":     result.Error,
			"duration":  result.Duration.String(),
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
			gateway.WithResponseMeta(map[string]interface{}{
				"input_tokens":  taskSummary.InputTokens,
				"output_tokens": taskSummary.OutputTokens,
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
	d.gw.SendResponse(clientID, gateway.RespExecutionSummary, "执行摘要", tableData, gateway.WithSessionID(sessionID))
}

func (d *Daemon) Gateway() *gateway.Server {
	return d.gw
}

func (d *Daemon) App() *core.App {
	return d.app
}

func (d *Daemon) Scheduler() *scheduler.Scheduler {
	return d.scheduler
}

func (d *Daemon) SchedulerDB() *scheduler.FileSchedulerStore {
	return d.schedulerDB
}

func (d *Daemon) Addr() string {
	return d.addr
}

func (d *Daemon) WSPath() string {
	return d.wsPath
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

func buildActionStartMarkdown(action goreactcore.ActionStartData) string {
	paramsStr := formatParams(action.Params)
	return fmt.Sprintf("### ⚡ 调用工具: `%s`\n\n参数: %s\n", action.ToolName, paramsStr)
}

func buildActionResultMarkdown(result goreactcore.ActionResultData) string {
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
