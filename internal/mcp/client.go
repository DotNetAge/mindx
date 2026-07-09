package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
		io.Copy(io.Discard, stderr)
	}()

	// initialize handshake
	if err := c.handshake(ctx); err != nil {
		c.closeUnlocked()
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
	return nil
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
		c.cmd.Process.Kill()
	}
	c.running.Wait()
	return nil
}

func (c *stdioClient) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	return c.sendRequest(ctx, method, params)
}

// ── SSEClient ───────────────────────────────────────────────────────────────

// SSEClient communicates with an MCP server using SSE for server→client
// and HTTP POST for client→server messages.
type sseClient struct {
	sseURL     string
	postURL    string
	httpClient *http.Client
	tracker    *rpcTracker

	mu    sync.Mutex
	alive bool
	stop  context.CancelFunc
}

func newSSEClient(cfg ServerConfig, creds map[string]string) (*sseClient, error) {
	return &sseClient{
		sseURL:     cfg.URL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		tracker:    newRPCTracker(),
	}, nil
}

func (c *sseClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	c.stop = cancel

	// Open SSE connection
	req, err := http.NewRequestWithContext(ctx, "GET", c.sseURL, nil)
	if err != nil {
		return fmt.Errorf("create SSE request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("connect SSE: %w", err)
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
		return fmt.Errorf("failed to receive endpoint from SSE server")
	}

	c.postURL = postEndpoint

	// Start background goroutine to receive SSE events
	go c.readSSE(ctx, scanner, resp.Body)

	// initialize handshake
	if err := c.handshake(ctx); err != nil {
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
	return nil
}

func (c *sseClient) readSSE(ctx context.Context, scanner *bufio.Scanner, body io.ReadCloser) {
	defer body.Close()

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

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.postURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("create POST request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("POST request: %w", err)
	}
	httpResp.Body.Close()

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

// ── HTTPClient ──────────────────────────────────────────────────────────────

// HTTPClient communicates with an MCP server via standard HTTP POST.
type httpClient struct {
	endpoint   string
	httpClient *http.Client
	tracker    *rpcTracker
	mu         sync.Mutex
	alive      bool
}

func newHTTPClient(cfg ServerConfig, creds map[string]string) (*httpClient, error) {
	return &httpClient{
		endpoint:   cfg.URL,
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

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
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
