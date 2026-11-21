package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/DotNetAge/gort/pkg/channel"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var (
	ErrAlreadyRunning      = fmt.Errorf("server is already running")
	ErrNotRunning          = fmt.Errorf("server is not running")
	ErrSessionNotFound     = fmt.Errorf("session not found")
	ErrSessionLimitReached = fmt.Errorf("session limit reached")
)

// ---------------------------------------------------------------------------
// Server Configuration
// ---------------------------------------------------------------------------

type WSConfig struct {
	AllowedOrigins []string
	AllowAllInDev  bool
}

func matchOrigin(origin, pattern string) bool {
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(origin, prefix)
	}
	return origin == pattern
}

func defaultWSConfig() *WSConfig {
	return &WSConfig{
		AllowedOrigins: []string{
			"http://localhost:*",
			"http://127.0.0.1:*",
			"ws://localhost:*",
			"ws://127.0.0.1:*",
			// Wails 壳（mindx-app）的 WebView 页面 origin：dev 带端口、生产无端口
			"wails://wails.localhost:*",
			"wails://wails.localhost",
			// Allow common LAN subnets
			"http://192.168.*:*",
			"ws://192.168.*:*",
			"http://10.*:*",
			"ws://10.*:*",
			"http://172.16.*:*",
			"ws://172.16.*:*",
			"http://172.17.*:*",
			"ws://172.17.*:*",
			"http://172.18.*:*",
			"ws://172.18.*:*",
			"http://172.19.*:*",
			"ws://172.19.*:*",
			"http://172.20.*:*",
			"ws://172.20.*:*",
			"http://172.21.*:*",
			"ws://172.21.*:*",
			"http://172.22.*:*",
			"ws://172.22.*:*",
			"http://172.23.*:*",
			"ws://172.23.*:*",
			"http://172.24.*:*",
			"ws://172.24.*:*",
			"http://172.25.*:*",
			"ws://172.25.*:*",
			"http://172.26.*:*",
			"ws://172.26.*:*",
			"http://172.27.*:*",
			"ws://172.27.*:*",
			"http://172.28.*:*",
			"ws://172.28.*:*",
			"http://172.29.*:*",
			"ws://172.29.*:*",
			"http://172.30.*:*",
			"ws://172.30.*:*",
			"http://172.31.*:*",
			"ws://172.31.*:*",
		},
		AllowAllInDev: true,
	}
}

func WithWSConfig(cfg *WSConfig) Option {
	return func(s *Server) { s.wsConfig = cfg }
}

func newUpgrader(cfg *WSConfig) *websocket.Upgrader {
	if cfg == nil {
		cfg = defaultWSConfig()
	}
	return &websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")

			// Allow empty Origin — non-browser clients (Go, curl, etc.)
			// do not send an Origin header.
			if origin == "" {
				return true
			}

			if cfg.AllowAllInDev && os.Getenv("GO_ENV") == "development" {
				slog.Debug("origin allowed (dev mode)", "origin", origin)
				return true
			}
			for _, allowed := range cfg.AllowedOrigins {
				if matchOrigin(origin, allowed) {
					return true
				}
			}
			slog.Warn("rejected WebSocket connection",
				"origin", origin,
				"allowed", cfg.AllowedOrigins,
			)
			return false
		},
	}
}

type Option func(*Server)

func WithAddr(addr string) Option {
	return func(s *Server) { s.addr = addr }
}

func WithPort(port int) Option {
	return func(s *Server) { s.addr = fmt.Sprintf(":%d", port) }
}

func WithPath(path string) Option {
	return func(s *Server) { s.path = path }
}

func WithHandler(h MessageHandler) Option {
	return func(s *Server) { s.handler = h }
}

func WithSessionTimeout(d time.Duration) Option {
	return func(s *Server) { s.sessionTimeout = d }
}

func WithHeartbeat(cfg *HeartbeatConfig) Option {
	return func(s *Server) {
		s.heartbeatCfg = cfg
		s.hbMonitor = newHeartbeatMonitor(cfg)
	}
}

func WithDisconnectHandler(h func(clientID string)) Option {
	return func(s *Server) { s.disconnectHandler = h }
}

// WithConnectHandler 设置新客户端建立连接时的回调。
// 用于断连恢复场景：客户端重连后可据此补发在途会话的挂起状态（如权限请求）。
func WithConnectHandler(h func(clientID string)) Option {
	return func(s *Server) { s.connectHandler = h }
}

