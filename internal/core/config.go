package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type DaemonConfig struct {
	Enabled bool   `json:"enabled"`
	Port    int    `json:"port,omitempty"`
	Path    string `json:"path,omitempty"`
}

type MindxConfig struct {
	Version       int          `json:"version"`
	Initialized   bool         `json:"initialized"`
	LastAgent     string       `json:"last_agent,omitempty"`
	LastSessionID string       `json:"last_session_id,omitempty"`
	DefaultModel  string       `json:"default_model,omitempty"`
	Daemon        DaemonConfig `json:"daemon"`

	filePath string `json:"-"`
}

func DefaultMindxConfig(workspaceDir string) *MindxConfig {
	return &MindxConfig{
		Version: 1,
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
		return nil, fmt.Errorf("读取 mindx.json 失败: %w", err)
	}

	cfg := &MindxConfig{filePath: filePath}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析 mindx.json 失败: %w", err)
	}

	return cfg, nil
}

func (c *MindxConfig) Save() error {
	if c.filePath == "" {
		return fmt.Errorf("mindx.json 路径未设置")
	}

	dir := filepath.Dir(c.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 mindx.json 失败: %w", err)
	}

	return os.WriteFile(c.filePath, data, 0644)
}

func (c *MindxConfig) MarkInitialized() error {
	c.Initialized = true
	return c.Save()
}
