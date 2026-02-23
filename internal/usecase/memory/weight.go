package memory

import (
	"encoding/json"
	"mindx/internal/core"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"strings"
	"time"
)

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
