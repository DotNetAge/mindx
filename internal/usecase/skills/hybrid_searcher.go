package skills

import (
	"mindx/internal/entity"
	"sync"
	"time"
)

// HybridSearcher 混合检索器
// 结合向量搜索和关键词搜索，提供更准确的 Skill 匹配
type HybridSearcher struct {
	vectorIndex  *VectorIndex
	keywordIndex *KeywordIndex

	// 权重配置
	vectorWeight  float64 // 向量搜索权重（默认 0.7）
	keywordWeight float64 // 关键词搜索权重（默认 0.3）

	// 缓存
	cache      map[string]*CachedResult
	cacheMu    sync.RWMutex
	cacheSize  int           // 最大缓存条目数
	cacheTTL   time.Duration // 缓存过期时间
	cacheStats CacheStats    // 缓存统计
}

// CachedResult 缓存的搜索结果
type CachedResult struct {
	Matches   []*entity.SkillMatch
	Timestamp time.Time
}

// CacheStats 缓存统计
type CacheStats struct {
	Hits   int64
	Misses int64
	Evicts int64
}

// HybridSearchConfig 混合检索配置
type HybridSearchConfig struct {
	VectorWeight  float64       // 向量搜索权重 [0.0, 1.0]
	KeywordWeight float64       // 关键词搜索权重 [0.0, 1.0]
	CacheSize     int           // 缓存大小（0 表示禁用缓存）
	CacheTTL      time.Duration // 缓存过期时间
}

// DefaultHybridSearchConfig 默认配置
func DefaultHybridSearchConfig() *HybridSearchConfig {
	return &HybridSearchConfig{
		VectorWeight:  0.7,
		KeywordWeight: 0.3,
		CacheSize:     100,
		CacheTTL:      5 * time.Minute,
	}
}

// NewHybridSearcher 创建混合检索器
func NewHybridSearcher(vectorIndex *VectorIndex, keywordIndex *KeywordIndex, config *HybridSearchConfig) *HybridSearcher {
	if config == nil {
		config = DefaultHybridSearchConfig()
	}

	// 归一化权重
	totalWeight := config.VectorWeight + config.KeywordWeight
	if totalWeight == 0 {
		config.VectorWeight = 0.7
		config.KeywordWeight = 0.3
		totalWeight = 1.0
	}

	return &HybridSearcher{
		vectorIndex:   vectorIndex,
		keywordIndex:  keywordIndex,
		vectorWeight:  config.VectorWeight / totalWeight,
		keywordWeight: config.KeywordWeight / totalWeight,
		cache:         make(map[string]*CachedResult),
		cacheSize:     config.CacheSize,
		cacheTTL:      config.CacheTTL,
	}
}

// Search 混合检索
func (s *HybridSearcher) Search(query string, topK int) ([]*entity.SkillMatch, error) {
	// 1. 检查缓存
	if s.cacheSize > 0 {
		if cached := s.getFromCache(query); cached != nil {
			return cached, nil
		}
	}

	// 2. 提取关键词
	keywords := tokenize(query)

	// 3. 向量搜索
	vectorMatches, err := s.vectorIndex.Search(query, topK*2)
	if err != nil {
		// 向量搜索失败，回退到关键词搜索
		keywordMatches := s.keywordIndex.Search(keywords, topK)
		return s.convertKeywordMatches(keywordMatches), nil
	}

	// 4. 关键词搜索
	keywordMatches := s.keywordIndex.Search(keywords, topK*2)

	// 5. 融合结果
	finalMatches := s.fuseResults(vectorMatches, keywordMatches)

	// 6. 返回 TopK
	if len(finalMatches) > topK {
		finalMatches = finalMatches[:topK]
	}

	// 7. 缓存结果
	if s.cacheSize > 0 {
		s.putToCache(query, finalMatches)
	}

	return finalMatches, nil
}

// SearchWithWeights 使用自定义权重搜索
func (s *HybridSearcher) SearchWithWeights(query string, topK int, vectorWeight, keywordWeight float64) ([]*entity.SkillMatch, error) {
	// 临时修改权重
	oldVectorWeight := s.vectorWeight
	oldKeywordWeight := s.keywordWeight

	totalWeight := vectorWeight + keywordWeight
	if totalWeight > 0 {
		s.vectorWeight = vectorWeight / totalWeight
		s.keywordWeight = keywordWeight / totalWeight
	}

	// 执行搜索
	matches, err := s.Search(query, topK)

	// 恢复权重
	s.vectorWeight = oldVectorWeight
	s.keywordWeight = oldKeywordWeight

	return matches, err
}

