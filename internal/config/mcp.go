package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const MCPServersFile = "mcp_servers"

type MCPServersConfig struct {
	MCPServers map[string]MCPServerEntry `json:"mcpServers"`
}

type MCPServerEntry struct {
	// Type: "stdio" (local subprocess) or "sse" (remote HTTP SSE). Default: "stdio"
	Type    string            `json:"type,omitempty"`
	// stdio fields
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	// sse fields
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	// common
	Enabled bool              `json:"enabled"`
}

// GetType returns the transport type, defaulting to "stdio" for backward compatibility.
func (e MCPServerEntry) GetType() string {
	if e.Type == "" {
		return "stdio"
	}
	return e.Type
}

// LoadMCPServersConfig 加载 MCP 服务器配置
// 文件不存在时返回空配置（不报错）
func LoadMCPServersConfig() (*MCPServersConfig, error) {
	workspaceConfigPath, err := GetWorkspaceConfigPath()
	if err != nil {
		return &MCPServersConfig{MCPServers: make(map[string]MCPServerEntry)}, nil
	}

	configFile := filepath.Join(workspaceConfigPath, MCPServersFile+".json")
	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &MCPServersConfig{MCPServers: make(map[string]MCPServerEntry)}, nil
		}
		return nil, err
	}

	cfg := &MCPServersConfig{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if cfg.MCPServers == nil {
		cfg.MCPServers = make(map[string]MCPServerEntry)
	}
	return cfg, nil
}

// SaveMCPServersConfig 保存 MCP 服务器配置
func SaveMCPServersConfig(cfg *MCPServersConfig) error {
	workspaceConfigPath, err := GetWorkspaceConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	configFile := filepath.Join(workspaceConfigPath, MCPServersFile+".json")
	return os.WriteFile(configFile, data, 0644)
}

var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// ResolveEnvVars 解析环境变量占位符 ${VAR_NAME}
func ResolveEnvVars(env map[string]string) map[string]string {
	return ResolveEnvVarsWithContext(env, nil)
}

// ResolveEnvVarsWithContext 解析环境变量占位符 ${VAR_NAME}
// 优先从 localEnv 中查找，找不到再从 os.Getenv 中查找
func ResolveEnvVarsWithContext(env map[string]string, localEnv map[string]string) map[string]string {
	resolved := make(map[string]string, len(env))
	for k, v := range env {
		resolved[k] = envVarPattern.ReplaceAllStringFunc(v, func(match string) string {
			varName := strings.TrimSuffix(strings.TrimPrefix(match, "${"), "}")
			if localEnv != nil {
				if val, ok := localEnv[varName]; ok {
					return val
				}
			}
			return os.Getenv(varName)
		})
	}
	return resolved
}