// WithAuthenticator 设置 WebSocket 升级前的握手鉴权钩子（可选）。
// 返回非 nil 时拒绝升级（HTTP 401，错误文案回给连接方），nil 放行；
// 未设置时不做任何校验（保持向后兼容）。
func WithAuthenticator(h func(*http.Request) error) Option {
	return func(s *Server) { s.authenticator = h }
}

// WithMessageAuthenticator 启用首条消息认证（可选）。
// 背景：浏览器端 WebSocket API 不支持自定义升级请求头，凭证无法走
// Authorization 头，只能连上后以首条消息携带（OWASP WebSocket 备选方案）。
// 启用后：升级鉴权钩子拒绝的连接不再立即 401，而是升级为「待认证」连接，
// 首条消息交给钩子校验——nil 放行进入正常消息分发（计时停止），非 nil
// 把失败原因回给连接方并断开；超时未认证的连接强制断开（防连接洪泛
// 占满握手资源）。升级鉴权已放行的连接（私网/环回）不受影响。
func WithMessageAuthenticator(h func(clientID string, msg []byte) error, timeout time.Duration) Option {
	return func(s *Server) {
		s.msgAuthenticator = h
		s.msgAuthTimeout = timeout
	}
}

// ---------------------------------------------------------------------------
// Handler Types
// ---------------------------------------------------------------------------

type MessageHandler func(msg *Message)

// MethodHandler handles JSON-RPC method calls.
type MethodHandler func(ctx context.Context, params json.RawMessage) (any, error)

// NotificationHandler handles JSON-RPC notifications.
type NotificationHandler func(ctx context.Context, params json.RawMessage)

// ---------------------------------------------------------------------------
// Server
// ---------------------------------------------------------------------------

type client struct {
	id       string
	send     chan []byte
	conn     *websocket.Conn
	ctx      context.Context
	done     context.CancelFunc
	sessions *SessionManager
	closed   sync.Once // ensures cleanup runs once
	// pendingAuth 首条消息认证待定态：升级鉴权未过但启用消息认证时为 true，
	// 首条消息经 msgAuthenticator 校验通过后置 false（仅 readPump 单协程读写）。
	pendingAuth bool
	authTimer   *time.Timer // 认证超时计时器（认证成功即停）

	// sendMu 保护 send 通道的关闭与写入：客户端断连时 close(c.send)，
	// 若此时仍有 handler 协程回写结果，会触发 "send on closed channel" panic，
	// 直接拖垮整个 daemon。写入与关闭必须互斥。
	sendMu     sync.RWMutex
	sendClosed bool

	mu        sync.RWMutex
	pending   map[any]chan *Response // ID → response channel
	handlers  map[string]NotificationHandler
	handlerMu sync.RWMutex
}

type Server struct {
	addr              string
	path              string
	handler           MessageHandler
	sessionTimeout    time.Duration
	heartbeatCfg      *HeartbeatConfig
	hbMonitor         *HeartbeatMonitor
	wsConfig          *WSConfig
	upgrader          *websocket.Upgrader
	disconnectHandler func(clientID string)
	connectHandler    func(clientID string)
	authenticator     func(*http.Request) error // 升级前握手鉴权（可选，nil = 不校验）
	msgAuthenticator  func(clientID string, msg []byte) error // 首条消息认证（可选，nil = 不启用）
	msgAuthTimeout    time.Duration

	mu       sync.RWMutex
	server   *http.Server
	running  bool
	clients  map[string]*client
	register chan *client
	channels sync.Map
	startErr chan error

	methods   map[string]MethodHandler
	methodsMu sync.RWMutex

	commands    map[string]MethodHandler
	commandsMu  sync.RWMutex
	commandMeta map[string]CommandMeta
}

// CommandContext provides context for command execution.
type CommandContext struct {
	ClientID  string
	SessionID string
	Args      string
	server    *Server
}

// ---- Context helpers ----

type clientIDKey struct{}
type sessionIDKey struct{}

// WithClientID embeds the client ID into the context so command handlers
// (wrapped by RegisterCommand) can extract it for RespondWithType.
func WithClientID(ctx context.Context, clientID string) context.Context {
	return context.WithValue(ctx, clientIDKey{}, clientID)
}

// ClientIDFromContext extracts the client ID previously stored by WithClientID.
func ClientIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(clientIDKey{}).(string); ok {
		return id
	}
	return ""
}

