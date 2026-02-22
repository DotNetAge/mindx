package memory

import (
	"mindx/internal/config"
	"mindx/internal/core"
	"mindx/internal/entity"
	"mindx/internal/usecase/embedding"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/muesli/clusters"
	openai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type Memory struct {
	store            core.Store
	llmClient        *openai.Client
	summaryModel     string
	keywordModel     string
	config           *config.VectorStoreConfig
	logger           logging.Logger
	embeddingService *embedding.EmbeddingService
}

func NewMemory(
	cfg *config.GlobalConfig,
	llmClient *openai.Client,
	logger logging.Logger,
	store core.Store,
	embeddingService *embedding.EmbeddingService,
) (*Memory, error) {
	if cfg.VectorStore.Type == "" {
		cfg.VectorStore.Type = "badger"
	}
	if cfg.VectorStore.DataPath == "" {
		cfg.VectorStore.DataPath = filepath.Join("data", "memory")
	}

	if err := os.MkdirAll(cfg.VectorStore.DataPath, 0755); err != nil {
		return nil, fmt.Errorf("创建记忆存储目录失败: %w", err)
	}

	memory := &Memory{
		store:            store,
		llmClient:        llmClient,
		summaryModel:     cfg.Memory.SummaryModel,
		keywordModel:     cfg.Memory.KeywordModel,
		config:           &cfg.VectorStore,
		logger:           logger,
		embeddingService: embeddingService,
	}

	logger.Info(i18n.T("memory.init_success"),
		logging.String(i18n.T("memory.type"), cfg.VectorStore.Type),
		logging.String(i18n.T("memory.path"), cfg.VectorStore.DataPath))

	return memory, nil
}

func (m *Memory) Record(point core.MemoryPoint) error {
	m.logger.Debug(i18n.T("memory.start_record"), logging.Int(i18n.T("memory.keywords_count"), len(point.Keywords)))

	if point.CreatedAt.IsZero() {
		point.CreatedAt = time.Now()
	}
	point.UpdatedAt = time.Now()

	if len(point.Vector) == 0 {
		if m.embeddingService != nil {
			combinedText := strings.Join(point.Keywords, " ") + " " + point.Summary + " " + point.Content
			vector, err := m.embeddingService.GenerateEmbedding(combinedText)
			if err != nil {
				m.logger.Warn(i18n.T("memory.gen_vector_failed"), logging.Err(err))
				vector = []float64{}
			}
			point.Vector = vector
		} else {
			point.Vector = []float64{}
		}
	}

	if err := m.storeMemory(point); err != nil {
		m.logger.Error(i18n.T("memory.store_failed"), logging.Err(err))
		return fmt.Errorf("存储记忆失败: %w", err)
	}

	m.logger.Info(i18n.T("memory.record_success"),
		logging.Int(i18n.T("memory.id"), point.ID),
		logging.Int(i18n.T("memory.keywords_count"), len(point.Keywords)),
		logging.Float64(i18n.T("memory.total_weight"), point.TotalWeight))

	go func() {
		if err := m.CleanupExpiredMemories(); err != nil {
			m.logger.Error(i18n.T("memory.cleanup_failed"), logging.Err(err))
		}
	}()

	return nil
}

func (m *Memory) Search(terms string) ([]core.MemoryPoint, error) {
	m.logger.Debug(i18n.T("memory.start_search"), logging.String(i18n.T("memory.terms"), terms))

	if m.embeddingService == nil {
		return []core.MemoryPoint{}, nil
	}

	termVector, err := m.embeddingService.GenerateEmbedding(terms)
	if err != nil {
		m.logger.Error(i18n.T("memory.gen_search_vector_failed"), logging.Err(err))
		if m.store == nil {
			return []core.MemoryPoint{}, nil
		}
		entries, err := m.store.Search(nil, 10)
		if err != nil {
			m.logger.Error(i18n.T("memory.vector_search_failed"), logging.Err(err))
			return nil, fmt.Errorf("向量搜索失败: %w", err)
		}
		filtered := m.filterByKeywords(entries, terms)
		sorted := m.sortByWeight(filtered, 3)
		return sorted, nil
	}

	allMemories, err := m.getAllMemories()
	if err != nil {
		m.logger.Error(i18n.T("memory.get_memories_failed"), logging.Err(err))
		return nil, fmt.Errorf("获取记忆点失败: %w", err)
	}

	var candidatePoints []core.MemoryPoint
	for _, memoryPoint := range allMemories {
		if len(memoryPoint.Vector) == 0 || len(termVector) == 0 {
			continue
		}

		similarity := m.calculateCosineSimilarity(memoryPoint.Vector, termVector)
		if similarity >= 0.5 {
			candidatePoints = append(candidatePoints, memoryPoint)
		}
	}

	var candidateEntries []entity.VectorEntry
	for _, point := range candidatePoints {
		metadataBytes, err := json.Marshal(point)
		if err != nil {
			continue
		}
		candidateEntries = append(candidateEntries, entity.VectorEntry{
			Metadata: metadataBytes,
		})
	}

	filteredPoints := m.filterByKeywords(candidateEntries, terms)
	m.logger.Debug(i18n.T("memory.after_filter"), logging.Int(i18n.T("memory.count"), len(filteredPoints)))

	sorted := m.sortByWeight(filteredPoints, 3)

	m.logger.Info(i18n.T("memory.search_complete"), logging.Int(i18n.T("memory.found"), len(sorted)))

	return sorted, nil
}

func (m *Memory) generateSummary(text string) (string, error) {
	if m.llmClient == nil {
		if len(text) > 200 {
			return text[:200] + "...", nil
		}
		return text, nil
	}

	resp, err := m.llmClient.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: m.summaryModel,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "请将以下对话内容精炼成一段简洁的摘要，保留关键信息：",
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: text,
				},
			},
		},
	)

	if err != nil {
		return "", err
	}

	if len(resp.Choices) > 0 {
		return resp.Choices[0].Message.Content, nil
	}

	return text, nil
}

