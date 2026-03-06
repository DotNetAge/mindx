package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"mindx/pkg/logging"
)

// MCPClient MCP 客户端
type MCPClient struct {
	server *MCPServer
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	mu     sync.Mutex
	logger logging.Logger
}

// NewMCPClient 创建 MCP 客户端
func NewMCPClient(server *MCPServer) *MCPClient {
	return &MCPClient{
		server: server,
		logger: logging.GetSystemLogger().Named("mcp_client"),
	}
}

// Connect 连接到 MCP 服务器
func (c *MCPClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger.Debug("starting MCP server",
		logging.String("server", c.server.Name),
		logging.String("command", c.server.Command),
	)

	// 创建命令
	c.cmd = exec.CommandContext(ctx, c.server.Command, c.server.Args...)

	// 设置环境变量
	if len(c.server.Env) > 0 {
		env := make([]string, 0, len(c.server.Env))
		for k, v := range c.server.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		c.cmd.Env = append(c.cmd.Env, env...)
	}

	// 获取标准输入输出
	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin: %w", err)
	}

	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout: %w", err)
	}

	c.stderr, err = c.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr: %w", err)
	}

	// 启动进程
	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	c.logger.Info("MCP server started", logging.String("server", c.server.Name))

	// 发送初始化请求
	if err := c.initialize(ctx); err != nil {
		c.Close()
		return fmt.Errorf("initialization failed: %w", err)
	}

	return nil
}

// initialize 初始化 MCP 连接
func (c *MCPClient) initialize(ctx context.Context) error {
	// 发送 initialize 请求
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "mindx",
				"version": "1.0.0",
			},
		},
	}

	if err := c.sendRequest(request); err != nil {
		return err
	}

	// 读取响应
	response, err := c.readResponse()
	if err != nil {
		return err
	}

	// 检查错误
	if errObj, ok := response["error"]; ok {
		return fmt.Errorf("initialization error: %v", errObj)
	}

	c.logger.Debug("MCP server initialized", logging.String("server", c.server.Name))

	return nil
}

// DiscoverTools 发现工具
func (c *MCPClient) DiscoverTools(ctx context.Context) ([]*MCPTool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 发送 tools/list 请求
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}

	if err := c.sendRequest(request); err != nil {
		return nil, err
	}

	// 读取响应
	response, err := c.readResponse()
	if err != nil {
		return nil, err
	}

	// 检查错误
	if errObj, ok := response["error"]; ok {
		return nil, fmt.Errorf("tools/list error: %v", errObj)
	}

	// 解析工具列表
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	toolsData, ok := result["tools"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid tools format")
	}

	tools := make([]*MCPTool, 0, len(toolsData))
	for _, toolData := range toolsData {
		toolMap, ok := toolData.(map[string]interface{})
		if !ok {
			continue
		}

		tool := &MCPTool{
			Name:        toolMap["name"].(string),
			Description: toolMap["description"].(string),
			Schema:      toolMap["inputSchema"].(map[string]interface{}),
		}

		tools = append(tools, tool)
	}

	c.logger.Debug("tools discovered",
		logging.String("server", c.server.Name),
		logging.Int("count", len(tools)),
	)

	return tools, nil
}

// ExecuteTool 执行工具
func (c *MCPClient) ExecuteTool(ctx context.Context, name string, params map[string]interface{}) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 发送 tools/call 请求
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      name,
			"arguments": params,
		},
	}

	if err := c.sendRequest(request); err != nil {
		return "", err
	}

	// 读取响应
	response, err := c.readResponse()
	if err != nil {
		return "", err
	}

	// 检查错误
	if errObj, ok := response["error"]; ok {
		return "", fmt.Errorf("tool execution error: %v", errObj)
	}

	// 解析结果
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid response format")
	}

	// 提取内容
	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	firstContent, ok := content[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid content format")
	}

	text, ok := firstContent["text"].(string)
	if !ok {
		return "", fmt.Errorf("no text in content")
	}

	return text, nil
}

// sendRequest 发送请求
func (c *MCPClient) sendRequest(request map[string]interface{}) error {
	data, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	data = append(data, '\n')

	if _, err := c.stdin.Write(data); err != nil {
		return fmt.Errorf("failed to write request: %w", err)
	}

	return nil
}

// readResponse 读取响应
func (c *MCPClient) readResponse() (map[string]interface{}, error) {
	reader := bufio.NewReader(c.stdout)

	line, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(line, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

// Close 关闭连接
func (c *MCPClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cmd == nil || c.cmd.Process == nil {
		return nil
	}

	c.logger.Debug("closing MCP client", logging.String("server", c.server.Name))

	// 关闭标准输入输出
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.stdout != nil {
		c.stdout.Close()
	}
	if c.stderr != nil {
		c.stderr.Close()
	}

	// 终止进程
	if err := c.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to kill process: %w", err)
	}

	// 等待进程结束
	c.cmd.Wait()

	c.logger.Info("MCP client closed", logging.String("server", c.server.Name))

	return nil
}
