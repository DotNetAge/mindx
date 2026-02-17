package training

import (
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type TrainingMode string

const (
	ModeMessage TrainingMode = "message"
	ModeLora    TrainingMode = "lora"
)

type Trainer struct {
	collector     *Collector
	filter        *Filter
	generator     *Generator
	validator     *Validator
	configUpdater *ConfigUpdater
	baseModel     string
	dataDir       string
	ollamaURL     string
	minCorpus     int
	mode          TrainingMode
	trainingDir   string
	logger        logging.Logger
}

func NewTrainer(
	collector *Collector,
	filter *Filter,
	generator *Generator,
	validator *Validator,
	configUpdater *ConfigUpdater,
	baseModel string,
	dataDir string,
	ollamaURL string,
	minCorpus int,
	mode TrainingMode,
	trainingDir string,
	logger logging.Logger,
) *Trainer {
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}
	if mode == "" {
		mode = ModeMessage
	}
	if trainingDir == "" {
		trainingDir = "training"
	}
	return &Trainer{
		collector:     collector,
		filter:        filter,
		generator:     generator,
		validator:     validator,
		configUpdater: configUpdater,
		baseModel:     baseModel,
		dataDir:       dataDir,
		ollamaURL:     ollamaURL,
		minCorpus:     minCorpus,
		mode:          mode,
		trainingDir:   trainingDir,
		logger:        logger,
	}
}

func (t *Trainer) checkOllamaHealth() error {
	resp, err := http.Get(t.ollamaURL + "/api/tags")
	if err != nil {
		return fmt.Errorf(i18n.TWithData("training.ollama.unavailable", map[string]interface{}{"Error": err.Error()}))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(i18n.TWithData("training.ollama.status_error", map[string]interface{}{"Code": resp.StatusCode}))
	}

	return nil
}