// WithCtxSessionID embeds the session ID into the context.
func WithCtxSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDKey{}, sessionID)
}

// SessionIDFromContext extracts the session ID previously stored by WithCtxSessionID.
func SessionIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(sessionIDKey{}).(string); ok {
		return id
	}
	return ""
}

// CommandHandler is the signature for command handlers (backward compatibility alias).
type CommandHandler = func(ctx *CommandContext) (any, error)

// RespondWithType sends a typed response to the client (includes session_id).
func (ctx *CommandContext) RespondWithType(t ResponseType, title string, data interface{}) {
	ctx.server.SendResponse(ctx.ClientID, t, title, data, WithSessionID(ctx.SessionID))
}

// Server returns the underlying gateway server.
func (ctx *CommandContext) Server() *Server {
	return ctx.server
}

// RegisterCommand registers a command with metadata.
// The command is automatically registered as a JSON-RPC method with the same name.
func (s *Server) RegisterCommand(meta CommandMeta, handler func(ctx *CommandContext) (any, error)) {
	s.commandsMu.Lock()
	defer s.commandsMu.Unlock()

	if s.commands == nil {
		s.commands = make(map[string]MethodHandler)
		s.commandMeta = make(map[string]CommandMeta)
	}

	s.commands[meta.Name] = func(ctx context.Context, params json.RawMessage) (any, error) {
		var args struct {
			Args string `json:"args"`
		}
		if params != nil {
			if err := json.Unmarshal(params, &args); err != nil {
				return nil, fmt.Errorf("invalid params: %w", err)
			}
		}

		cmdCtx := &CommandContext{
			ClientID:  ClientIDFromContext(ctx),
			SessionID: SessionIDFromContext(ctx),
			Args:      args.Args,
			server:    s,
		}
		return handler(cmdCtx)
	}

	s.commandMeta[meta.Name] = meta

	s.methodsMu.Lock()
	s.methods[meta.Name] = s.commands[meta.Name]
	s.methodsMu.Unlock()
}

// CommandList returns all registered command metadata.
func (s *Server) CommandList() []CommandMeta {
	s.commandsMu.RLock()
	defer s.commandsMu.RUnlock()

	result := make([]CommandMeta, 0, len(s.commandMeta))
	for _, meta := range s.commandMeta {
		result = append(result, meta)
	}
	return result
}

func New(opts ...Option) *Server {
	s := &Server{
		addr:           ":8081",
		path:           "/ws",
		sessionTimeout: 30 * time.Minute,
		clients:        make(map[string]*client),
		register:       make(chan *client),
		methods:        make(map[string]MethodHandler),
		commands:       make(map[string]MethodHandler),
		commandMeta:    make(map[string]CommandMeta),
	}

	// Register built-in JSON-RPC methods
	s.methods["command.list"] = s.handleCommandList

	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Server) handleCommandList(ctx context.Context, params json.RawMessage) (any, error) {
	return s.CommandList(), nil
}

// RegisterMethod registers a JSON-RPC method handler.
func (s *Server) RegisterMethod(method string, handler MethodHandler) {
	s.methodsMu.Lock()
	defer s.methodsMu.Unlock()
	s.methods[method] = handler
}

// OnNotification registers a handler for server-initiated notifications on a client.
func (s *Server) OnNotification(clientID, method string, handler NotificationHandler) {
	s.mu.RLock()
	c, ok := s.clients[clientID]
	s.mu.RUnlock()
	if !ok {
		return
	}
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	if c.handlers == nil {
		c.handlers = make(map[string]NotificationHandler)
	}
	c.handlers[method] = handler
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

func (s *Server) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return ErrAlreadyRunning
	}
	s.running = true
	s.mu.Unlock()

	s.upgrader = newUpgrader(s.wsConfig)
	go s.hubLoop()

	mux := http.NewServeMux()
	mux.HandleFunc(s.path, s.handleWS)
	s.server = &http.Server{Addr: s.addr, Handler: mux}

	s.startErr = make(chan error, 1)
	go func() {
		slog.Info("gateway server starting", "addr", s.addr)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("gateway server exited unexpectedly", "error", err)
			select {
			case s.startErr <- err:
			default:
			}
		}
		close(s.startErr)
		slog.Info("gateway server stopped")
	}()

	addr := s.addr
	if strings.HasPrefix(addr, ":") {
		addr = "localhost" + addr
	}
	const maxProbe = 2 * time.Second
	probeTicker := time.NewTicker(20 * time.Millisecond)
	defer probeTicker.Stop()
	deadline := time.After(maxProbe)

	for {
		select {
		case err := <-s.startErr:
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
			return fmt.Errorf("gateway start failed: %w", err)
		case <-deadline:
			return fmt.Errorf("gateway start timed out after %v: server did not become reachable on %s", maxProbe, addr)
		case <-probeTicker.C:
			conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
			if err == nil {
				conn.Close()
				slog.Info("gateway server ready", "addr", addr)
				return nil
			}
		}
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return ErrNotRunning
	}
	s.running = false
	s.mu.Unlock()

	close(s.register)

	s.mu.Lock()
	for id, c := range s.clients {
		c.sessions.Close()
		c.sessions.cleanupClient(c.id)
		delete(s.clients, id)
	}
	s.mu.Unlock()

	if s.server != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		return s.server.Shutdown(shutdownCtx)
	}
	return nil
}

