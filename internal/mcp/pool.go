package mcp

import (
	"context"
	"fmt"
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
	storage     Storage
	credStore   CredentialStore
	log         Logger

	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

type managedConn struct {
	client   MCPClient
	lastUsed time.Time
	ttl      time.Duration
	mu       sync.Mutex
}

// NewConnectionPool creates a new connection pool.
// Call StartReapLoop to begin the reap loop.
func NewConnectionPool(log Logger, storage Storage, credStore CredentialStore) *ConnectionPool {
	if log == nil {
		log = nopLogger{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &ConnectionPool{
		servers:     make(map[string]ServerConfig),
		connections: make(map[string]*managedConn),
		storage:     storage,
		credStore:   credStore,
		log:         log,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// LoadConfig loads server configurations from storage and rebuilds
// the server index. Does NOT establish connections.
func (p *ConnectionPool) LoadConfig(ctx context.Context) error {
	servers, err := LoadServers(p.storage)
	if err != nil {
		return fmt.Errorf("load server config: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.servers = make(map[string]ServerConfig, len(servers))
	for _, s := range servers {
		p.servers[s.Name] = s
	}
	return nil
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
		mc.client.Close()
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
		client.Close()
		p.log.Error("mcp: connection failed", err, "server", name)
		return fmt.Errorf("connect to %s: %w", name, err)
	}

	ttl := time.Duration(cfg.IdleTTLSecs) * time.Second
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	p.mu.Lock()
	// Close existing connection if any
	if mc, ok := p.connections[name]; ok {
		mc.client.Close()
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
	p.mu.RLock()
	mc, ok := p.connections[serverName]
	p.mu.RUnlock()

	if !ok || !mc.client.IsAlive() {
		if !ok {
			p.log.Debug("mcp: no connection, auto-connecting", "server", serverName)
		} else {
			p.log.Debug("mcp: connection dead, reconnecting", "server", serverName)
		}
		if err := p.Connect(ctx, serverName); err != nil {
			p.log.Error("mcp: auto-connect for call failed", err, "server", serverName)
			return "", fmt.Errorf("connect for call: %w", err)
		}
		p.mu.RLock()
		mc = p.connections[serverName]
		p.mu.RUnlock()
	}

	mc.touch()

	p.log.Debug("mcp: calling tool", "server", serverName, "tool", toolName)
	result, err := ToolsCall(ctx, mc.client, toolName, args)
	if err != nil {
		p.log.Error("mcp: tool call failed", err, "server", serverName, "tool", toolName)
		return "", err
	}
	p.log.Debug("mcp: tool call succeeded", "server", serverName, "tool", toolName)
	return result, nil
}

// Disconnect closes a specific server connection.
func (p *ConnectionPool) Disconnect(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if mc, ok := p.connections[name]; ok {
		mc.client.Close()
		delete(p.connections, name)
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
		mc.client.Close()
		delete(p.connections, name)
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
		mc.mu.Lock()
		idle := time.Since(mc.lastUsed)
		mc.mu.Unlock()
		if idle > mc.ttl {
			mc.client.Close()
			delete(p.connections, name)
			p.log.Info("mcp: connection reaped (idle timeout)", "server", name, "idle", idle.Round(time.Second))
		}
	}
}

func (m *managedConn) touch() {
	m.mu.Lock()
	m.lastUsed = time.Now()
	m.mu.Unlock()
}

func (p *ConnectionPool) resolveCreds(cfg ServerConfig) (map[string]string, error) {
	result := make(map[string]string)
	if cfg.CredentialRef != "" && p.credStore != nil {
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

	p.mu.RLock()
	mc := p.connections[serverName]
	p.mu.RUnlock()

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