func (t *Trainer) RunTraining() (*TrainingReport, error) {
	startTime := time.Now()
	trainingID := fmt.Sprintf("train_%d", startTime.Unix())

	t.logger.Info(i18n.T("training.start"), logging.String("training_id", trainingID))
	t.logger.Info(i18n.T("training.base_model"), logging.String("model", t.baseModel))

	if err := t.checkOllamaHealth(); err != nil {
		return nil, fmt.Errorf(i18n.TWithData("training.ollama.health_check_failed", map[string]interface{}{"Error": err.Error()}))
	}
	t.logger.Info(i18n.T("training.ollama_healthy"))

	t.logger.Info(i18n.T("training.step1_collect"))
	memoryPoints, err := t.collector.CollectMemoryPoints()
	if err != nil {
		return nil, fmt.Errorf(i18n.TWithData("training.collect_failed", map[string]interface{}{"Error": err.Error()}))
	}
	t.logger.Info(i18n.T("training.collected_points"), logging.Int("count", len(memoryPoints)))

	pairs := t.collector.ExtractTrainingPairs(memoryPoints)
	t.logger.Info(i18n.T("training.extracted_pairs"), logging.Int("count", len(pairs)))

	if len(pairs) < t.minCorpus {
		t.logger.Warn(i18n.T("training.corpus_too_small"),
			logging.Int("pairs", len(pairs)),
			logging.Int("min_required", t.minCorpus))
		return &TrainingReport{
			TrainingID: trainingID,
			StartTime:  startTime,
			EndTime:    time.Now(),
			BaseModel:  t.baseModel,
			Status:     "skipped",
			Error:      fmt.Sprintf("Insufficient corpus: %d < %d", len(pairs), t.minCorpus),
		}, nil
	}

	t.logger.Info(i18n.T("training.step2_filter"))
	filteredPairs, err := t.filter.FilterCorpus(pairs)
	if err != nil {
		return nil, fmt.Errorf(i18n.TWithData("training.filter_failed", map[string]interface{}{"Error": err.Error()}))
	}
	t.logger.Info(i18n.T("training.filtered_pairs"), logging.Int("count", len(filteredPairs)))

	if len(filteredPairs) < t.minCorpus/2 {
		t.logger.Warn(i18n.T("training.filtered_too_few"))
		return &TrainingReport{
			TrainingID:    trainingID,
			StartTime:     startTime,
			EndTime:       time.Now(),
			BaseModel:     t.baseModel,
			TotalPairs:    len(pairs),
			FilteredPairs: len(filteredPairs),
			Status:        "skipped",
			Error:         "Insufficient filtered corpus",
		}, nil
	}

	t.logger.Info(i18n.T("training.step3_generate"))
	timestamp := time.Now().Format("20060102_150405")
	trainDataPath := filepath.Join(t.dataDir, "training", fmt.Sprintf("train_data_%s.jsonl", timestamp))
	if err := t.generator.GenerateJSONL(filteredPairs, trainDataPath); err != nil {
		return nil, fmt.Errorf(i18n.TWithData("training.generate_failed", map[string]interface{}{"Error": err.Error()}))
	}
	t.logger.Info(i18n.T("training.generated_dataset"), logging.String("path", trainDataPath))

	t.logger.Info(i18n.T("training.step4_testcases"))
	testCases := t.validator.ExtractTestCases(filteredPairs, 20)
	t.logger.Info(i18n.T("training.generated_testcases"), logging.Int("count", len(testCases)))

	t.logger.Info(i18n.T("training.step5_validate_base"))
	baseScore, err := t.validator.ValidateModel(t.baseModel, testCases)
	if err != nil {
		t.logger.Warn(i18n.T("training.base_validation_failed"), logging.Err(err))
		baseScore = 0
	} else {
		t.logger.Info(i18n.T("training.base_score"), logging.Float64("score", baseScore))
	}

	t.logger.Info(i18n.T("training.step6_create_model"))
	newModelName := fmt.Sprintf("%s-personal-%s", t.baseModel, timestamp[:8])

	var createErr error
	switch t.mode {
	case ModeLora:
		createErr = t.runLoRAFinetune(trainDataPath, newModelName)
	default:
		createErr = t.createMessageModel(filteredPairs, newModelName, timestamp)
	}

	if createErr != nil {
		return nil, createErr
	}
	t.logger.Info(i18n.T("training.created_new_model"), logging.String("model_name", newModelName), logging.String("mode", string(t.mode)))

	t.logger.Info(i18n.T("training.step7_validate_new"))
	newScore, err := t.validator.ValidateModel(newModelName, testCases)
	if err != nil {
		t.logger.Warn(i18n.T("training.new_validation_failed"), logging.Err(err))
		newScore = 0
	} else {
		t.logger.Info(i18n.T("training.new_score"), logging.Float64("score", newScore))
	}

	improved := newScore > baseScore
	status := "success"
	if !improved {
		status = "completed_no_improvement"
		t.logger.Warn(i18n.T("training.no_improvement"),
			logging.Float64("base_score", baseScore),
			logging.Float64("new_score", newScore))
	}

	t.logger.Info(i18n.T("training.step8_update_config"))
	if improved {
		if err := t.configUpdater.UpdateLeftBrainModel(newModelName); err != nil {
			t.logger.Warn(i18n.T("training.config_update_failed"), logging.Err(err))
			status = "success_config_failed"
		} else {
			t.logger.Info(i18n.T("training.config_updated"), logging.String("new_model", newModelName))
		}
	}

	if err := t.collector.UpdateLastTrainTime(time.Now()); err != nil {
		t.logger.Warn(i18n.T("training.time_update_failed"), logging.Err(err))
	}

	report := TrainingReport{
		TrainingID:      trainingID,
		StartTime:       startTime,
		EndTime:         time.Now(),
		BaseModel:       t.baseModel,
		NewModel:        newModelName,
		TotalPairs:      len(pairs),
		FilteredPairs:   len(filteredPairs),
		TrainingPairs:   len(filteredPairs),
		TrainingTime:    time.Since(startTime).String(),
		BaseScore:       baseScore,
		ValidationScore: newScore,
		Improved:        improved,
		Status:          status,
	}

	reportPath := filepath.Join(t.dataDir, "training", fmt.Sprintf("report_%s.json", timestamp))
	if err := t.generator.GenerateTrainingReport(report, reportPath); err != nil {
		t.logger.Warn(i18n.T("training.report_failed"), logging.Err(err))
	} else {
		t.logger.Info(i18n.T("training.report_saved"), logging.String("path", reportPath))
	}

	t.logger.Info(i18n.T("training.completed"),
		logging.String("status", status),
		logging.String("duration", report.TrainingTime),
		logging.Bool("improved", improved))

	return &report, nil
}

func (t *Trainer) createOllamaModel(modelName, modelfilePath string) error {
	_, err := exec.LookPath("ollama")
	if err != nil {
		return t.createOllamaModelViaAPI(modelName, modelfilePath)
	}

	cmd := exec.Command("ollama", "create", modelName, "-f", modelfilePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(i18n.TWithData("training.ollama_create_failed", map[string]interface{}{"Error": err.Error()}))
	}

	t.logger.Debug("ollama create output", logging.String("output", string(output)))
	return nil
}

func (t *Trainer) createOllamaModelViaAPI(modelName, modelfilePath string) error {
	modelfileContent, err := os.ReadFile(modelfilePath)
	if err != nil {
		return fmt.Errorf(i18n.TWithData("training.read_modelfile_failed", map[string]interface{}{"Error": err.Error()}))
	}

	reqBody := map[string]string{
		"name":      modelName,
		"modelfile": string(modelfileContent),
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf(i18n.TWithData("training.serialize_failed", map[string]interface{}{"Error": err.Error()}))
	}

	resp, err := http.Post(t.ollamaURL+"/api/create", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf(i18n.TWithData("training.api_request_failed", map[string]interface{}{"Error": err.Error()}))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(i18n.TWithData("training.api_error", map[string]interface{}{"Code": resp.StatusCode}))
	}

	return nil
}