func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

func (s *Server) hubLoop() {
	for c := range s.register {
		s.mu.Lock()
		if !s.running {
			s.mu.Unlock()
			return
		}
		s.clients[c.id] = c
		// 连接数必须在锁内读取：锁外 len(s.clients) 与 removeClient 的
		// map 写构成数据竞争（race detector 实测报警）
		total := len(s.clients)
		s.mu.Unlock()

		if s.connectHandler != nil {
			s.connectHandler(c.id)
		}

		if s.hbMonitor != nil {
			s.hbMonitor.setConnectionCount(total)
			s.hbMonitor.notifyStateChange(c.id, StateDisconnected.String(), StateConnected.String())
		}

		slog.Info("client connected", "id", c.id, "total", total)
	}
}

// ---------------------------------------------------------------------------
// WebSocket Handler
// ---------------------------------------------------------------------------

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	slog.Info("incoming ws request",
		"remote", r.RemoteAddr,
		"method", r.Method,
		"path", r.URL.Path,
		"origin", r.Header.Get("Origin"),
	)

	// 升级鉴权钩子：拒绝时默认 HTTP 401（错误文案直传连接方，不建立会话）；
	// 启用首条消息认证时转为「待认证」连接——升级放行，凭首条消息过闸。
	var pendingAuth bool
	if s.authenticator != nil {
		if err := s.authenticator(r); err != nil {
			if s.msgAuthenticator == nil {
				slog.Warn("ws handshake rejected by authenticator",
					"remote", r.RemoteAddr,
					"error", err,
				)
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}
			pendingAuth = true
		}
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("ws upgrade failed", "error", err)
		return
	}

	ctx, done := context.WithCancel(context.Background())
	c := &client{
		id:          uuid.New().String(),
		send:        make(chan []byte, 256),
		conn:        conn,
		ctx:         ctx,
		done:        done,
		sessions:    newSessionManager(s.sessionTimeout),
		pending:     make(map[any]chan *Response),
		pendingAuth: pendingAuth,
	}
	// 待认证连接限时：超时未认证即断开（认证成功由 dispatchMessage 停表）
	if pendingAuth && s.msgAuthTimeout > 0 {
		timeout := s.msgAuthTimeout
		client := c
		client.authTimer = time.AfterFunc(timeout, func() {
			slog.Warn("ws pending auth timed out, disconnecting",
				"client_id", client.id,
				"remote", r.RemoteAddr,
				"timeout", timeout,
			)
			_ = client.conn.Close()
		})
	}
	slog.Debug("ws client connected, starting pumps", "client_id", c.id, "pending_auth", pendingAuth)
	s.register <- c
	go s.writePump(c)
	go s.readPump(c)
}

// ---------------------------------------------------------------------------
// Read Pump
// ---------------------------------------------------------------------------

