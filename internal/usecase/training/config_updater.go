package training

import (
	cfg "mindx/internal/config"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type ConfigUpdater struct {
	configPath string
	logger     logging.Logger
}

func NewConfigUpdater(configPath string, logger logging.Logger) *ConfigUpdater {
	if configPath == "" {
		configPath = "config/models.json"
	}
	return &ConfigUpdater{
		configPath: configPath,
		logger:     logger,
	}
}

func (c *ConfigUpdater) UpdateLeftBrainModel(newModelName string) error {
	data, err := os.ReadFile(c.configPath)
	if err != nil {
		return fmt.Errorf(i18n.TWithData("configupdater.read_failed", map[string]interface{}{"Error": err.Error()}))
	}

	var config cfg.ModelsConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf(i18n.TWithData("configupdater.parse_failed", map[string]interface{}{"Error": err.Error()}))
	}

	backupPath := c.configPath + ".backup." + time.Now().Format("20060102_150405")
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf(i18n.TWithData("configupdater.backup_failed", map[string]interface{}{"Error": err.Error()}))
	}
	c.logger.Info(i18n.T("configupdater.config_backed_up"), logging.String("path", backupPath))

	if config.BrainModels != nil {
		config.BrainModels["leftbrain"] = newModelName
	} else {
		config.BrainModels = map[string]string{
			"leftbrain": newModelName,
		}
	}

	modelExists := false
	for i, model := range config.Models {
		if model.Name == newModelName {
			config.Models[i].Description = fmt.Sprintf("个性化微调版 - %s", time.Now().Format("2006-01-02"))
			modelExists = true
			break
		}
	}

	if !modelExists {
		config.Models = append(config.Models, cfg.ModelConfig{
			Name:        newModelName,
			Domain:      "subconscious",
			APIKey:      "ollama",
			BaseURL:     "http://localhost:11434",
			Temperature: 0.7,
			MaxTokens:   800,
			Description: fmt.Sprintf("个性化微调版 - %s", time.Now().Format("2006-01-02")),
		})
	}

	newData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf(i18n.TWithData("configupdater.marshal_failed", map[string]interface{}{"Error": err.Error()}))
	}

	if err := os.WriteFile(c.configPath, newData, 0644); err != nil {
		return fmt.Errorf(i18n.TWithData("configupdater.write_failed", map[string]interface{}{"Error": err.Error()}))
	}

	c.logger.Info(i18n.T("configupdater.config_updated"),
		logging.String(i18n.T("configupdater.new_model"), newModelName),
		logging.String(i18n.T("configupdater.backup"), backupPath))

	return nil
}

func (c *ConfigUpdater) RollbackModel(backupPath string) error {
	backupData, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf(i18n.TWithData("configupdater.read_failed", map[string]interface{}{"Error": err.Error()}))
	}

	if err := os.WriteFile(c.configPath, backupData, 0644); err != nil {
		return fmt.Errorf(i18n.TWithData("configupdater.restore_failed", map[string]interface{}{"Error": err.Error()}))
	}

	c.logger.Info(i18n.T("configupdater.config_rolled_back"), logging.String(i18n.T("configupdater.backup"), backupPath))
	return nil
}

func (c *ConfigUpdater) GetLatestBackupPath() (string, error) {
	configDir := filepath.Dir(c.configPath)
	configFile := filepath.Base(c.configPath)

	entries, err := os.ReadDir(configDir)
	if err != nil {
		return "", fmt.Errorf(i18n.TWithData("configupdater.read_dir_failed", map[string]interface{}{"Error": err.Error()}))
	}

	backupPattern := regexp.MustCompile(`^` + regexp.QuoteMeta(configFile) + `\.backup\.(\d{8}_\d{6})$`)

	type backupInfo struct {
		path      string
		timestamp time.Time
	}
	var backups []backupInfo

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		matches := backupPattern.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}

		timestamp, err := time.Parse("20060102_150405", matches[1])
		if err != nil {
			continue
		}

		backups = append(backups, backupInfo{
			path:      filepath.Join(configDir, entry.Name()),
			timestamp: timestamp,
		})
	}

	if len(backups) == 0 {
		return "", fmt.Errorf(i18n.T("configupdater.no_backup"))
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].timestamp.After(backups[j].timestamp)
	})

	return backups[0].path, nil
}

func (c *ConfigUpdater) ListBackups() ([]string, error) {
	configDir := filepath.Dir(c.configPath)
	configFile := filepath.Base(c.configPath)

	entries, err := os.ReadDir(configDir)
	if err != nil {
		return nil, fmt.Errorf(i18n.TWithData("configupdater.read_dir_failed", map[string]interface{}{"Error": err.Error()}))
	}

	var backups []string
	prefix := configFile + ".backup."

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if strings.HasPrefix(entry.Name(), prefix) {
			backups = append(backups, filepath.Join(configDir, entry.Name()))
		}
	}

	sort.Sort(sort.Reverse(sort.StringSlice(backups)))
	return backups, nil
}

func (c *ConfigUpdater) GetCurrentLeftBrainModel() (string, error) {
	data, err := os.ReadFile(c.configPath)
	if err != nil {
		return "", fmt.Errorf(i18n.TWithData("configupdater.read_failed", map[string]interface{}{"Error": err.Error()}))
	}

	var config cfg.ModelsConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf(i18n.TWithData("configupdater.parse_failed", map[string]interface{}{"Error": err.Error()}))
	}

	if config.BrainModels == nil {
		return "", fmt.Errorf(i18n.T("configupdater.no_brain_models"))
	}

	model, ok := config.BrainModels["leftbrain"]
	if !ok {
		return "", fmt.Errorf(i18n.T("configupdater.leftbrain_not_configured"))
	}

	return model, nil
}
