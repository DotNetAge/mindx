package processors

import (
	"context"
	"fmt"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/pkg/logging"
)

// IntentProcessor 意图识别处理器
// 职责：从用户输入中识别意图类型、提取关键词
// MVP 简化版：只做基础识别 + 云端降级，不做置信度检查
type IntentProcessor struct {
	localThinking core.Thinking // 本地模型（左脑）
	cloudThinking core.Thinking // 云端模型（右脑，降级用）
	logger        logging.Logger
}

// NewIntentProcessor 创建意图识别处理器
func NewIntentProcessor(localThinking, cloudThinking core.Thinking) *IntentProcessor {
	return &IntentProcessor{
		localThinking: localThinking,
		cloudThinking: cloudThinking,
		logger:        logging.GetSystemLogger().Named("intent_processor"),
	}
}

// Name 返回处理器名称
func (p *IntentProcessor) Name() string {
	return "IntentProcessor"
}

// Process 处理意图识别
func (p *IntentProcessor) Process(ctx context.Context, thinkCtx *entity.ThinkContext) error {
	p.logger.Debug("intent recognition started",
		logging.String("input", thinkCtx.Input),
		logging.String("session_id", thinkCtx.SessionID),
	)

	// 1. 尝试使用本地模型识别
	result, err := p.recognizeWithLocal(ctx, thinkCtx.Input)
	if err != nil {
		p.logger.Warn("local model failed, fallback to cloud",
			logging.Err(err),
		)

		// 2. 降级到云端模型
		result, err = p.recognizeWithCloud(ctx, thinkCtx.Input)
		if err != nil {
			p.logger.Error("cloud model also failed",
				logging.Err(err),
			)
			return fmt.Errorf("intent recognition failed: %w", err)
		}
	}

	// 3. 填充意图上下文
	thinkCtx.Intent = &entity.IntentContext{
		Type:       result.Intent,
		Keywords:   result.Keywords,
		Confidence: 1.0, // MVP: 暂不计算置信度
	}

	p.logger.Info("intent recognized",
		logging.String("type", result.Intent),
		logging.Int("keywords_count", len(result.Keywords)),
	)

	return nil
}

// recognizeWithLocal 使用本地模型识别意图
func (p *IntentProcessor) recognizeWithLocal(ctx context.Context, input string) (*core.ThinkingResult, error) {
	result, err := p.localThinking.Think(ctx, input, nil, "", true)
	if err != nil {
		return nil, fmt.Errorf("local thinking failed: %w", err)
	}

	// 验证结果
	if result.Intent == "" {
		return nil, fmt.Errorf("empty intent from local model")
	}

	return result, nil
}

// recognizeWithCloud 使用云端模型识别意图
func (p *IntentProcessor) recognizeWithCloud(ctx context.Context, input string) (*core.ThinkingResult, error) {
	result, err := p.cloudThinking.Think(ctx, input, nil, "", true)
	if err != nil {
		return nil, fmt.Errorf("cloud thinking failed: %w", err)
	}

	// 验证结果
	if result.Intent == "" {
		return nil, fmt.Errorf("empty intent from cloud model")
	}

	return result, nil
}