func (m *Memory) generateKeywords(text string) ([]string, error) {
	if m.llmClient == nil {
		return m.simpleTokenize(text), nil
	}

	type KeywordsResponse struct {
		Keywords []string `json:"keywords" required:"true" description:"List of keywords extracted from the text"`
	}

	resp, err := m.llmClient.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: m.keywordModel,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "请从以下对话内容中提取 3-5 个最重要的关键词：",
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: text,
				},
			},
			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
				JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
					Name: "keywords",
					Schema: &jsonschema.Definition{
						Type: jsonschema.Object,
						Properties: map[string]jsonschema.Definition{
							"keywords": {
								Type: jsonschema.Array,
								Items: &jsonschema.Definition{
									Type: jsonschema.String,
								},
							},
						},
						Required: []string{"keywords"},
					},
					Strict: true,
				},
			},
		},
	)

	if err != nil {
		return nil, err
	}

	if len(resp.Choices) > 0 {
		var result struct {
			Keywords []string `json:"keywords"`
		}
		if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &result); err == nil {
			return result.Keywords, nil
		}
	}

	return m.simpleTokenize(text), nil
}

func (m *Memory) calculateTimeWeight(t time.Time) float64 {
	days := time.Since(t).Hours() / 24
	if days <= 3 {
		return 1.0 / (1.0 + 0.8*days)
	}
	return 1.0 / (1.0 + 0.3*days)
}

func (m *Memory) calculateRepeatWeight(text string) float64 {
	if m.store == nil {
		return 1.0
	}

	allMemories, err := m.store.Search(nil, 1000)
	if err != nil {
		m.logger.Error(i18n.T("memory.get_history_failed"), logging.Err(err))
		return 1.0
	}
	repeatCount := 0
	textLower := strings.ToLower(text)

	for _, mem := range allMemories {
		var memoryPoint core.MemoryPoint
		if err := json.Unmarshal(mem.Metadata, &memoryPoint); err != nil {
			continue
		}

		kwMatch := false
		for _, kw := range memoryPoint.Keywords {
			if strings.Contains(textLower, strings.ToLower(kw)) {
				kwMatch = true
				break
			}
		}
		contentSimilar := strings.Contains(textLower, strings.ToLower(memoryPoint.Summary)) ||
			strings.Contains(strings.ToLower(memoryPoint.Summary), textLower)

		if kwMatch && contentSimilar {
			repeatCount++
		}
	}

	repeatWeight := 1.0 + float64(repeatCount)*0.2
	if repeatWeight > 2.0 {
		repeatWeight = 2.0
	}
	return repeatWeight
}

