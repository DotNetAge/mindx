package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ── MCPClient interface ─────────────────────────────────────────────────────

// MCPClient abstracts the transport layer to an MCP server.
// Implementations handle stdio, SSE, and HTTP transports.
type MCPClient interface {
	// Connect establishes connection and performs MCP initialize handshake.
	Connect(ctx context.Context) error
	// Close terminates the connection.
	Close() error
	// IsAlive checks whether the connection is still usable.
	IsAlive() bool
	// Call sends a JSON-RPC request and waits for the response.
	Call(ctx context.Context, method string, params any) (json.RawMessage, error)
	// Notify sends a JSON-RPC notification (no id, no response expected).
	Notify(ctx context.Context, method string, params any) error
}

// ── Client Factory ──────────────────────────────────────────────────────────

// NewClient creates an MCPClient for the given server configuration.
// creds maps credential references to resolved credential values.
func NewClient(cfg ServerConfig, creds map[string]string) (MCPClient, error) {
	switch cfg.Type {
	case ServerTypeStdio:
		return newStdioClient(cfg, creds)
	case ServerTypeSSE:
		return newSSEClient(cfg, creds)
	case ServerTypeHTTP:
		return newHTTPClient(cfg, creds)
	default:
		return nil, fmt.Errorf("unsupported server type: %s", cfg.Type)
	}
}

// injectCreds replaces credential refs in env with resolved values.
func injectCreds(env map[string]string, creds map[string]string) map[string]string {
	result := make(map[string]string, len(env))
	for k, v := range env {
		result[k] = v
	}
	for ref, val := range creds {
		result[ref] = val
	}
	return result
}

// applyHeaders 应用配置的 headers 并注入凭据（与 env 的 injectCreds 同语义），
// 用于 SSE/HTTP 传输的鉴权（如 Authorization、x-api-key）。
func applyHeaders(req *http.Request, headers, creds map[string]string) {
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	for ref, val := range creds {
		if ref != "" && val != "" {
			req.Header.Set(ref, val)
		}
	}
}

// ── StdioClient ─────────────────────────────────────────────────────────────

// StdioClient communicates with a subprocess via stdin/stdout using
// MCP JSON-RPC with newline-delimited messages.
type stdioClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner

	tracker *rpcTracker
	stopCh  chan struct{}
	running sync.WaitGroup
	mu      sync.Mutex
	alive   bool
}

func newStdioClient(cfg ServerConfig, creds map[string]string) (*stdioClient, error) {
	env := injectCreds(cfg.Env, creds)
	return &stdioClient{
		cmd:     buildCommand(cfg.Command, cfg.Args, env),
		tracker: newRPCTracker(),
		stopCh:  make(chan struct{}),
	}, nil
}

func buildCommand(command string, args []string, env map[string]string) *exec.Cmd {
	cmd := exec.Command(command, args...)
	// 继承系统环境（PATH 等必需），再叠加 mcp.json 配置的 env（同名覆盖）
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	return cmd
}

func (c *stdioClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	stdin, err := c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := c.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("start process: %w", err)
	}

	c.stdin = stdin
	c.stdout = bufio.NewScanner(stdout)

	// start read loop to receive responses
	c.running.Add(1)
	go c.readLoop()

	// drain stderr in background
	c.running.Add(1)
	go func() {
		defer c.running.Done()
		_, _ = io.Copy(io.Discard, stderr)
	}()

	// initialize handshake
	if err := c.handshake(ctx); err != nil {
		_ = c.closeUnlocked()
		return fmt.Errorf("initialize handshake: %w", err)
	}

	c.alive = true
	return nil
}

func (c *stdioClient) handshake(ctx context.Context) error {
	result, err := c.sendRequest(ctx, "initialize", initializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities:    clientCapabilities{},
		ClientInfo:      clientInfo{Name: "mindx", Version: "1.0"},
	})
	if err != nil {
		return err
	}
	var initResp initializeResult
	if err := json.Unmarshal(result, &initResp); err != nil {
		return fmt.Errorf("parse initialize result: %w", err)
	}
	// 协议要求：initialize 响应后、其他请求前发送 notifications/initialized
	return c.Notify(ctx, "notifications/initialized", nil)
}

