package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// Client Configuration
// ---------------------------------------------------------------------------

type Client struct {
	addr      string
	conn      *websocket.Conn
	send      chan []byte
	done      chan struct{}
	once      sync.Once
	sendMu    sync.Mutex
	respMu    sync.RWMutex
	pending   map[any]chan *Response // ID → response channel
	running   bool
	clientID  string
	sessionID string

	// Notification handlers
	handlerMu sync.RWMutex
	handlers  map[string]NotificationHandler // method → handler

	// Connection state
	stateMu       sync.RWMutex
	state         ConnectionState
	onStateChange func(oldState, newState ConnectionState)
}

// NewClient creates a new JSON-RPC WebSocket client.
func NewClient(addr string) *Client {
	return &Client{
		addr:     addr,
		pending:  make(map[any]chan *Response),
		handlers: make(map[string]NotificationHandler),
		send:     make(chan []byte, 256),
		done:     make(chan struct{}),
		state:    StateDisconnected,
	}
}

// ---------------------------------------------------------------------------
// Connection Management
// ---------------------------------------------------------------------------

// Connect establishes the WebSocket connection.
func (c *Client) Connect(ctx context.Context) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	header := http.Header{}
	conn, _, err := dialer.DialContext(ctx, c.addr, header)
	if err != nil {
		return fmt.Errorf("websocket connect failed: %w", err)
	}

	c.conn = conn
	c.running = true
	c.setState(StateConnected)

	go c.readLoop()
	go c.writeLoop()

	slog.Info("connected to gateway", "addr", c.addr)
	return nil
}

// ConnectSync connects without requiring a context (for backward compatibility).
func (c *Client) ConnectSync() error {
	return c.Connect(context.Background())
}

func (c *Client) Close() error {
	var err error

	// Close 会被两条路径并发调用（调用方 defer 与 readLoop 退出时的 defer），
	// 整个清理过程必须幂等：done 只允许关闭一次，重复 close 会 panic；
	// conn 重复关闭虽不 panic 但会返回 use of closed connection 错误。
	c.once.Do(func() {
		if c.conn == nil {
			return
		}
		c.setState(StateDisconnected)
		c.running = false

		// Signal all goroutines to exit
		close(c.done)

		// Cancel all pending requests
		c.respMu.Lock()
		for id, ch := range c.pending {
			close(ch)
			delete(c.pending, id)
		}
		c.respMu.Unlock()

		err = c.conn.Close()
	})

	return err
}

func (c *Client) IsConnected() bool {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.running && c.state == StateConnected
}

func (c *Client) Port() int {
	parts := strings.Split(c.addr, ":")
	if len(parts) == 2 {
		var port int
		fmt.Sscanf(parts[1], "%d", &port)
		return port
	}
	return 0
}

// ---------------------------------------------------------------------------
// JSON-RPC Methods
// ---------------------------------------------------------------------------

// Call sends a request and waits for a response.
func (c *Client) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := uuid.New().String()
	ch := make(chan *Response, 1)

	c.respMu.Lock()
	c.pending[id] = ch
	c.respMu.Unlock()

	defer func() {
		c.respMu.Lock()
		delete(c.pending, id)
		c.respMu.Unlock()
	}()

	var paramsBytes json.RawMessage
	if params != nil {
		var err error
		paramsBytes, err = json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal params failed: %w", err)
		}
	}

	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  paramsBytes,
	}

	c.sendMu.Lock()
	if err := c.conn.WriteJSON(req); err != nil {
		c.sendMu.Unlock()
		return nil, fmt.Errorf("send failed: %w", err)
	}
	c.sendMu.Unlock()

	slog.Debug("call", "method", method, "id", id)

	select {
	case resp := <-ch:
		if resp == nil {
			return nil, fmt.Errorf("connection closed")
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		slog.Debug("response received", "id", id, "method", method)
		return resp.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, fmt.Errorf("connection closed")
	}
}

// Notify sends a notification (no response expected).
func (c *Client) Notify(method string, params any) error {
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("marshal params failed: %w", err)
	}

	notif := Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsBytes,
	}

	c.sendMu.Lock()
	err = c.conn.WriteJSON(notif)
	c.sendMu.Unlock()

	if err != nil {
		return fmt.Errorf("notify failed: %w", err)
	}

	slog.Debug("notify", "method", method)
	return nil
}

// On registers a handler for server-initiated notifications.
func (c *Client) On(method string, handler NotificationHandler) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.handlers[method] = handler
}

// ---------------------------------------------------------------------------
// Legacy Compatibility API (for mindx migration)
// ---------------------------------------------------------------------------

