package training

import (
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Generator struct {
	outputDir string
	logger    logging.Logger
}

func NewGenerator(outputDir string, logger logging.Logger) (*Generator, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf(i18n.TWithData("generator.create_dir_failed", map[string]interface{}{"Error": err.Error()}))
	}

	return &Generator{
		outputDir: outputDir,
		logger:    logger,
	}, nil
}

func (g *Generator) GenerateJSONL(pairs []TrainingPair, outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf(i18n.TWithData("generator.create_dir_failed", map[string]interface{}{"Error": err.Error()}))
	}

	g.logger.Info(i18n.T("generator.generating_dataset"),
		logging.String("path", outputPath),
		logging.Int("pairs", len(pairs)))

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf(i18n.TWithData("generator.create_file_failed", map[string]interface{}{"Error": err.Error()}))
	}
	defer file.Close()

	encoder := json.NewEncoder(file)

	for _, pair := range pairs {
		trainData := map[string]string{
			"prompt":     fmt.Sprintf("用户: %s", pair.Prompt),
			"completion": fmt.Sprintf("助手: %s", pair.Completion),
		}

		if err := encoder.Encode(trainData); err != nil {
			return fmt.Errorf(i18n.TWithData("generator.encode_failed", map[string]interface{}{"Error": err.Error()}))
		}
	}

	g.logger.Info(i18n.T("generator.dataset_complete"), logging.Int(i18n.T("generator.count"), len(pairs)))
	return nil
}

func (g *Generator) GenerateModelfile(baseModel, outputPath string) error {
	modelfileContent := fmt.Sprintf(`FROM %s

PARAMETER temperature 0.7
PARAMETER top_p 0.9
PARAMETER num_ctx 4096

SYSTEM 你是一个智能助手，基于用户的对话数据进行了个性化调整，更懂用户的需求和习惯。
`, baseModel)

	if err := os.WriteFile(outputPath, []byte(modelfileContent), 0644); err != nil {
		return fmt.Errorf(i18n.TWithData("generator.write_modelfile_failed", map[string]interface{}{"Error": err.Error()}))
	}

	g.logger.Info(i18n.T("generator.modelfile_generated"), logging.String("path", outputPath))
	return nil
}

func (g *Generator) GenerateModelfileWithMessages(baseModel string, pairs []TrainingPair, outputPath string) error {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# 个性化模型 - 生成时间: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("# 训练数据: %d 条对话\n\n", len(pairs)))
	sb.WriteString(fmt.Sprintf("FROM %s\n\n", baseModel))

	sb.WriteString("PARAMETER temperature 0.7\n")
	sb.WriteString("PARAMETER top_p 0.9\n")
	sb.WriteString("PARAMETER num_ctx 4096\n")
	sb.WriteString("PARAMETER stop \"用户:\"\n")
	sb.WriteString("PARAMETER stop \"user:\"\n\n")

	sb.WriteString("SYSTEM \"\"\"\n")
	sb.WriteString("你是一个智能助手，基于用户的对话历史进行了个性化调整。\n")
	sb.WriteString("你更了解用户的说话习惯、兴趣偏好和常用表达方式。\n")
	sb.WriteString("请用自然、友好的语气回应用户。\n")
	sb.WriteString("\"\"\"\n\n")

	maxMessages := 50
	if len(pairs) > maxMessages {
		step := len(pairs) / maxMessages
		var selectedPairs []TrainingPair
		for i := 0; i < maxMessages; i++ {
			idx := i * step
			if idx < len(pairs) {
				selectedPairs = append(selectedPairs, pairs[idx])
			}
		}
		pairs = selectedPairs
	}

	sb.WriteString("# 对话示例\n")
	for _, pair := range pairs {
		userMsg := escapeModelfileString(pair.Prompt)
		assistantMsg := escapeModelfileString(pair.Completion)

		if len(userMsg) > 500 {
			userMsg = userMsg[:500] + "..."
		}
		if len(assistantMsg) > 800 {
			assistantMsg = assistantMsg[:800] + "..."
		}

		sb.WriteString(fmt.Sprintf("MESSAGE user %s\n", userMsg))
		sb.WriteString(fmt.Sprintf("MESSAGE assistant %s\n", assistantMsg))
	}

	if err := os.WriteFile(outputPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf(i18n.TWithData("generator.write_modelfile_failed", map[string]interface{}{"Error": err.Error()}))
	}

	g.logger.Info(i18n.T("generator.modelfile_with_messages"),
		logging.String("path", outputPath),
		logging.Int(i18n.T("generator.messages"), len(pairs)*2))

	return nil
}

func escapeModelfileString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	return s
}

func (g *Generator) GenerateTrainingReport(report TrainingReport, outputPath string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf(i18n.TWithData("generator.marshal_report_failed", map[string]interface{}{"Error": err.Error()}))
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf(i18n.TWithData("generator.create_dir_failed", map[string]interface{}{"Error": err.Error()}))
	}

	g.logger.Info(i18n.T("generator.saving_report"), logging.String("path", outputPath))

	return os.WriteFile(outputPath, data, 0644)
}

type TrainingReport struct {
	TrainingID      string    `json:"training_id"`
	StartTime       time.Time `json:"start_time"`
	EndTime         time.Time `json:"end_time"`
	BaseModel       string    `json:"base_model"`
	NewModel        string    `json:"new_model"`
	TotalPairs      int       `json:"total_pairs"`
	FilteredPairs   int       `json:"filtered_pairs"`
	TrainingPairs   int       `json:"training_pairs"`
	TrainingTime    string    `json:"training_time"`
	BaseScore       float64   `json:"base_score"`
	ValidationScore float64   `json:"validation_score"`
	Improved        bool      `json:"improved"`
	Status          string    `json:"status"`
	Error           string    `json:"error,omitempty"`
}