func (c *stdioClient) sendRequest(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.tracker.nextRequestID()
	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resultCh := make(chan *rpcResponse, 1)
	call := &pendingCall{
		id:     id,
		result: resultCh,
		done:   ctx.Done(),
	}
	c.tracker.register(call)

	if _, err := c.stdin.Write(append(data, '\n')); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	select {
	case resp := <-resultCh:
		if resp == nil {
			return nil, fmt.Errorf("connection closed")
		}
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *stdioClient) readLoop() {
	defer c.running.Done()
	for c.stdout.Scan() {
		line := c.stdout.Bytes()
		if len(line) == 0 {
			continue
		}
		var resp rpcResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			continue
		}
		if resp.ID != nil {
			c.tracker.resolve(&resp)
		}
	}
}

func (c *stdioClient) IsAlive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.alive && c.cmd.ProcessState == nil
}

func (c *stdioClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closeUnlocked()
}

func (c *stdioClient) closeUnlocked() error {
	c.alive = false
	c.tracker.cancelAll()
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	c.running.Wait()
	return nil
}

func (c *stdioClient) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	return c.sendRequest(ctx, method, params)
}

// Notify 写入一条 notification（无 id，不注册 tracker、不等待响应）。
// 注意：不持 c.mu——Connect 在锁内调用 handshake→Notify，可重入加锁会自死锁；
// 与 sendRequest 同样假定调用方串行（Connect 阶段串行，运行期 Call 单飞）。
func (c *stdioClient) Notify(ctx context.Context, method string, params any) error {
	data, err := json.Marshal(rpcRequest{JSONRPC: "2.0", Method: method, Params: params})
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}
	if _, err := c.stdin.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write notification: %w", err)
	}
	return nil
}

// ── SSEClient ───────────────────────────────────────────────────────────────

// SSEClient communicates with an MCP server using SSE for server→client
// and HTTP POST for client→server messages.
type sseClient struct {
	sseURL   string
	postURL  string
	headers  map[string]string
	creds    map[string]string
	sseHTTP  *http.Client // SSE 长连接专用：不设 Timeout（Timeout 会掐断整个事件流）
	postHTTP *http.Client // POST 单请求专用：per-request 超时控制
	tracker  *rpcTracker

	mu    sync.Mutex
	alive bool
	stop  context.CancelFunc
}

func newSSEClient(cfg ServerConfig, creds map[string]string) (*sseClient, error) {
	return &sseClient{
		sseURL:   cfg.URL,
		headers:  cfg.Headers,
		creds:    creds,
		sseHTTP:  &http.Client{},
		postHTTP: &http.Client{Timeout: 30 * time.Second},
		tracker:  newRPCTracker(),
	}, nil
}

