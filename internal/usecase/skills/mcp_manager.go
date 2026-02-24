package skills

import (
	"context"
	"fmt"
	"mindx/internal/config"
	"mindx/pkg/logging"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type MCPServerStatus string

const (
	MCPServerStatusConnected    MCPServerStatus = "connected"
	MCPServerStatusDisconnected MCPServerStatus = "disconnected"
	MCPServerStatusError        MCPServerStatus = "error"
)

type MCPServerState struct {
	Name   string                 `json:"name"`
	Config config.MCPServerEntry  `json:"config"`
	Status MCPServerStatus        `json:"status"`
	Error  string                 `json:"error,omitempty"`
	Tools  []*mcp.Tool            `json:"tools,omitempty"`

	client  *mcp.Client
	session *mcp.ClientSession
}

type MCPManager struct {
	logger  logging.Logger
	mu      sync.RWMutex
	servers map[string]*MCPServerState
}

func NewMCPManager(logger logging.Logger) *MCPManager {
	return &MCPManager{
		logger:  logger.Named("MCPManager"),
		servers: make(map[string]*MCPServerState),
	}
}

// ConnectServer 连接 MCP server（支持 stdio 和 sse 两种传输方式）
func (m *MCPManager) ConnectServer(ctx context.Context, name string, entry config.MCPServerEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 如果已存在，先关闭旧连接
	if existing, ok := m.servers[name]; ok && existing.session != nil {
		_ = existing.session.Close()
	}

	state := &MCPServerState{
		Name:   name,
		Config: entry,
		Status: MCPServerStatusDisconnected,
	}

	// 创建 MCP 客户端
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "mindx",
		Version: "v1.0.0",
	}, nil)

	// 根据传输类型构造 transport
	var transport mcp.Transport
	switch entry.GetType() {
	case "sse":
		sseTransport := &mcp.SSEClientTransport{
			Endpoint: entry.URL,
		}
		// 用 entry.Env 作为本地变量上下文解析 headers 中的 ${VAR} 占位符
		if len(entry.Headers) > 0 {
			resolvedHeaders := config.ResolveEnvVarsWithContext(entry.Headers, entry.Env)
			sseTransport.HTTPClient = &http.Client{
				Transport: &headerRoundTripper{
					base:    http.DefaultTransport,
					headers: resolvedHeaders,
				},
			}
		}
		transport = sseTransport
	default: // "stdio"
		cmd := exec.Command(entry.Command, entry.Args...)
		// 继承当前进程的完整环境变量，再覆盖用户配置的变量
		cmd.Env = os.Environ()
		if len(entry.Env) > 0 {
			resolvedEnv := config.ResolveEnvVars(entry.Env)
			for k, v := range resolvedEnv {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
			}
		}
		// 工作目录设为用户 HOME，避免依赖 mindx 进程的 pwd
		if home, err := os.UserHomeDir(); err == nil {
			cmd.Dir = home
		}
		transport = &mcp.CommandTransport{Command: cmd}
	}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		state.Status = MCPServerStatusError
		state.Error = err.Error()
		m.servers[name] = state
		m.logger.Error("MCP server 连接失败",
			logging.String("server", name),
			logging.Err(err))
		return fmt.Errorf("connect to MCP server %s: %w", name, err)
	}

	state.client = client
	state.session = session
	state.Status = MCPServerStatusConnected

	// 发现工具
	toolsResult, err := session.ListTools(ctx, nil)
	if err != nil {
		m.logger.Warn("MCP server 工具发现失败",
			logging.String("server", name),
			logging.Err(err))
		state.Error = fmt.Sprintf("tools/list failed: %s", err.Error())
	} else {
		state.Tools = toolsResult.Tools
		toolNames := make([]string, 0, len(toolsResult.Tools))
		for _, t := range toolsResult.Tools {
			toolNames = append(toolNames, t.Name)
		}
		m.logger.Info("MCP server 工具发现完成",
			logging.String("server", name),
			logging.Int("tools_count", len(toolsResult.Tools)),
			logging.String("tools", strings.Join(toolNames, ", ")))
	}

	m.servers[name] = state
	return nil
}

// DisconnectServer 断开 MCP server 连接
func (m *MCPManager) DisconnectServer(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.servers[name]
	if !ok {
		return fmt.Errorf("MCP server not found: %s", name)
	}

	if state.session != nil {
		if err := state.session.Close(); err != nil {
			m.logger.Warn("MCP server 关闭失败",
				logging.String("server", name),
				logging.Err(err))
		}
	}

	state.session = nil
	state.client = nil
	state.Status = MCPServerStatusDisconnected
	state.Tools = nil
	state.Error = ""
	return nil
}

// CallTool 调用 MCP server 上的工具
func (m *MCPManager) CallTool(ctx context.Context, serverName, toolName string, args map[string]any) (string, error) {
	m.mu.RLock()
	state, ok := m.servers[serverName]
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("MCP server not found: %s", serverName)
	}
	if state.session == nil || state.Status != MCPServerStatusConnected {
		return "", fmt.Errorf("MCP server not connected: %s (status: %s)", serverName, state.Status)
	}

	m.logger.Info("调用 MCP 工具",
		logging.String("server", serverName),
		logging.String("tool", toolName))

	result, err := state.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		// 连接可能已断开，更新状态
		m.mu.Lock()
		state.Status = MCPServerStatusError
		state.Error = err.Error()
		m.mu.Unlock()
		return "", fmt.Errorf("MCP tool call failed: %w", err)
	}

	if result.IsError {
		return "", fmt.Errorf("MCP tool returned error: %s", extractTextContent(result.Content))
	}

	return extractTextContent(result.Content), nil
}

// extractTextContent 从 MCP Content 列表中提取文本
func extractTextContent(contents []mcp.Content) string {
	var parts []string
	for _, c := range contents {
		if tc, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// GetDiscoveredTools 获取某 server 发现的工具列表
func (m *MCPManager) GetDiscoveredTools(serverName string) ([]*mcp.Tool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, ok := m.servers[serverName]
	if !ok {
		return nil, fmt.Errorf("MCP server not found: %s", serverName)
	}
	return state.Tools, nil
}

// GetServerState 获取某 server 的状态
func (m *MCPManager) GetServerState(name string) (*MCPServerState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, ok := m.servers[name]
	return state, ok
}

// ListServers 列出所有 server 状态
func (m *MCPManager) ListServers() []*MCPServerState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*MCPServerState, 0, len(m.servers))
	for _, state := range m.servers {
		result = append(result, state)
	}
	return result
}

// RemoveServer 移除 server（断开连接并从列表中删除）
func (m *MCPManager) RemoveServer(name string) error {
	if err := m.DisconnectServer(name); err != nil {
		// 即使断开失败也继续删除
		m.logger.Warn("断开 MCP server 失败", logging.String("server", name), logging.Err(err))
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.servers, name)
	return nil
}

// Close 关闭所有 MCP server 连接
func (m *MCPManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, state := range m.servers {
		if state.session != nil {
			m.logger.Info("关闭 MCP server 连接", logging.String("server", name))
			if err := state.session.Close(); err != nil {
				m.logger.Warn("MCP server 关闭失败",
					logging.String("server", name),
					logging.Err(err))
			}
		}
	}
	m.servers = make(map[string]*MCPServerState)
	return nil
}

// headerRoundTripper 在每个 HTTP 请求中注入自定义 headers（用于 SSE 认证）
type headerRoundTripper struct {
	base    http.RoundTripper
	headers map[string]string
}

func (rt *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range rt.headers {
		req.Header.Set(k, v)
	}
	return rt.base.RoundTrip(req)
}
