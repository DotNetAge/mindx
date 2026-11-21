package gateway

import "encoding/json"

// ---------------------------------------------------------------------------
// JSON-RPC 2.0 Message Types
// ---------------------------------------------------------------------------

// Request represents a JSON-RPC 2.0 request.
// A Request has an ID and expects a Response with the same ID.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`              // string or number, nil for notifications
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Notification represents a JSON-RPC 2.0 notification.
// Notifications have no ID and expect no response.
type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Error represents a JSON-RPC 2.0 error.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Standard JSON-RPC 2.0 error codes
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// AuthFailed 认证失败（-32001，服务端自定义区段 -32000..-32099）：
// 首条消息认证未通过或连接凭证无效。
const AuthFailed = -32001

// IsNotification returns true if the message is a notification (no ID).
func IsNotification(raw json.RawMessage) bool {
	var msg struct {
		ID *json.RawMessage `json:"id"`
	}
	if json.Unmarshal(raw, &msg) != nil {
		return false
	}
	return msg.ID == nil
}

// HasResultOrError returns true if the message is a response (has result or error).
func HasResultOrError(raw json.RawMessage) bool {
	var msg struct {
		Result *json.RawMessage `json:"result"`
		Error  *Error           `json:"error"`
	}
	if json.Unmarshal(raw, &msg) != nil {
		return false
	}
	return msg.Result != nil || msg.Error != nil
}

// ---------------------------------------------------------------------------
// Command Metadata — Standard model shared between server and client
// ---------------------------------------------------------------------------

// CommandScope determines where a command is executed.
type CommandScope string

const (
	// ScopeLocal means the command runs only on the client side.
	ScopeLocal CommandScope = "local"
	// ScopeRemote means the command runs only on the server side.
	ScopeRemote CommandScope = "remote"
	// ScopeBoth means the command has implementations on both sides.
	ScopeBoth CommandScope = "both"
)

// CommandMeta is the standard metadata model for a slash command.
// It replaces the legacy CommandInfo type.
type CommandMeta struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    string         `json:"category,omitempty"`    // "agent" | "system" | "ui"
	Scope       CommandScope   `json:"scope"`                 // local | remote | both
	Example     string         `json:"example,omitempty"`     // usage example
	Params      string         `json:"params,omitempty"`      // parameter description
}