func (c *sseClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// SSE 是独立长连接：生命周期不绑定调用方 ctx（工具调用结束不能断流），Close 时统一回收
	streamCtx, cancel := context.WithCancel(context.Background())
	c.stop = cancel

	// Open SSE connection
	req, err := http.NewRequestWithContext(streamCtx, "GET", c.sseURL, nil)
	if err != nil {
		cancel()
		return fmt.Errorf("create SSE request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	applyHeaders(req, c.headers, c.creds)

	resp, err := c.sseHTTP.Do(req)
	if err != nil {
		cancel()
		return fmt.Errorf("connect SSE: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		cancel()
		return fmt.Errorf("connect SSE: 服务端返回 %d", resp.StatusCode)
	}

	// Read SSE events to get the POST endpoint
	scanner := bufio.NewScanner(resp.Body)
	var postEndpoint string
	var gotEndpoint bool

	// Read first event — should be the endpoint event
	for scanner.Scan() && !gotEndpoint {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: endpoint") {
			if scanner.Scan() {
				dataLine := scanner.Text()
				if strings.HasPrefix(dataLine, "data: ") {
					postEndpoint = strings.TrimPrefix(dataLine, "data: ")
					gotEndpoint = true
				}
			}
		}
	}

	if !gotEndpoint {
		resp.Body.Close()
		cancel()
		return fmt.Errorf("failed to receive endpoint from SSE server")
	}

	// endpoint 可能是相对路径（官方 SDK 返回 "/message?sessionId=..."），按 SSE URL 解析为绝对地址
	if ref, err := url.Parse(strings.TrimSpace(postEndpoint)); err == nil {
		if base, err := url.Parse(c.sseURL); err == nil {
			postEndpoint = base.ResolveReference(ref).String()
		}
	}

	c.postURL = postEndpoint

	// Start background goroutine to receive SSE events
	go c.readSSE(streamCtx, scanner, resp.Body)

	// initialize handshake：独立超时，不复用调用方 ctx
	hsCtx, hsCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer hsCancel()
	if err := c.handshake(hsCtx); err != nil {
		cancel()
		resp.Body.Close()
		return fmt.Errorf("initialize handshake: %w", err)
	}

	c.alive = true
	return nil
}

func (c *sseClient) handshake(ctx context.Context) error {
	result, err := c.sendRequest(ctx, "initialize", initializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities:    clientCapabilities{},
		ClientInfo:      clientInfo{Name: "mindx", Version: "1.0"},
	})
	if err != nil {
		return err
	}
	var initResp initializeResult
	if err := json.Unmarshal(result, &initResp); err != nil {
		return fmt.Errorf("parse initialize result: %w", err)
	}
	// 协议要求：initialize 响应后、其他请求前发送 notifications/initialized
	return c.Notify(ctx, "notifications/initialized", nil)
}

func (c *sseClient) readSSE(ctx context.Context, scanner *bufio.Scanner, body io.ReadCloser) {
	defer body.Close()
	// 流结束（服务端断开/网络异常）后标记失活，pool.Call 的自动重连据此重建连接
	defer func() {
		c.mu.Lock()
		c.alive = false
		c.mu.Unlock()
	}()

	var eventName string
	var dataBuffer strings.Builder

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		if line == "" {
			// Empty line = end of event
			if eventName == "message" && dataBuffer.Len() > 0 {
				var resp rpcResponse
				if err := json.Unmarshal([]byte(dataBuffer.String()), &resp); err == nil && resp.ID != nil {
					c.tracker.resolve(&resp)
				}
			}
			eventName = ""
			dataBuffer.Reset()
			continue
		}

		if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			dataBuffer.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
}

func (c *sseClient) postJSON(ctx context.Context, payload []byte) error {
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.postURL, strings.NewReader(string(payload)))
	if err != nil {
		return fmt.Errorf("create POST request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	applyHeaders(httpReq, c.headers, c.creds)

	httpResp, err := c.postHTTP.Do(httpReq)
	if err != nil {
		return fmt.Errorf("POST request: %w", err)
	}
	defer httpResp.Body.Close()
	// 官方实现对 notification 返回 202 Accepted；非 2x 视为失败
	if httpResp.StatusCode >= 300 {
		return fmt.Errorf("POST 请求被拒绝: %d", httpResp.StatusCode)
	}
	_, _ = io.Copy(io.Discard, httpResp.Body)
	return nil
}

func (c *sseClient) sendRequest(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.tracker.nextRequestID()
	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resultCh := make(chan *rpcResponse, 1)
	call := &pendingCall{
		id:     id,
		result: resultCh,
		done:   ctx.Done(),
	}
	c.tracker.register(call)

	if err := c.postJSON(ctx, body); err != nil {
		c.tracker.remove(id)
		return nil, err
	}

	// Response comes via SSE event stream, not POST response body.
	// Wait for the tracker to resolve via the SSE read loop.
	select {
	case resp := <-resultCh:
		if resp == nil {
			return nil, fmt.Errorf("connection closed")
		}
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *sseClient) IsAlive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.alive
}

func (c *sseClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.alive = false
	c.tracker.cancelAll()
	if c.stop != nil {
		c.stop()
	}
	return nil
}

func (c *sseClient) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	return c.sendRequest(ctx, method, params)
}

// Notify POST 一条 notification（无 id，不等待 SSE 响应，服务端以 202 Accepted 确认）。
func (c *sseClient) Notify(ctx context.Context, method string, params any) error {
	data, err := json.Marshal(rpcRequest{JSONRPC: "2.0", Method: method, Params: params})
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}
	return c.postJSON(ctx, data)
}

// ── HTTPClient ──────────────────────────────────────────────────────────────

// HTTPClient communicates with an MCP server via standard HTTP POST
// (Streamable HTTP：initialize 响应的 Mcp-Session-Id 会在后续请求中回传)。
type httpClient struct {
	endpoint   string
	headers    map[string]string
	creds      map[string]string
	httpClient *http.Client
	tracker    *rpcTracker
	sessionID  string // initialize 响应下发的 Mcp-Session-Id
	mu         sync.Mutex
	alive      bool
}

func newHTTPClient(cfg ServerConfig, creds map[string]string) (*httpClient, error) {
	return &httpClient{
		endpoint:   cfg.URL,
		headers:    cfg.Headers,
		creds:      creds,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		tracker:    newRPCTracker(),
	}, nil
}