func (t *Trainer) createMessageModel(pairs []TrainingPair, modelName, timestamp string) error {
	modelfilePath := filepath.Join(t.dataDir, "training", fmt.Sprintf("Modelfile_%s", timestamp))
	if err := t.generator.GenerateModelfileWithMessages(t.baseModel, pairs, modelfilePath); err != nil {
		return fmt.Errorf(i18n.TWithData("training.modelfile_failed", map[string]interface{}{"Error": err.Error()}))
	}
	t.logger.Info(i18n.T("training.modelfile_generated"), logging.String("path", modelfilePath))

	if err := t.createOllamaModel(modelName, modelfilePath); err != nil {
		return fmt.Errorf(i18n.TWithData("training.create_model_failed", map[string]interface{}{"Error": err.Error()}))
	}
	return nil
}

func (t *Trainer) runLoRAFinetune(dataPath, modelName string) error {
	venvPython := filepath.Join(t.trainingDir, ".venv", "bin", "python")
	if _, err := os.Stat(venvPython); os.IsNotExist(err) {
		return fmt.Errorf(i18n.TWithData("training.venv_not_found", map[string]interface{}{"Dir": t.trainingDir}))
	}

	finetuneScript := filepath.Join(t.trainingDir, "finetune.py")
	if _, err := os.Stat(finetuneScript); os.IsNotExist(err) {
		return fmt.Errorf(i18n.TWithData("training.script_not_found", map[string]interface{}{"Path": finetuneScript}))
	}

	outputDir := filepath.Join(t.trainingDir, "output", modelName)

	t.logger.Info(i18n.T("training.lora_start"), logging.String("data", dataPath))

	cmd := exec.Command(venvPython, finetuneScript,
		"--data", dataPath,
		"--output", outputDir,
		"--model", t.baseModel,
		"--epochs", "3",
		"--batch-size", "1",
		"--learning-rate", "2e-4",
		"--max-length", "512",
		"--lora-r", "8",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf(i18n.TWithData("training.lora_failed", map[string]interface{}{"Error": err.Error()}))
	}

	t.logger.Info(i18n.T("training.lora_completed"))

	mergedDir := filepath.Join(outputDir, "merged")
	ggufPath := filepath.Join(outputDir, fmt.Sprintf("%s.gguf", modelName))

	exportScript := filepath.Join(t.trainingDir, "export_ollama.sh")
	if _, err := os.Stat(exportScript); err == nil {
		cmd := exec.Command(exportScript, mergedDir, modelName)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			t.logger.Warn(i18n.T("training.export_gguf_failed"), logging.Err(err))
		}
	}

	modelfilePath := filepath.Join(outputDir, "Modelfile")
	modelfileContent := fmt.Sprintf(`FROM %s

PARAMETER temperature 0.7
PARAMETER top_p 0.9
PARAMETER num_ctx 4096

SYSTEM """You are a personalized intelligent assistant."""
`, ggufPath)

	if err := os.WriteFile(modelfilePath, []byte(modelfileContent), 0644); err != nil {
		return fmt.Errorf(i18n.TWithData("training.write_modelfile_failed", map[string]interface{}{"Error": err.Error()}))
	}

	if err := t.createOllamaModel(modelName, modelfilePath); err != nil {
		return fmt.Errorf(i18n.TWithData("training.create_model_failed", map[string]interface{}{"Error": err.Error()}))
	}

	return nil
}

func (t *Trainer) ListModels() ([]string, error) {
	resp, err := http.Get(t.ollamaURL + "/api/tags")
	if err != nil {
		return nil, fmt.Errorf(i18n.TWithData("training.get_models_failed", map[string]interface{}{"Error": err.Error()}))
	}
	defer resp.Body.Close()

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf(i18n.TWithData("training.parse_response_failed", map[string]interface{}{"Error": err.Error()}))
	}

	names := make([]string, 0, len(result.Models))
	for _, m := range result.Models {
		names = append(names, m.Name)
	}

	return names, nil
}

func (t *Trainer) DeleteModel(modelName string) error {
	if strings.Contains(modelName, t.baseModel) && !strings.Contains(modelName, "personal") {
		return fmt.Errorf(i18n.TWithData("training.cannot_delete_base", map[string]interface{}{"Model": modelName}))
	}

	reqBody := map[string]string{"name": modelName}
	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("DELETE", t.ollamaURL+"/api/delete", bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf(i18n.TWithData("training.delete_model_failed", map[string]interface{}{"Error": err.Error()}))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf(i18n.TWithData("training.delete_model_failed", map[string]interface{}{"Error": fmt.Sprintf("status %d: %s", resp.StatusCode, string(body))}))
	}

	t.logger.Info(i18n.T("training.model_deleted"), logging.String("model", modelName))
	return nil
}