// fuseResults 融合向量搜索和关键词搜索的结果
func (s *HybridSearcher) fuseResults(vectorMatches []*entity.SkillMatch, keywordMatches []*SkillMatch) []*entity.SkillMatch {
	// 1. 构建 Skill 名称到分数的映射
	scoreMap := make(map[string]*scoreEntry)

	// 2. 添加向量搜索分数
	for _, match := range vectorMatches {
		name := match.Skill.Name
		if _, ok := scoreMap[name]; !ok {
			scoreMap[name] = &scoreEntry{
				skill: match.Skill,
			}
		}
		scoreMap[name].vectorScore = match.Score
	}

	// 3. 添加关键词搜索分数
	for _, match := range keywordMatches {
		name := match.Name
		if _, ok := scoreMap[name]; !ok {
			// 从 vectorIndex 获取完整的 Skill（因为 keywordIndex 存储的是旧的 SkillDef）
			skill, err := s.vectorIndex.GetSkill(name)
			if err != nil {
				continue
			}
			scoreMap[name] = &scoreEntry{
				skill: skill,
			}
		}
		scoreMap[name].keywordScore = match.Score
	}

	// 4. 计算融合分数
	matches := make([]*entity.SkillMatch, 0, len(scoreMap))
	for _, entry := range scoreMap {
		// 归一化分数到 [0, 1]
		normalizedVectorScore := s.normalizeScore(entry.vectorScore)
		normalizedKeywordScore := s.normalizeScore(entry.keywordScore)

		// 加权融合
		fusedScore := s.vectorWeight*normalizedVectorScore + s.keywordWeight*normalizedKeywordScore

		matches = append(matches, &entity.SkillMatch{
			Skill: entry.skill,
			Score: fusedScore,
		})
	}

	// 5. 按分数排序
	s.sortByScore(matches)

	return matches
}

// convertKeywordMatches 转换关键词匹配结果为 entity.SkillMatch
func (s *HybridSearcher) convertKeywordMatches(keywordMatches []*SkillMatch) []*entity.SkillMatch {
	matches := make([]*entity.SkillMatch, 0, len(keywordMatches))

	for _, km := range keywordMatches {
		// 从 vectorIndex 获取完整的 Skill
		skill, err := s.vectorIndex.GetSkill(km.Name)
		if err != nil {
			continue
		}

		matches = append(matches, &entity.SkillMatch{
			Skill: skill,
			Score: km.Score,
		})
	}

	return matches
}

// scoreEntry 分数条目
type scoreEntry struct {
	skill        *entity.Skill
	vectorScore  float64
	keywordScore float64
}

// normalizeScore 归一化分数到 [0, 1]
func (s *HybridSearcher) normalizeScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

// sortByScore 按分数排序（降序）
func (s *HybridSearcher) sortByScore(matches []*entity.SkillMatch) {
	n := len(matches)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if matches[j].Score < matches[j+1].Score {
				matches[j], matches[j+1] = matches[j+1], matches[j]
			}
		}
	}
}

// getFromCache 从缓存获取结果
func (s *HybridSearcher) getFromCache(query string) []*entity.SkillMatch {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()

	cached, ok := s.cache[query]
	if !ok {
		s.cacheStats.Misses++
		return nil
	}

	// 检查是否过期
	if time.Since(cached.Timestamp) > s.cacheTTL {
		s.cacheStats.Misses++
		return nil
	}

	s.cacheStats.Hits++
	return cached.Matches
}

// putToCache 将结果放入缓存
func (s *HybridSearcher) putToCache(query string, matches []*entity.SkillMatch) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	// 检查缓存大小
	if len(s.cache) >= s.cacheSize {
		// 简单的 LRU：删除最旧的条目
		s.evictOldest()
	}

	s.cache[query] = &CachedResult{
		Matches:   matches,
		Timestamp: time.Now(),
	}
}

// evictOldest 驱逐最旧的缓存条目
func (s *HybridSearcher) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	first := true
	for key, cached := range s.cache {
		if first || cached.Timestamp.Before(oldestTime) {
			oldestKey = key
			oldestTime = cached.Timestamp
			first = false
		}
	}

	if oldestKey != "" {
		delete(s.cache, oldestKey)
		s.cacheStats.Evicts++
	}
}

// ClearCache 清空缓存
func (s *HybridSearcher) ClearCache() {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	s.cache = make(map[string]*CachedResult)
}

// GetCacheStats 获取缓存统计
func (s *HybridSearcher) GetCacheStats() CacheStats {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()

	return s.cacheStats
}

// GetCacheHitRate 获取缓存命中率
func (s *HybridSearcher) GetCacheHitRate() float64 {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()

	total := s.cacheStats.Hits + s.cacheStats.Misses
	if total == 0 {
		return 0
	}

	return float64(s.cacheStats.Hits) / float64(total)
}

// SetWeights 设置权重
func (s *HybridSearcher) SetWeights(vectorWeight, keywordWeight float64) {
	totalWeight := vectorWeight + keywordWeight
	if totalWeight == 0 {
		return
	}

	s.vectorWeight = vectorWeight / totalWeight
	s.keywordWeight = keywordWeight / totalWeight
}

// GetWeights 获取当前权重
func (s *HybridSearcher) GetWeights() (vectorWeight, keywordWeight float64) {
	return s.vectorWeight, s.keywordWeight
}
