package core

import (
	"context"
	"mindx/internal/entity"
	"time"
)

// Processor 处理器接口
// 每个处理器负责单一职责，修改 ThinkContext 的特定部分
type Processor interface {
	// Name 返回处理器名称（用于日志和监控）
	Name() string

	// Process 处理上下文，修改 ThinkContext
	// 返回 error 表示处理失败，Pipeline 将中断或触发降级
	Process(ctx context.Context, thinkCtx *entity.ThinkContext) error
}

// ProcessorMetrics 处理器性能指标
type ProcessorMetrics struct {
	ProcessorName string
	ExecutionTime time.Duration
	Success       bool
	Error         error
	Timestamp     time.Time
}

// ProcessorError 处理器错误
type ProcessorError struct {
	ProcessorName string
	Error         error
	Timestamp     time.Time
}
