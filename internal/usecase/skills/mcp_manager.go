package skills

import (
	"encoding/json"
	"fmt"
	"mindx/pkg/logging"
	"sync"
)

type MCPServerConfig struct {
	Name    string                 `json:"name"`
	Command string                 `json:"command"`
	Args    []string               `json:"args,omitempty"`
	Env     map[string]string      `json:"env,omitempty"`
}

type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

type MCPManager struct {
	logger  logging.Logger
	mu      sync.RWMutex
	servers map[string]*MCPServer
}

type MCPServer struct {
	config MCPServerConfig
	tools  map[string]*MCPTool
}

func NewMCPManager(logger logging.Logger) *MCPManager {
	return &MCPManager{
		logger:  logger.Named("MCPManager"),
		servers: make(map[string]*MCPServer),
	}
}

func (m *MCPManager) RegisterServer(config MCPServerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.servers[config.Name]; exists {
		return fmt.Errorf("mcp server already registered: %s", config.Name)
	}

	server := &MCPServer{
		config: config,
		tools:  make(map[string]*MCPTool),
	}

	m.servers[config.Name] = server
	m.logger.Info("MCP server registered", logging.String("name", config.Name))
	return nil
}

func (m *MCPManager) GetServer(name string) (*MCPServer, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	server, exists := m.servers[name]
	return server, exists
}

func (m *MCPManager) ListServers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.servers))
	for name := range m.servers {
		names = append(names, name)
	}
	return names
}

func (m *MCPManager) CallTool(serverName, toolName string, params map[string]any) (string, error) {
	m.mu.RLock()
	server, exists := m.servers[serverName]
	m.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("mcp server not found: %s", serverName)
	}

	return server.callTool(toolName, params, m.logger)
}

func (s *MCPServer) callTool(toolName string, params map[string]any, logger logging.Logger) (string, error) {
	logger.Info("Calling MCP tool", 
		logging.String("server", s.config.Name),
		logging.String("tool", toolName),
		logging.Any("params", params))

	result, err := json.Marshal(map[string]any{
		"status":  "success",
		"tool":    toolName,
		"server":  s.config.Name,
		"params":  params,
		"message": "MCP tool call placeholder - actual implementation would connect to real MCP server",
	})

	if err != nil {
		return "", err
	}

	return string(result), nil
}

func (m *MCPManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, server := range m.servers {
		m.logger.Info("Closing MCP server connection", logging.String("name", name))
		_ = server
	}
	m.servers = make(map[string]*MCPServer)
	return nil
}
