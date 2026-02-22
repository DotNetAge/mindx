package config

import (
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/viper"

	apperrors "mindx/internal/errors"
)

func InitVippers() (srvCfg *GlobalConfig, channelsCfg *ChannelsConfig, capabilitiesCfg *CapabilityConfig, modelsCfg *ModelsConfig, err error) {
	srvCfg, err = LoadServerConfig()
	if err != nil {
		return nil, nil, nil, nil, apperrors.Wrap(err, apperrors.ErrTypeConfig, "加载server配置失败")
	}

	channelsCfg, err = LoadChannelsConfig()
	if err != nil {
		return nil, nil, nil, nil, apperrors.Wrap(err, apperrors.ErrTypeConfig, "加载channels配置失败")
	}

	capabilitiesCfg, err = LoadCapabilitiesConfig()
	if err != nil {
		return nil, nil, nil, nil, apperrors.Wrap(err, apperrors.ErrTypeConfig, "加载capabilities配置失败")
	}

	modelsCfg, err = LoadModelsConfig()
	if err != nil {
		return nil, nil, nil, nil, apperrors.Wrap(err, apperrors.ErrTypeConfig, "加载models配置失败")
	}

	SetModelsManager(NewModelsManager(modelsCfg, srvCfg))

	return srvCfg, channelsCfg, capabilitiesCfg, modelsCfg, nil
}

func LoadServerConfig() (*GlobalConfig, error) {
	workspaceConfigPath, err := GetWorkspaceConfigPath()
	if err != nil {
		return nil, err
	}

	configFile := filepath.Join(workspaceConfigPath, "server.yml")
	if _, err := os.Stat(configFile); err == nil {
		v := viper.New()
		v.SetConfigFile(configFile)
		if err := v.ReadInConfig(); err != nil {
			return nil, err
		}

		sub := v.Sub("server")
		if sub == nil {
			return nil, os.ErrNotExist
		}

		cfg := &GlobalConfig{}
		if err := sub.Unmarshal(cfg); err != nil {
			return nil, err
		}

		return cfg, nil
	}

	installConfigPath, err := GetInstallConfigPath()
	if err != nil {
		return nil, err
	}

	templateFile := filepath.Join(installConfigPath, "server.yaml.template")
	if _, err := os.Stat(templateFile); err != nil {
		return nil, err
	}

	destFile := filepath.Join(workspaceConfigPath, "server.yml")
	if err := copyFile(templateFile, destFile); err != nil {
		return nil, err
	}

	return LoadServerConfig()
}

func LoadChannelsConfig() (*ChannelsConfig, error) {
	workspaceConfigPath, err := GetWorkspaceConfigPath()
	if err != nil {
		return nil, err
	}

	configFile := filepath.Join(workspaceConfigPath, "channels.yml")
	if _, err := os.Stat(configFile); err == nil {
		v := viper.New()
		v.SetConfigFile(configFile)
		if err := v.ReadInConfig(); err != nil {
			return nil, err
		}

		cfg := &ChannelsConfig{}
		if err := v.Unmarshal(cfg); err != nil {
			return nil, err
		}

		return cfg, nil
	}

	installConfigPath, err := GetInstallConfigPath()
	if err != nil {
		return nil, err
	}

	templateFile := filepath.Join(installConfigPath, "channels.json.template")
	if _, err := os.Stat(templateFile); err != nil {
		return nil, err
	}

	destFile := filepath.Join(workspaceConfigPath, "channels.yml")
	if err := copyFile(templateFile, destFile); err != nil {
		return nil, err
	}

	return LoadChannelsConfig()
}

func LoadCapabilitiesConfig() (*CapabilityConfig, error) {
	workspaceConfigPath, err := GetWorkspaceConfigPath()
	if err != nil {
		return nil, err
	}

	configFile := filepath.Join(workspaceConfigPath, "capabilities.yml")
	if _, err := os.Stat(configFile); err == nil {
		v := viper.New()
		v.SetConfigFile(configFile)
		if err := v.ReadInConfig(); err != nil {
			return nil, err
		}

		cfg := &CapabilityConfig{}
		if err := v.Unmarshal(cfg); err != nil {
			return nil, err
		}

		return cfg, nil
	}

	installConfigPath, err := GetInstallConfigPath()
	if err != nil {
		return nil, err
	}

	templateFile := filepath.Join(installConfigPath, "capabilities.json.template")
	if _, err := os.Stat(templateFile); err != nil {
		return nil, err
	}

	destFile := filepath.Join(workspaceConfigPath, "capabilities.yml")
	if err := copyFile(templateFile, destFile); err != nil {
		return nil, err
	}

	return LoadCapabilitiesConfig()
}

func LoadModelsConfig() (*ModelsConfig, error) {
	workspaceConfigPath, err := GetWorkspaceConfigPath()
	if err != nil {
		return nil, err
	}

	configFile := filepath.Join(workspaceConfigPath, "models.yml")
	if _, err := os.Stat(configFile); err == nil {
		v := viper.New()
		v.SetConfigFile(configFile)
		if err := v.ReadInConfig(); err != nil {
			return nil, err
		}

		cfg := &ModelsConfig{}
		if err := v.Unmarshal(cfg); err != nil {
			return nil, err
		}

		return cfg, nil
	}

	installConfigPath, err := GetInstallConfigPath()
	if err != nil {
		return nil, err
	}

	templateFile := filepath.Join(installConfigPath, "models.json.template")
	if _, err := os.Stat(templateFile); err == nil {
		destFile := filepath.Join(workspaceConfigPath, "models.yml")
		if err := copyFile(templateFile, destFile); err != nil {
			return nil, err
		}
		return LoadModelsConfig()
	}

	return &ModelsConfig{
		Models: []ModelConfig{},
	}, nil
}

func SaveChannelsConfig(cfg *ChannelsConfig) error {
	workspaceConfigPath, err := GetWorkspaceConfigPath()
	if err != nil {
		return err
	}

	configFile := filepath.Join(workspaceConfigPath, "channels.yml")
	return cfg.Save(configFile)
}

func SaveServerConfig(cfg *GlobalConfig) error {
	workspaceConfigPath, err := GetWorkspaceConfigPath()
	if err != nil {
		return err
	}

	configFile := filepath.Join(workspaceConfigPath, "server.yml")

	v := viper.New()
	v.Set("server", cfg)

	if err := v.WriteConfigAs(configFile); err != nil {
		return err
	}

	return nil
}

func SaveModelsConfig(cfg *ModelsConfig) error {
	workspaceConfigPath, err := GetWorkspaceConfigPath()
	if err != nil {
		return err
	}

	configFile := filepath.Join(workspaceConfigPath, "models.yml")

	v := viper.New()
	v.SetConfigFile(configFile)
	v.Set("models", cfg.Models)

	if err := v.WriteConfigAs(configFile); err != nil {
		return err
	}

	return nil
}

func SaveCapabilitiesConfig(cfg *CapabilityConfig) error {
	workspaceConfigPath, err := GetWorkspaceConfigPath()
	if err != nil {
		return err
	}

	configFile := filepath.Join(workspaceConfigPath, "capabilities.yml")

	v := viper.New()
	v.SetConfigFile(configFile)
	v.Set("capabilities", cfg.Capabilities)
	v.Set("default_capability", cfg.DefaultCapability)
	v.Set("fallback_to_local", cfg.FallbackToLocal)
	v.Set("description", cfg.Description)

	if err := v.WriteConfigAs(configFile); err != nil {
		return err
	}

	return nil
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	return destFile.Sync()
}