// SendCommand sends a command to the server and returns the raw response.
// Each command is called as a direct JSON-RPC method (e.g., "agents", "help").
func (c *Client) SendCommand(name string, args string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	params := map[string]string{
		"args": args,
	}

	result, err := c.Call(ctx, name, params)
	if err != nil {
		return "", err
	}

	return string(result), nil
}

// Done returns a channel that is closed when the client connection is dropped.
// This allows consumers (e.g., TUI wait loops) to detect disconnection
// and unblock waiting goroutines instead of hanging forever.
func (c *Client) Done() <-chan struct{} {
	return c.done
}

// OnReceived registers a handler for all incoming messages (legacy).
func (c *Client) OnReceived(handler func(message string)) {
	c.On("message", func(ctx context.Context, params json.RawMessage) {
		var msg struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(params, &msg); err != nil {
			slog.Warn("failed to parse message notification", "error", err)
			return
		}
		handler(msg.Text)
	})
}

// OnResponse registers a typed response handler (legacy compatibility).
// This dispatches based on the ResponseEnvelope.Type field.
func (c *Client) OnResponse(responseType ResponseType, handler func(env *ResponseEnvelope, orig *Message)) {
	c.On(string(responseType), func(ctx context.Context, params json.RawMessage) {
		var env ResponseEnvelope
		if err := json.Unmarshal(params, &env); err != nil {
			slog.Warn("failed to parse response envelope", "error", err)
			return
		}
		handler(&env, nil)
	})
}

// GetCommands returns the list of registered commands with metadata.
// This calls the "command.list" method on the server.
func (c *Client) GetCommands() ([]CommandMeta, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := c.Call(ctx, "command.list", nil)
	if err != nil {
		return nil, err
	}

	var cmds []CommandMeta
	if err := json.Unmarshal(result, &cmds); err != nil {
		return nil, err
	}
	return cmds, nil
}

// OnResponseFallback registers a fallback handler for untyped responses (for backward compatibility).
func (c *Client) OnResponseFallback(handler func(env *ResponseEnvelope, orig *Message)) {
	// This is a no-op in the new JSON-RPC client.
	// Use On() for specific response types instead.
}

// Send sends a text message to a specific client (server-side compatibility).
// This is a no-op on the client side.
func (c *Client) Send(to string, message string) {
	c.Notify("message", map[string]string{"to": to, "text": message})
}

// ---------------------------------------------------------------------------
// Read Loop
// ---------------------------------------------------------------------------

func (c *Client) readLoop() {
	defer c.Close()

	c.conn.SetReadLimit(10 << 20)
	c.conn.SetReadDeadline(time.Now().Add(10 * time.Minute))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(10 * time.Minute))
		return nil
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if c.running {
				slog.Debug("read error", "error", err)
			}
			return
		}

		slog.Debug("recv", "size", len(raw), "preview", truncatePreview(string(raw), 200))

		// A single WebSocket message may contain multiple JSON-RPC messages
		// separated by newlines (sent by writePump batching).
		lines := strings.Split(string(raw), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// Determine message type
			if IsNotification([]byte(line)) {
				c.handleNotification([]byte(line))
			} else if HasResultOrError([]byte(line)) {
				c.handleResponse([]byte(line))
			} else {
				slog.Warn("unknown message type", "raw", line)
			}
		}
	}
}

func (c *Client) handleResponse(raw []byte) {
	var resp Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		slog.Warn("failed to parse response", "error", err)
		return
	}

	c.respMu.RLock()
	ch, ok := c.pending[resp.ID]
	c.respMu.RUnlock()

	if ok {
		select {
		case ch <- &resp:
		default:
		}
	}
}

func (c *Client) handleNotification(raw []byte) {
	var notif Notification
	if err := json.Unmarshal(raw, &notif); err != nil {
		slog.Warn("failed to parse notification", "error", err)
		return
	}

	c.handlerMu.RLock()
	handler, ok := c.handlers[notif.Method]
	c.handlerMu.RUnlock()

	if ok {
		handler(context.Background(), notif.Params)
	} else {
		slog.Debug("no handler for notification", "method", notif.Method)
	}
}

// ---------------------------------------------------------------------------
// Write Loop
// ---------------------------------------------------------------------------

func (c *Client) writeLoop() {
	pingPeriod := 54 * time.Second
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case data, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(data)
			for i := 0; i < len(c.send); i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}
			w.Close()
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if c.conn.WriteMessage(websocket.PingMessage, nil) != nil {
				return
			}
		case <-c.done:
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Connection State
// ---------------------------------------------------------------------------

func (c *Client) setState(state ConnectionState) {
	c.stateMu.Lock()
	old := c.state
	c.state = state
	c.stateMu.Unlock()

	if c.onStateChange != nil && old != state {
		c.onStateChange(old, state)
	}
}

func (c *Client) OnStateChange(fn func(oldState, newState ConnectionState)) {
	c.onStateChange = fn
}
