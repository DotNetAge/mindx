package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"mindx/pkg/logging"
)

// MCPTool MCP 工具定义
type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	ServerName  string                 `json:"server_name"`
	Schema      map[string]interface{} `json:"schema"`
}

// MCPServer MCP 服务器配置
type MCPServer struct {
	Name    string            `json:"name"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

// MCPManager MCP 管理器
type MCPManager struct {
	configPath string
	servers    map[string]*MCPServer
	clients    map[string]*MCPClient
	tools      map[string]*MCPTool
	mu         sync.RWMutex
	logger     logging.Logger
}

// NewMCPManager 创建 MCP 管理器
func NewMCPManager(configPath string) *MCPManager {
	return &MCPManager{
		configPath: configPath,
		servers:    make(map[string]*MCPServer),
		clients:    make(map[string]*MCPClient),
		tools:      make(map[string]*MCPTool),
		logger:     logging.GetSystemLogger().Named("mcp_manager"),
	}
}

// LoadConfig 加载 MCP 配置
func (mm *MCPManager) LoadConfig() error {
	mm.logger.Info("loading MCP config", logging.String("path", mm.configPath))

	// 读取配置文件
	data, err := os.ReadFile(mm.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			mm.logger.Warn("MCP config file not found, skipping")
			return nil
		}
		return fmt.Errorf("failed to read config: %w", err)
	}

	// 解析配置
	var config struct {
		Servers map[string]*MCPServer `json:"servers"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	mm.mu.Lock()
	mm.servers = config.Servers
	mm.mu.Unlock()

	mm.logger.Info("MCP config loaded", logging.Int("servers", len(config.Servers)))

	return nil
}

// Connect 连接到所有 MCP 服务器
func (mm *MCPManager) Connect() error {
	mm.mu.RLock()
	servers := make([]*MCPServer, 0, len(mm.servers))
	for _, server := range mm.servers {
		servers = append(servers, server)
	}
	mm.mu.RUnlock()

	mm.logger.Info("connecting to MCP servers", logging.Int("count", len(servers)))

	// 连接每个服务器
	for _, server := range servers {
		if err := mm.connectServer(server); err != nil {
			mm.logger.Error("failed to connect to server",
				logging.String("server", server.Name),
				logging.Err(err),
			)
			// 继续连接其他服务器
			continue
		}
	}

	return nil
}

// connectServer 连接到单个 MCP 服务器
func (mm *MCPManager) connectServer(server *MCPServer) error {
	mm.logger.Debug("connecting to server", logging.String("name", server.Name))

	// 创建客户端
	client := NewMCPClient(server)

	// 连接
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}

	// 发现工具
	tools, err := client.DiscoverTools(ctx)
	if err != nil {
		client.Close()
		return fmt.Errorf("tool discovery failed: %w", err)
	}

	mm.mu.Lock()
	mm.clients[server.Name] = client
	for _, tool := range tools {
		tool.ServerName = server.Name
		mm.tools[tool.Name] = tool
	}
	mm.mu.Unlock()

	mm.logger.Info("server connected",
		logging.String("server", server.Name),
		logging.Int("tools", len(tools)),
	)

	return nil
}

// GetTool 获取 MCP 工具
func (mm *MCPManager) GetTool(name string) (*MCPTool, error) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	tool, ok := mm.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}

	return tool, nil
}

// ListTools 列出所有 MCP 工具
func (mm *MCPManager) ListTools() []string {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	names := make([]string, 0, len(mm.tools))
	for name := range mm.tools {
		names = append(names, name)
	}

	return names
}

// ExecuteTool 执行 MCP 工具
func (mm *MCPManager) ExecuteTool(name string, params map[string]interface{}) (string, error) {
	// 获取工具
	tool, err := mm.GetTool(name)
	if err != nil {
		return "", err
	}

	// 获取客户端
	mm.mu.RLock()
	client, ok := mm.clients[tool.ServerName]
	mm.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("client not found for server: %s", tool.ServerName)
	}

	// 执行工具
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := client.ExecuteTool(ctx, tool.Name, params)
	if err != nil {
		mm.logger.Error("tool execution failed",
			logging.String("tool", name),
			logging.Err(err),
		)
		return "", err
	}

	mm.logger.Info("tool executed successfully", logging.String("tool", name))

	return result, nil
}

// Close 关闭所有连接
func (mm *MCPManager) Close() error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	mm.logger.Info("closing all MCP connections")

	for name, client := range mm.clients {
		if err := client.Close(); err != nil {
			mm.logger.Error("failed to close client",
				logging.String("server", name),
				logging.Err(err),
			)
		}
	}

	mm.clients = make(map[string]*MCPClient)
	mm.tools = make(map[string]*MCPTool)

	return nil
}

// HasTool 检查工具是否存在
func (mm *MCPManager) HasTool(name string) bool {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	_, ok := mm.tools[name]
	return ok
}

// GetToolCount 获取工具数量
func (mm *MCPManager) GetToolCount() int {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	return len(mm.tools)
}

// GetServerCount 获取服务器数量
func (mm *MCPManager) GetServerCount() int {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	return len(mm.clients)
}
