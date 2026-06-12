package core

import (
	"fmt"
	"os"
	"path/filepath"

	goharnessconfig "github.com/DotNetAge/goharness/config"
	"gopkg.in/yaml.v3"
)

type ProvidersConfig struct {
	Providers []goharnessconfig.ProviderConfig `yaml:"providers"`
}

func LoadProvidersFile(path string) ([]*goharnessconfig.ProviderConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read provider file: %w", err)
	}

	var cfg ProvidersConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse provider file: %w", err)
	}

	result := make([]*goharnessconfig.ProviderConfig, 0, len(cfg.Providers))
	for i := range cfg.Providers {
		if cfg.Providers[i].Name == "" {
			return nil, fmt.Errorf("provider config missing name")
		}
		result = append(result, &cfg.Providers[i])
	}
	return result, nil
}

func SaveProvidersFile(path string, providers []*goharnessconfig.ProviderConfig) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create provider directory: %w", err)
	}

	wrapper := ProvidersConfig{
		Providers: make([]goharnessconfig.ProviderConfig, 0, len(providers)),
	}
	for _, p := range providers {
		if p != nil {
			wrapper.Providers = append(wrapper.Providers, *p)
		}
	}

	data, err := yaml.Marshal(wrapper)
	if err != nil {
		return fmt.Errorf("failed to marshal providers: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}
