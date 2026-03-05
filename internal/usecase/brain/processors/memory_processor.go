package processors

import (
	"context"
	"fmt"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/pkg/logging"
)

// MemoryRetrievalProcessor 记忆检索处理器
// 职责：基于意图关键词检索相关记忆点
// MVP 简化版：只做关键词匹配，不做向量相似度搜索
type MemoryRetrievalProcessor struct {
	memory core.Memory
	topK   int // 返回最多 K 个记忆点
	logger logging.Logger
}

// NewMemoryRetrievalProcessor 创建记忆检索处理器
func NewMemoryRetrievalProcessor(memory core.Memory, topK int) *MemoryRetrievalProcessor {
	if topK <= 0 {
		topK = 5 // 默认返回 5 个
	}

	return &MemoryRetrievalProcessor{
		memory: memory,
		topK:   topK,
		logger: logging.GetSystemLogger().Named("memory_processor"),
	}
}

// Name 返回处理器名称
func (p *MemoryRetrievalProcessor) Name() string {
	return "MemoryRetrievalProcessor"
}

// Process 处理记忆检索
func (p *MemoryRetrievalProcessor) Process(ctx context.Context, thinkCtx *entity.ThinkContext) error {
	// 1. 检查是否有意图（依赖 IntentProcessor）
	if thinkCtx.Intent == nil {
		p.logger.Debug("no intent found, skip memory retrieval")
		return nil
	}

	// 2. 检查是否有关键词
	if len(thinkCtx.Intent.Keywords) == 0 {
		p.logger.Debug("no keywords found, skip memory retrieval")
		return nil
	}

	p.logger.Debug("memory retrieval started",
		logging.Int("keywords_count", len(thinkCtx.Intent.Keywords)),
	)

	// 3. 构建搜索词（将关键词拼接）
	searchTerms := p.buildSearchTerms(thinkCtx.Intent.Keywords)

	// 4. 搜索记忆
	memories, err := p.memory.Search(searchTerms)
	if err != nil {
		// 记忆检索失败不影响核心功能，只记录警告
		p.logger.Warn("memory search failed",
			logging.Err(err),
		)
		return nil
	}

	// 5. 限制返回数量
	if len(memories) > p.topK {
		memories = memories[:p.topK]
	}

	// 6. 转换为 entity.MemoryPoint
	entityMemories := p.convertToEntityMemories(memories)

	// 7. 填充到上下文
	thinkCtx.Memories = entityMemories

	p.logger.Info("memory retrieval completed",
		logging.Int("found_count", len(entityMemories)),
	)

	return nil
}

// buildSearchTerms 构建搜索词
func (p *MemoryRetrievalProcessor) buildSearchTerms(keywords []string) string {
	if len(keywords) == 0 {
		return ""
	}

	// 简单拼接关键词
	result := ""
	for i, keyword := range keywords {
		if i > 0 {
			result += " "
		}
		result += keyword
	}

	return result
}

// convertToEntityMemories 转换记忆点格式
func (p *MemoryRetrievalProcessor) convertToEntityMemories(coreMemories []core.MemoryPoint) []*entity.MemoryPoint {
	result := make([]*entity.MemoryPoint, 0, len(coreMemories))

	for _, mem := range coreMemories {
		result = append(result, &entity.MemoryPoint{
			ID:        fmt.Sprintf("%d", mem.ID),
			Content:   mem.Content,
			Keywords:  mem.Keywords,
			Timestamp: mem.CreatedAt,
		})
	}

	return result
}
