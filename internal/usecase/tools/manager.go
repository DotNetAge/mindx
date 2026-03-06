package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"mindx/pkg/logging"
)

// Tool 本地工具定义
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Version     string                 `json:"version"`
	Type        string                 `json:"type"` // go, python, shell, builtin
	Command     string                 `json:"command"`
	Parameters  map[string]interface{} `json:"parameters"`
	Timeout     int                    `json:"timeout"` // 秒
	WorkDir     string                 `json:"-"`       // 工具所在目录
}

// ToolManager 本地工具管理器
type ToolManager struct {
	toolsDir string
	tools    map[string]*Tool
	mu       sync.RWMutex
	logger   logging.Logger
}

// NewToolManager 创建工具管理器
func NewToolManager(toolsDir string) *ToolManager {
	return &ToolManager{
		toolsDir: toolsDir,
		tools:    make(map[string]*Tool),
		logger:   logging.GetSystemLogger().Named("tool_manager"),
	}
}

// LoadTools 加载所有本地工具
func (tm *ToolManager) LoadTools() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.logger.Info("loading tools", logging.String("dir", tm.toolsDir))

	// 检查目录是否存在
	if _, err := os.Stat(tm.toolsDir); os.IsNotExist(err) {
		tm.logger.Warn("tools directory not found", logging.String("dir", tm.toolsDir))
		return nil
	}

	// 扫描工具目录
	entries, err := os.ReadDir(tm.toolsDir)
	if err != nil {
		return fmt.Errorf("failed to read tools directory: %w", err)
	}

	loadedCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		toolName := entry.Name()
		toolDir := filepath.Join(tm.toolsDir, toolName)

		// 加载工具
		tool, err := tm.loadTool(toolDir)
		if err != nil {
			tm.logger.Warn("failed to load tool",
				logging.String("tool", toolName),
				logging.Err(err),
			)
			continue
		}

		tm.tools[tool.Name] = tool
		loadedCount++

		tm.logger.Debug("tool loaded",
			logging.String("name", tool.Name),
			logging.String("type", tool.Type),
		)
	}

	tm.logger.Info("tools loaded",
		logging.Int("count", loadedCount),
	)

	return nil
}

// loadTool 加载单个工具
func (tm *ToolManager) loadTool(toolDir string) (*Tool, error) {
	// 读取 tool.json
	configPath := filepath.Join(toolDir, "tool.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read tool.json: %w", err)
	}

	// 解析配置
	var tool Tool
	if err := json.Unmarshal(data, &tool); err != nil {
		return nil, fmt.Errorf("failed to parse tool.json: %w", err)
	}

	// 验证必需字段
	if tool.Name == "" {
		return nil, fmt.Errorf("tool name is required")
	}
	if tool.Type == "" {
		return nil, fmt.Errorf("tool type is required")
	}

	// 设置工作目录
	tool.WorkDir = toolDir

	// 设置默认超时
	if tool.Timeout == 0 {
		tool.Timeout = 30
	}

	return &tool, nil
}

// GetTool 获取指定工具
func (tm *ToolManager) GetTool(name string) (*Tool, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tool, ok := tm.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}

	return tool, nil
}

// ListTools 列出所有工具
func (tm *ToolManager) ListTools() []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	names := make([]string, 0, len(tm.tools))
	for name := range tm.tools {
		names = append(names, name)
	}

	return names
}

// HasTool 检查工具是否存在
func (tm *ToolManager) HasTool(name string) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	_, ok := tm.tools[name]
	return ok
}

// GetToolCount 获取工具数量
func (tm *ToolManager) GetToolCount() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return len(tm.tools)
}

// ReloadTool 重新加载指定工具
func (tm *ToolManager) ReloadTool(name string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 查找工具目录
	toolDir := filepath.Join(tm.toolsDir, name)
	if _, err := os.Stat(toolDir); os.IsNotExist(err) {
		return fmt.Errorf("tool directory not found: %s", name)
	}

	// 重新加载
	tool, err := tm.loadTool(toolDir)
	if err != nil {
		return fmt.Errorf("failed to reload tool: %w", err)
	}

	tm.tools[tool.Name] = tool

	tm.logger.Info("tool reloaded", logging.String("name", tool.Name))

	return nil
}

// Clear 清空所有工具
func (tm *ToolManager) Clear() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.tools = make(map[string]*Tool)
}
