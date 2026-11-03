package svc

// 新会话标题提炼（阻塞式先行）：会话首条提问（人首问 defaultHandler / Agent
// 首跑调度任务 scheduled_exec）先用独立 LLM 调用（core.NewCaller + 默认模型 +
// PROMPT_SESSION_TITLE）把问题提炼成简短标题并回填（RenameSession），完成后
// 才继续执行——侧栏从头显示提炼标题而非截断原文。
//
// 本功能与翻译（handler_translate）/提交信息（handler_commit_message）同类，
// 是原有功能的独立补充：独立成文、挂消费侧钩子，不碰 session.create 协议与
// 会话创建语义。
//
// 回填语义与补录的关系：pkg/session 的 AppendMessage 补录（首条 user 消息 →
// 标题）仅在 Title 为空时生效；本流程在首轮 append 之前改名，补录自然跳过。
// 提炼失败 / 超时 / 无默认模型时静默回退补录行为（原文截断标题），不重试、
// 不阻塞会话执行。

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	harness "github.com/DotNetAge/goharness/config"
	goharnesssession "github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/mindx/internal/core"
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

// maybeGenerateSessionTitle 在首轮执行前为全新会话提炼并回填标题。
//
// 触发门控（仅全新会话）：meta 查询失败（ErrSessionNotFound 之外的错误视为
// 存储异常）直接跳过回退补录；meta 存在但已有标题（用户重命名或已补录）或
// 已有消息的会话不是新会话，跳过。question 为空（纯图片消息）跳过。
func (d *Daemon) maybeGenerateSessionTitle(sessionID, agentName, question string) {
	if question == "" || d.app == nil || d.app.SessDB() == nil {
		return
	}
	sessDB := d.app.SessDB()

	meta, err := sessDB.GetSessionMeta(sessionID)
	if err == nil {
		if meta.Title != "" || meta.MessageCount > 0 {
			return
		}
	} else if !errors.Is(err, goharnesssession.ErrSessionNotFound) {
		d.logger.Warn("session title: get meta failed, skip title generation",
			"session_id", sessionID, "error", err)
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

	// 回填走 RenameSession：session.create 先行的正常路径下目录已存在。
	// 裸发消息（未经 session.create）时目录尚不存在且 Create 无法指定 ID，
	// 改名失败留待首轮 append 的补录兜底（原文截断标题）。
	if renameErr := sessDB.RenameSession(sessionID, title); renameErr != nil {
		d.logger.Warn("session title: rename failed",
			"session_id", sessionID, "title", title, "error", renameErr)
		return
	}
	d.logger.Info("session title: generated",
		"session_id", sessionID, "agent", agentName, "title", title, "elapsed", time.Since(start))
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

// sanitizeSessionTitle 清洗模型输出：去首尾空白与包裹引号、折叠空白（含换行）、
// 截断到长度上限。
func sanitizeSessionTitle(raw string) string {
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
