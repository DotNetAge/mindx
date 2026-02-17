package training

import (
	"mindx/internal/utils"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"fmt"
	"sort"
	"strings"
)

type Filter struct {
	similarityThreshold float64
	logger              logging.Logger
}

func NewFilter(logger logging.Logger) *Filter {
	return &Filter{
		similarityThreshold: 0.9,
		logger:              logger,
	}
}

func (f *Filter) FilterCorpus(pairs []TrainingPair) ([]TrainingPair, error) {
	if len(pairs) == 0 {
		return nil, nil
	}

	cleaned := f.cleanLowValuePairs(pairs)
	f.logger.Debug(i18n.T("filter.clean_low_value"),
		logging.Int(i18n.T("filter.original"), len(pairs)),
		logging.Int(i18n.T("filter.after_clean"), len(cleaned)))

	deduped := f.deduplicateByContent(cleaned)
	f.logger.Debug(i18n.T("filter.content_dedup"),
		logging.Int(i18n.T("filter.after_clean"), len(cleaned)),
		logging.Int(i18n.T("filter.after_dedup"), len(deduped)))

	return deduped, nil
}

func (f *Filter) cleanLowValuePairs(pairs []TrainingPair) []TrainingPair {
	var result []TrainingPair

	lowValuePatterns := []string{
		"好的", "嗯", "哦", "啊", "行", "可以", "是的", "对的",
		"谢谢", "再见", "拜拜", "晚安", "早安",
		"哈哈", "嘻嘻", "呵呵",
		"???", "!!!", "...",
	}

	for _, pair := range pairs {
		prompt := strings.TrimSpace(pair.Prompt)
		completion := strings.TrimSpace(pair.Completion)

		if len(prompt) < 5 {
			continue
		}

		if len(completion) < 10 {
			continue
		}

		isLowValue := false
		for _, pattern := range lowValuePatterns {
			if prompt == pattern {
				isLowValue = true
				break
			}
		}
		if isLowValue {
			continue
		}

		if f.containsSensitiveInfo(prompt) || f.containsSensitiveInfo(completion) {
			continue
		}

		result = append(result, pair)
	}

	return result
}

