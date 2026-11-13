package svc

// 新会话标题提炼：daemon 检测到首问需要创建全新会话时，先以阻塞方式调用
// LLM（core.NewCaller + 默认模型 + PROMPT_SESSION_TITLE）把首问提炼成简短
// 标题，得到标题后才开始调用创建会话——标题随创建落库（WithTitleOption，
// 见 ensureScheduleSession）；会话已由客户端先行创建（session.create 空壳）
// 时，标题在首条消息 append 之前写入（RenameSession）。侧栏从头显示提炼
// 标题而非首问截断原文。用户之后手动重命名会话与标题提炼无关（已有标题的
// 会话不参与提炼）。
//
// 提炼失败 / 超时 / 无默认模型时静默跳过，回退 pkg/session 的补录（首条
// user 消息截断为标题），不重试、不阻塞会话执行。

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	harness "github.com/DotNetAge/goharness/config"
	goharnesssession "github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/gort/pkg/gateway"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/internal/i18n"
	"github.com/google/uuid"
)

// sessionTitleTimeout 标题提炼的外层超时：llmcaller.Call 内置 4 分钟超时，
// 对阻塞式先行过长——超时即放弃，回退补录，保证新会话首轮不被拖死。
const sessionTitleTimeout = 12 * time.Second

// sessionTitleMaxRunes 标题长度上限（rune 计），超出截断；与提示词的 20 字
// 约束双保险，防模型不守约。
const sessionTitleMaxRunes = 40

// sessionTitleAgentName Token 用量记账的智能体名（对齐 commit_message 惯例）。
const sessionTitleAgentName = "session_title"

// maybeGenerateSessionTitle 在首轮执行前为全新会话提炼并落库标题。
//
// 门控（仅全新会话的首问）：会话尚未创建（meta 不存在）时跳过——需要创建
// 全新会话的流程（ensureScheduleSession）先提炼标题、得到标题后再创建会话，
// 此处无事可做；已有标题或已有消息的会话不是新会话，跳过；question 为空
// （纯图片消息）跳过。meta 与 metaErr 由调用方传入（与 agent 裁定共用一次
// 读取，避免消息路径重复遍历目录树）。
func (d *Daemon) maybeGenerateSessionTitle(sessionID, agentName, question string, meta *goharnesssession.SessionInfo, metaErr error) {
	if question == "" || d.app == nil || d.app.SessDB() == nil {
		return
	}
	sessDB := d.app.SessDB()

	if metaErr != nil {
		// 存储异常与会话未创建区分开：前者留痕便于排查；会话尚未创建时
		// 标题提炼先于创建会话完成，无需处理
		if !errors.Is(metaErr, goharnesssession.ErrSessionNotFound) {
			d.logger.Warn("session title: get meta failed, skip title generation",
				"session_id", sessionID, "error", metaErr)
		}
		return
	}
	if meta == nil || meta.Title != "" || meta.MessageCount > 0 {
		return
	}

	start := time.Now()
	title, err := d.generateSessionTitle(question)
	if err != nil {
		d.logger.Warn("session title: generation failed, fallback to raw-title backfill",
			"session_id", sessionID, "error", err)
		return
	}
	if title == "" {
		return
	}

	// 会话由客户端先行创建（session.create 空壳），目录已存在；
	// 标题在首条消息 append 之前写入，落库失败回退补录（原文截断标题）。
	if renameErr := sessDB.RenameSession(sessionID, title); renameErr != nil {
		d.logger.Warn("session title: rename failed",
			"session_id", sessionID, "title", title, "error", renameErr)
		return
	}
	d.logger.Info("session title: generated",
		"session_id", sessionID, "agent", agentName, "title", title, "elapsed", time.Since(start))
}

// refineSubSessionTitle 是注入 goharness Runtime（WithTitleFn）的子代理会话
// 标题提炼回调：把派工任务描述提炼成简短标题 → RenameSession 落库 → 返回
// 新标题（goharness 据返回值发射 SubtaskTitleUpdated 事件刷新前端标题）。
// 复用主会话首问的提炼器 generateSessionTitle（12s 兜底，失败静默）。
// 返回空串表示提炼/落库失败——goharness 保持受理快照标题（任务截断）兜底。
//
// 与主会话首问提炼（maybeGenerateSessionTitle）的差异：子会话每次受理都会
// 提炼并覆盖旧标题——同一子会话被复用承接新任务时，侧栏标题始终反映
// 「正在处理的任务」；无「已有标题不参与提炼」的门控（子会话标题属派工
// 状态而非用户命名资产，用户手动改子会话标题会被下一次派工覆盖，可接受）。
func (d *Daemon) refineSubSessionTitle(sessionID, task string) string {
	if task == "" || d.app == nil || d.app.SessDB() == nil {
		return ""
	}
	title, err := d.generateSessionTitle(task)
	if err != nil || title == "" {
		d.logger.Warn("session title: sub-session generation failed, keep snapshot title",
			"session_id", sessionID, "error", err)
		return ""
	}
	if renameErr := d.app.SessDB().RenameSession(sessionID, title); renameErr != nil {
		d.logger.Warn("session title: sub-session rename failed",
			"session_id", sessionID, "title", title, "error", renameErr)
		return ""
	}
	d.logger.Info("session title: sub-session title generated",
		"session_id", sessionID, "title", title)

	// 标题刷新直接广播到 gateway（不依赖主会话 exec 总线存活）：goharness 的
	// SubtaskTitleUpdated 事件经主会话 exec EventBus 异步发射，而主会话首轮
	// 通常在标题提炼完成前就已结束（Run 返回即摘除订阅），事件会静默丢失
	//——侧栏只能刷新后才见新标题。落库成功的瞬间由 daemon 亲自广播，前端
	// 侧栏/spawn 卡片/localStorage 三处即时刷新（处理器幂等，与 goharness
	// 事件链重复无害）。
	broadcastResp(d.gw, gateway.ResponseType("subtask_title_updated"),
		i18n.T("svc.event.subtask.title.updated"),
		map[string]any{"session_id": sessionID, "title": title},
		gateway.WithSessionID(sessionID))
	return title
}

