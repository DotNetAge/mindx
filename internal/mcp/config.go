package mcp

import (
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

// ── Logger ──────────────────────────────────────────────────────────────────

// Logger defines the logging interface for the MCP package.
// Matches goharness/logging.Logger so the Daemon's logger can be passed directly.
type Logger interface {
	Info(msg string, keyvals ...any)
	Error(msg string, err error, keyvals ...any)
	Debug(msg string, keyvals ...any)
	Warn(msg string, keyvals ...any)
}

// ── Storage keys ────────────────────────────────────────────────────────────

const (
	keyServers  = "mcp:servers"
	keyManifest = "mcp:manifest"
)

// ── Storage backend ─────────────────────────────────────────────────────────
// Storage abstracts the underlying key-value store.
// bbolt.DB is the production implementation.

type Storage interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte) error
	Delete(key string) error
}

// ── Server Config ───────────────────────────────────────────────────────────

// ServerType represents the transport type of an MCP server.
type ServerType string

const (
	ServerTypeStdio ServerType = "stdio"
	ServerTypeSSE   ServerType = "sse"
	ServerTypeHTTP  ServerType = "http"
)

// ServerConfig represents the configuration for a single MCP server.
type ServerConfig struct {
	Name          string            `json:"name"`
	Type          ServerType        `json:"type"`
	Command       string            `json:"command,omitempty"`
	Args          []string          `json:"args,omitempty"`
	URL           string            `json:"url,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	CredentialRef string            `json:"credential_ref,omitempty"`
	IdleTTLSecs   int               `json:"idle_ttl_secs"`
}

// Validate checks that required fields are present based on server type.
func (c *ServerConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("server name is required")
	}
	if c.IdleTTLSecs <= 0 {
		c.IdleTTLSecs = 300
	}
	switch c.Type {
	case ServerTypeStdio:
		if c.Command == "" {
			return fmt.Errorf("command is required for stdio server")
		}
	case ServerTypeSSE:
		fallthrough
	case ServerTypeHTTP:
		if c.URL == "" {
			return fmt.Errorf("url is required for %s server", c.Type)
		}
	default:
		return fmt.Errorf("unknown server type: %s", c.Type)
	}
	return nil
}

// ── Tool Manifest ───────────────────────────────────────────────────────────

// ToolManifestEntry represents a single enabled MCP tool in the manifest.
type ToolManifestEntry struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Server      string         `json:"server"`
	MCPName     string         `json:"mcp_name"`
	InputSchema map[string]any `json:"input_schema"`
}

// ToolManifest holds the full set of enabled MCP tools.
type ToolManifest struct {
	Version   int                 `json:"version"`
	UpdatedAt string              `json:"updated_at"`
	Tools     []ToolManifestEntry `json:"tools"`
}

// ── File system helpers ─────────────────────────────────────────────────────

// LoadServers reads the server config list from storage.
func LoadServers(store Storage) ([]ServerConfig, error) {
	data, err := store.Get(keyServers)
	if err != nil {
		return nil, fmt.Errorf("failed to read server config: %w", err)
	}
	if data == nil {
		return nil, nil
	}
	var servers []ServerConfig
	if err := json.Unmarshal(data, &servers); err != nil {
		return nil, fmt.Errorf("failed to parse server config: %w", err)
	}
	return servers, nil
}

// SaveServers writes the server config list to storage.
func SaveServers(store Storage, servers []ServerConfig) error {
	data, err := json.Marshal(servers)
	if err != nil {
		return fmt.Errorf("failed to marshal server config: %w", err)
	}
	return store.Set(keyServers, data)
}

// LoadManifest reads the tool manifest from storage.
func LoadManifest(store Storage) (*ToolManifest, error) {
	data, err := store.Get(keyManifest)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}
	if data == nil {
		return &ToolManifest{Version: 1, Tools: []ToolManifestEntry{}}, nil
	}
	var m ToolManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}
	if m.Tools == nil {
		m.Tools = []ToolManifestEntry{}
	}
	return &m, nil
}

// SaveManifest writes the tool manifest to storage.
func SaveManifest(store Storage, m *ToolManifest) error {
	m.Version++
	m.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}
	return store.Set(keyManifest, data)
}

// ── bbolt adapter ───────────────────────────────────────────────────────────

// bboltStore adapts *bbolt.DB to the Storage interface.
type bboltStore struct {
	db *bolt.DB
}

func (s *bboltStore) Get(key string) ([]byte, error) {
	var val []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("mcp"))
		if b == nil {
			return nil
		}
		v := b.Get([]byte(key))
		if v != nil {
			val = make([]byte, len(v))
			copy(val, v)
		}
		return nil
	})
	return val, err
}

func (s *bboltStore) Set(key string, value []byte) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("mcp"))
		if err != nil {
			return err
		}
		return b.Put([]byte(key), value)
	})
}

func (s *bboltStore) Delete(key string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("mcp"))
		if b == nil {
			return nil
		}
		return b.Delete([]byte(key))
	})
}

// NewStorage creates a Storage backed by bbolt.DB.
func NewStorage(db *bolt.DB) Storage {
	return &bboltStore{db: db}
}
