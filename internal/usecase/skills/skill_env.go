package skills

import (
	"fmt"
	"mindx/pkg/logging"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v2"
)

type SkillEnvConfig struct {
	Name string   `yaml:"name"`
	Envs []EnvVar `yaml:"envs,omitempty"`
}

type EnvVar struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type SkillsConfig struct {
	Skills []SkillEnvConfig `yaml:"skills"`
}

type EnvManager struct {
	configFile string
	logger     logging.Logger
	mu         sync.RWMutex
	envs       map[string]map[string]string
}

func NewEnvManager(workspaceDir string, logger logging.Logger) *EnvManager {
	configFile := filepath.Join(workspaceDir, "skills.yml")
	return &EnvManager{
		configFile: configFile,
		logger:     logger.Named("EnvManager"),
		envs:       make(map[string]map[string]string),
	}
}

func (e *EnvManager) LoadEnv() error {
	if _, err := os.Stat(e.configFile); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(e.configFile)
	if err != nil {
		return fmt.Errorf("failed to read skills config: %w", err)
	}

	var cfg SkillsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse skills config: %w", err)
	}

	for _, skillCfg := range cfg.Skills {
		skillEnvs := make(map[string]string)
		for _, env := range skillCfg.Envs {
			skillEnvs[env.Name] = env.Value
		}
		e.envs[skillCfg.Name] = skillEnvs
	}

	return nil
}

func (e *EnvManager) SaveEnv() error {
	dir := filepath.Dir(e.configFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	var cfg SkillsConfig
	for name, skillEnvs := range e.envs {
		skillCfg := SkillEnvConfig{Name: name}
		for envName, envValue := range skillEnvs {
			skillCfg.Envs = append(skillCfg.Envs, EnvVar{
				Name:  envName,
				Value: envValue,
			})
		}
		cfg.Skills = append(cfg.Skills, skillCfg)
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("failed to serialize skills config: %w", err)
	}

	if err := os.WriteFile(e.configFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write skills config: %w", err)
	}

	return nil
}

func (e *EnvManager) PrepareExecutionEnv(skillName string, sensitiveKeys []string) (map[string]string, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	env := make(map[string]string)
	for _, envStr := range os.Environ() {
		parts := strings.SplitN(envStr, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}

	if skillEnv, exists := e.envs[skillName]; exists {
		for key, value := range skillEnv {
			envKey := fmt.Sprintf("SKILL_%s_%s", strings.ToUpper(skillName), strings.ToUpper(key))
			env[envKey] = value
		}
	}

	return env, nil
}

func (e *EnvManager) SetSkillEnv(skillName string, vars map[string]string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.envs[skillName] == nil {
		e.envs[skillName] = make(map[string]string)
	}

	for key, value := range vars {
		e.envs[skillName][key] = value
	}

	return e.SaveEnv()
}

func (e *EnvManager) GetSkillEnv(skillName string) map[string]string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if env, exists := e.envs[skillName]; exists {
		result := make(map[string]string)
		for k, v := range env {
			result[k] = v
		}
		return result
	}

	return make(map[string]string)
}
