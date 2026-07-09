package mcp

import (
	"encoding/json"
	"fmt"
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

// rpcNotification is a JSON-RPC 2.0 notification (no id).
type rpcNotification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
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

// rpcTracker handles request ID generation and response routing.
// Thread-safe; used by MCPClient implementations.
type rpcTracker struct {
	mu      sync.Mutex
	nextID  atomic.Int64
	pending map[any]*pendingCall
}

func newRPCTracker() *rpcTracker {
	return &rpcTracker{
		pending: make(map[any]*pendingCall),
	}
}

func (t *rpcTracker) nextRequestID() any {
	return t.nextID.Add(1)
}

func (t *rpcTracker) register(call *pendingCall) {
	t.mu.Lock()
	t.pending[call.id] = call
	t.mu.Unlock()
}

func (t *rpcTracker) resolve(resp *rpcResponse) {
	t.mu.Lock()
	call, ok := t.pending[resp.ID]
	if ok {
		delete(t.pending, resp.ID)
	}
	t.mu.Unlock()
	if ok {
		select {
		case call.result <- resp:
		default:
		}
	}
}

func (t *rpcTracker) cancelAll() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, call := range t.pending {
		close(call.result)
	}
	t.pending = make(map[any]*pendingCall)
}