func (m *Memory) calculateEmphasisWeight(text string) float64 {
	textLower := strings.ToLower(text)
	emphasisLevels := map[string]float64{
		"务必":  0.4,
		"关键":  0.35,
		"重要":  0.3,
		"记住":  0.25,
		"一定要": 0.25,
		"千万别": 0.25,
		"must":      0.4,
		"key":       0.35,
		"important": 0.3,
		"remember":  0.25,
		"never":     0.25,
	}

	maxWeight := 0.2
	for word, weight := range emphasisLevels {
		if strings.Contains(textLower, word) && weight > maxWeight {
			maxWeight = weight
		}
	}

	if strings.Contains(text, "！") || strings.Contains(text, "!!") ||
		strings.Contains(textLower, "重要重要") || strings.Contains(textLower, "记住记住") {
		maxWeight += 0.05
	}
	return maxWeight
}

func (m *Memory) calculateTotalWeight(timeWeight, repeatWeight, emphasisWeight float64, scene string) float64 {
	var timeRatio, emphasisRatio, repeatRatio float64
	switch scene {
	case "chat":
		timeRatio, emphasisRatio, repeatRatio = 0.6, 0.25, 0.15
	case "knowledge":
		timeRatio, emphasisRatio, repeatRatio = 0.2, 0.4, 0.4
	default:
		timeRatio, emphasisRatio, repeatRatio = 0.4, 0.35, 0.25
	}
	return timeWeight*timeRatio + emphasisWeight*emphasisRatio + repeatWeight*repeatRatio
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

func (m *Memory) filterByKeywords(entries []entity.VectorEntry, terms string) []core.MemoryPoint {
	var filtered []core.MemoryPoint
	termsLower := strings.ToLower(terms)

	for _, entry := range entries {
		var point core.MemoryPoint
		if err := json.Unmarshal(entry.Metadata, &point); err != nil {
			continue
		}

		similarity := m.calculateKeywordSimilarity(point.Keywords, termsLower)
		if similarity > 0.6 {
			filtered = append(filtered, point)
		}
	}

	return filtered
}

func (m *Memory) calculateKeywordSimilarity(keywords []string, terms string) float64 {
	if len(keywords) == 0 {
		return 0
	}

	matchCount := 0
	for _, kw := range keywords {
		kwLower := strings.ToLower(kw)
		if strings.Contains(terms, kwLower) || strings.Contains(kwLower, terms) {
			matchCount++
		}
	}

	return float64(matchCount) / float64(len(keywords))
}

func (m *Memory) sortByWeight(points []core.MemoryPoint, topN int) []core.MemoryPoint {
	if len(points) == 0 {
		return []core.MemoryPoint{}
	}

	sort.Slice(points, func(i, j int) bool {
		return points[i].TotalWeight > points[j].TotalWeight
	})

	if topN > len(points) {
		topN = len(points)
	}

	return points[:topN]
}

func (m *Memory) calculateCosineSimilarity(vec1, vec2 []float64) float64 {
	var dotProduct, norm1, norm2 float64
	for i := range vec1 {
		if i >= len(vec2) {
			break
		}
		dotProduct += vec1[i] * vec2[i]
		norm1 += vec1[i] * vec1[i]
		norm2 += vec2[i] * vec2[i]
	}
	if norm1 == 0 || norm2 == 0 {
		return 0
	}
	return dotProduct / (math.Sqrt(norm1) * math.Sqrt(norm2))
}

func (m *Memory) simpleTokenize(text string) []string {
	tokens := strings.Fields(text)
	var keywords []string

	for _, token := range tokens {
		token = strings.Trim(token, "，。！？、；：\"\"''（）")
		if len(token) >= 2 {
			keywords = append(keywords, token)
		}
	}

	if len(keywords) > 5 {
		keywords = keywords[:5]
	}

	return keywords
}

func (m *Memory) CleanupExpiredMemories() error {
	m.logger.Info(i18n.T("memory.start_cleanup"))

	allMemories, err := m.getAllMemories()
	if err != nil {
		m.logger.Error(i18n.T("memory.get_memories_failed"), logging.Err(err))
		return fmt.Errorf("获取记忆点失败: %w", err)
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
		return fmt.Errorf("获取记忆点失败: %w", err)
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
		return fmt.Errorf("未找到ID为%d的记忆点", id)
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
		return fmt.Errorf("更新记忆权重失败: %w", err)
	}

	m.logger.Info(i18n.T("memory.adjust_weight_success"), logging.Int(i18n.T("memory.id"), id), logging.Float64(i18n.T("memory.new_weight"), targetPoint.TotalWeight))
	return nil
}

func (m *Memory) Optimize() error {
	m.logger.Info(i18n.T("memory.start_optimize"))

	if err := m.CleanupExpiredMemories(); err != nil {
		m.logger.Error(i18n.T("memory.cleanup_failed"), logging.Err(err))
		return fmt.Errorf("清理过期记忆失败: %w", err)
	}

	m.logger.Info(i18n.T("memory.optimize_complete"))
	return nil
}

func (m *Memory) ClusterConversations(conversations []entity.ConversationLog) error {
	m.logger.Info(i18n.T("memory.start_cluster"), logging.Int(i18n.T("memory.conversation_count"), len(conversations)))

	if len(conversations) == 0 {
		m.logger.Info(i18n.T("memory.no_conversation"))
		return nil
	}

	memoryPoints := make([]core.MemoryPoint, 0, len(conversations))
	textsToEmbed := make([]string, 0, len(conversations))
	embeddingIndices := make(map[int]int)

	for _, conv := range conversations {
		coreContent := m.extractConversationContent(conv)
		keywords, err := m.generateKeywords(coreContent)
		if err != nil {
			m.logger.Warn(i18n.T("memory.gen_keywords_failed"), logging.Err(err))
			keywords = m.simpleTokenize(coreContent)
		}

		summary := conv.Topic
		if summary == "" {
			summary, err = m.generateSummary(coreContent)
			if err != nil {
				m.logger.Warn(i18n.T("memory.gen_summary_failed"), logging.Err(err))
				summary = coreContent
			}
		}

		combinedText := strings.Join(keywords, " ") + " " + summary + " " + coreContent
		textsToEmbed = append(textsToEmbed, combinedText)
		embeddingIndices[len(memoryPoints)] = len(textsToEmbed) - 1

		timeWeight := m.calculateTimeWeight(conv.EndTime)
		repeatWeight := m.calculateRepeatWeight(coreContent)
		emphasisWeight := m.calculateEmphasisWeight(coreContent)
		totalWeight := m.calculateTotalWeight(timeWeight, repeatWeight, emphasisWeight, "chat")

		point := core.MemoryPoint{
			Keywords:       keywords,
			Content:        coreContent,
			Summary:        summary,
			Vector:         []float64{},
			ClusterID:      -1,
			TimeWeight:     timeWeight,
			RepeatWeight:   repeatWeight,
			EmphasisWeight: emphasisWeight,
			TotalWeight:    totalWeight,
			CreatedAt:      conv.StartTime,
			UpdatedAt:      conv.EndTime,
		}

		memoryPoints = append(memoryPoints, point)
	}

	if len(textsToEmbed) > 0 && m.embeddingService != nil {
		vectors, err := m.embeddingService.GenerateBatchEmbeddings(textsToEmbed)
		if err != nil {
			m.logger.Warn(i18n.T("memory.batch_gen_vector_failed"), logging.Err(err))
			for i := range memoryPoints {
				if textIdx, ok := embeddingIndices[i]; ok && textIdx < len(textsToEmbed) {
					vector, err := m.embeddingService.GenerateEmbedding(textsToEmbed[textIdx])
					if err != nil {
						m.logger.Warn(i18n.T("memory.gen_vector_failed"), logging.Err(err))
						vector = []float64{}
					}
					memoryPoints[i].Vector = vector
				}
			}
		} else {
			for i := range memoryPoints {
				if textIdx, ok := embeddingIndices[i]; ok && textIdx < len(vectors) {
					memoryPoints[i].Vector = vectors[textIdx]
				}
			}
		}
	} else {
		for i := range memoryPoints {
			memoryPoints[i].Vector = []float64{}
		}
	}

	if len(memoryPoints) >= 2 {
		var points []clusters.Observation
		pointIndices := make(map[int]int)

		for i, mem := range memoryPoints {
			if len(mem.Vector) > 0 {
				points = append(points, clusters.Coordinates(mem.Vector))
				pointIndices[len(points)-1] = i
			}
		}

		if len(points) >= 2 {
			k := m.determineOptimalK(len(points))

			m.logger.Info(i18n.T("memory.use_kmeans"),
				logging.Int(i18n.T("memory.k"), k),
				logging.Int(i18n.T("memory.total_points"), len(points)))

			kmClusters, err := clusters.New(k, points)
			if err != nil {
				m.logger.Error(i18n.T("memory.create_kmeans_failed"), logging.Err(err))
				for _, point := range memoryPoints {
					if err := m.Record(point); err != nil {
						m.logger.Error(i18n.T("memory.store_failed"), logging.Err(err))
						continue
					}
				}
				m.logger.Info(i18n.T("memory.kmeans_failed_direct_store"))
				return nil
			}

			for _, point := range points {
				nearestIdx := kmClusters.Nearest(point)
				kmClusters[nearestIdx].Append(point)
			}

			kmClusters.Recenter()

			storedCount := 0
			for cid, cluster := range kmClusters {
				var clusterMemories []core.MemoryPoint
				for _, obs := range cluster.Observations {
					for pointIdx, memIdx := range pointIndices {
						obsCoords, ok := obs.(clusters.Coordinates)
						pointCoords, ok2 := points[pointIdx].(clusters.Coordinates)
						if ok && ok2 {
							if len(obsCoords) == len(pointCoords) {
								match := true
								for i := range obsCoords {
									if obsCoords[i] != pointCoords[i] {
										match = false
										break
									}
								}
								if match {
									memoryPoints[memIdx].ClusterID = cid
									clusterMemories = append(clusterMemories, memoryPoints[memIdx])
									break
								}
							}
						}
					}
				}

				if len(clusterMemories) > 0 {
					combinedPoint, err := m.generateCombinedMemoryPoint(clusterMemories, cid)
					if err != nil {
						m.logger.Error(i18n.T("memory.gen_combined_failed"), logging.Err(err))
						for _, mem := range clusterMemories {
							if m.store != nil {
								if err := m.Record(mem); err != nil {
									m.logger.Error(i18n.T("memory.store_cluster_mem_failed"), logging.Err(err))
									continue
								}
								storedCount++
							} else {
								storedCount++
							}
						}
					} else {
						if m.store != nil {
							if err := m.Record(combinedPoint); err != nil {
								m.logger.Error(i18n.T("memory.store_combined_failed"), logging.Err(err))
							} else {
								storedCount++
							}
						} else {
							storedCount++
						}
					}
				}
			}

			m.logger.Info(i18n.T("memory.cluster_complete"),
				logging.Int(i18n.T("memory.total_clusters"), len(kmClusters)),
				logging.Int(i18n.T("memory.stored_points"), storedCount))
		} else {
			for _, point := range memoryPoints {
				if err := m.Record(point); err != nil {
					m.logger.Error(i18n.T("memory.store_failed"), logging.Err(err))
					continue
				}
			}
			m.logger.Info(i18n.T("memory.no_valid_vector"))
		}
	} else if len(memoryPoints) == 1 {
		if err := m.Record(memoryPoints[0]); err != nil {
			m.logger.Error(i18n.T("memory.store_failed"), logging.Err(err))
			return fmt.Errorf("存储记忆点失败: %w", err)
		}
		m.logger.Info(i18n.T("memory.single_mem_store_complete"))
	}

	return nil
}

func (m *Memory) extractConversationContent(conv entity.ConversationLog) string {
	var contentBuilder strings.Builder
	for _, msg := range conv.Messages {
		contentBuilder.WriteString(msg.Sender + ": " + msg.Content + "\n")
	}

	text := contentBuilder.String()
	text = strings.TrimSpace(text)
	if len(text) > 1000 {
		text = text[:1000] + "..."
	}
	return text
}

func (m *Memory) determineOptimalK(pointCount int) int {
	k := int(math.Sqrt(float64(pointCount) / 2))

	if k < 2 {
		k = 2
	}
	if k > 10 {
		k = 10
	}

	if pointCount < 10 {
		k = 2
	} else if pointCount < 20 {
		k = 3
	} else if pointCount < 50 {
		k = 4
	}

	return k
}

func (m *Memory) generateCombinedMemoryPoint(points []core.MemoryPoint, clusterID int) (core.MemoryPoint, error) {
	if len(points) == 0 {
		return core.MemoryPoint{}, fmt.Errorf("no memory points provided")
	}

	var contentBuilder strings.Builder
	keywordMap := make(map[string]int)
	var startTime, endTime time.Time
	startTime = points[0].CreatedAt
	endTime = points[0].UpdatedAt

	for _, point := range points {
		contentBuilder.WriteString(point.Content + "\n")

		for _, keyword := range point.Keywords {
			keywordMap[keyword]++
		}

		if point.CreatedAt.Before(startTime) {
			startTime = point.CreatedAt
		}
		if point.UpdatedAt.After(endTime) {
			endTime = point.UpdatedAt
		}
	}

	type keywordFreq struct {
		keyword string
		freq    int
	}
	var keywordFreqs []keywordFreq
	for keyword, freq := range keywordMap {
		keywordFreqs = append(keywordFreqs, keywordFreq{keyword, freq})
	}

	sort.Slice(keywordFreqs, func(i, j int) bool {
		return keywordFreqs[i].freq > keywordFreqs[j].freq
	})

	var keywords []string
	for i, kf := range keywordFreqs {
		if i >= 10 {
			break
		}
		keywords = append(keywords, kf.keyword)
	}

	combinedContent := contentBuilder.String()
	combinedContent = strings.TrimSpace(combinedContent)
	if len(combinedContent) > 1500 {
		combinedContent = combinedContent[:1500] + "..."
	}

	combinedSummary, err := m.generateSummary(combinedContent)
	if err != nil {
		m.logger.Warn(i18n.T("memory.gen_combined_summary_failed"), logging.Err(err))
		combinedSummary = points[0].Summary
	}

	var combinedVector []float64
	if len(points[0].Vector) > 0 {
		combinedVector = make([]float64, len(points[0].Vector))
		for _, point := range points {
			if len(point.Vector) == len(combinedVector) {
				for i, val := range point.Vector {
					combinedVector[i] += val
				}
			}
		}
		for i := range combinedVector {
			combinedVector[i] /= float64(len(points))
		}
	}

	var totalTimeWeight, totalRepeatWeight, totalEmphasisWeight, totalTotalWeight float64
	for _, point := range points {
		totalTimeWeight += point.TimeWeight
		totalRepeatWeight += point.RepeatWeight
		totalEmphasisWeight += point.EmphasisWeight
		totalTotalWeight += point.TotalWeight
	}

	averageTimeWeight := totalTimeWeight / float64(len(points))
	averageRepeatWeight := totalRepeatWeight / float64(len(points))
	averageEmphasisWeight := totalEmphasisWeight / float64(len(points))
	averageTotalWeight := totalTotalWeight / float64(len(points))

	combinedPoint := core.MemoryPoint{
		Keywords:       keywords,
		Content:        combinedContent,
		Summary:        combinedSummary,
		Vector:         combinedVector,
		ClusterID:      clusterID,
		TimeWeight:     averageTimeWeight,
		RepeatWeight:   averageRepeatWeight,
		EmphasisWeight: averageEmphasisWeight,
		TotalWeight:    averageTotalWeight,
		CreatedAt:      startTime,
		UpdatedAt:      endTime,
	}

	return combinedPoint, nil
}

func (m *Memory) getAllMemories() ([]core.MemoryPoint, error) {
	if m.store == nil {
		return []core.MemoryPoint{}, nil
	}

	allMemories, err := m.store.Search(nil, 1000)
	if err != nil {
		return nil, fmt.Errorf("获取记忆点失败: %w", err)
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

func (m *Memory) Close() error {
	return m.store.Close()
}
