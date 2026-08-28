package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goharnesssession "github.com/DotNetAge/goharness/session"
	"github.com/DotNetAge/mindx/internal/core"
	"github.com/DotNetAge/mindx/pkg/rpc"
	"github.com/google/uuid"
)

// handleCommitMessage 根据 git diff 生成提交信息。
//
// 流程：
//  1. 使用 PROMPT_COMMIT_MESSAGE 作为系统提示词
//  2. 调用 LLM 对 diff 生成 Conventional Commits 提交信息
//  3. 记录 Token 用量
//  4. 返回生成的提交信息
func (d *Daemon) handleCommitMessage(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.CommitMessageParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Diff == "" {
		return nil, fmt.Errorf("diff is required")
	}

	// ── 获取默认模型配置 ────────────────────────────────────────
	modelCfg := d.app.ResolveDefaultModel()
	if modelCfg == nil {
		return nil, fmt.Errorf("no default model configured")
	}

	// ── 调用 LLM 生成提交信息 ───────────────────────────────────
	caller := core.NewCaller(modelCfg, core.PROMPT_COMMIT_MESSAGE)

	result, err := caller.Call(p.Diff)
	if err != nil {
		return nil, fmt.Errorf("generate commit message failed: %w", err)
	}

	// ── 记录 Token 用量 ────────────────────────────────────────
	if result.Tokens.TotalTokens > 0 {
		cachedTokens := 0
		if result.Tokens.PromptTokensDetails != nil {
			cachedTokens = result.Tokens.PromptTokensDetails.CachedTokens
		}
		reasoningTokens := 0
		if result.Tokens.CompletionTokensDetails != nil {
			reasoningTokens = result.Tokens.CompletionTokensDetails.ReasoningTokens
		}
		record := goharnesssession.TokenUsageRecord{
			ID:               uuid.New().String(),
			ModelName:        modelCfg.Name,
			ProviderName:     modelCfg.Provider,
			AgentName:        "commit_message",
			PromptTokens:     result.Tokens.PromptTokens,
			CompletionTokens: result.Tokens.CompletionTokens,
			CachedTokens:     cachedTokens,
			ReasoningTokens:  reasoningTokens,
			TotalTokens:      result.Tokens.TotalTokens,
			Timestamp:        time.Now(),
		}
		if err := d.app.TokenUsageStore().AppendWithSource(context.Background(), record, "commit_message"); err != nil {
			d.logger.Warn("failed to record token usage for commit_message", "error", err)
		}
	}

	return rpc.CommitMessageResult{Message: result.Result}, nil
}
