package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// generateSessionID generates a unique session identifier.
// Format: sess_<nanosecond_timestamp_mod_100000000>
func generateSessionID() string {
	return fmt.Sprintf("sess_%d", time.Now().UnixNano()%100000000)
}

// SessionMeta represents session-level metadata persisted to <session_dir>/meta.json.
// This struct is defined and used by MindX (application layer).
// GoReact (framework layer) does not depend on this type.
type SessionMeta struct {
	SessionID string    `json:"session_id"`
	AgentName string    `json:"agent_name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Directory bindings
	HomeDir           string `json:"home_dir"`            // Layer 1: ~/.mindx
	ProjectWorkingDir string `json:"project_working_dir"` // Layer 2: captured via os.Getwd()

	// Runtime stats
	MessageCount   int       `json:"message_count"`
	LastActivityAt time.Time `json:"last_activity_at"`

	// Compaction state
	Cursor int `json:"cursor"` // Position of compaction cursor (0 = no compaction)
}

// NewSessionMeta creates a new session metadata instance with the captured project directory.
func NewSessionMeta(sessionID, agentName, projectDir string) (*SessionMeta, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	now := time.Now()
	return &SessionMeta{
		SessionID:         sessionID,
		AgentName:         agentName,
		CreatedAt:         now,
		UpdatedAt:         now,
		HomeDir:           homeDir,
		ProjectWorkingDir: projectDir,
	}, nil
}

// Save persists the metadata to meta.json in the given session directory.
func (m *SessionMeta) Save(sessionDirPath string) error {
	m.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session meta: %w", err)
	}

	metaPath := filepath.Join(sessionDirPath, "meta.json")
	if err := os.MkdirAll(sessionDirPath, 0755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	return os.WriteFile(metaPath, data, 0600)
}

// LoadSessionMeta loads metadata from meta.json in the given session directory.
func LoadSessionMeta(sessionDirPath string) (*SessionMeta, error) {
	metaPath := filepath.Join(sessionDirPath, "meta.json")

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("read session meta: %w", err)
	}

	var meta SessionMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("unmarshal session meta: %w", err)
	}

	return &meta, nil
}
