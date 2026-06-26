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

// handleOptimize 处理用户输入优化请求。
//
// 流程：
//  1. 使用 PROMPT_OPTIMIZE_USERINPUT 作为系统提示词
//  2. 调用 LLM 对用户输入进行扩写、补全、去噪等优化
//  3. 记录 Token 用量
//  4. 返回优化结果
func (d *Daemon) handleOptimize(_ context.Context, params json.RawMessage) (any, error) {
	var p rpc.OptimizeParams
	if err := unmarshalParams(params, &p); err != nil {
		return nil, err
	}
	if p.Text == "" {
		return nil, fmt.Errorf("text is required")
	}

	// ── 获取默认模型配置 ────────────────────────────────────────
	modelCfg := d.app.ResolveDefaultModel()
	if modelCfg == nil {
		return nil, fmt.Errorf("no default model configured")
	}

	// ── 调用 LLM 优化 ──────────────────────────────────────────
	caller := core.NewCaller(modelCfg, core.PROMPT_OPTIMIZE_USERINPUT)

	result, err := caller.Call(p.Text)
	if err != nil {
		return nil, fmt.Errorf("optimize failed: %w", err)
	}

	// ── 记录 Token 用量 ────────────────────────────────────────
	if result.Tokens.TotalTokens > 0 {
		record := goharnesssession.TokenUsageRecord{
			ID:               uuid.New().String(),
			ModelName:        modelCfg.Name,
			ProviderName:     modelCfg.Provider,
			AgentName:        "optimize",
			PromptTokens:     result.Tokens.PromptTokens,
			CompletionTokens: result.Tokens.CompletionTokens,
			TotalTokens:      result.Tokens.TotalTokens,
			Timestamp:        time.Now(),
		}
		if err := d.app.TokenUsageStore().AppendWithSource(context.Background(), record, "optimize"); err != nil {
			d.logger.Warn("failed to record token usage for optimize", "error", err)
		}
	}

	return rpc.OptimizeResult{Text: result.Result}, nil
}
