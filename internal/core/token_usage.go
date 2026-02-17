package core

import (
	"mindx/internal/entity"
	"time"
)

// TokenUsageRepository Token 使用记录仓库接口
type TokenUsageRepository interface {
	// Save 保存 Token 使用记录
	Save(usage *entity.TokenUsage) error

	// GetByID 根据 ID 获取记录
	GetByID(id int) (*entity.TokenUsage, error)

	// GetByModel 根据模型名称获取记录
	GetByModel(model string, limit int) ([]*entity.TokenUsage, error)

	// GetByTimeRange 根据时间范围获取记录
	GetByTimeRange(start, end time.Time) ([]*entity.TokenUsage, error)

	// GetSummary 获取汇总统计
	GetSummary() (*entity.TokenUsageSummary, error)

	// GetSummaryByModel 按模型分组获取统计
	GetSummaryByModel() ([]*entity.TokenUsageByModelSummary, error)

	// Delete 删除记录
	Delete(id int) error

	// Close 关闭仓库
	Close() error
}
