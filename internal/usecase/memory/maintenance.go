package memory

import (
	"encoding/json"
	"fmt"
	"mindx/internal/core"
	"mindx/internal/entity"
	apperrors "mindx/internal/errors"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"strings"
	"time"
)

func (m *Memory) CleanupExpiredMemories() error {
	m.logger.Info(i18n.T("memory.start_cleanup"))

	allMemories, err := m.getAllMemories()
	if err != nil {
		m.logger.Error(i18n.T("memory.get_memories_failed"), logging.Err(err))
		return apperrors.Wrap(err, apperrors.ErrTypeMemory, "获取记忆点失败")
	}

	deletedCount := 0
	for _, memoryPoint := range allMemories {
		if memoryPoint.TotalWeight < 0.1 && time.Since(memoryPoint.CreatedAt).Hours() > 30*24 {
			key := m.generateMemoryKey(memoryPoint.CreatedAt)
			if err := m.store.Delete(key); err != nil {
				m.logger.Error(i18n.T("memory.del_low_weight_failed"), logging.Err(err), logging.Int(i18n.T("memory.id"), memoryPoint.ID))
				continue
			}
			deletedCount++
			continue
		}

		if strings.TrimSpace(memoryPoint.Content) == "" || len(memoryPoint.Keywords) == 0 {
			key := m.generateMemoryKey(memoryPoint.CreatedAt)
			if err := m.store.Delete(key); err != nil {
				m.logger.Error(i18n.T("memory.del_invalid_failed"), logging.Err(err), logging.Int(i18n.T("memory.id"), memoryPoint.ID))
				continue
			}
			deletedCount++
			continue
		}
	}

	m.logger.Info(i18n.T("memory.cleanup_complete"), logging.Int(i18n.T("memory.deleted"), deletedCount))
	return nil
}
func (m *Memory) AdjustMemoryWeight(id int, multiple float64) error {
	m.logger.Info(i18n.T("memory.start_adjust_weight"), logging.Int(i18n.T("memory.id"), id), logging.Float64(i18n.T("memory.multiple"), multiple))

	allMemories, err := m.getAllMemories()
	if err != nil {
		m.logger.Error(i18n.T("memory.get_memories_failed"), logging.Err(err))
		return apperrors.Wrap(err, apperrors.ErrTypeMemory, "获取记忆点失败")
	}

	var targetPoint core.MemoryPoint
	found := false

	for _, mem := range allMemories {
		if mem.ID == id {
			targetPoint = mem
			found = true
			break
		}
	}

	if !found {
		m.logger.Error(i18n.T("memory.target_not_found"), logging.Int(i18n.T("memory.id"), id))
		return apperrors.New(apperrors.ErrTypeMemory, fmt.Sprintf("未找到ID为%d的记忆点", id))
	}

	targetPoint.TotalWeight = targetPoint.TotalWeight * multiple
	if targetPoint.TotalWeight > 3.0 {
		targetPoint.TotalWeight = 3.0
	} else if targetPoint.TotalWeight < 0.1 {
		targetPoint.TotalWeight = 0.1
	}
	targetPoint.UpdatedAt = time.Now()

	if err := m.storeMemory(targetPoint); err != nil {
		m.logger.Error(i18n.T("memory.update_weight_failed"), logging.Err(err), logging.Int(i18n.T("memory.id"), id))
		return apperrors.Wrap(err, apperrors.ErrTypeMemory, "更新记忆权重失败")
	}

	m.logger.Info(i18n.T("memory.adjust_weight_success"), logging.Int(i18n.T("memory.id"), id), logging.Float64(i18n.T("memory.new_weight"), targetPoint.TotalWeight))
	return nil
}

func (m *Memory) Optimize() error {
	m.logger.Info(i18n.T("memory.start_optimize"))

	if err := m.CleanupExpiredMemories(); err != nil {
		m.logger.Error(i18n.T("memory.cleanup_failed"), logging.Err(err))
		return apperrors.Wrap(err, apperrors.ErrTypeMemory, "清理过期记忆失败")
	}

	m.logger.Info(i18n.T("memory.optimize_complete"))
	return nil
}

// DecayWeights 定期权重衰减
// 扫描所有记忆点，重新计算 TimeWeight，删除 TotalWeight < 0.05 的记忆
func (m *Memory) DecayWeights() error {
	m.logger.Info("开始记忆权重衰减")

	allMemories, err := m.getAllMemories()
	if err != nil {
		return err
	}

	decayedCount := 0
	deletedCount := 0
	now := time.Now()

	for _, mem := range allMemories {
		daysSinceCreation := now.Sub(mem.CreatedAt).Hours() / 24
		// 时间衰减：使用指数衰减函数
		newTimeWeight := 1.0 / (1.0 + daysSinceCreation/30.0)
		mem.TimeWeight = newTimeWeight
		mem.TotalWeight = (mem.TimeWeight + mem.RepeatWeight + mem.EmphasisWeight) / 3.0
		mem.UpdatedAt = now

		if mem.TotalWeight < 0.05 {
			key := m.generateMemoryKey(mem.CreatedAt)
			if err := m.store.Delete(key); err != nil {
				m.logger.Warn("删除低权重记忆失败", logging.Err(err))
				continue
			}
			deletedCount++
			continue
		}

		if err := m.storeMemory(mem); err != nil {
			m.logger.Warn("更新记忆权重失败", logging.Err(err))
			continue
		}
		decayedCount++
	}

	m.logger.Info("记忆权重衰减完成",
		logging.Int("decayed", decayedCount),
		logging.Int("deleted", deletedCount))
	return nil
}

func (m *Memory) getAllMemories() ([]core.MemoryPoint, error) {
	if m.store == nil {
		return []core.MemoryPoint{}, nil
	}

	allMemories, err := m.store.Search(nil, 1000)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.ErrTypeMemory, "获取记忆点失败")
	}

	memoryPoints := make([]core.MemoryPoint, 0, len(allMemories))
	for _, mem := range allMemories {
		var memoryPoint core.MemoryPoint
		if err := json.Unmarshal(mem.Metadata, &memoryPoint); err != nil {
			continue
		}
		memoryPoints = append(memoryPoints, memoryPoint)
	}

	return memoryPoints, nil
}

func (m *Memory) GetAllMemoryPoints() ([]core.MemoryPoint, error) {
	return m.getAllMemories()
}

func (m *Memory) parseMemoryPoint(entry entity.VectorEntry) (core.MemoryPoint, error) {
	var memoryPoint core.MemoryPoint
	err := json.Unmarshal(entry.Metadata, &memoryPoint)
	return memoryPoint, err
}

func (m *Memory) generateMemoryKey(t time.Time) string {
	return fmt.Sprintf("memory_%d", t.UnixNano())
}

func (m *Memory) storeMemory(point core.MemoryPoint) error {
	if m.store == nil {
		return nil
	}

	key := fmt.Sprintf("memory_%d", time.Now().UnixNano())
	metadata := map[string]any{
		"memory_point": point,
	}

	return m.store.Put(key, point.Vector, metadata)
}
