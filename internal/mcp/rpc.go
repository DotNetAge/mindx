package mcp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
)

// ── JSON-RPC 2.0 message types ──────────────────────────────────────────────

// rpcRequest is a JSON-RPC 2.0 request sent to an MCP server.
type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// rpcResponse is a JSON-RPC 2.0 response from an MCP server.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *rpcError) Error() string {
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

// ── MCP protocol types ──────────────────────────────────────────────────────

// initialize params/result
type initializeParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    clientCapabilities `json:"capabilities"`
	ClientInfo      clientInfo         `json:"clientInfo"`
}

type clientCapabilities struct{}

type clientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type initializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    serverCapabilities `json:"capabilities"`
	ServerInfo      serverInfo         `json:"serverInfo"`
}

type serverCapabilities struct {
	Tools *toolsCapability `json:"tools,omitempty"`
}

type toolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// tools/list
type toolsListResult struct {
	Tools []toolDef `json:"tools"`
}

type toolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	// server 是运行时注入的所属 server 名，不参与 JSON 序列化。
	server string `json:"-"`
}

// tools/call
type toolsCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type toolsCallResult struct {
	Content []contentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

type contentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// ── Request/response tracking ───────────────────────────────────────────────

// pendingCall tracks a single outstanding JSON-RPC request.
type pendingCall struct {
	id     any
	result chan *rpcResponse
	done   <-chan struct{}
}

// normalizeID 把请求/响应 ID 规范化为统一字符串形式。
// 关键：JSON 响应的数字 ID 反序列化后是 float64，而本端生成的请求 ID 是 int64，
// map[any] 按 interface 全等比较时 int64(7) != float64(7)——必须归一化后才能匹配。
func normalizeID(v any) string {
	switch n := v.(type) {
	case string:
		return n
	case json.Number:
		return n.String()
	case int64:
		return strconv.FormatInt(n, 10)
	case int:
		return strconv.Itoa(n)
	case float64:
		return strconv.FormatFloat(n, 'f', -1, 64)
	default:
		return fmt.Sprint(v)
	}
}

// rpcTracker handles request ID generation and response routing.
// Thread-safe; used by MCPClient implementations.
type rpcTracker struct {
	mu      sync.Mutex
	nextID  atomic.Int64
	pending map[string]*pendingCall // key: normalizeID 后的请求 ID
}

func newRPCTracker() *rpcTracker {
	return &rpcTracker{
		pending: make(map[string]*pendingCall),
	}
}

func (t *rpcTracker) nextRequestID() any {
	return t.nextID.Add(1)
}

func (t *rpcTracker) register(call *pendingCall) {
	t.mu.Lock()
	t.pending[normalizeID(call.id)] = call
	t.mu.Unlock()
}

func (t *rpcTracker) resolve(resp *rpcResponse) {
	key := normalizeID(resp.ID)
	t.mu.Lock()
	call, ok := t.pending[key]
	if ok {
		delete(t.pending, key)
	}
	t.mu.Unlock()
	if ok {
		select {
		case call.result <- resp:
		default:
		}
	}
}

// remove 注销未得到响应的挂起调用（POST 发送失败等场景，防止 pending 泄漏）。
func (t *rpcTracker) remove(id any) {
	t.mu.Lock()
	delete(t.pending, normalizeID(id))
	t.mu.Unlock()
}

func (t *rpcTracker) cancelAll() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, call := range t.pending {
		close(call.result)
	}
	t.pending = make(map[string]*pendingCall)
}
