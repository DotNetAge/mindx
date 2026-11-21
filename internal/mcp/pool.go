package mcp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ── Credential Resolver ─────────────────────────────────────────────────────

// CredentialStore resolves and stores credential references.
// Matches core.CredentialStore interface so Daemon can pass it directly.
type CredentialStore interface {
	Get(key string) (string, error)
	Set(key, value string) error
}

// ── ConnectionPool ──────────────────────────────────────────────────────────

// ConnectionPool manages lifecycle of MCP server connections.
// Implements lazy connection, idle TTL reaping, and auto-reconnect.
type ConnectionPool struct {
	servers     map[string]ServerConfig
	connections map[string]*managedConn
	credStore   CredentialStore
	log         Logger

	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

// managedConn 是一条托管连接。
//
// mu 同时保护 lastUsed/refs/closed：Call 借用连接期间（refs>0）不得关闭底层
// client，否则会对着已关闭的连接发工具调用（结果不确定，可能 panic）。
// 以前 Call 只持有指针、读锁在取出后立即释放，Disconnect/reap/CloseAll
// 可以在窗口内把它关掉——典型的 TOCTOU。
type managedConn struct {
	client   MCPClient
	lastUsed time.Time
	ttl      time.Duration
	mu       sync.Mutex
	refs     int  // 借用中计数
	closed   bool // 已标记关闭（不再允许借用）
}

// acquire 借用连接：未关闭则 refs++ 并刷新空闲时间。
func (mc *managedConn) acquire() bool {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if mc.closed || mc.client == nil {
		return false
	}
	mc.refs++
	mc.lastUsed = time.Now()
	return true
}

// release 归还连接。若期间已被标记关闭且再无人使用，则此刻真正关闭底层连接。
func (mc *managedConn) release() {
	mc.mu.Lock()
	if mc.refs > 0 {
		mc.refs--
	}
	mc.lastUsed = time.Now()
	closeNow := mc.closed && mc.refs == 0 && mc.client != nil
	mc.mu.Unlock()
	if closeNow {
		_ = mc.client.Close()
	}
}

// markClosed 标记连接不再可用；无借用者时立即关闭，有借用者则在最后一次
// release 时关闭（不会出现「借用中连接被抽掉」）。
func (mc *managedConn) markClosed() {
	mc.mu.Lock()
	mc.closed = true
	closeNow := mc.refs == 0 && mc.client != nil
	mc.mu.Unlock()
	if closeNow {
		_ = mc.client.Close()
	}
}

// touch 只刷新空闲时间（不建立借用关系）。用于「确认可复用」这类不进入
// 阻塞调用的路径；真正要发请求的调用方必须用 acquire/release。
func (mc *managedConn) touch() {
	mc.mu.Lock()
	mc.lastUsed = time.Now()
	mc.mu.Unlock()
}

// idleFor 返回空闲时长（仅在无借用者时有效）。
func (mc *managedConn) idleFor() (time.Duration, bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if mc.closed || mc.refs > 0 {
		return 0, false
	}
	return time.Since(mc.lastUsed), true
}

// NewConnectionPool creates a new connection pool.
// Call StartReapLoop to begin the reap loop.
func NewConnectionPool(log Logger, credStore CredentialStore) *ConnectionPool {
	if log == nil {
		log = nopLogger{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &ConnectionPool{
		servers:     make(map[string]ServerConfig),
		connections: make(map[string]*managedConn),
		credStore:   credStore,
		log:         log,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// LoadConfig 从 mcp.json 读取 server 配置，重建内存索引。
// 不建立实际连接。enabled=false 的 server 跳过（不进索引、不参与连接）。
func (p *ConnectionPool) LoadConfig(ctx context.Context, configPath string) error {
	servers, err := LoadServers(configPath)
	if err != nil {
		return fmt.Errorf("读取 mcp.json 失败: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.servers = make(map[string]ServerConfig, len(servers))
	for _, s := range servers {
		if !s.Enabled {
			continue
		}
		p.servers[s.Name] = s
	}
	return nil
}

// ActiveServerNames 返回当前已加载（enabled=true）的 server 名列表。
// 顺序不确定——调用方不依赖特定顺序时可直接使用。
func (p *ConnectionPool) ActiveServerNames() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	names := make([]string, 0, len(p.servers))
	for name := range p.servers {
		names = append(names, name)
	}
	return names
}

// AddServer adds a server config to the in-memory index.
// Does NOT persist to storage.
func (p *ConnectionPool) AddServer(cfg ServerConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.servers[cfg.Name] = cfg
}

// RemoveServer disconnects and removes a server from the index.
func (p *ConnectionPool) RemoveServer(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if mc, ok := p.connections[name]; ok {
		_ = mc.client.Close()
		delete(p.connections, name)
		p.log.Debug("mcp: pool disconnected removed server", "server", name)
	}
	delete(p.servers, name)
}

// Connect establishes a connection to the server (idempotent).
func (p *ConnectionPool) Connect(ctx context.Context, name string) error {
	p.mu.Lock()
	cfg, ok := p.servers[name]
	if !ok {
		p.mu.Unlock()
		return fmt.Errorf("server %q not configured", name)
	}

	// Return existing connection if alive
	if mc, ok := p.connections[name]; ok && mc.client.IsAlive() {
		mc.touch()
		p.mu.Unlock()
		return nil
	}
	p.mu.Unlock()

	p.log.Info("mcp: connecting to server", "server", name, "type", string(cfg.Type))

	// Build new client
	creds, err := p.resolveCreds(cfg)
	if err != nil {
		p.log.Error("mcp: failed to resolve credentials", err, "server", name)
		return fmt.Errorf("resolve credentials for %s: %w", name, err)
	}

	client, err := NewClient(cfg, creds)
	if err != nil {
		p.log.Error("mcp: failed to create client", err, "server", name)
		return fmt.Errorf("create client for %s: %w", name, err)
	}

	if err := client.Connect(ctx); err != nil {
		_ = client.Close()
		p.log.Error("mcp: connection failed", err, "server", name)
		return fmt.Errorf("connect to %s: %w", name, err)
	}

	ttl := time.Duration(cfg.IdleTTLSecs) * time.Second
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	p.mu.Lock()
	// Close existing connection if any（走 markClosed：借用中的连接等它归还再关）
	if mc, ok := p.connections[name]; ok {
		mc.markClosed()
	}
	p.connections[name] = &managedConn{
		client:   client,
		lastUsed: time.Now(),
		ttl:      ttl,
	}
	p.mu.Unlock()

	p.log.Info("mcp: connected", "server", name, "ttl", ttl)
	return nil
}

// Call sends a tool invocation to an MCP server.
// Auto-connects and auto-reconnects if needed.
func (p *ConnectionPool) Call(ctx context.Context, serverName, toolName string, args map[string]any) (string, error) {
	mc, err := p.acquireConn(ctx, serverName)
	if err != nil {
		return "", err
	}
	defer mc.release()

	p.log.Debug("mcp: calling tool", "server", serverName, "tool", toolName)
	result, callErr := ToolsCall(ctx, mc.client, toolName, args)
	if callErr != nil {
		p.log.Error("mcp: tool call failed", callErr, "server", serverName, "tool", toolName)
		return "", callErr
	}
	p.log.Debug("mcp: tool call succeeded", "server", serverName, "tool", toolName)
	return result, nil
}

// acquireConn 取出可用连接并借用（refs++）。不持有 p.mu 进入工具调用，
// 但借用期间该连接不会被 Disconnect / reap / CloseAll 关闭。
func (p *ConnectionPool) acquireConn(ctx context.Context, serverName string) (*managedConn, error) {
	p.mu.RLock()
	mc := p.connections[serverName]
	p.mu.RUnlock()

	if mc != nil && mc.client.IsAlive() && mc.acquire() {
		return mc, nil
	}

	if mc == nil {
		p.log.Debug("mcp: no connection, auto-connecting", "server", serverName)
	} else {
		p.log.Debug("mcp: connection dead, reconnecting", "server", serverName)
	}
	if err := p.Connect(ctx, serverName); err != nil {
		p.log.Error("mcp: auto-connect for call failed", err, "server", serverName)
		return nil, fmt.Errorf("connect for call: %w", err)
	}

	p.mu.RLock()
	mc = p.connections[serverName]
	p.mu.RUnlock()
	if mc == nil || !mc.acquire() {
		return nil, fmt.Errorf("连接未就绪")
	}
	return mc, nil
}

// Disconnect closes a specific server connection.
func (p *ConnectionPool) Disconnect(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if mc, ok := p.connections[name]; ok {
		// 先移出索引（不再接受新借用），再标记关闭（借用中则等归还）
		delete(p.connections, name)
		mc.markClosed()
		p.log.Info("mcp: disconnected", "server", name)
	}
}

// CloseAll shuts down all connections and stops the reap loop.
func (p *ConnectionPool) CloseAll() {
	p.log.Info("mcp: closing all connections")
	p.cancel()
	p.mu.Lock()
	defer p.mu.Unlock()
	for name, mc := range p.connections {
		delete(p.connections, name)
		mc.markClosed()
	}
	p.log.Info("mcp: all connections closed")
}

// StartReapLoop begins periodic idle connection cleanup.
// interval is how often to scan for idle connections.
func (p *ConnectionPool) StartReapLoop(interval time.Duration) {
	go p.reapLoop(interval)
}

func (p *ConnectionPool) reapLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.reap()
		}
	}
}

func (p *ConnectionPool) reap() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for name, mc := range p.connections {
		idle, reclaimable := mc.idleFor()
		if !reclaimable || idle <= mc.ttl {
			continue // 借用中或未超时：本轮跳过
		}
		delete(p.connections, name)
		mc.markClosed()
		p.log.Info("mcp: connection reaped (idle timeout)", "server", name, "idle", idle.Round(time.Second))
	}
}

// needsCredentialValue 判断某个 env/header 声明是否「值还没填」。
//
// 不能只看空串：连接器配置里为了让人看懂，占位值写成 `<YOUR_AMAP_KEY>`、
// `your_token_here` 这类形式。它们不是真值，却因为非空被当成「已配置」，
// 导致用户填进凭据库的密钥永远解析不出来——这是「填了 Key 还是连不上」的根因。
func needsCredentialValue(v string) bool {
	t := strings.TrimSpace(v)
	if t == "" {
		return true
	}
	// 占位值可能是整值（`<YOUR_AK>`），也可能嵌在前缀里（`Bearer <TOKEN>`）
	if i := strings.Index(t, "<"); i >= 0 && strings.Index(t[i:], ">") > 0 {
		return true
	}
	lower := strings.ToLower(t)
	if strings.HasPrefix(lower, "your_") || strings.HasPrefix(lower, "your-") {
		return true
	}
	if strings.HasSuffix(lower, "_here") {
		return true
	}
	return strings.Contains(lower, "xxx")
}

// fillPlaceholder 把密钥填进声明模板，保留模板里除占位符以外的部分。
//
// 为什么不能整值覆盖：HTTP 连接器的 header 常写成 `Bearer <TOKEN>`，
// 直接换成用户粘贴的纯 token 会少掉 Bearer 前缀，鉴权必然失败。
func fillPlaceholder(template, secret string) string {
	if start := strings.Index(template, "<"); start >= 0 {
		if end := strings.Index(template[start:], ">"); end > 0 {
			return template[:start] + secret + template[start+end+1:]
		}
	}
	return secret
}

func (p *ConnectionPool) resolveCreds(cfg ServerConfig) (map[string]string, error) {
	result := make(map[string]string)
	if p.credStore == nil {
		return result, nil
	}
	// 候选键的优先级：账号组 → 连接器自身。
	// 组级（`group:<组>:<env名>`）是常态：阿里云一套 AK 顶着 15 个产品，
	// 存一份、轮换一次；连接器级（`<mcp_name>:<env_name>`）留给个别要独立密钥的场景。
	candidates := func(key string) []string {
		keys := make([]string, 0, 2)
		if cfg.CredentialGroup != "" {
			keys = append(keys, "group:"+cfg.CredentialGroup+":"+key)
		}
		keys = append(keys, cfg.Name+":"+key)
		return keys
	}
	lookup := func(key string) (string, bool) {
		for _, k := range candidates(key) {
			if v, err := p.credStore.Get(k); err == nil && v != "" {
				return v, true
			}
		}
		return "", false
	}
	// env 与 headers 两处声明都可能要密钥（HTTP 类连接器走 Authorization）
	for key, val := range cfg.Env {
		if !needsCredentialValue(val) {
			continue
		}
		if v, ok := lookup(key); ok {
			result[key] = fillPlaceholder(val, v)
		}
	}
	for key, val := range cfg.Headers {
		if !needsCredentialValue(val) {
			continue
		}
		if v, ok := lookup(key); ok {
			result[key] = fillPlaceholder(val, v)
		}
	}
	// 既有 credential_ref 机制保留：手写 mcp.json 的单凭据引用（ref 既是
	// 存储键又是注入名——env 变量名或 SSE/HTTP header 名）。
	if cfg.CredentialRef != "" {
		val, err := p.credStore.Get(cfg.CredentialRef)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", cfg.CredentialRef, err)
		}
		if val != "" {
			result[cfg.CredentialRef] = val
		}
	}
	return result, nil
}

// DiscoverTools connects to a server, retrieves the tool list, then disconnects.
func (p *ConnectionPool) DiscoverTools(ctx context.Context, serverName string) ([]toolDef, error) {
	p.log.Info("mcp: discovering tools", "server", serverName)
	if err := p.Connect(ctx, serverName); err != nil {
		return nil, err
	}

	mc, err := p.acquireConn(ctx, serverName)
	if err != nil {
		return nil, err
	}
	defer mc.release()

	tools, err := ToolsList(ctx, mc.client)
	if err != nil {
		p.log.Error("mcp: tools/list failed", err, "server", serverName)
		return nil, err
	}

	p.log.Info("mcp: tools discovered", "server", serverName, "count", len(tools))
	return tools, nil
}

// TestConnection attempts to connect to a server, then immediately disconnects.
func (p *ConnectionPool) TestConnection(ctx context.Context, serverName string) error {
	p.log.Info("mcp: testing connection", "server", serverName)
	if err := p.Connect(ctx, serverName); err != nil {
		return err
	}
	p.Disconnect(serverName)
	p.log.Info("mcp: connection test passed", "server", serverName)
	return nil
}
