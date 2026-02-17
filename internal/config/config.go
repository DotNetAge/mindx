package config

import (
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

func InitVippers() (srvCfg *GlobalConfig, channelsCfg *ChannelsConfig, capabilitiesCfg *CapabilityConfig) {
	srvCfg, err := LoadServerConfig()
	if err != nil {
		log.Fatal("加载server配置失败：", err)
	}

	channelsCfg, err = LoadChannelsConfig()
	if err != nil {
		log.Fatal("加载channels配置失败：", err)
	}

	capabilitiesCfg, err = LoadCapabilitiesConfig()
	if err != nil {
		log.Fatal("加载capabilities配置失败：", err)
	}

	return srvCfg, channelsCfg, capabilitiesCfg
}

func LoadServerConfig() (*GlobalConfig, error) {
	workspaceConfigPath, err := GetWorkspaceConfigPath()
	if err != nil {
		return nil, err
	}

	configFiles := []string{"server.yaml", "server.yml", "server.json"}

	for _, configFile := range configFiles {
		fullPath := filepath.Join(workspaceConfigPath, configFile)
		if _, err := os.Stat(fullPath); err == nil {
			v := viper.New()
			v.SetConfigFile(fullPath)
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
	}

	installConfigPath, err := GetInstallConfigPath()
	if err != nil {
		return nil, err
	}

	templateFile := filepath.Join(installConfigPath, "server.yaml.template")
	if _, err := os.Stat(templateFile); err != nil {
		return nil, err
	}

	destFile := filepath.Join(workspaceConfigPath, "server.yaml")
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

	configFiles := []string{"channels.yaml", "channels.yml", "channels.json"}

	for _, configFile := range configFiles {
		fullPath := filepath.Join(workspaceConfigPath, configFile)
		if _, err := os.Stat(fullPath); err == nil {
			v := viper.New()
			v.SetConfigFile(fullPath)
			if err := v.ReadInConfig(); err != nil {
				return nil, err
			}

			cfg := &ChannelsConfig{}
			if err := v.Unmarshal(cfg); err != nil {
				return nil, err
			}

			return cfg, nil
		}
	}

	installConfigPath, err := GetInstallConfigPath()
	if err != nil {
		return nil, err
	}

	templateFile := filepath.Join(installConfigPath, "channels.json.template")
	if _, err := os.Stat(templateFile); err != nil {
		return nil, err
	}

	destFile := filepath.Join(workspaceConfigPath, "channels.json")
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

	configFiles := []string{"capabilities.yaml", "capabilities.yml", "capabilities.json"}

	for _, configFile := range configFiles {
		fullPath := filepath.Join(workspaceConfigPath, configFile)
		if _, err := os.Stat(fullPath); err == nil {
			v := viper.New()
			v.SetConfigFile(fullPath)
			if err := v.ReadInConfig(); err != nil {
				return nil, err
			}

			cfg := &CapabilityConfig{}
			if err := v.Unmarshal(cfg); err != nil {
				return nil, err
			}

			return cfg, nil
		}
	}

	installConfigPath, err := GetInstallConfigPath()
	if err != nil {
		return nil, err
	}

	templateFile := filepath.Join(installConfigPath, "capabilities.json.template")
	if _, err := os.Stat(templateFile); err != nil {
		return nil, err
	}

	destFile := filepath.Join(workspaceConfigPath, "capabilities.json")
	if err := copyFile(templateFile, destFile); err != nil {
		return nil, err
	}

	return LoadCapabilitiesConfig()
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
