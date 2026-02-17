package training

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mindx/internal/usecase/embedding"
	"mindx/internal/utils"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"net/http"
	"sort"
	"strings"
	"time"
)

type Validator struct {
	ollamaURL    string
	embeddingSvc *embedding.EmbeddingService
	minAccuracy  float64
	logger       logging.Logger
	client       *http.Client
}

func NewValidator(ollamaURL string, embeddingSvc *embedding.EmbeddingService, logger logging.Logger) *Validator {
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}
	ollamaURL = strings.TrimSuffix(ollamaURL, "/v1")
	ollamaURL = strings.TrimSuffix(ollamaURL, "/")

	return &Validator{
		ollamaURL:    ollamaURL,
		embeddingSvc: embeddingSvc,
		minAccuracy:  0.7,
		logger:       logger,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (v *Validator) ValidateModel(modelName string, testCases []TestCase) (float64, error) {
	if len(testCases) == 0 {
		return 0, fmt.Errorf(i18n.T("validator.no_testcases"))
	}

	var correctCount int
	var totalScore float64

	for i, tc := range testCases {
		response, err := v.generateResponse(modelName, tc.Prompt)
		if err != nil {
			v.logger.Warn(i18n.T("validator.validation_failed"),
				logging.String(i18n.T("validator.prompt"), tc.Prompt[:min(50, len(tc.Prompt))]),
				logging.Err(err))
			continue
		}

		similarity := v.calculateSimilarity(response, tc.ExpectedAnswer)
		totalScore += similarity

		if similarity >= v.minAccuracy {
			correctCount++
		}

		v.logger.Debug(i18n.T("validator.testcase"),
			logging.Int(i18n.T("validator.index"), i),
			logging.String(i18n.T("validator.prompt"), tc.Prompt[:min(30, len(tc.Prompt))]),
			logging.Float64(i18n.T("validator.similarity"), similarity))

		time.Sleep(100 * time.Millisecond)
	}

	accuracy := float64(correctCount) / float64(len(testCases))
	avgScore := totalScore / float64(len(testCases))

	v.logger.Info(i18n.T("validator.validation_result"),
		logging.String(i18n.T("validator.model"), modelName),
		logging.String(i18n.T("validator.correct"), fmt.Sprintf("%d/%d", correctCount, len(testCases))),
		logging.Float64(i18n.T("validator.accuracy"), accuracy),
		logging.Float64(i18n.T("validator.avg_similarity"), avgScore))

	return avgScore, nil
}

func (v *Validator) generateResponse(modelName, prompt string) (string, error) {
	reqBody := map[string]any{
		"model":  modelName,
		"stream": false,
		"options": map[string]any{
			"temperature": 0.3,
		},
		"messages": []map[string]string{
			{"role": "system", "content": "你是一个智能助手，请简明扼要地回答用户的问题。"},
			{"role": "user", "content": prompt},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf(i18n.TWithData("validator.serialize_failed", map[string]interface{}{"Error": err.Error()}))
	}

	resp, err := v.client.Post(v.ollamaURL+"/api/chat", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf(i18n.TWithData("validator.api_request_failed", map[string]interface{}{"Error": err.Error()}))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf(i18n.TWithData("validator.api_error", map[string]interface{}{"Code": resp.StatusCode}))
	}

	var result struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf(i18n.TWithData("validator.parse_response_failed", map[string]interface{}{"Error": err.Error()}))
	}

	if result.Message.Content == "" {
		return "", fmt.Errorf(i18n.T("validator.empty_response"))
	}

	return strings.TrimSpace(result.Message.Content), nil
}

func (v *Validator) calculateSimilarity(text1, text2 string) float64 {
	if v.embeddingSvc == nil {
		return v.calculateStringOverlap(text1, text2)
	}

	vec1, err := v.embeddingSvc.GenerateEmbedding(text1)
	if err != nil {
		v.logger.Warn(i18n.T("validator.vector_failed"), logging.Err(err))
		return v.calculateStringOverlap(text1, text2)
	}

	vec2, err := v.embeddingSvc.GenerateEmbedding(text2)
	if err != nil {
		v.logger.Warn(i18n.T("validator.vector_failed"), logging.Err(err))
		return v.calculateStringOverlap(text1, text2)
	}

	return utils.CalculateCosineSimilarity(vec1, vec2)
}

func (v *Validator) calculateStringOverlap(text1, text2 string) float64 {
	if len(text1) == 0 || len(text2) == 0 {
		return 0
	}

	chars1 := make(map[rune]bool)
	for _, r := range text1 {
		chars1[r] = true
	}

	overlap := 0
	for _, r := range text2 {
		if chars1[r] {
			overlap++
		}
	}

	return float64(overlap) / float64(min(len(text1), len(text2)))
}

func (v *Validator) ExtractTestCases(pairs []TrainingPair, count int) []TestCase {
	if len(pairs) == 0 {
		return nil
	}

	if len(pairs) <= count {
		count = len(pairs)
	}

	type scoredPair struct {
		pair  TrainingPair
		score float64
	}
	var scored []scoredPair

	for _, pair := range pairs {
		score := float64(len(pair.Prompt)) + float64(len(pair.Completion))
		scored = append(scored, scoredPair{pair: pair, score: score})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	var testCases []TestCase
	step := len(scored) / count
	if step < 1 {
		step = 1
	}

	for i := 0; i < count && i*step < len(scored); i++ {
		pair := scored[i*step].pair
		testCases = append(testCases, TestCase{
			Prompt:         pair.Prompt,
			ExpectedAnswer: pair.Completion,
		})
	}

	return testCases
}

func (v *Validator) QuickValidate(modelName string, prompts []string) (map[string]string, error) {
	results := make(map[string]string)

	for _, prompt := range prompts {
		response, err := v.generateResponse(modelName, prompt)
		if err != nil {
			results[prompt] = fmt.Sprintf("%s: %v", i18n.T("validator.error_prefix"), err)
			continue
		}
		results[prompt] = response
	}

	return results, nil
}

type TestCase struct {
	Prompt         string `json:"prompt"`
	ExpectedAnswer string `json:"expected_answer"`
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
