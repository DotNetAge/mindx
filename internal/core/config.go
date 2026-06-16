package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DotNetAge/goharness/rule"
	"github.com/DotNetAge/mindx/internal/i18n"
)

type DaemonConfig struct {
	Enabled        bool   `json:"enabled"`
	Port           int    `json:"port,omitempty"`
	Path           string `json:"path,omitempty"`
	AutoStart      bool   `json:"autostart,omitempty"`
	Installed      bool   `json:"installed,omitempty"`
	InstallMethod  string `json:"install_method,omitempty"` // "launchd" | "systemd" | "snapctl" | "schtasks" | "dbus"
}

type PythonConfig struct {
	Detected bool   `json:"detected"`
	Version  string `json:"version,omitempty"`
	VenvPath string `json:"venv_path,omitempty"`
}

type MindxConfig struct {
	Version         int          `json:"version"`
	AppVersion      string       `json:"-"` // runtime app version, not persisted
	Initialized     bool         `json:"initialized"`
	LastAgent       string       `json:"last_agent,omitempty"`
	LastSessionID   string       `json:"last_session_id,omitempty"`
	LastModel       string       `json:"last_model,omitempty"`
	DefaultModel    string       `json:"default_model,omitempty"`
	DefaultProvider string       `json:"default_provider,omitempty"`
	EmbedderModel   string       `json:"embedder_model,omitempty"`
	Daemon          DaemonConfig `json:"daemon"`
	Python          PythonConfig `json:"python"`

	// PermissionRules stores user-defined allow/deny/ask rules.
	PermissionRules *rule.PermissionRules `json:"permission_rules,omitempty"`

	// Language is the UI language (e.g. "zh", "en"). Defaults to system locale.
	Language string `json:"language,omitempty"`

	// InstalledVersion records the version that was last installed/updated.
	// Used by the auto-updater to track which version is on disk.
	InstalledVersion string `json:"installed_version,omitempty"`

	filePath string `json:"-"`
}

// EmbedderModelPath 返回 Embedder ONNX 模型文件的完整路径。
// 模型文件约定存放在 <workspaceDir>/data/models/<EmbedderModel>。
// 如果未配置 EmbedderModel 则返回空字符串。
func (c *MindxConfig) EmbedderModelPath(workspaceDir string) string {
	if c.EmbedderModel == "" {
		return ""
	}
	return filepath.Join(workspaceDir, "data", "models", c.EmbedderModel)
}

// HasEmbedder 报告是否已配置 Embedder 模型（Memory 可用）。
func (c *MindxConfig) HasEmbedder() bool {
	return c.EmbedderModel != ""
}

func DefaultMindxConfig(workspaceDir string) *MindxConfig {
	return &MindxConfig{
		Version:  1,
		filePath: filepath.Join(workspaceDir, "mindx.json"),
	}
}

func LoadMindxConfig(workspaceDir string) (*MindxConfig, error) {
	filePath := filepath.Join(workspaceDir, "mindx.json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultMindxConfig(workspaceDir), nil
		}
		return nil, fmt.Errorf(i18n.T("config.error.read.failed"), err)
	}

	cfg := &MindxConfig{filePath: filePath}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf(i18n.T("config.error.parse.failed"), err)
	}

	return cfg, nil
}

func (c *MindxConfig) Save() error {
	if c.filePath == "" {
		return errors.New(i18n.T("config.error.path.unset"))
	}

	dir := filepath.Dir(c.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf(i18n.T("config.error.mkdir.failed"), err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf(i18n.T("config.error.serialize.failed"), err)
	}

	// 原子写入：先写临时文件再 rename，避免进程崩溃导致配置文件损坏
	tmpPath := c.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write temp config: %w", err)
	}
	return os.Rename(tmpPath, c.filePath)
}

func (c *MindxConfig) MarkInitialized() error {
	c.Initialized = true
	return c.Save()
}