func (c *httpClient) Connect(ctx context.Context) error {
	result, err := c.sendRequest(ctx, "initialize", initializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities:    clientCapabilities{},
		ClientInfo:      clientInfo{Name: "mindx", Version: "1.0"},
	})
	if err != nil {
		return fmt.Errorf("initialize handshake: %w", err)
	}
	var initResp initializeResult
	if err := json.Unmarshal(result, &initResp); err != nil {
		return fmt.Errorf("parse initialize result: %w", err)
	}
	// 协议要求：initialize 响应后、其他请求前发送 notifications/initialized
	if err := c.Notify(ctx, "notifications/initialized", nil); err != nil {
		return fmt.Errorf("send initialized notification: %w", err)
	}

	c.mu.Lock()
	c.alive = true
	c.mu.Unlock()
	return nil
}

func (c *httpClient) sendRequest(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.tracker.nextRequestID()
	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	applyHeaders(httpReq, c.headers, c.creds)
	// Streamable HTTP：initialize 之后服务端下发的 session id 必须回传，否则后续请求被拒绝
	c.mu.Lock()
	if c.sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", c.sessionID)
	}
	c.mu.Unlock()

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer httpResp.Body.Close()

	// 捕获服务端下发的 session id
	if sid := httpResp.Header.Get("Mcp-Session-Id"); sid != "" {
		c.mu.Lock()
		c.sessionID = sid
		c.mu.Unlock()
	}

	// Streamable HTTP 允许服务端以 SSE 流返回响应：取流中首个 data 行按 JSON 解析
	var respBody []byte
	if strings.Contains(httpResp.Header.Get("Content-Type"), "text/event-stream") {
		scanner := bufio.NewScanner(httpResp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if data, ok := strings.CutPrefix(line, "data:"); ok {
				respBody = []byte(strings.TrimSpace(data))
				break
			}
		}
		if respBody == nil {
			return nil, fmt.Errorf("响应流中没有 data 事件")
		}
	} else {
		var err2 error
		respBody, err2 = io.ReadAll(httpResp.Body)
		if err2 != nil {
			return nil, fmt.Errorf("read response: %w", err2)
		}
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, rpcResp.Error
	}
	return rpcResp.Result, nil
}

func (c *httpClient) IsAlive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.alive
}

func (c *httpClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.alive = false
	c.tracker.cancelAll()
	return nil
}

func (c *httpClient) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	return c.sendRequest(ctx, method, params)
}

// Notify POST 一条 notification（无 id，不等待响应，服务端以 202 Accepted 确认）。
func (c *httpClient) Notify(ctx context.Context, method string, params any) error {
	data, err := json.Marshal(rpcRequest{JSONRPC: "2.0", Method: method, Params: params})
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	applyHeaders(httpReq, c.headers, c.creds)
	c.mu.Lock()
	if c.sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", c.sessionID)
	}
	c.mu.Unlock()

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode >= 300 {
		return fmt.Errorf("通知被拒绝: %d", httpResp.StatusCode)
	}
	_, _ = io.Copy(io.Discard, httpResp.Body)
	return nil
}

// ── Convenience methods (shared across all clients) ─────────────────────────

// ToolsList calls tools/list on the MCP server and returns discovered tools.
func ToolsList(ctx context.Context, client MCPClient) ([]toolDef, error) {
	result, err := client.Call(ctx, "tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("tools/list: %w", err)
	}
	var resp toolsListResult
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse tools/list: %w", err)
	}
	return resp.Tools, nil
}

// ToolsCall calls tools/call on the MCP server and returns the text result.
func ToolsCall(ctx context.Context, client MCPClient, toolName string, args map[string]any) (string, error) {
	result, err := client.Call(ctx, "tools/call", toolsCallParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		return "", fmt.Errorf("tools/call %q: %w", toolName, err)
	}
	var resp toolsCallResult
	if err := json.Unmarshal(result, &resp); err != nil {
		return "", fmt.Errorf("parse tools/call: %w", err)
	}
	if resp.IsError {
		var msg string
		for _, block := range resp.Content {
			if block.Type == "text" {
				msg += block.Text
			}
		}
		return "", fmt.Errorf("tool error: %s", msg)
	}
	var output string
	for _, block := range resp.Content {
		if block.Type == "text" {
			output += block.Text
		}
	}
	return output, nil
}
