package brain

import (
	"context"
	"fmt"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/pkg/logging"
	"time"
)

// Pipeline 处理器管线
// 按顺序执行多个处理器，每个处理器修改共享的 ThinkContext
type Pipeline struct {
	processors []core.Processor
	logger     logging.Logger
}

// NewPipeline 创建新的处理器管线
func NewPipeline(processors ...core.Processor) *Pipeline {
	return &Pipeline{
		processors: processors,
		logger:     logging.GetSystemLogger().Named("pipeline"),
	}
}

// Execute 执行管线
func (p *Pipeline) Execute(ctx context.Context, thinkCtx *entity.ThinkContext) error {
	p.logger.Info("pipeline execution started",
		logging.String("session_id", thinkCtx.SessionID),
		logging.String("input", thinkCtx.Input),
	)

	for _, processor := range p.processors {
		if err := p.executeProcessor(ctx, processor, thinkCtx); err != nil {
			p.logger.Error("pipeline execution failed",
				logging.String("processor", processor.Name()),
				logging.Err(err),
			)
			return fmt.Errorf("processor %s failed: %w", processor.Name(), err)
		}
	}

	p.logger.Info("pipeline execution completed",
		logging.String("session_id", thinkCtx.SessionID),
		logging.Duration("duration", thinkCtx.Duration()),
		logging.Bool("has_errors", thinkCtx.HasErrors()),
	)

	return nil
}

// executeProcessor 执行单个处理器
func (p *Pipeline) executeProcessor(ctx context.Context, processor core.Processor, thinkCtx *entity.ThinkContext) error {
	start := time.Now()
	processorName := processor.Name()

	p.logger.Debug("processor started",
		logging.String("processor", processorName),
	)

	// 执行处理器
	err := processor.Process(ctx, thinkCtx)
	duration := time.Since(start)

	if err != nil {
		// 记录错误到上下文
		thinkCtx.AddError(processorName, err)

		p.logger.Warn("processor failed",
			logging.String("processor", processorName),
			logging.Duration("duration", duration),
			logging.Err(err),
		)

		return err
	}

	p.logger.Debug("processor completed",
		logging.String("processor", processorName),
		logging.Duration("duration", duration),
	)

	return nil
}

// GetProcessors 获取所有处理器
func (p *Pipeline) GetProcessors() []core.Processor {
	return p.processors
}
