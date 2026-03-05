package processors

import (
	"context"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/pkg/logging"
	"strings"
)

// ResponseProcessor 响应生成处理器
// 职责：综合所有上下文信息，生成最终响应
// MVP 版本：完整实现（这是核心功能）
type ResponseProcessor struct {
	thinking core.Thinking
	logger   logging.Logger
}

// NewResponseProcessor 创建响应生成处理器
func NewResponseProcessor(thinking core.Thinking) *ResponseProcessor {
	return &ResponseProcessor{
		thinking: thinking,
		logger:   logging.GetSystemLogger().Named("response_processor"),
	}
}

// Name 返回处理器名称
func (p *ResponseProcessor) Name() string {
	return "ResponseProcessor"
}

// Process 处理响应生成
func (p *ResponseProcessor) Process(ctx context.Context, thinkCtx *entity.ThinkContext) error {
	p.logger.Debug("response generation started",
		logging.String("session_id", thinkCtx.SessionID),
	)

	// 1. 构建响应生成的参考信息
	references := p.buildReferences(thinkCtx)

	// 2. 调用 LLM 生成响应
	result, err := p.thinking.Think(
		ctx,
		thinkCtx.Input,
		nil,       // MVP: 暂不传递历史对话
		references, // 传递参考信息
		false,     // 不需要 JSON 结果
	)
	if err != nil {
		p.logger.Error("response generation failed",
			logging.Err(err),
		)
		return err
	}

	// 3. 填充响应
	thinkCtx.Response = result.Answer

	// 4. 填充其他字段（如果有）
	if result.SendTo != "" {
		thinkCtx.SendTo = result.SendTo
	}

	p.logger.Info("response generated",
		logging.Int("response_length", len(thinkCtx.Response)),
	)

	return nil
}

// buildReferences 构建参考信息
func (p *ResponseProcessor) buildReferences(thinkCtx *entity.ThinkContext) string {
	var builder strings.Builder

	// 1. 添加意图信息
	if thinkCtx.Intent != nil {
		builder.WriteString("用户意图：")
		builder.WriteString(thinkCtx.Intent.Type)
		builder.WriteString("\n")

		if len(thinkCtx.Intent.Keywords) > 0 {
			builder.WriteString("关键词：")
			builder.WriteString(strings.Join(thinkCtx.Intent.Keywords, ", "))
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}

	// 2. 添加记忆信息
	if len(thinkCtx.Memories) > 0 {
		builder.WriteString("相关记忆：\n")
		for i, mem := range thinkCtx.Memories {
			if i >= 3 { // 最多显示 3 条记忆
				break
			}
			builder.WriteString("- ")
			builder.WriteString(mem.Content)
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}

	// 3. 添加工具执行结果
	if len(thinkCtx.ToolResults) > 0 {
		builder.WriteString("工具执行结果：\n")
		for _, result := range thinkCtx.ToolResults {
			builder.WriteString("- ")
			builder.WriteString(result.FunctionName)
			builder.WriteString(": ")
			if result.Error != "" {
				builder.WriteString("执行失败 - ")
				builder.WriteString(result.Error)
			} else {
				builder.WriteString(result.Result)
			}
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}

	// 4. 添加匹配的技能信息
	if len(thinkCtx.MatchedSkills) > 0 {
		builder.WriteString("匹配的技能：")
		builder.WriteString(thinkCtx.MatchedSkills[0].Name)
		builder.WriteString("\n\n")
	}

	return builder.String()
}
