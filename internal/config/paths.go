package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	ConfigDir    = "config"
	SkillsDir    = "skills"
	LogsDir      = "logs"
	DataDir      = "data"
	DocumentsDir = "documents"
	ImagesDir    = "images"
	SessionsDir  = "sessions"
	VectorsDir   = "vectors"
	MemoryDir    = "memory"
	ModelsDir    = "models"

	CapabilitiesFile = "capabilities"
	ChannelsFile     = "channels"
	ModelsFile       = "models"
	ServerFile       = "server"
	TopicsFile       = "topics"
	SkillsConfigFile = "skills"
	VersionFile      = "VERSION"

	SystemLogFile    = "system.log"
	TokenUsageDBFile = "token_usage.db"
)

var (
	ErrMINDXPathNotSet      = errors.New("MINDX_PATH environment variable not set")
	ErrMINDXWorkspaceNotSet = errors.New("MINDX_WORKSPACE environment variable not set")

	buildVersion   string = "dev"
	buildTime      string
	buildGitCommit string
)

func SetBuildInfo(version, time, commit string) {
	if version != "" {
		buildVersion = version
	}
	if time != "" {
		buildTime = time
	}
	if commit != "" {
		buildGitCommit = commit
	}
}

func GetBuildInfo() (version, time, commit string) {
	return buildVersion, buildTime, buildGitCommit
}

func GetInstallPath() (string, error) {
	path := os.Getenv("MINDX_PATH")
	if path == "" {
		execPath, err := os.Executable()
		if err == nil {
			path = filepath.Dir(execPath)
		} else {
			path, err = os.Getwd()
			if err != nil {
				return "", ErrMINDXPathNotSet
			}
		}
	}
	return path, nil
}

func GetWorkspacePath() (string, error) {
	path := os.Getenv("MINDX_WORKSPACE")
	if path == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			path, err = os.Getwd()
			if err != nil {
				return "", ErrMINDXWorkspaceNotSet
			}
		} else {
			path = filepath.Join(homeDir, ".mindx")
		}
	}
	return path, nil
}

func GetInstallConfigPath() (string, error) {
	installPath, err := GetInstallPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(installPath, ConfigDir), nil
}

func GetWorkspaceConfigPath() (string, error) {
	workspacePath, err := GetWorkspacePath()
	if err != nil {
		return "", err
	}
	return filepath.Join(workspacePath, ConfigDir), nil
}

func GetInstallSkillsPath() (string, error) {
	installPath, err := GetInstallPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(installPath, SkillsDir), nil
}

func GetWorkspaceLogsPath() (string, error) {
	workspacePath, err := GetWorkspacePath()
	if err != nil {
		return "", err
	}
	return filepath.Join(workspacePath, LogsDir), nil
}

func GetWorkspaceDataPath() (string, error) {
	workspacePath, err := GetWorkspacePath()
	if err != nil {
		return "", err
	}
	return filepath.Join(workspacePath, DataDir), nil
}

func GetWorkspaceDocumentsPath() (string, error) {
	workspacePath, err := GetWorkspacePath()
	if err != nil {
		return "", err
	}
	return filepath.Join(workspacePath, DocumentsDir), nil
}

func GetWorkspaceImagesPath() (string, error) {
	workspacePath, err := GetWorkspacePath()
	if err != nil {
		return "", err
	}
	return filepath.Join(workspacePath, ImagesDir), nil
}

func GetWorkspaceSessionsPath() (string, error) {
	dataPath, err := GetWorkspaceDataPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataPath, SessionsDir), nil
}

func GetWorkspaceVectorsPath() (string, error) {
	dataPath, err := GetWorkspaceDataPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataPath, VectorsDir), nil
}

func GetWorkspaceMemoryPath() (string, error) {
	dataPath, err := GetWorkspaceDataPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataPath, MemoryDir), nil
}

func GetWorkspaceModelsPath() (string, error) {
	dataPath, err := GetWorkspaceDataPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataPath, ModelsDir), nil
}

func GetWorkspaceSkillsConfigPath() (string, error) {
	workspacePath, err := GetWorkspacePath()
	if err != nil {
		return "", err
	}
	return filepath.Join(workspacePath, SkillsConfigFile), nil
}

func GetSystemLogPath() (string, error) {
	logsPath, err := GetWorkspaceLogsPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(logsPath, SystemLogFile), nil
}

func GetTokenUsageDBPath() (string, error) {
	dataPath, err := GetWorkspaceDataPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataPath, TokenUsageDBFile), nil
}

func EnsureWorkspace() error {
	workspacePath, err := GetWorkspacePath()
	if err != nil {
		return err
	}

	installPath, err := GetInstallPath()
	if err != nil {
		return err
	}

	dirs := []string{
		filepath.Join(workspacePath, ConfigDir),
		filepath.Join(workspacePath, LogsDir),
		filepath.Join(workspacePath, DataDir),
		filepath.Join(workspacePath, DocumentsDir),
		filepath.Join(workspacePath, ImagesDir),
		filepath.Join(workspacePath, DataDir, SessionsDir),
		filepath.Join(workspacePath, DataDir, VectorsDir),
		filepath.Join(workspacePath, DataDir, MemoryDir),
		filepath.Join(workspacePath, DataDir, ModelsDir),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	configFiles := []string{
		CapabilitiesFile,
		ChannelsFile,
		ModelsFile,
		ServerFile,
		TopicsFile,
	}

	installConfigDir := filepath.Join(installPath, ConfigDir)
	workspaceConfigDir := filepath.Join(workspacePath, ConfigDir)

	for _, file := range configFiles {
		for _, ext := range []string{".yml", ".yaml", ".json"} {
			srcFile := filepath.Join(installConfigDir, file+ext)
			dstFile := filepath.Join(workspaceConfigDir, file+ext)

			if _, err := os.Stat(srcFile); err == nil {
				if _, err := os.Stat(dstFile); os.IsNotExist(err) {
					data, err := os.ReadFile(srcFile)
					if err != nil {
						return fmt.Errorf("failed to read %s: %w", srcFile, err)
					}
					if err := os.WriteFile(dstFile, data, 0644); err != nil {
						return fmt.Errorf("failed to write %s: %w", dstFile, err)
					}
				}
				break
			}
		}
	}

	return nil
}

func GetVersion() (string, error) {
	if buildVersion != "" && buildVersion != "dev" {
		return buildVersion, nil
	}

	installPath, err := GetInstallPath()
	if err != nil {
		return "", err
	}

	versionPath := filepath.Join(installPath, VersionFile)
	data, err := os.ReadFile(versionPath)
	if err != nil {
		return "", fmt.Errorf("failed to read version file: %w", err)
	}

	return strings.TrimSpace(string(data)), nil
}