func (s *Server) readPump(c *client) {
	defer func() {
		slog.Debug("readPump exit", "client_id", c.id)
		c.done()
		c.sessions.Close()
		c.sessions.cleanupClient(c.id)
		s.removeClient(c.id)
		c.conn.Close()
		if s.hbMonitor != nil {
			s.hbMonitor.setConnectionCount(s.ClientCount())
			s.hbMonitor.notifyTimeout(c.id)
		}
	}()

	c.conn.SetReadLimit(10 << 20)
	readTimeout := 60 * time.Second
	if s.heartbeatCfg != nil {
		readTimeout = s.heartbeatCfg.ReadTimeout
	}
	c.conn.SetReadDeadline(time.Now().Add(readTimeout))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(readTimeout))
		if s.hbMonitor != nil {
			s.hbMonitor.recordPong()
		}
		return nil
	})

	slog.Debug("readPump started", "client_id", c.id)

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			slog.Debug("readPump read error",
				"client_id", c.id,
				"error", err,
			)
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Error("read error", "id", c.id, "error", err)
			}
			return
		}

		slog.Debug("readPump message",
			"client_id", c.id,
			"size", len(raw),
			"preview", truncatePreview(string(raw), 200),
		)

		lines := strings.Split(string(raw), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			s.dispatchMessage(c, []byte(line))
		}
	}
}

func (s *Server) dispatchMessage(c *client, raw []byte) {
	// 待认证连接：首条消息交认证钩子过闸，不进入常规分发
	//（通过则摘除待定态、停超时计时；失败则回告原因并断开）
	if c.pendingAuth {
		if s.msgAuthenticator == nil {
			return
		}
		if err := s.msgAuthenticator(c.id, raw); err != nil {
			slog.Warn("first-message auth failed",
				"client_id", c.id,
				"error", err,
			)
			s.sendToClient(c, &Response{
				JSONRPC: "2.0",
				ID:      nil,
				Error: &Error{
					Code:    AuthFailed,
					Message: err.Error(),
				},
			})
			_ = c.conn.Close()
			return
		}
		c.pendingAuth = false
		if c.authTimer != nil {
			c.authTimer.Stop()
		}
		slog.Debug("first-message auth passed", "client_id", c.id)
		return
	}

	// Check if it's a JSON-RPC message
	if !strings.HasPrefix(strings.TrimSpace(string(raw)), "{") {
		slog.Warn("non-JSON message rejected", "id", c.id)
		return
	}

	// Determine if it's a request/notification or response
	if HasResultOrError(raw) {
		// This is a response from client (e.g., to a server-initiated call)
		slog.Debug("dispatch response", "client_id", c.id)
		s.handleResponse(c, raw)
		return
	}

	// It's a request or notification from client
	if IsNotification(raw) {
		slog.Debug("dispatch notification", "client_id", c.id)
		s.handleNotification(c, raw)
	} else {
		slog.Debug("dispatch request", "client_id", c.id)
		s.handleRequest(c, raw)
	}
}

func (s *Server) handleRequest(c *client, raw []byte) {
	var req Request
	if err := json.Unmarshal(raw, &req); err != nil {
		slog.Warn("handleRequest: failed to parse", "error", err)
		s.sendToClient(c, Response{
			JSONRPC: "2.0",
			ID:      nil,
			Error:   &Error{Code: ParseError, Message: "Parse error"},
		})
		return
	}

	slog.Debug("handleRequest", "method", req.Method, "client_id", c.id)

	s.methodsMu.RLock()
	handler, ok := s.methods[req.Method]
	s.methodsMu.RUnlock()

	if !ok {
		slog.Debug("method not found", "method", req.Method)
		s.sendToClient(c, Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: MethodNotFound, Message: "Method not found: " + req.Method},
		})
		return
	}

	// Execute handler asynchronously
	go func() {
		slog.Debug("handler start", "method", req.Method)
		result, err := handler(WithClientID(c.ctx, c.id), req.Params)
		slog.Debug("handler done", "method", req.Method, "has_error", err != nil)
		if err != nil {
			slog.Warn("handler error", "method", req.Method, "error", err)
			s.sendToClient(c, Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &Error{Code: InternalError, Message: err.Error()},
			})
			return
		}

		resultBytes, err := json.Marshal(result)
		if err != nil {
			slog.Warn("handler marshal error", "method", req.Method, "error", err)
			s.sendToClient(c, Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &Error{Code: InternalError, Message: fmt.Sprintf("marshal result failed: %v", err)},
			})
			return
		}
		slog.Debug("handler response",
			"method", req.Method,
			"response_size", len(resultBytes),
		)
		s.sendToClient(c, Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  resultBytes,
		})
	}()
}

