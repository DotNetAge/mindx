package memory

import (
	"math"
	"mindx/internal/core"
	"mindx/internal/entity"
	apperrors "mindx/internal/errors"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"sort"
	"strings"
	"time"

	"github.com/muesli/clusters"
)

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
		m.clusterAndStore(memoryPoints)
	} else if len(memoryPoints) == 1 {
		if err := m.Record(memoryPoints[0]); err != nil {
			m.logger.Error(i18n.T("memory.store_failed"), logging.Err(err))
			return apperrors.Wrap(err, apperrors.ErrTypeMemory, "存储记忆点失败")
		}
		m.logger.Info(i18n.T("memory.single_mem_store_complete"))
	}

	return nil
}

func (m *Memory) clusterAndStore(memoryPoints []core.MemoryPoint) {
	var points []clusters.Observation
	pointIndices := make(map[int]int)

	for i, mem := range memoryPoints {
		if len(mem.Vector) > 0 {
			points = append(points, clusters.Coordinates(mem.Vector))
			pointIndices[len(points)-1] = i
		}
	}

	if len(points) < 2 {
		for _, point := range memoryPoints {
			if err := m.Record(point); err != nil {
				m.logger.Error(i18n.T("memory.store_failed"), logging.Err(err))
				continue
			}
		}
		m.logger.Info(i18n.T("memory.no_valid_vector"))
		return
	}

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
		return
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
		return core.MemoryPoint{}, apperrors.New(apperrors.ErrTypeMemory, "no memory points provided")
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

	n := float64(len(points))
	combinedPoint := core.MemoryPoint{
		Keywords:       keywords,
		Content:        combinedContent,
		Summary:        combinedSummary,
		Vector:         combinedVector,
		ClusterID:      clusterID,
		TimeWeight:     totalTimeWeight / n,
		RepeatWeight:   totalRepeatWeight / n,
		EmphasisWeight: totalEmphasisWeight / n,
		TotalWeight:    totalTotalWeight / n,
		CreatedAt:      startTime,
		UpdatedAt:      endTime,
	}

	return combinedPoint, nil
}

