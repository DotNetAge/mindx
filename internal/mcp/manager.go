package mcp

import (
	"context"
	"sync"
	"time"

	"github.com/DotNetAge/goharness/tools"
)

// Manager is the top-level orchestrator for MCP integration.
// It owns the connection pool, storage, and credential store.
// The Daemon holds a single Manager instance.
type Manager struct {
	pool      *ConnectionPool
	storage   Storage
	credStore CredentialStore
	rpc       *RPCHandler
	log       Logger

	mu sync.RWMutex
}

// NewManager creates a new MCP Manager with the given storage and credential store.
// It immediately loads server config from storage and starts the reap loop.
func NewManager(log Logger, storage Storage, credStore CredentialStore) *Manager {
	if log == nil {
		log = nopLogger{}
	}
	m := &Manager{
		storage:   storage,
		credStore: credStore,
		log:       log,
	}

	m.pool = NewConnectionPool(log, storage, credStore)
	m.rpc = NewRPCHandler(m)

	// Load existing server config
	servers, err := LoadServers(m.storage)
	if err != nil {
		m.log.Error("mcp: failed to load server config", err)
	} else if len(servers) > 0 {
		m.log.Info("mcp: loaded server config", "count", len(servers))
		for _, s := range servers {
			m.log.Debug("mcp: server config", "name", s.Name, "type", string(s.Type))
		}
	}
	_ = m.pool.LoadConfig(context.Background())

	// Start idle connection reaper (every 30 seconds)
	m.pool.StartReapLoop(30 * time.Second)
	m.log.Info("mcp: manager initialized", "reap_interval", "30s")

	return m
}

// RPCHandler returns the JSON-RPC handler for WebUI configuration.
func (m *Manager) RPCHandler() *RPCHandler {
	return m.rpc
}

// EnabledTools returns all enabled MCP tools as goharness FuncTool instances.
// Called by Runtime during createRuntime() to register MCP tools.
func (m *Manager) EnabledTools() []tools.FuncTool {
	manifest, err := LoadManifest(m.storage)
	if err != nil || manifest == nil {
		if err != nil {
			m.log.Error("mcp: failed to load manifest for EnabledTools", err)
		}
		return nil
	}

	tools := BuildTools(manifest, m.pool)
	m.log.Debug("mcp: providing enabled tools", "count", len(tools))
	return tools
}

// AddServer persists a server config and adds it to the pool index.
func (m *Manager) AddServer(ctx context.Context, cfg ServerConfig) error {
	servers, err := LoadServers(m.storage)
	if err != nil {
		return err
	}

	// Upsert: replace if exists
	found := false
	for i, s := range servers {
		if s.Name == cfg.Name {
			servers[i] = cfg
			found = true
			break
		}
	}
	if !found {
		servers = append(servers, cfg)
	}

	if err := SaveServers(m.storage, servers); err != nil {
		m.log.Error("mcp: failed to save server config", err, "server", cfg.Name)
		return err
	}

	m.pool.AddServer(cfg)
	m.log.Info("mcp: server added", "name", cfg.Name, "type", string(cfg.Type))
	return nil
}

// RemoveServer removes a server from storage, pool, and cleans up manifest.
func (m *Manager) RemoveServer(name string) error {
	// Remove from server config
	servers, err := LoadServers(m.storage)
	if err != nil {
		return err
	}
	filtered := make([]ServerConfig, 0, len(servers))
	for _, s := range servers {
		if s.Name != name {
			filtered = append(filtered, s)
		}
	}
	if err := SaveServers(m.storage, filtered); err != nil {
		m.log.Error("mcp: failed to save server config after remove", err, "server", name)
		return err
	}

	// Clean up manifest entries for this server
	manifest, err := LoadManifest(m.storage)
	if err == nil && manifest != nil {
		tools := make([]ToolManifestEntry, 0, len(manifest.Tools))
		for _, t := range manifest.Tools {
			if t.Server != name {
				tools = append(tools, t)
			}
		}
		manifest.Tools = tools
		_ = SaveManifest(m.storage, manifest)
	}

	m.pool.RemoveServer(name)
	m.log.Info("mcp: server removed", "name", name)
	return nil
}

// Shutdown gracefully closes all connections and stops the reap loop.
func (m *Manager) Shutdown() {
	m.log.Info("mcp: shutting down")
	m.pool.CloseAll()
	m.log.Info("mcp: shutdown complete")
}

// ── noop logger fallback ────────────────────────────────────────────────────

type nopLogger struct{}

func (nopLogger) Info(string, ...any)         {}
func (nopLogger) Error(string, error, ...any) {}
func (nopLogger) Debug(string, ...any)        {}
func (nopLogger) Warn(string, ...any)         {}

var _ Logger = nopLogger{}
