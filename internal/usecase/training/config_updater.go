package training

import (
	"fmt"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
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
	return fmt.Errorf("config updater needs to be rewritten for new YAML config system")
}

func (c *ConfigUpdater) RollbackModel(backupPath string) error {
	backupData, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	if err := os.WriteFile(c.configPath, backupData, 0644); err != nil {
		return fmt.Errorf("failed to restore config: %w", err)
	}

	c.logger.Info(i18n.T("configupdater.config_rolled_back"), logging.String(i18n.T("configupdater.backup"), backupPath))
	return nil
}

func (c *ConfigUpdater) GetLatestBackupPath() (string, error) {
	configDir := filepath.Dir(c.configPath)
	configFile := filepath.Base(c.configPath)

	entries, err := os.ReadDir(configDir)
	if err != nil {
		return "", fmt.Errorf("failed to read dir: %w", err)
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
		return "", fmt.Errorf("no backup found")
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
		return nil, fmt.Errorf("failed to read dir: %w", err)
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
	return "", fmt.Errorf("config updater needs to be rewritten for new YAML config system")
}
