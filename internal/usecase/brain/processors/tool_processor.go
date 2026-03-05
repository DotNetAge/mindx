package processors

import (
	"context"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/pkg/logging"
)

// ToolExecutionProcessor 工具执行处理器
// 职责：调用 LLM 决定工具调用，并执行工具
// MVP 版本：完整实现（这是核心功能）
type ToolExecutionProcessor struct {
	thinking     core.Thinking
	skillManager core.SkillManager
	logger       logging.Logger
}

// NewToolExecutionProcessor 创建工具执行处理器
func NewToolExecutionProcessor(thinking core.Thinking, skillManager core.SkillManager) *ToolExecutionProcessor {
	return &ToolExecutionProcessor{
		thinking:     thinking,
		skillManager: skillManager,
		logger:       logging.GetSystemLogger().Named("tool_processor"),
	}
}

// Name 返回处理器名称
func (p *ToolExecutionProcessor) Name() string {
	return "ToolExecutionProcessor"
}

// Process 处理工具执行
func (p *ToolExecutionProcessor) Process(ctx context.Context, thinkCtx *entity.ThinkContext) error {
	// 1. 检查是否有可用工具
	if len(thinkCtx.Tools) == 0 {
		p.logger.Debug("no tools available, skip tool execution")
		return nil
	}

	p.logger.Debug("tool execution started",
		logging.Int("tools_count", len(thinkCtx.Tools)),
	)

	// 2. 转换工具 Schema 为 core.ToolSchema
	coreTools := p.convertToToolSchemas(thinkCtx.Tools)

	// 3. 调用 LLM 决定工具调用
	toolCallResult, err := p.thinking.ThinkWithTools(
		ctx,
		thinkCtx.Input,
		nil, // MVP: 暂不传递历史对话
		coreTools,
	)
	if err != nil {
		p.logger.Warn("LLM tool decision failed",
			logging.Err(err),
		)
		// 工具决策失败不影响流程
		return nil
	}

	// 4. 检查是否决定不调用工具
	if toolCallResult.NoCall {
		p.logger.Debug("LLM decided not to call any tools")
		return nil
	}

	// 5. 执行工具调用
	results := make([]entity.ToolExecResult, 0)

	// 处理单个工具调用（兼容旧版）
	if toolCallResult.Function != nil {
		result := p.executeToolCall(ctx, toolCallResult.ToolCallID, toolCallResult.Function)
		results = append(results, result)
	}

	// 处理批量工具调用
	for _, toolCall := range toolCallResult.ToolCalls {
		result := p.executeToolCall(ctx, toolCall.ToolCallID, toolCall.Function)
		results = append(results, result)
	}

	// 6. 填充工具执行结果
	thinkCtx.ToolResults = results

	p.logger.Info("tool execution completed",
		logging.Int("executed_count", len(results)),
	)

	return nil
}

// executeToolCall 执行单个工具调用
func (p *ToolExecutionProcessor) executeToolCall(ctx context.Context, toolCallID string, function *core.ToolCallFunction) entity.ToolExecResult {
	if function == nil {
		return entity.ToolExecResult{
			ToolCallID:   toolCallID,
			FunctionName: "",
			Error:        "function is nil",
		}
	}

	p.logger.Debug("executing tool",
		logging.String("tool_name", function.Name),
		logging.String("tool_call_id", toolCallID),
	)

	// 执行工具
	result, err := p.skillManager.ExecuteFunc(*function)
	if err != nil {
		p.logger.Warn("tool execution failed",
			logging.String("tool_name", function.Name),
			logging.Err(err),
		)

		return entity.ToolExecResult{
			ToolCallID:   toolCallID,
			FunctionName: function.Name,
			Arguments:    function.Arguments,
			Error:        err.Error(),
		}
	}

	return entity.ToolExecResult{
		ToolCallID:   toolCallID,
		FunctionName: function.Name,
		Arguments:    function.Arguments,
		Result:       result,
	}
}

// convertToToolSchemas 转换工具 Schema 格式
func (p *ToolExecutionProcessor) convertToToolSchemas(entityTools []entity.ToolSchema) []*core.ToolSchema {
	coreTools := make([]*core.ToolSchema, 0, len(entityTools))

	for _, tool := range entityTools {
		coreTools = append(coreTools, &core.ToolSchema{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			Params:      tool.Function.Parameters,
		})
	}

	return coreTools
}