func (f *Filter) containsSensitiveInfo(text string) bool {
	sensitivePatterns := []string{
		"密码", "password", "passwd",
		"银行卡", "信用卡", "卡号",
		"身份证", "身份证号",
		"api_key", "apikey", "secret",
	}

	lowerText := strings.ToLower(text)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerText, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

func (f *Filter) deduplicateByContent(pairs []TrainingPair) []TrainingPair {
	seen := make(map[string]int)
	var result []TrainingPair

	for _, pair := range pairs {
		key := fmt.Sprintf("%s|%s", pair.Prompt, pair.Completion)

		if idx, exists := seen[key]; exists {
			if len(pair.Topic) > 0 && len(result[idx].Topic) == 0 {
				result[idx].Topic = pair.Topic
			}
			continue
		}

		seen[key] = len(result)
		result = append(result, pair)
	}

	return result
}

func (f *Filter) DeduplicateByVector(pairs []TrainingPair, getEmbedding func(string) ([]float64, error)) ([]TrainingPair, error) {
	if len(pairs) == 0 {
		return nil, nil
	}

	if getEmbedding == nil {
		f.logger.Warn(i18n.T("filter.no_vector_func"))
		return pairs, nil
	}

	type pairWithVector struct {
		pair   TrainingPair
		vector []float64
	}

	var validPairs []pairWithVector
	var failedCount int

	for _, pair := range pairs {
		vec, err := getEmbedding(pair.Prompt)
		if err != nil {
			failedCount++
			continue
		}
		validPairs = append(validPairs, pairWithVector{
			pair:   pair,
			vector: vec,
		})
	}

	if failedCount > 0 {
		f.logger.Warn(i18n.T("filter.vectorize_partial_failed"),
			logging.Int(i18n.T("filter.failed_count"), failedCount),
			logging.Int(i18n.T("filter.success_count"), len(validPairs)))
	}

	if len(validPairs) == 0 {
		return pairs, nil
	}

	var result []TrainingPair
	var vectors [][]float64

	for _, pv := range validPairs {
		isDuplicate := false

		for i, existingVec := range vectors {
			similarity := utils.CalculateCosineSimilarity(pv.vector, existingVec)
			if similarity > f.similarityThreshold {
				if len(pv.pair.Completion) > len(result[i].Completion) {
					result[i] = pv.pair
					vectors[i] = pv.vector
				}
				isDuplicate = true
				break
			}
		}

		if !isDuplicate {
			result = append(result, pv.pair)
			vectors = append(vectors, pv.vector)
		}
	}

	f.logger.Debug(i18n.T("filter.vector_dedup_complete"),
		logging.Int(i18n.T("filter.original"), len(validPairs)),
		logging.Int(i18n.T("filter.after_dedup"), len(result)))

	return result, nil
}

func (f *Filter) SelectHighQualityPairs(pairs []TrainingPair, maxSize int) []TrainingPair {
	if len(pairs) <= maxSize {
		return pairs
	}

	type scoredPair struct {
		pair  TrainingPair
		score float64
	}
	var scored []scoredPair

	for _, pair := range pairs {
		score := f.calculateQualityScore(pair)
		scored = append(scored, scoredPair{pair: pair, score: score})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	var result []TrainingPair
	for i := 0; i < maxSize && i < len(scored); i++ {
		result = append(result, scored[i].pair)
	}

	return result
}

func (f *Filter) calculateQualityScore(pair TrainingPair) float64 {
	score := 0.0

	promptLen := len(pair.Prompt)
	completionLen := len(pair.Completion)

	if promptLen >= 20 && promptLen <= 500 {
		score += 20
	} else if promptLen >= 500 && promptLen <= 1000 {
		score += 15
	} else if promptLen > 1000 {
		score += 10
	}

	if completionLen >= 50 && completionLen <= 800 {
		score += 30
	} else if completionLen >= 800 && completionLen <= 1500 {
		score += 25
	} else if completionLen > 1500 {
		score += 15
	}

	uniqueChars := make(map[rune]bool)
	for _, r := range pair.Prompt {
		uniqueChars[r] = true
	}
	for _, r := range pair.Completion {
		uniqueChars[r] = true
	}
	totalLen := promptLen + completionLen
	if totalLen > 0 {
		complexity := float64(len(uniqueChars)) / float64(totalLen)
		score += complexity * 30
	}

	if pair.Topic != "" && len(pair.Topic) > 5 {
		score += 20
	}

	return score
}

func (f *Filter) GetStatistics(pairs []TrainingPair) map[string]interface{} {
	if len(pairs) == 0 {
		return map[string]interface{}{
			"total": 0,
		}
	}

	var totalPromptLen, totalCompletionLen int
	var maxPromptLen, maxCompletionLen int
	var minPromptLen, minCompletionLen int = 999999, 999999
	topics := make(map[string]int)

	for _, pair := range pairs {
		promptLen := len(pair.Prompt)
		completionLen := len(pair.Completion)

		totalPromptLen += promptLen
		totalCompletionLen += completionLen

		if promptLen > maxPromptLen {
			maxPromptLen = promptLen
		}
		if promptLen < minPromptLen {
			minPromptLen = promptLen
		}
		if completionLen > maxCompletionLen {
			maxCompletionLen = completionLen
		}
		if completionLen < minCompletionLen {
			minCompletionLen = completionLen
		}

		if pair.Topic != "" {
			topics[pair.Topic]++
		}
	}

	return map[string]interface{}{
		"total":               len(pairs),
		"avg_prompt_len":      totalPromptLen / len(pairs),
		"avg_completion_len":  totalCompletionLen / len(pairs),
		"max_prompt_len":      maxPromptLen,
		"min_prompt_len":      minPromptLen,
		"max_completion_len":  maxCompletionLen,
		"min_completion_len":  minCompletionLen,
		"unique_topics":       len(topics),
	}
}