func (s *Server) handleNotification(c *client, raw []byte) {
	var notif Notification
	if err := json.Unmarshal(raw, &notif); err != nil {
		return
	}

	// Only forward "user.message" notifications to the legacy handler.
	// Other notifications (e.g. "table", "thinking_delta" sent by RespondWithType)
	// are client-bound and must NOT be routed through the legacy path.
	if notif.Method != "user.message" {
		return
	}

	// Forward to legacy handler if registered
	if s.handler != nil {
		msg := &Message{
			ID:          uuid.New().String(),
			ClientID:    c.id,
			Direction:   DirectionInbound,
			Data:        notif.Params,
			ContentType: "application/json",
			Timestamp:   time.Now(),
		}
		s.handler(msg)
	}
}

func (s *Server) handleResponse(c *client, raw []byte) {
	var resp Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		return
	}

	c.mu.RLock()
	ch, ok := c.pending[resp.ID]
	c.mu.RUnlock()

	if ok {
		select {
		case ch <- &resp:
		default:
		}
	}
}

// ---------------------------------------------------------------------------
// Write Pump
// ---------------------------------------------------------------------------

func (s *Server) writePump(c *client) {
	pingPeriod := 54 * time.Second
	if s.heartbeatCfg != nil {
		pingPeriod = s.heartbeatCfg.PingPeriod
	}
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	slog.Debug("writePump started", "client_id", c.id)
	defer slog.Debug("writePump exit", "client_id", c.id)

	for {
		select {
		case data, ok := <-c.send:
			if !ok {
				slog.Debug("writePump send channel closed", "client_id", c.id)
				c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				slog.Debug("writePump NextWriter error",
					"client_id", c.id,
					"error", err,
				)
				return
			}
			slog.Debug("writePump write",
				"client_id", c.id,
				"size", len(data),
			)
			w.Write(data)
			for i := 0; i < len(c.send); i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}
			w.Close()
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if s.hbMonitor != nil {
				s.hbMonitor.recordPing()
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				slog.Debug("writePump ping error",
					"client_id", c.id,
					"error", err,
				)
				return
			}
		case <-c.ctx.Done():
			slog.Debug("writePump context done", "client_id", c.id)
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Client Management
// ---------------------------------------------------------------------------

func (s *Server) removeClient(id string) {
	if s.disconnectHandler != nil {
		s.disconnectHandler(id)
	}
	s.mu.Lock()
	c, ok := s.clients[id]
	if ok {
		delete(s.clients, id)
	}
	// 连接数必须在锁内读取，理由同 hubLoop
	total := len(s.clients)
	s.mu.Unlock()

	if ok {
		c.closed.Do(func() {
			c.sendMu.Lock()
			c.sendClosed = true
			close(c.send)
			c.sendMu.Unlock()
		})
	}
	slog.Info("client disconnected", "id", id, "total", total)
}

func (s *Server) ClientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}

func (s *Server) GetClient(clientID string) *client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.clients[clientID]
}

// DisconnectClient 主动断开指定客户端的存活连接（吊销即断连场景：
// 凭证被吊销/设备被拉黑时，服务端强制关闭其在线连接）。
// 断连走正常关闭链路——readPump 读到连接关闭后退出，依次触发
// disconnectHandler、会话清理与客户端摘除。返回连接是否存在。
func (s *Server) DisconnectClient(clientID string) bool {
	c := s.GetClient(clientID)
	if c == nil {
		return false
	}
	_ = c.conn.Close()
	return true
}

// ---------------------------------------------------------------------------
// Send Methods
// ---------------------------------------------------------------------------

func (s *Server) sendToClient(c *client, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		slog.Error("failed to marshal message", "error", err)
		return
	}
	// 与 removeClient 的 close(c.send) 互斥：持读锁期间通道保证未关闭。
	c.sendMu.RLock()
	defer c.sendMu.RUnlock()
	if c.sendClosed {
		slog.Debug("send to closed client, dropping message",
			"client_id", c.id,
			"size", len(data),
		)
		return
	}
	select {
	case c.send <- data:
		slog.Debug("sendToClient queued",
			"client_id", c.id,
			"size", len(data),
			"buffer_usage", len(c.send),
			"buffer_cap", cap(c.send),
		)
	default:
		slog.Warn("send buffer full, dropping message",
			"client_id", c.id,
			"size", len(data),
			"buffer_usage", len(c.send),
			"buffer_cap", cap(c.send),
		)
	}
}

// Notify sends a notification to a specific client.
func (s *Server) Notify(clientID, method string, params any) error {
	c := s.GetClient(clientID)
	if c == nil {
		return fmt.Errorf("client %s not found", clientID)
	}

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("marshal params failed: %w", err)
	}
	notif := Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsBytes,
	}
	s.sendToClient(c, notif)
	return nil
}