// generateSessionTitle 调用默认模型把用户提问提炼成会话标题。
// 外层以 sessionTitleTimeout 兜底（Call 内部固定 4 分钟超时，阻塞式先行不可
// 等那么久）；Token 用量在调用协程内即时记账，超时放弃时不丢已完成调用的用量。
func (d *Daemon) generateSessionTitle(question string) (string, error) {
	modelCfg := d.app.ResolveDefaultModel()
	if modelCfg == nil {
		return "", fmt.Errorf("no default model configured")
	}

	caller := core.NewCaller(modelCfg, core.PROMPT_SESSION_TITLE)
	type callOutcome struct {
		title string
		err   error
	}
	done := make(chan callOutcome, 1)
	go func() {
		// 标题提炼只需要意图主干，超长问题截断后再送
		input := question
		if runes := []rune(input); len(runes) > 2000 {
			input = string(runes[:2000])
		}
		result, callErr := caller.Call(input)
		if callErr != nil {
			done <- callOutcome{err: callErr}
			return
		}
		// 用量在调用协程内即时落库：外层超时放弃结果时不丢已完成调用的用量
		d.recordSessionTitleUsage(d.sessionTitleUsage(modelCfg, result))
		done <- callOutcome{title: sanitizeSessionTitle(result.Result)}
	}()

	select {
	case outcome := <-done:
		if outcome.err != nil {
			return "", fmt.Errorf("generate session title failed: %w", outcome.err)
		}
		return outcome.title, nil
	case <-time.After(sessionTitleTimeout):
		return "", fmt.Errorf("session title generation timeout after %s", sessionTitleTimeout)
	}
}

// hasSessionTitleMarkup 检测模型把内部特殊标记当正文吐出来的字面残迹：
// 全角竖线 ｜（U+FF5C）是 DeepSeek 系 tokenizer 特殊标记（<｜tool▁calls▁begin｜>
// 等）的特征字符；<| 与 |> 是通用 <|...|> 形态的边界。正常标题不会含这些
// 字符，命中即整条判生成失败（调用方回退快照标题/原文截断），防垃圾落库。
// 实证样本：<｜｜DSML｜｜ calls> <｜｜DSML｜｜ invoke name="（deepseek-flash
// 子会话标题调用偶发泄漏，输入为纯中文派工文本）。
func hasSessionTitleMarkup(s string) bool {
	return strings.ContainsRune(s, '｜') || strings.Contains(s, "<|") || strings.Contains(s, "|>")
}

// sanitizeSessionTitle 清洗模型输出：含特殊标记残迹整条拒绝（返回空串，调用方
// 走失败回退）；去首尾空白与包裹引号、折叠空白（含换行）、截断到长度上限。
func sanitizeSessionTitle(raw string) string {
	if hasSessionTitleMarkup(raw) {
		return ""
	}
	title := strings.TrimSpace(raw)
	title = strings.Trim(title, "\"“”'「」『』`")
	title = strings.Join(strings.Fields(title), " ")
	if runes := []rune(title); len(runes) > sessionTitleMaxRunes {
		title = string(runes[:sessionTitleMaxRunes])
	}
	return title
}

// sessionTitleUsage 组装标题提炼的 Token 用量记账记录。
func (d *Daemon) sessionTitleUsage(modelCfg *harness.ModelConfig, result core.LLMResult) goharnesssession.TokenUsageRecord {
	record := goharnesssession.TokenUsageRecord{
		ID:           uuid.New().String(),
		ModelName:    modelCfg.Name,
		ProviderName: modelCfg.Provider,
		AgentName:    sessionTitleAgentName,
		Timestamp:    time.Now(),
	}
	if result.Tokens.TotalTokens <= 0 {
		return record
	}
	record.PromptTokens = result.Tokens.PromptTokens
	record.CompletionTokens = result.Tokens.CompletionTokens
	record.TotalTokens = result.Tokens.TotalTokens
	if result.Tokens.PromptTokensDetails != nil {
		record.CachedTokens = result.Tokens.PromptTokensDetails.CachedTokens
	}
	if result.Tokens.CompletionTokensDetails != nil {
		record.ReasoningTokens = result.Tokens.CompletionTokensDetails.ReasoningTokens
	}
	return record
}

// recordSessionTitleUsage 落库 Token 用量（失败仅告警，不影响主流程）。
func (d *Daemon) recordSessionTitleUsage(record goharnesssession.TokenUsageRecord) {
	if record.TotalTokens <= 0 {
		return
	}
	if err := d.app.TokenUsageStore().AppendWithSource(context.Background(), record, sessionTitleAgentName); err != nil {
		d.logger.Warn("session title: failed to record token usage", "error", err)
	}
}
