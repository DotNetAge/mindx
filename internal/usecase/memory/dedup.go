package memory

import (
	"encoding/json"
	"mindx/internal/core"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"strings"
	"time"
)

// DeduplicateMemory 语义去重
// 搜索相似度 > 0.85 的已有记忆，如果找到则合并内容
// 返回合并后的记忆点和是否发生了合并
func (m *Memory) DeduplicateMemory(newPoint *core.MemoryPoint) (*core.MemoryPoint, bool) {
	if m.store == nil || len(newPoint.Vector) == 0 {
		return newPoint, false
	}

	// 搜索高相似度的已有记忆
	results, err := m.store.SearchWithThreshold(newPoint.Vector, 1, 0.85)
	if err != nil || len(results) == 0 {
		return newPoint, false
	}

	// 解析已有记忆点
	var existingPoint core.MemoryPoint
	if err := json.Unmarshal(results[0].Metadata, &existingPoint); err != nil {
		return newPoint, false
	}

	// 合并内容：保留更完整的版本
	merged := mergeMemoryPoints(existingPoint, *newPoint)

	m.logger.Info(i18n.T("memory.dedup_merged"),
		logging.Int(i18n.T("memory.id"), existingPoint.ID),
		logging.Int("keywords_count", len(merged.Keywords)),
	)

	return &merged, true
}

// mergeMemoryPoints 合并两个记忆点
func mergeMemoryPoints(existing, new core.MemoryPoint) core.MemoryPoint {
	merged := existing

	// 保留更长的内容
	if len(new.Content) > len(existing.Content) {
		merged.Content = new.Content
	}

	// 保留更长的摘要
	if len(new.Summary) > len(existing.Summary) {
		merged.Summary = new.Summary
	}

	// 合并关键词（去重）
	keywordSet := make(map[string]bool)
	for _, kw := range existing.Keywords {
		keywordSet[strings.TrimSpace(kw)] = true
	}
	for _, kw := range new.Keywords {
		kw = strings.TrimSpace(kw)
		if kw != "" {
			keywordSet[kw] = true
		}
	}
	merged.Keywords = make([]string, 0, len(keywordSet))
	for kw := range keywordSet {
		merged.Keywords = append(merged.Keywords, kw)
	}

	// 取较高权重
	if new.TotalWeight > existing.TotalWeight {
		merged.TotalWeight = new.TotalWeight
	}
	merged.RepeatWeight += 0.1 // 重复出现增加权重
	merged.UpdatedAt = time.Now()

	// 使用新的向量
	if len(new.Vector) > 0 {
		merged.Vector = new.Vector
	}

	return merged
}