// BroadcastNotification sends a notification to all connected clients.
func (s *Server) BroadcastNotification(method string, params any) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		slog.Error("failed to marshal broadcast params", "error", err)
		return
	}
	for _, c := range s.clients {
		s.sendToClient(c, Notification{
			JSONRPC: "2.0",
			Method:  method,
			Params:  paramsBytes,
		})
	}
}

// Call sends a request to a client and waits for response.
func (s *Server) Call(ctx context.Context, clientID, method string, params any) (json.RawMessage, error) {
	c := s.GetClient(clientID)
	if c == nil {
		return nil, fmt.Errorf("client %s not found", clientID)
	}

	id := uuid.New().String()
	ch := make(chan *Response, 1)

	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	paramsBytes, _ := json.Marshal(params)
	s.sendToClient(c, Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  paramsBytes,
	})

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("rpc error: %s", resp.Error.Message)
		}
		return resp.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// SendResponse sends a typed response envelope to a client (backward compatibility).
func (s *Server) SendResponse(clientID string, respType ResponseType, title string, data interface{}, opts ...ResponseOption) error {
	env := ResponseEnvelope{
		Type:  respType,
		Title: title,
		Data:  data,
	}
	for _, opt := range opts {
		opt(&env)
	}
	return s.Notify(clientID, string(respType), env)
}

// ResponseOption is a function that modifies a ResponseEnvelope.
type ResponseOption func(env *ResponseEnvelope)

// WithSessionID sets the session_id on a response envelope.
func WithSessionID(sessionID string) ResponseOption {
	return func(env *ResponseEnvelope) {
		env.SessionID = sessionID
	}
}

// WithResponseMeta adds metadata to a response envelope (backward compatibility).
func WithResponseMeta(meta map[string]interface{}) ResponseOption {
	return func(env *ResponseEnvelope) {
		env.Meta = meta
	}
}

// StopAllChannels stops all channel operations (backward compatibility).
func (s *Server) StopAllChannels(ctx context.Context) error {
	// No-op: JSON-RPC doesn't have channels. This method is preserved for backward compatibility.
	return nil
}

// ---------------------------------------------------------------------------
// Legacy Compatibility (for channel.GatewaySender interface)
// ---------------------------------------------------------------------------

func (s *Server) Send(to string, message string) {
	s.Notify(to, "message", map[string]string{"text": message})
}

func (s *Server) BroadcastMessage(message string) {
	s.BroadcastNotification("message", map[string]string{"text": message})
}

// Broadcast sends a text message to all clients (for channel.GatewaySender interface).
func (s *Server) Broadcast(message string) {
	s.BroadcastNotification("message", map[string]string{"text": message})
}

func (s *Server) SendJSON(to string, v interface{}) error {
	return s.Notify(to, "json", v)
}

func (s *Server) BroadcastJSON(v interface{}) error {
	s.BroadcastNotification("json", v)
	return nil
}

func (s *Server) SendFile(to string, filename string) error {
	params := map[string]interface{}{"filename": filename}
	return s.Notify(to, "send_file", params)
}

func (s *Server) BroadcastFile(filename string) error {
	return s.SendFile("*", filename)
}

func (s *Server) SendBatch(to string, msgs []*Message) {
	for _, msg := range msgs {
		s.Notify(to, "batch", msg)
	}
}

func (s *Server) BroadcastBatch(msgs []*Message) {
	s.SendBatch("*", msgs)
}

func (s *Server) RegisterChannel(ch channel.Channel) {
	if gwSender, ok := ch.(interface{ SetGatewaySender(channel.GatewaySender) }); ok {
		gwSender.SetGatewaySender(s)
	}
	s.channels.Store(ch.Name(), ch)
}

func (s *Server) GetChannel(name string) (channel.Channel, bool) {
	v, ok := s.channels.Load(name)
	if !ok {
		return nil, false
	}
	return v.(channel.Channel), ok
}

func (s *Server) Channels() map[string]channel.Channel {
	result := make(map[string]channel.Channel)
	s.channels.Range(func(k, v interface{}) bool {
		result[k.(string)] = v.(channel.Channel)
		return true
	})
	return result
}

func WithChannels(chs ...channel.Channel) Option {
	return func(s *Server) {
		for _, ch := range chs {
			s.RegisterChannel(ch)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func truncatePreview(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
